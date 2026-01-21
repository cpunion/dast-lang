package compile

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/ir"
)

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

func constValueToIr(v ast.ConstValue, typeName string) ir.Value {
	switch v.Kind {
	case ast.ConstInt:
		intType := ""
		if isIntTypeName(typeName) {
			intType = typeName
		}
		return ir.Value{Kind: ir.KindInt, Int: v.Int, IntType: intType}
	case ast.ConstBool:
		return ir.Value{Kind: ir.KindBool, Bool: v.Bool}
	case ast.ConstString:
		return ir.Value{Kind: ir.KindString, Str: v.Str}
	default:
		return ir.Value{Kind: ir.KindUnit}
	}
}

func (c *Compiler) constZero() int {
	t := c.newTemp()
	c.emit(&ir.Const{Dst: t, Value: ir.Value{Kind: ir.KindInt, Int: 0}})
	return t
}

func formatType(t ast.Type) string {
	base := ""
	if t.IsArray {
		if t.Elem != nil {
			base = "[" + formatType(*t.Elem) + "]"
		} else {
			base = "[]"
		}
	} else {
		base = t.Name
	}
	if t.IsRef {
		// IR uses *T for all references (both & and &mut)
		return "*" + base
	}
	return base
}

func isIntTypeName(name string) bool {
	switch name {
	case "i8", "i16", "i32", "i64", "i128",
		"u8", "u16", "u32", "u64", "u128",
		"isize", "usize", "char":
		return true
	default:
		return false
	}
}

func (c *Compiler) computeEnumTags() {
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
	}
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
	c.scopeStack = append(c.scopeStack, map[string]VarInfo{})
}

func (c *Compiler) popScope() {
	if len(c.scopeStack) == 0 {
		return
	}
	c.scopeStack = c.scopeStack[:len(c.scopeStack)-1]
}

// declareValueVar declares an immutable value variable (param or let)
// The temp ID is stored and returned directly when accessed
func (c *Compiler) declareValueVar(name string, temp int) {
	c.scopeStack[len(c.scopeStack)-1][name] = VarInfo{Temp: temp, Mutable: false}
}

// declareMutVar declares a mutable variable (let mut)
// Returns the IR name to use in load/store instructions
func (c *Compiler) declareMutVar(name string) string {
	count := c.nameCount[name]
	c.nameCount[name] = count + 1
	irName := name
	if count > 0 {
		irName = fmt.Sprintf("%s#%d", name, count)
	}
	c.scopeStack[len(c.scopeStack)-1][name] = VarInfo{Temp: -1, Name: irName, Mutable: true}
	return irName
}

func (c *Compiler) lookupVar(name string) (VarInfo, bool) {
	for i := len(c.scopeStack) - 1; i >= 0; i-- {
		if v, ok := c.scopeStack[i][name]; ok {
			return v, true
		}
	}
	return VarInfo{}, false
}
