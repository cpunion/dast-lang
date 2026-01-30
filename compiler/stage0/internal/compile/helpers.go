package compile

import (
	"fmt"
	"os"
	"strings"

	"dastlang/internal/ast"
	"dastlang/internal/ir"
	"dastlang/internal/source"
)

type scope struct {
	vars  map[string]VarInfo
	order []string
}

// varState captures per-variable state that must merge across branches.
type varState struct {
	Moved    bool
	Borrowed bool
}

func enumHasVariant(decl *ast.EnumDecl, name string) bool {
	for _, v := range decl.Variants {
		if v.Name == name {
			return true
		}
	}
	return false
}

func enumVariant(decl *ast.EnumDecl, name string) *ast.VariantDef {
	for i := range decl.Variants {
		if decl.Variants[i].Name == name {
			return &decl.Variants[i]
		}
	}
	return nil
}

func findField(decl *ast.StructDecl, name string) *ast.FieldDef {
	for i := range decl.Fields {
		if decl.Fields[i].Name == name {
			return &decl.Fields[i]
		}
	}
	return nil
}

func constValueToIr(v ast.ConstValue, typeName string) ir.Value {
	switch v.Kind {
	case ast.ConstInt:
		intType := ""
		if isIntTypeName(typeName) {
			intType = typeName
		}
		return ir.Value{Kind: ir.KindInt, Int: v.Int, IntType: intType}
	case ast.ConstChar:
		return ir.Value{Kind: ir.KindInt, Int: v.Int, IntType: "char"}
	case ast.ConstBool:
		return ir.Value{Kind: ir.KindBool, Bool: v.Bool}
	case ast.ConstString:
		return ir.Value{Kind: ir.KindString, Str: v.Str}
	case ast.ConstFloat:
		return ir.Value{Kind: ir.KindFloat, FloatText: v.FloatText}
	default:
		return ir.Value{Kind: ir.KindUnit}
	}
}

func (c *Compiler) constZero() int {
	t := c.newTemp()
	c.setTempType(t, "i64")
	c.emit(&ir.BinOp{Dst: t, Op: "+", Lhs: ir.IntOperand(0), Rhs: ir.IntOperand(0)})
	return t
}

func (c *Compiler) setTempBorrowed(temp int, borrowed bool) {
	if temp < 0 {
		return
	}
	if c.tempBorrowed == nil {
		c.tempBorrowed = map[int]bool{}
	}
	c.tempBorrowed[temp] = borrowed
}

// markTempBorrowedVar propagates a borrow on a temp back to any variable that
// currently owns that temp, preventing premature drops of the base value.
func (c *Compiler) markTempBorrowedVar(temp int) {
	if temp < 0 {
		return
	}
	for si := len(c.scopeStack) - 1; si >= 0; si-- {
		sc := c.scopeStack[si]
		for name, info := range sc.vars {
			if info.Temp == temp && !info.Borrowed {
				info.Borrowed = true
				sc.vars[name] = info
			}
		}
		c.scopeStack[si] = sc
	}
}

func (c *Compiler) isTempBorrowed(temp int) bool {
	if temp < 0 || c.tempBorrowed == nil {
		return false
	}
	return c.tempBorrowed[temp]
}

func (c *Compiler) operandBorrowed(op ir.Operand) bool {
	if op.IsConst {
		// String literals are backed by static data and must not be freed.
		return op.Const.Kind == ir.KindString
	}
	return c.isTempBorrowed(op.Temp)
}

func (c *Compiler) callResultBorrowed(callee string) bool {
	switch callee {
	// Lexer/token accessors return borrowed views into lexer storage.
	case "token_at", "token_kind_at", "token_start_at", "token_len_at":
		return true
	case "lexer_peek", "lexer_peek_n", "lexer_next":
		return true
	case "peek_kind", "peek_kind_n", "tok_kind":
		return true
	default:
		return false
	}
}

func (c *Compiler) snapshotScopes() []scope {
	out := make([]scope, len(c.scopeStack))
	for i, sc := range c.scopeStack {
		vars := make(map[string]VarInfo, len(sc.vars))
		for name, info := range sc.vars {
			vars[name] = info
		}
		order := append([]string(nil), sc.order...)
		out[i] = scope{vars: vars, order: order}
	}
	return out
}

func (c *Compiler) restoreScopes(snapshot []scope) {
	c.scopeStack = snapshot
}

func (c *Compiler) captureVarState(base []scope) map[string]varState {
	out := map[string]varState{}
	if len(base) != len(c.scopeStack) {
		return out
	}
	for i := 0; i < len(base); i++ {
		for name := range base[i].vars {
			if cur, ok := c.scopeStack[i].vars[name]; ok {
				out[fmt.Sprintf("%d:%s", i, name)] = varState{
					Moved:    cur.Moved,
					Borrowed: cur.Borrowed,
				}
			}
		}
	}
	return out
}

func (c *Compiler) applyVarState(base []scope, state map[string]varState) {
	if len(base) != len(c.scopeStack) {
		return
	}
	for i := 0; i < len(base); i++ {
		for name, info := range c.scopeStack[i].vars {
			key := fmt.Sprintf("%d:%s", i, name)
			if val, ok := state[key]; ok {
				info.Moved = val.Moved
				info.Borrowed = val.Borrowed
				c.scopeStack[i].vars[name] = info
			}
		}
	}
}

func termIsReturn(term ir.Term) bool {
	if term == nil {
		return false
	}
	_, ok := term.(*ir.Return)
	return ok
}

func formatType(t ast.Type) string {
	base := ""
	if t.IsTuple {
		if len(t.TupleElems) == 0 {
			base = "unit"
		} else {
			elemTypes := make([]string, 0, len(t.TupleElems))
			for _, e := range t.TupleElems {
				elemTypes = append(elemTypes, formatType(e))
			}
			base = tupleTypeName(elemTypes)
		}
	} else if t.IsArray {
		if t.Elem != nil {
			base = "[" + formatType(*t.Elem) + "]"
		} else {
			base = "[]"
		}
	} else {
		base = t.Name
	}
	if t.IsRef {
		// Treat &str (and &String) as "str" for ABI compatibility.
		if !t.IsMut && (base == "String" || base == "str") {
			return "str"
		}
		// IR uses *T for other references (both & and &mut)
		return "*" + base
	}
	return base
}

func tupleTypeName(elemTypes []string) string {
	var sb strings.Builder
	sb.WriteString("__tuple")
	sb.WriteString(fmt.Sprintf("%d", len(elemTypes)))
	for _, t := range elemTypes {
		sb.WriteByte('_')
		sb.WriteString(mangleTypeName(t))
	}
	return sb.String()
}

func isIntTypeName(name string) bool {
	switch strings.TrimSpace(name) {
	case "int", "i8", "i16", "i32", "i64", "i128",
		"u8", "u16", "u32", "u64", "u128",
		"isize", "usize":
		return true
	default:
		return false
	}
}

func isCharTypeName(name string) bool {
	return strings.TrimSpace(name) == "char"
}

func isFloatTypeName(name string) bool {
	n := strings.TrimSpace(name)
	return n == "f32" || n == "f64"
}

func isUntypedIntTypeName(name string) bool {
	return strings.TrimSpace(name) == "untyped-int"
}

func isUntypedFloatTypeName(name string) bool {
	return strings.TrimSpace(name) == "untyped-float"
}

func defaultUntypedTypeName(name string) string {
	switch strings.TrimSpace(name) {
	case "untyped-int":
		return "i64"
	case "untyped-float":
		return "f64"
	default:
		return name
	}
}

func (c *Compiler) computeEnumTags() {
	debugEnum := os.Getenv("DAST_ENUM_DEBUG") != ""
	for name, decl := range c.enums {
		tagType := decl.Repr
		if tagType == "" {
			tagType = "i32"
		}
		c.enumTagType[name] = tagType
		tags := map[string]int64{}
		next := int64(0)
		for _, variant := range decl.Variants {
			if variant.HasValue {
				next = variant.Value
			}
			tags[variant.Name] = next
			next++
		}
		c.enumTags[name] = tags
		if debugEnum && name == "Stmt" {
			fmt.Fprintf(os.Stderr, "stage0: enum Stmt tags (%s):", tagType)
			for _, variant := range decl.Variants {
				fmt.Fprintf(os.Stderr, " %s=%d", variant.Name, tags[variant.Name])
			}
			fmt.Fprintln(os.Stderr)
		}
	}
}

func (c *Compiler) enumDeclToIR(decl *ast.EnumDecl) *ir.EnumDecl {
	tagType := decl.Repr
	if tagType == "" {
		tagType = "i32"
	}
	variants := make([]ir.EnumVariant, 0, len(decl.Variants))
	for _, v := range decl.Variants {
		tag, _, ok := c.enumTagInfo(decl.Name, v.Name)
		if !ok {
			tag = 0
		}
		payloadType := "unit"
		if v.Payload != nil {
			payloadType = formatType(*v.Payload)
		}
		variants = append(variants, ir.EnumVariant{Name: v.Name, Tag: tag, PayloadType: payloadType})
	}
	return &ir.EnumDecl{Name: decl.Name, TypeParams: nil, TagType: tagType, Variants: variants}
}

func (c *Compiler) enumTagInfo(enumName string, variant string) (int64, string, bool) {
	tags, ok := c.enumTags[enumName]
	if !ok {
		return 0, "", false
	}
	tag, ok := tags[variant]
	if !ok {
		return 0, "", false
	}
	tagType := c.enumTagType[enumName]
	if tagType == "" {
		tagType = "i32"
	}
	return tag, tagType, true
}

func (c *Compiler) resolveEnumName(variant string) (string, bool) {
	found := ""
	for name, decl := range c.enums {
		if enumHasVariant(decl, variant) {
			if found != "" && found != name {
				return "", false
			}
			found = name
		}
	}
	if found == "" {
		return "", false
	}
	return found, true
}

func (c *Compiler) emit(inst ir.Instr) {
	blk := c.currentBlock()
	if blk.Term != nil {
		return
	}
	blk.Instr = append(blk.Instr, inst)
}

func (c *Compiler) emitTerm(term ir.Term) {
	blk := c.currentBlock()
	if blk.Term != nil {
		return
	}
	blk.Term = term
}

func (c *Compiler) newTemp() int {
	id := c.tempID
	c.tempID++
	if c.current != nil {
		c.current.TempCount = c.tempID
	}
	return id
}

func (c *Compiler) newTempWithType(typ string) int {
	id := c.newTemp()
	c.tempTypes[id] = typ
	return id
}

func (c *Compiler) setTempType(temp int, typ string) {
	c.tempTypes[temp] = typ
}

func (c *Compiler) newBlock(prefix string) *ir.Block {
	label := fmt.Sprintf("%s%d", prefix, c.blockID)
	c.blockID++
	blk := &ir.Block{Label: label}
	c.current.Blocks = append(c.current.Blocks, blk)
	return blk
}

func (c *Compiler) currentBlock() *ir.Block {
	return c.curBlock
}

func (c *Compiler) setCurrentBlock(blk *ir.Block) {
	c.curBlock = blk
}

func (c *Compiler) pushScope() {
	c.scopeStack = append(c.scopeStack, scope{vars: map[string]VarInfo{}})
}

func (c *Compiler) popScope() {
	if len(c.scopeStack) == 0 {
		return
	}
	sc := c.scopeStack[len(c.scopeStack)-1]
	c.emitScopeDrops(sc)
	c.scopeStack = c.scopeStack[:len(c.scopeStack)-1]
}

// declareValueVar declares an immutable value variable (param or let)
// The temp ID is stored and returned directly when accessed
func (c *Compiler) declareValueVar(name string, temp int, typ string) {
	sc := &c.scopeStack[len(c.scopeStack)-1]
	sc.vars[name] = VarInfo{Temp: temp, Mutable: false, RefTemp: -1, Type: typ, Param: false}
	sc.order = append(sc.order, name)
}

// declareParamVar declares a function parameter (value semantics, no auto-drop).
func (c *Compiler) declareParamVar(name string, temp int, typ string) {
	sc := &c.scopeStack[len(c.scopeStack)-1]
	borrowed := isBorrowedTypeName(typ)
	sc.vars[name] = VarInfo{Temp: temp, Mutable: false, RefTemp: -1, Type: typ, Param: true, Borrowed: borrowed}
	sc.order = append(sc.order, name)
}

// declareMutVar declares a mutable variable (let mut)
// Returns the IR name to use in load/store instructions
func (c *Compiler) declareMutVar(name string, typ string) string {
	count := c.nameCount[name]
	c.nameCount[name] = count + 1
	irName := name
	if count > 0 {
		irName = fmt.Sprintf("%s#%d", name, count)
	}
	sc := &c.scopeStack[len(c.scopeStack)-1]
	sc.vars[name] = VarInfo{Temp: -1, Name: irName, Mutable: true, RefTemp: -1, Type: typ, Param: false}
	sc.order = append(sc.order, name)
	return irName
}

func (c *Compiler) declareRefVar(name string, refTemp int, mutable bool, typ string) {
	sc := &c.scopeStack[len(c.scopeStack)-1]
	sc.vars[name] = VarInfo{Temp: -1, Name: "", Mutable: mutable, RefTemp: refTemp, Type: typ, Param: false}
	sc.order = append(sc.order, name)
}

func (c *Compiler) lookupVar(name string) (VarInfo, bool) {
	for i := len(c.scopeStack) - 1; i >= 0; i-- {
		if v, ok := c.scopeStack[i].vars[name]; ok {
			return v, true
		}
	}
	return VarInfo{}, false
}

func (c *Compiler) updateVar(name string, update func(*VarInfo)) bool {
	for i := len(c.scopeStack) - 1; i >= 0; i-- {
		if v, ok := c.scopeStack[i].vars[name]; ok {
			update(&v)
			c.scopeStack[i].vars[name] = v
			return true
		}
	}
	return false
}

// markImmutable marks a variable as immutable (for let without mut)
func (c *Compiler) markImmutable(name string) {
	for i := len(c.scopeStack) - 1; i >= 0; i-- {
		if v, ok := c.scopeStack[i].vars[name]; ok {
			v.Mutable = false
			c.scopeStack[i].vars[name] = v
			return
		}
	}
}

func (c *Compiler) markClosure(name string) {
	for i := len(c.scopeStack) - 1; i >= 0; i-- {
		if v, ok := c.scopeStack[i].vars[name]; ok {
			v.Closure = true
			c.scopeStack[i].vars[name] = v
			return
		}
	}
}

func (c *Compiler) pushLoop(breakLabel, continueLabel, label, breakValueName string) {
	c.loopStack = append(c.loopStack, loopContext{
		breakLabel:    breakLabel,
		continueLabel: continueLabel,
		scopeDepth:    len(c.scopeStack),
		label:         label,
		breakValueName: breakValueName,
	})
}

func (c *Compiler) popLoop() {
	if len(c.loopStack) == 0 {
		return
	}
	c.loopStack = c.loopStack[:len(c.loopStack)-1]
}

func (c *Compiler) currentLoop() (loopContext, bool) {
	if len(c.loopStack) == 0 {
		return loopContext{}, false
	}
	return c.loopStack[len(c.loopStack)-1], true
}

func (c *Compiler) findLoop(label string) (loopContext, bool) {
	if len(c.loopStack) == 0 {
		return loopContext{}, false
	}
	if label == "" {
		return c.loopStack[len(c.loopStack)-1], true
	}
	for i := len(c.loopStack) - 1; i >= 0; i-- {
		if c.loopStack[i].label == label {
			return c.loopStack[i], true
		}
	}
	return loopContext{}, false
}

func (c *Compiler) operandToTemp(op ir.Operand, typ string) int {
	if !op.IsConst {
		return op.Temp
	}
	borrowed := c.operandBorrowed(op)
	if typ == "" {
		typ = "i64"
	}
	name := c.declareMutVar("__tmp", typ)
	c.emit(&ir.StoreVar{Name: name, Src: op})
	c.updateVar("__tmp", func(v *VarInfo) {
		v.Borrowed = borrowed
	})
	dst := c.newTemp()
	c.setTempType(dst, typ)
	c.emit(&ir.LoadVar{Dst: dst, Name: name})
	c.setTempBorrowed(dst, borrowed)
	return dst
}

// compileEnumVariant compiles an enum variant construction using MakeStruct.
// Enums are represented as structs with _tag (integer) and _payload (value) fields.
func (c *Compiler) compileEnumVariant(enumName, variant string, args []ast.Expr, span source.Span) int {
	tag, tagType, ok := c.enumTagInfo(enumName, variant)
	if !ok {
		c.diag.Add(span, fmt.Sprintf("unknown enum variant '%s.%s'", enumName, variant))
		return c.constZero()
	}

	// Create payload (unit if no args)
	payload := ir.ConstOperand(ir.Value{Kind: ir.KindUnit})
	if len(args) > 0 {
		payload = c.compileOperandMove(args[0])
	}

	// Create struct with _tag and _payload fields
	dst := c.newTemp()
	c.setTempType(dst, enumName)
	c.setTempBorrowed(dst, false)
	c.emit(&ir.MakeStruct{
		Dst:  dst,
		Name: enumName,
		Fields: []ir.StructFieldInit{
			{Name: "_tag", Src: ir.ConstOperand(ir.Value{Kind: ir.KindInt, Int: tag, IntType: tagType})},
			{Name: "_payload", Src: payload},
		},
	})
	return dst
}
