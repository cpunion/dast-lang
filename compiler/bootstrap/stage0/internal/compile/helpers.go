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

func constValueToIr(v ast.ConstValue) ir.Value {
	switch v.Kind {
	case ast.ConstInt:
		return ir.Value{Kind: ir.KindInt, Int: v.Int}
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

func (c *Compiler) newBlock(prefix string) *ir.Block {
	label := fmt.Sprintf("%s_%d", prefix, c.blockID)
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
	c.scopeStack = append(c.scopeStack, map[string]string{})
}

func (c *Compiler) popScope() {
	if len(c.scopeStack) == 0 {
		return
	}
	c.scopeStack = c.scopeStack[:len(c.scopeStack)-1]
}

func (c *Compiler) declareVar(name string) string {
	count := c.nameCount[name]
	c.nameCount[name] = count + 1
	irName := fmt.Sprintf("%s#%d", name, count)
	c.scopeStack[len(c.scopeStack)-1][name] = irName
	return irName
}

func (c *Compiler) lookupVar(name string) (string, bool) {
	for i := len(c.scopeStack) - 1; i >= 0; i-- {
		if v, ok := c.scopeStack[i][name]; ok {
			return v, true
		}
	}
	return "", false
}
