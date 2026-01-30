package compile

import (
	"fmt"
	"os"
	"strings"

	"dastlang/internal/ir"
)

func normalizeTypeName(t string) string {
	return strings.TrimSpace(t)
}

func isRefTypeName(t string) bool {
	t = normalizeTypeName(t)
	return strings.HasPrefix(t, "*")
}

func derefTypeName(t string) string {
	t = normalizeTypeName(t)
	if strings.HasPrefix(t, "*") {
		return strings.TrimPrefix(t, "*")
	}
	return t
}

func isArrayTypeName(t string) bool {
	t = normalizeTypeName(t)
	return strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]")
}

func arrayElemTypeName(t string) string {
	t = normalizeTypeName(t)
	if len(t) < 2 || t[0] != '[' || t[len(t)-1] != ']' {
		return ""
	}
	return strings.TrimSpace(t[1 : len(t)-1])
}

func isStringTypeName(t string) bool {
	t = normalizeTypeName(t)
	return t == "String"
}

func (c *Compiler) isStructTypeName(t string) bool {
	t = normalizeTypeName(t)
	if t == "" || isRefTypeName(t) || isArrayTypeName(t) || isStringTypeName(t) || t == "str" || isIntTypeName(t) || t == "bool" || t == "unit" {
		return false
	}
	if _, ok := c.structs[t]; ok {
		return true
	}
	if c.prog.TypeDecls != nil {
		if _, ok := c.prog.TypeDecls[t]; ok {
			return true
		}
	}
	return false
}

func (c *Compiler) isEnumTypeName(t string) bool {
	t = normalizeTypeName(t)
	if _, ok := c.enums[t]; ok {
		return true
	}
	return false
}

func isCopyTypeName(t string) bool {
	t = normalizeTypeName(t)
	if t == "" || t == "unit" {
		return true
	}
	if isRefTypeName(t) {
		return true
	}
	if t == "str" {
		return true
	}
	if isIntTypeName(t) || t == "bool" {
		return true
	}
	return false
}

func isBorrowedTypeName(t string) bool {
	t = normalizeTypeName(t)
	base := t
	if idx := strings.LastIndex(base, "::"); idx >= 0 {
		base = base[idx+2:]
	}
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	if idx := strings.LastIndex(base, "."); idx >= 0 {
		base = base[idx+1:]
	}
	// Arrays of borrowed element types are also borrowed; otherwise we end up
	// emitting drop calls for element helpers that we intentionally suppress.
	if isArrayTypeName(t) {
		elem := arrayElemTypeName(t)
		if isBorrowedTypeName(elem) {
			return true
		}
	}
	switch base {
	// Tokens are borrowed views into lexer storage and must not be auto-dropped
	// when passed around by value in stage0.
	case "Token", "TokenKind", "Tokens":
		return true
	// Stage2 AST values are frequently moved through indexing/match without full
	// move tracking yet; treat them as borrowed to avoid double-free/use-after-free.
	case "ParseError", "Visibility", "ConstValue", "ConstDecl", "Param", "GenericParam", "Field", "StructDecl", "Variant", "EnumDecl",
		"IdentExpr", "IntLit", "StringLit", "BoolLit", "UnaryExpr", "BinaryExpr", "CallExpr", "IndexExpr", "FieldExpr",
		"FieldInit", "StructLit", "ArrayLit", "RefExpr", "DerefExpr", "MethodCallExpr", "IfLetExpr", "ClosureParam", "ClosureExpr",
		"BlockExpr", "QuotePart", "QuoteExpr", "CompileExpr", "MacroCallExpr", "Expr",
		"LetStmt", "ReturnStmt", "ExprStmt", "IfStmt", "WhileStmt", "BreakStmt", "ContinueStmt",
		"WildcardPattern", "VariantPattern", "StructPatternField", "StructPattern", "BindPattern", "RangePattern", "LiteralPattern", "Pattern",
		"MatchArm", "MatchStmt", "Block", "Stmt", "AssignStmt", "FunctionDecl", "ImplDecl", "ImportItemSpec", "ImportDecl",
		"TraitMethod", "AssociatedType", "AssociatedTypeImpl", "TraitDecl", "ImplTrait", "TraitBound", "TypeAlias", "Program", "CompileItem",
		"MacroValue", "MacroEnv", "MacroCtx", "EvalResult", "QuoteSplice", "RenameMap",
		// Stage2 IR values are also passed by value pervasively; until move tracking
		// is complete, treat them as borrowed to prevent drop-time corruption.
		"IrArraySafety", "IrArrayValue", "IrBinOp", "IrBlock", "IrBranch", "IrCall", "IrCallClosure",
		"IrConstMap", "IrEnumDecl", "IrEnumHeaderParse", "IrEnumNameParse", "IrEnumValue", "IrEnumVariant", "IrEnumVariantParse",
		"IrFoldResult", "IrFunction", "IrGetField", "IrIndex", "IrInlineCandidate", "IrInlineResult", "IrInstr", "IrJump",
		"IrLineParse", "IrLoadVar", "IrMakeArray", "IrMakeStruct", "IrNoTerm", "IrOperand", "IrOperandParse", "IrParseResult",
		"IrProgram", "IrReturn", "IrSetField", "IrSetIndex", "IrStoreVar", "IrStrSet", "IrStructFieldInit", "IrStructFieldValue",
		"IrStructValue", "IrTerm", "IrValue", "IrVarConstMap",
		// Stage2 backend/driver helpers are still value-heavy; avoid auto-drop.
		"StringMap", "StringListMap", "CgInfer", "CgTypeArgs", "ResolvedCall", "TempFieldList", "FuncInstance", "FuncInstanceResult",
		"QbeCallArg", "QbeDerefResult", "QbeDispatchHelper", "QbeEmitter", "QbeExprLines", "QbeFuncCtx", "QbeStringConst", "QbeStructField", "QbeStructLayout",
		"ArgSplit", "ArgSplitPkg", "ImportLoad", "ImportSpec", "DepSpec", "Manifest", "ManifestInfo", "ManifestResult",
		"WorkspaceInfo", "PackageInfo", "FileUnit", "LoadCtx", "LoadResult", "LineCol", "LoadState", "ParseResult", "StringArrayParse", "TestCollect":
		return true
	default:
		return false
	}
}

func (c *Compiler) needsDropType(t string) bool {
	t = normalizeTypeName(t)
	if t == "" {
		return false
	}
	if os.Getenv("DAST_DISABLE_DROP") != "" {
		return false
	}
	if isBorrowedTypeName(t) {
		return false
	}
	if isCopyTypeName(t) {
		return false
	}
	if isStringTypeName(t) || isArrayTypeName(t) {
		return true
	}
	if c.isStructTypeName(t) || c.isEnumTypeName(t) {
		return true
	}
	return false
}

func mangleTypeName(t string) string {
	var sb strings.Builder
	for _, r := range t {
		switch {
		case r >= 'a' && r <= 'z':
			sb.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			sb.WriteRune(r)
		case r >= '0' && r <= '9':
			sb.WriteRune(r)
		default:
			sb.WriteRune('_')
		}
	}
	return sb.String()
}

func dropFuncNameForType(t string) string {
	t = normalizeTypeName(t)
	if isArrayTypeName(t) {
		elem := arrayElemTypeName(t)
		return "__drop_array_" + mangleTypeName(elem)
	}
	return "__drop_" + mangleTypeName(t)
}

func (c *Compiler) ensureDropFunc(t string) string {
	t = normalizeTypeName(t)
	if !c.needsDropType(t) {
		return ""
	}
	name := dropFuncNameForType(t)
	if _, ok := c.dropFuncs[t]; ok {
		return name
	}
	c.dropFuncs[t] = struct{}{}
	// Ensure nested drop functions
	if isArrayTypeName(t) {
		elem := arrayElemTypeName(t)
		c.ensureDropFunc(elem)
	}
	if c.isStructTypeName(t) {
		if decl, ok := c.structs[t]; ok {
			for _, f := range decl.Fields {
				c.ensureDropFunc(formatType(f.Type))
			}
		} else if c.prog.TypeDecls != nil {
			if td, ok := c.prog.TypeDecls[t]; ok {
				for _, f := range td.Fields {
					c.ensureDropFunc(f.Type)
				}
			}
		}
	}
	if c.isEnumTypeName(t) {
		if decl, ok := c.enums[t]; ok {
			for _, v := range decl.Variants {
				if v.Payload != nil {
					c.ensureDropFunc(formatType(*v.Payload))
				}
			}
		}
	}
	return name
}

func (c *Compiler) emitDropCall(op ir.Operand, typ string) {
	typ = normalizeTypeName(typ)
	if c.operandBorrowed(op) {
		return
	}
	if !c.needsDropType(typ) {
		return
	}
	name := c.ensureDropFunc(typ)
	if name == "" {
		return
	}
	c.emit(&ir.Call{Dst: -1, Callee: name, Args: []ir.Operand{op}})
}

func (c *Compiler) emitDropForVar(info VarInfo) {
	if info.Param {
		// Parameters are treated as borrowed in stage0 to avoid double-free
		// when callers pass field/index expressions without move tracking.
		return
	}
	if info.Borrowed {
		return
	}
	if info.Moved {
		return
	}
	if !c.needsDropType(info.Type) {
		return
	}
	if info.RefTemp >= 0 {
		return
	}
	if info.Temp >= 0 {
		c.emitDropCall(ir.TempOperand(info.Temp), info.Type)
		return
	}
	if info.Name != "" {
		t := c.newTemp()
		c.setTempType(t, info.Type)
		c.emit(&ir.LoadVar{Dst: t, Name: info.Name})
		c.emitDropCall(ir.TempOperand(t), info.Type)
	}
}

func (c *Compiler) emitScopeDrops(sc scope) {
	if c.currentBlock() == nil || c.currentBlock().Term != nil {
		return
	}
	for i := len(sc.order) - 1; i >= 0; i-- {
		name := sc.order[i]
		info, ok := sc.vars[name]
		if !ok {
			continue
		}
		c.emitDropForVar(info)
	}
}

func (c *Compiler) emitDropsFromDepth(depth int) {
	if c.currentBlock() == nil || c.currentBlock().Term != nil {
		return
	}
	if depth < 0 {
		depth = 0
	}
	for i := len(c.scopeStack) - 1; i >= depth; i-- {
		sc := c.scopeStack[i]
		for j := len(sc.order) - 1; j >= 0; j-- {
			name := sc.order[j]
			info, ok := sc.vars[name]
			if !ok {
				continue
			}
			c.emitDropForVar(info)
		}
	}
}

func (c *Compiler) emitDropHelpers() {
	if len(c.dropFuncs) == 0 {
		return
	}
	// Ensure drop helpers exist for common arrays of borrowed AST node types that
	// may not be registered consistently due to type qualification differences.
	// Only register these when the base type is actually present in the program
	// (or has already been requested), to avoid exploding IR output for small tests.
	for _, base := range []string{
		"ParseError", "Param", "GenericParam", "Field", "StructDecl", "Variant", "EnumDecl",
		"ImportItemSpec", "ImportDecl", "TraitMethod", "AssociatedType", "AssociatedTypeImpl",
		"TraitDecl", "ImplDecl", "ImplTrait", "TraitBound", "TypeAlias", "FunctionDecl",
		"MatchArm", "Stmt", "Expr", "Pattern", "CompileItem",
		// Stage2 IR types.
		"IrArraySafety", "IrArrayValue", "IrBinOp", "IrBlock", "IrBranch", "IrCall", "IrCallClosure",
		"IrConstMap", "IrEnumDecl", "IrEnumHeaderParse", "IrEnumNameParse", "IrEnumValue", "IrEnumVariant", "IrEnumVariantParse",
		"IrFoldResult", "IrFunction", "IrGetField", "IrIndex", "IrInlineCandidate", "IrInlineResult", "IrInstr", "IrJump",
		"IrLineParse", "IrLoadVar", "IrMakeArray", "IrMakeStruct", "IrNoTerm", "IrOperand", "IrOperandParse", "IrParseResult",
		"IrProgram", "IrReturn", "IrSetField", "IrSetIndex", "IrStoreVar", "IrStrSet", "IrStructFieldInit", "IrStructFieldValue",
		"IrStructValue", "IrTerm", "IrValue", "IrVarConstMap",
		// Stage2 backend/driver helpers.
		"StringMap", "StringListMap", "CgInfer", "CgTypeArgs", "ResolvedCall", "TempFieldList", "FuncInstance", "FuncInstanceResult",
		"QbeCallArg", "QbeDerefResult", "QbeDispatchHelper", "QbeEmitter", "QbeExprLines", "QbeFuncCtx", "QbeStringConst", "QbeStructField", "QbeStructLayout",
		"ArgSplit", "ArgSplitPkg", "ImportLoad", "ImportSpec", "DepSpec", "Manifest", "ManifestInfo", "ManifestResult",
		"WorkspaceInfo", "PackageInfo", "FileUnit", "LoadCtx", "LoadResult", "LineCol", "LoadState", "ParseResult", "StringArrayParse", "TestCollect",
	} {
		_, hasBaseDrop := c.dropFuncs[base]
		_, hasArrayDrop := c.dropFuncs["["+base+"]"]
		hasStruct := c.isStructTypeName(base)
		hasEnum := c.isEnumTypeName(base)
		hasTypeDecl := false
		if c.prog.TypeDecls != nil {
			_, hasTypeDecl = c.prog.TypeDecls[base]
		}
		if hasBaseDrop || hasArrayDrop || hasStruct || hasEnum || hasTypeDecl {
			c.dropFuncs["["+base+"]"] = struct{}{}
		}
	}
	debugDrop := os.Getenv("DAST_DROP_DEBUG") != ""
	seen := map[string]struct{}{}
	var emit func(t string)
	emit = func(t string) {
		if t == "" {
			return
		}
		if _, ok := seen[t]; ok {
			return
		}
		seen[t] = struct{}{}
		if debugDrop && (t == "TokenKind" || t == "Stmt") {
			_, hasStruct := c.structs[t]
			_, hasEnum := c.enums[t]
			_, hasTypeDecl := c.prog.TypeDecls[t]
			fmt.Fprintf(os.Stderr, "stage0: drop classify %s struct=%v enum=%v typedecl=%v\n", t, hasStruct, hasEnum, hasTypeDecl)
		}
			if isArrayTypeName(t) {
				elem := arrayElemTypeName(t)
				emit(elem)
				c.prog.Functions[dropFuncNameForType(t)] = buildDropArrayFunc(t, elem, c)
				return
			}
			if c.isStructTypeName(t) {
				// Ensure nested field drop helpers are emitted even when they were
				// never explicitly requested via ensureDropFunc.
				if decl, ok := c.structs[t]; ok && decl != nil {
					for _, f := range decl.Fields {
						ft := formatType(f.Type)
						if c.needsDropType(ft) {
							emit(ft)
						}
					}
				} else if c.prog.TypeDecls != nil {
					if td, ok := c.prog.TypeDecls[t]; ok && td != nil {
						for _, f := range td.Fields {
							ft := normalizeTypeName(f.Type)
							if c.needsDropType(ft) {
								emit(ft)
							}
						}
					}
				}
				c.prog.Functions[dropFuncNameForType(t)] = buildDropStructFunc(t, c)
				return
			}
			if c.isEnumTypeName(t) {
				if decl, ok := c.enums[t]; ok && decl != nil {
					for _, v := range decl.Variants {
						if v.Payload == nil {
							continue
						}
						pt := formatType(*v.Payload)
						if c.needsDropType(pt) {
							emit(pt)
						}
					}
				}
				c.prog.Functions[dropFuncNameForType(t)] = buildDropEnumFunc(t, c)
				return
			}
		if isStringTypeName(t) {
			c.prog.Functions[dropFuncNameForType(t)] = buildDropStringFunc(t)
			return
		}
	}
	for t := range c.dropFuncs {
		emit(t)
	}
}

type dropBuilder struct {
	fn        *ir.Function
	cur       *ir.Block
	tempID    int
	blockID   int
	tempTypes map[int]string
}

func newDropBuilder(name string, paramType string) *dropBuilder {
	fn := &ir.Function{Name: name, ReturnType: "unit"}
	fn.Params = append(fn.Params, ir.Var{Name: "v", Type: paramType})
	b := &dropBuilder{
		fn:        fn,
		cur:       nil,
		tempID:    1, // param is t0
		blockID:   0,
		tempTypes: map[int]string{0: paramType},
	}
	return b
}

func (b *dropBuilder) newTemp(typ string) int {
	id := b.tempID
	b.tempID++
	if typ != "" {
		b.tempTypes[id] = typ
	}
	return id
}

func (b *dropBuilder) newBlock(prefix string) *ir.Block {
	label := fmt.Sprintf("%s%d", prefix, b.blockID)
	b.blockID++
	blk := &ir.Block{Label: label}
	b.fn.Blocks = append(b.fn.Blocks, blk)
	return blk
}

func (b *dropBuilder) setBlock(blk *ir.Block) {
	b.cur = blk
}

func (b *dropBuilder) emit(inst ir.Instr) {
	if b.cur == nil {
		return
	}
	b.cur.Instr = append(b.cur.Instr, inst)
}

func (b *dropBuilder) emitTerm(term ir.Term) {
	if b.cur == nil {
		return
	}
	b.cur.Term = term
}

func (b *dropBuilder) finish() *ir.Function {
	b.fn.TempCount = b.tempID
	b.fn.TempTypes = make([]string, b.tempID)
	for i := 0; i < b.tempID; i++ {
		if t, ok := b.tempTypes[i]; ok {
			b.fn.TempTypes[i] = t
		}
	}
	return b.fn
}

func buildDropStringFunc(typ string) *ir.Function {
	b := newDropBuilder(dropFuncNameForType(typ), typ)
	entry := b.newBlock("entry")
	b.setBlock(entry)
	b.emit(&ir.Call{Dst: -1, Callee: "string_free", Args: []ir.Operand{ir.TempOperand(0)}})
	b.emitTerm(&ir.Return{Value: nil})
	return b.finish()
}

func buildDropArrayFunc(arrayType string, elemType string, c *Compiler) *ir.Function {
	name := dropFuncNameForType(arrayType)
	b := newDropBuilder(name, arrayType)
	entry := b.newBlock("entry")
	loop := b.newBlock("loop")
	done := b.newBlock("done")
	b.setBlock(entry)

	// if arr == 0 jump done
	zero := b.newTemp("i64")
	b.emit(&ir.BinOp{Dst: zero, Op: "+", Lhs: ir.IntOperand(0), Rhs: ir.IntOperand(0)})
	cmp := b.newTemp("bool")
	b.emit(&ir.BinOp{Dst: cmp, Op: "==", Lhs: ir.TempOperand(0), Rhs: ir.TempOperand(zero)})
	b.emitTerm(&ir.Branch{Cond: ir.TempOperand(cmp), Then: done.Label, Else: loop.Label})

	if c.needsDropType(elemType) {
		body := b.newBlock("body")
		b.setBlock(loop)
		// len = len(arr)
		lenTemp := b.newTemp("int")
		b.emit(&ir.Call{Dst: lenTemp, Callee: "len", Args: []ir.Operand{ir.TempOperand(0)}})
		b.emit(&ir.StoreVar{Name: "len", Src: ir.TempOperand(lenTemp)})
		// i = 0
		b.emit(&ir.StoreVar{Name: "i", Src: ir.IntOperand(0)})
		b.emitTerm(&ir.Jump{Target: body.Label})

		b.setBlock(body)
		iTemp := b.newTemp("int")
		b.emit(&ir.LoadVar{Dst: iTemp, Name: "i"})
		lenTemp2 := b.newTemp("int")
		b.emit(&ir.LoadVar{Dst: lenTemp2, Name: "len"})
		cond := b.newTemp("bool")
		b.emit(&ir.BinOp{Dst: cond, Op: "<", Lhs: ir.TempOperand(iTemp), Rhs: ir.TempOperand(lenTemp2)})
		next := b.newBlock("next")
		b.emitTerm(&ir.Branch{Cond: ir.TempOperand(cond), Then: next.Label, Else: done.Label})

		b.setBlock(next)
		elemTemp := b.newTemp(elemType)
		b.emit(&ir.Index{Dst: elemTemp, Array: ir.TempOperand(0), Index: ir.TempOperand(iTemp)})
		b.emit(&ir.Call{Dst: -1, Callee: dropFuncNameForType(elemType), Args: []ir.Operand{ir.TempOperand(elemTemp)}})
		inc := b.newTemp("int")
		b.emit(&ir.BinOp{Dst: inc, Op: "+", Lhs: ir.TempOperand(iTemp), Rhs: ir.IntOperand(1)})
		b.emit(&ir.StoreVar{Name: "i", Src: ir.TempOperand(inc)})
		b.emitTerm(&ir.Jump{Target: body.Label})
	} else {
		b.setBlock(loop)
		b.emitTerm(&ir.Jump{Target: done.Label})
	}

	b.setBlock(done)
	b.emit(&ir.Call{Dst: -1, Callee: "array_free", Args: []ir.Operand{ir.TempOperand(0)}})
	b.emitTerm(&ir.Return{Value: nil})
	return b.finish()
}

func buildDropStructFunc(structType string, c *Compiler) *ir.Function {
	name := dropFuncNameForType(structType)
	b := newDropBuilder(name, structType)
	entry := b.newBlock("entry")
	drop := b.newBlock("drop")
	done := b.newBlock("done")
	b.setBlock(entry)

	zero := b.newTemp("i64")
	b.emit(&ir.BinOp{Dst: zero, Op: "+", Lhs: ir.IntOperand(0), Rhs: ir.IntOperand(0)})
	cmp := b.newTemp("bool")
	b.emit(&ir.BinOp{Dst: cmp, Op: "==", Lhs: ir.TempOperand(0), Rhs: ir.TempOperand(zero)})
	b.emitTerm(&ir.Branch{Cond: ir.TempOperand(cmp), Then: done.Label, Else: drop.Label})

	b.setBlock(drop)
	if decl, ok := c.structs[structType]; ok {
		for _, f := range decl.Fields {
			ft := formatType(f.Type)
			if !c.needsDropType(ft) {
				continue
			}
			tmp := b.newTemp(ft)
			b.emit(&ir.GetField{Dst: tmp, Src: 0, Field: f.Name})
			b.emit(&ir.Call{Dst: -1, Callee: dropFuncNameForType(ft), Args: []ir.Operand{ir.TempOperand(tmp)}})
		}
	} else if c.prog.TypeDecls != nil {
		if td, ok := c.prog.TypeDecls[structType]; ok {
			for _, f := range td.Fields {
				ft := f.Type
				if !c.needsDropType(ft) {
					continue
				}
				tmp := b.newTemp(ft)
				b.emit(&ir.GetField{Dst: tmp, Src: 0, Field: f.Name})
				b.emit(&ir.Call{Dst: -1, Callee: dropFuncNameForType(ft), Args: []ir.Operand{ir.TempOperand(tmp)}})
			}
		}
	}
	b.emit(&ir.Call{Dst: -1, Callee: "struct_free", Args: []ir.Operand{ir.TempOperand(0)}})
	b.emitTerm(&ir.Jump{Target: done.Label})

	b.setBlock(done)
	b.emitTerm(&ir.Return{Value: nil})
	return b.finish()
}

func buildDropEnumFunc(enumType string, c *Compiler) *ir.Function {
	name := dropFuncNameForType(enumType)
	b := newDropBuilder(name, enumType)
	entry := b.newBlock("entry")
	check := b.newBlock("check")
	done := b.newBlock("done")
	b.setBlock(entry)

	zero := b.newTemp("i64")
	b.emit(&ir.BinOp{Dst: zero, Op: "+", Lhs: ir.IntOperand(0), Rhs: ir.IntOperand(0)})
	cmp := b.newTemp("bool")
	b.emit(&ir.BinOp{Dst: cmp, Op: "==", Lhs: ir.TempOperand(0), Rhs: ir.TempOperand(zero)})
	b.emitTerm(&ir.Branch{Cond: ir.TempOperand(cmp), Then: done.Label, Else: check.Label})

	b.setBlock(check)
	decl, ok := c.enums[enumType]
	if !ok || decl == nil {
		b.emit(&ir.Call{Dst: -1, Callee: "struct_free", Args: []ir.Operand{ir.TempOperand(0)}})
		b.emitTerm(&ir.Jump{Target: done.Label})
		b.setBlock(done)
		b.emitTerm(&ir.Return{Value: nil})
		return b.finish()
	}
	tagType := decl.Repr
	if tagType == "" {
		tagType = "i32"
	}
	tagTemp := b.newTemp(tagType)
	b.emit(&ir.GetField{Dst: tagTemp, Src: 0, Field: "_tag"})

	freeBlock := b.newBlock("free")
	b.setBlock(freeBlock)
	b.emit(&ir.Call{Dst: -1, Callee: "struct_free", Args: []ir.Operand{ir.TempOperand(0)}})
	b.emitTerm(&ir.Jump{Target: done.Label})

	nextBlock := freeBlock
	hasDroppable := false
	for i := len(decl.Variants) - 1; i >= 0; i-- {
		v := decl.Variants[i]
		if v.Payload == nil {
			continue
		}
		payloadType := formatType(*v.Payload)
		if !c.needsDropType(payloadType) {
			continue
		}
		hasDroppable = true
		dropBlock := b.newBlock("drop_v")
		checkBlock := b.newBlock("check_v")
		tagVal, _, ok := c.enumTagInfo(enumType, v.Name)
		if !ok {
			continue
		}
		cmpTemp := b.newTemp("bool")
		b.setBlock(checkBlock)
		b.emit(&ir.BinOp{Dst: cmpTemp, Op: "==", Lhs: ir.TempOperand(tagTemp), Rhs: ir.ConstOperand(ir.Value{Kind: ir.KindInt, Int: tagVal, IntType: tagType})})
		b.emitTerm(&ir.Branch{Cond: ir.TempOperand(cmpTemp), Then: dropBlock.Label, Else: nextBlock.Label})

		b.setBlock(dropBlock)
		payloadTemp := b.newTemp(payloadType)
		b.emit(&ir.GetField{Dst: payloadTemp, Src: 0, Field: "_payload"})
		b.emit(&ir.Call{Dst: -1, Callee: dropFuncNameForType(payloadType), Args: []ir.Operand{ir.TempOperand(payloadTemp)}})
		b.emitTerm(&ir.Jump{Target: freeBlock.Label})

		nextBlock = checkBlock
	}
	b.setBlock(check)
	if hasDroppable {
		// Jump to first check block
		b.emitTerm(&ir.Jump{Target: nextBlock.Label})
	} else {
		b.emitTerm(&ir.Jump{Target: freeBlock.Label})
	}

	b.setBlock(done)
	b.emitTerm(&ir.Return{Value: nil})
	return b.finish()
}
