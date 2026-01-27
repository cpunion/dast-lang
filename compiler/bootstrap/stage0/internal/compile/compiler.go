package compile

import (
	"dastlang/internal/ast"
	"dastlang/internal/diag"
	"dastlang/internal/ir"
)

type Compiler struct {
	prog         *ir.Program
	diag         *diag.Bag
	current      *ir.Function
	curBlock     *ir.Block
	blockID      int
	tempID       int
	tempTypes    map[int]string       // temp ID -> type string
	tempBorrowed map[int]bool         // temp ID -> borrowed/non-owning
	scopeStack   []scope              // name -> VarInfo
	nameCount    map[string]int
	structs      map[string]*ast.StructDecl
	enums        map[string]*ast.EnumDecl
	enumTags     map[string]map[string]int64
	enumTagType  map[string]string
	consts       map[string]ConstInfo
	funcRetTypes map[string]string // function name -> return type
	funcParamTypes map[string][]string // function name -> param types
	loopStack    []loopContext
	opts         Options
	closureID    int
	currentFuncName string
	dropFuncs    map[string]struct{}
}

// VarInfo tracks variable info for value semantics
type VarInfo struct {
	Temp    int    // temp ID for value vars, -1 for mutable vars
	Name    string // IR name (for mutable vars used in load/store)
	Mutable bool   // true if let mut
	RefTemp int    // temp ID for captured ref vars, -1 if not a ref capture
	Closure bool   // true if this var holds a closure value
	Type    string // IR type name (for drop/copy)
	Param   bool   // true if this variable is a function parameter
	Borrowed bool  // true if value is borrowed/non-owning
	Moved   bool   // true if moved
}

type ConstInfo struct {
	Value    ast.ConstValue
	TypeName string
}

type loopContext struct {
	breakLabel    string
	continueLabel string
	scopeDepth    int
	label         string
	breakValueName string
}

type Options struct {
	AllowCompile bool
}

func Compile(prog *ast.Program) (*ir.Program, *diag.Bag) {
	return CompileWithOptions(prog, Options{})
}

func CompileForMacro(prog *ast.Program) (*ir.Program, *diag.Bag) {
	return CompileWithOptions(prog, Options{AllowCompile: true})
}

func CompileWithOptions(prog *ast.Program, opts Options) (*ir.Program, *diag.Bag) {
	c := &Compiler{
		prog:         &ir.Program{Version: "v0", TypeDecls: map[string]*ir.TypeDecl{}, Enums: []*ir.EnumDecl{}, Functions: map[string]*ir.Function{}},
		diag:         &diag.Bag{},
		structs:      map[string]*ast.StructDecl{},
		enums:        map[string]*ast.EnumDecl{},
		nameCount:    map[string]int{},
		enumTags:     map[string]map[string]int64{},
		enumTagType:  map[string]string{},
		consts:       map[string]ConstInfo{},
		funcRetTypes: map[string]string{},
		funcParamTypes: map[string][]string{},
		tempBorrowed: map[int]bool{},
		opts:         opts,
		dropFuncs:    map[string]struct{}{},
	}
	// Initialize builtin function return types
	c.funcRetTypes["len"] = "i64"
	c.funcRetTypes["push"] = "unit"
	c.funcRetTypes["pop"] = "i64" // element type, but default to i64
	c.funcRetTypes["char_at"] = "i64"
	c.funcRetTypes["substr"] = "String"
	c.funcRetTypes["read_file"] = "String"
	c.funcRetTypes["read_dir"] = "[String]"
	c.funcRetTypes["write_file"] = "unit"
	c.funcRetTypes["mkdir"] = "unit"
	c.funcRetTypes["args"] = "[String]"
	c.funcRetTypes["print"] = "unit"
	c.funcRetTypes["println"] = "unit"
	c.funcRetTypes["exit"] = "unit"
	c.funcRetTypes["read_line"] = "String"
	c.funcRetTypes["read_bytes"] = "String"
	c.funcRetTypes["exec"] = "i64"
	c.funcRetTypes["int_to_string"] = "String"
	c.funcRetTypes["parse_int"] = "i32"
	c.funcRetTypes["string_to_int"] = "i64"
	c.funcRetTypes["has_prefix"] = "bool"
	c.funcRetTypes["string_clone"] = "String"
	c.funcRetTypes["ast_expr"] = "AstExpr"
	c.funcRetTypes["ast_stmt"] = "AstStmt"
	c.funcRetTypes["ast_item"] = "AstItem"
	c.funcRetTypes["ast_block"] = "AstBlock"
	c.funcRetTypes["ast_to_string"] = "String"
	c.funcRetTypes["gensym"] = "AstExpr"
	c.funcRetTypes["bind"] = "AstExpr"
	for _, item := range prog.Items {
		switch t := item.(type) {
		case *ast.StructDecl:
			c.structs[t.Name] = t
			// Add to IR type declarations
			fields := make([]ir.Var, 0, len(t.Fields))
			for _, f := range t.Fields {
				c.registerTupleTypesInType(f.Type)
				fields = append(fields, ir.Var{Name: f.Name, Type: formatType(f.Type)})
			}
			c.prog.TypeDecls[t.Name] = &ir.TypeDecl{Name: t.Name, Fields: fields}
		case *ast.EnumDecl:
			c.enums[t.Name] = t
			for _, v := range t.Variants {
				if v.Payload != nil {
					c.registerTupleTypesInType(*v.Payload)
				}
			}
		case *ast.ConstDecl:
			typeName := ""
			if t.Type != nil {
				c.registerTupleTypesInType(*t.Type)
				typeName = formatType(*t.Type)
			}
			c.consts[t.Name] = ConstInfo{Value: t.Value, TypeName: typeName}
		}
	}
	c.computeEnumTags()
	for _, item := range prog.Items {
		if e, ok := item.(*ast.EnumDecl); ok {
			c.prog.Enums = append(c.prog.Enums, c.enumDeclToIR(e))
		}
	}
	// Collect function return types before compiling
	for _, item := range prog.Items {
		switch t := item.(type) {
		case *ast.Function:
			params := make([]string, 0, len(t.Params))
			for _, p := range t.Params {
				c.registerTupleTypesInType(p.Type)
				params = append(params, formatType(p.Type))
			}
			c.funcParamTypes[t.Name] = params
			if t.ReturnType != nil {
				c.registerTupleTypesInType(*t.ReturnType)
				c.funcRetTypes[t.Name] = formatType(*t.ReturnType)
			} else {
				c.funcRetTypes[t.Name] = "unit"
			}
		case *ast.ImplDecl:
			for _, method := range t.Methods {
				name := t.TypeName + "." + method.Name
				params := make([]string, 0, len(method.Params))
				for _, p := range method.Params {
					c.registerTupleTypesInType(p.Type)
					params = append(params, formatType(p.Type))
				}
				c.funcParamTypes[name] = params
				if method.ReturnType != nil {
					c.registerTupleTypesInType(*method.ReturnType)
					c.funcRetTypes[name] = formatType(*method.ReturnType)
				} else {
					c.funcRetTypes[name] = "unit"
				}
			}
		case *ast.ImplTraitDecl:
			for _, method := range t.Methods {
				name := t.ForTypeName + "." + method.Name
				params := make([]string, 0, len(method.Params))
				for _, p := range method.Params {
					c.registerTupleTypesInType(p.Type)
					params = append(params, formatType(p.Type))
				}
				c.funcParamTypes[name] = params
				if method.ReturnType != nil {
					c.registerTupleTypesInType(*method.ReturnType)
					c.funcRetTypes[name] = formatType(*method.ReturnType)
				} else {
					c.funcRetTypes[name] = "unit"
				}
			}
		}
	}
	for _, item := range prog.Items {
		switch t := item.(type) {
		case *ast.Function:
			c.compileFunction(t)
		case *ast.ImplDecl:
			for _, method := range t.Methods {
				c.compileFunctionNamed(method, t.TypeName+"."+method.Name)
			}
		case *ast.ImplTraitDecl:
			for _, method := range t.Methods {
				c.compileFunctionNamed(method, t.ForTypeName+"."+method.Name)
			}
		}
	}
	c.emitDropHelpers()
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
	c.tempTypes = map[int]string{}
	c.closureID = 0
	c.currentFuncName = name

	irFn := &ir.Function{Name: name}
	c.current = irFn
	c.curBlock = nil
	c.pushScope()
	// Params use temp IDs internally, but we store names for formatting
	for _, param := range fn.Params {
		paramTemp := c.newTemp()
		c.declareParamVar(param.Name, paramTemp, formatType(param.Type))
		// Store with original name for IR output
		irFn.Params = append(irFn.Params, ir.Var{Name: param.Name, Type: formatType(param.Type)})
	}
	if fn.ReturnType != nil {
		irFn.ReturnType = formatType(*fn.ReturnType)
	} else {
		irFn.ReturnType = "unit"
	}
	entry := c.newBlock("entry")
	c.setCurrentBlock(entry)
	c.compileFunctionBody(fn)
	if c.currentBlock() != nil && c.currentBlock().Term == nil {
		c.emitDropsFromDepth(0)
		c.emitTerm(&ir.Return{Value: nil})
	}
	c.popScope()

	// Copy temp types to IR function
	irFn.TempTypes = make([]string, c.tempID)
	for i := 0; i < c.tempID; i++ {
		if typ, ok := c.tempTypes[i]; ok {
			irFn.TempTypes[i] = typ
		}
	}

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
				val := c.compileOperandMove(s.Expr)
				c.emitDropsFromDepth(0)
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

func (c *Compiler) compileBlockExprOperand(block *ast.Block) ir.Operand {
	if block == nil {
		return ir.ConstOperand(ir.Value{Kind: ir.KindUnit})
	}
	c.pushScope()
	result := ir.ConstOperand(ir.Value{Kind: ir.KindUnit})
	for i, stmt := range block.Stmts {
		if c.currentBlock().Term != nil {
			break
		}
		if i == len(block.Stmts)-1 {
			if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
				result = c.compileOperandMove(exprStmt.Expr)
				break
			}
		}
		c.compileStmt(stmt)
	}
	c.popScope()
	return result
}

func (c *Compiler) markMovedExpr(expr ast.Expr) {
	ident, ok := expr.(*ast.IdentExpr)
	if !ok {
		return
	}
	varInfo, ok := c.lookupVar(ident.Name)
	if !ok {
		return
	}
	if c.needsDropType(varInfo.Type) && !isCopyTypeName(varInfo.Type) && !isRefTypeName(varInfo.Type) {
		c.updateVar(ident.Name, func(v *VarInfo) { v.Moved = true })
	}
}

func (c *Compiler) callArgConsumes(callee string, index int) bool {
	switch callee {
	case "push":
		return index == 1
	case "string_free", "array_free", "struct_free":
		return true
	case "print", "println", "eprint", "eprintln", "len", "char_at", "substr", "string_clone", "read_file", "read_dir", "write_file", "mkdir", "args", "read_line", "read_bytes", "exec", "ast_expr", "ast_stmt", "ast_item", "ast_block", "ast_to_string", "gensym", "bind", "parse_int", "string_to_int", "has_prefix", "int_to_string", "pop", "exit":
		return false
	}
	if params, ok := c.funcParamTypes[callee]; ok {
		if index < len(params) {
			pt := params[index]
			if isRefTypeName(pt) {
				return false
			}
			if isCopyTypeName(pt) {
				return false
			}
			return c.needsDropType(pt)
		}
	}
	return true
}
