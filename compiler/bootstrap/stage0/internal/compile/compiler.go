package compile

import (
	"dastlang/internal/ast"
	"dastlang/internal/diag"
	"dastlang/internal/ir"
)

type Compiler struct {
	prog        *ir.Program
	diag        *diag.Bag
	current     *ir.Function
	curBlock    *ir.Block
	blockID     int
	tempID      int
	scopeStack  []map[string]string
	nameCount   map[string]int
	structs     map[string]*ast.StructDecl
	enums       map[string]*ast.EnumDecl
	enumTags    map[string]map[string]int64
	enumTagType map[string]string
	consts      map[string]ConstInfo
}

type ConstInfo struct {
	Value    ast.ConstValue
	TypeName string
}

func Compile(prog *ast.Program) (*ir.Program, *diag.Bag) {
	c := &Compiler{
		prog:        &ir.Program{Version: "v0", Functions: map[string]*ir.Function{}},
		diag:        &diag.Bag{},
		structs:     map[string]*ast.StructDecl{},
		enums:       map[string]*ast.EnumDecl{},
		nameCount:   map[string]int{},
		enumTags:    map[string]map[string]int64{},
		enumTagType: map[string]string{},
		consts:      map[string]ConstInfo{},
	}
	for _, item := range prog.Items {
		switch t := item.(type) {
		case *ast.StructDecl:
			c.structs[t.Name] = t
		case *ast.EnumDecl:
			c.enums[t.Name] = t
		case *ast.ConstDecl:
			typeName := ""
			if t.Type != nil {
				typeName = formatType(*t.Type)
			}
			c.consts[t.Name] = ConstInfo{Value: t.Value, TypeName: typeName}
		}
	}
	c.computeEnumTags()
	for _, item := range prog.Items {
		switch t := item.(type) {
		case *ast.Function:
			c.compileFunction(t)
		case *ast.ImplDecl:
			for _, method := range t.Methods {
				c.compileFunctionNamed(method, t.TypeName+"."+method.Name)
			}
		}
	}
	return c.prog, c.diag
}

func (c *Compiler) compileFunction(fn *ast.Function) {
	c.compileFunctionNamed(fn, fn.Name)
}

func (c *Compiler) compileFunctionNamed(fn *ast.Function, name string) {
	c.blockID = 0
	c.tempID = 0
	c.scopeStack = nil
	c.nameCount = map[string]int{}

	irFn := &ir.Function{Name: name}
	c.current = irFn
	c.curBlock = nil
	c.pushScope()
	for _, param := range fn.Params {
		irName := c.declareVar(param.Name)
		irFn.Params = append(irFn.Params, irName)
		irFn.ParamTypes = append(irFn.ParamTypes, formatType(param.Type))
	}
	if fn.ReturnType != nil {
		irFn.ReturnType = formatType(*fn.ReturnType)
	}
	entry := c.newBlock("entry")
	c.setCurrentBlock(entry)
	c.compileFunctionBody(fn)
	if c.currentBlock() != nil && c.currentBlock().Term == nil {
		c.emitTerm(&ir.Return{Value: nil})
	}
	c.popScope()

	c.prog.Functions[irFn.Name] = irFn
	if c.prog.Entry == "" && irFn.Name == "main" {
		c.prog.Entry = "main"
	}
}

func (c *Compiler) compileBlock(block *ast.Block) {
	c.pushScope()
	for _, stmt := range block.Stmts {
		if c.currentBlock().Term != nil {
			break
		}
		c.compileStmt(stmt)
	}
	c.popScope()
}

func (c *Compiler) compileFunctionBody(fn *ast.Function) {
	allowImplicit := true
	if fn.ReturnType != nil && !fn.ReturnType.IsArray {
		if fn.ReturnType.Name == "unit" || fn.ReturnType.Name == "()" {
			allowImplicit = false
		}
	}
	c.compileBlockWithTail(fn.Body, allowImplicit)
}

func (c *Compiler) compileBlockWithTail(block *ast.Block, allowImplicit bool) {
	if !allowImplicit {
		c.compileBlock(block)
		return
	}
	c.pushScope()
	for i, stmt := range block.Stmts {
		if c.currentBlock().Term != nil {
			break
		}
		if i == len(block.Stmts)-1 {
			switch s := stmt.(type) {
			case *ast.ExprStmt:
				val := c.compileExpr(s.Expr)
				c.emitTerm(&ir.Return{Value: &val})
				c.popScope()
				return
			case *ast.IfStmt:
				c.compileTailIf(s, allowImplicit)
				c.popScope()
				return
			case *ast.MatchStmt:
				c.compileTailMatch(s, allowImplicit)
				c.popScope()
				return
			}
		}
		c.compileStmt(stmt)
	}
	c.popScope()
}
