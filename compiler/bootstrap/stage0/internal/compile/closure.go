package compile

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/ir"
)

type nameSet struct {
	order []string
	set   map[string]struct{}
}

func newNameSet() *nameSet {
	return &nameSet{set: map[string]struct{}{}}
}

func (s *nameSet) add(name string) {
	if name == "" {
		return
	}
	if _, ok := s.set[name]; ok {
		return
	}
	s.set[name] = struct{}{}
	s.order = append(s.order, name)
}

func analyzeFreeVars(expr ast.Expr, params []string) ([]string, map[string]struct{}) {
	locals := map[string]struct{}{}
	for _, p := range params {
		locals[p] = struct{}{}
	}
	free := newNameSet()
	mutated := map[string]struct{}{}
	collectFreeVarsExpr(expr, locals, free, mutated)
	return free.order, mutated
}

func collectFreeVarsExpr(expr ast.Expr, locals map[string]struct{}, free *nameSet, mutated map[string]struct{}) {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		if _, ok := locals[e.Name]; !ok {
			free.add(e.Name)
		}
	case *ast.IntLit, *ast.BoolLit, *ast.StringLit:
		return
	case *ast.ArrayLit:
		for _, elem := range e.Elems {
			collectFreeVarsExpr(elem, locals, free, mutated)
		}
	case *ast.UnaryExpr:
		collectFreeVarsExpr(e.Expr, locals, free, mutated)
	case *ast.RefExpr:
		collectFreeVarsExpr(e.Expr, locals, free, mutated)
	case *ast.DerefExpr:
		collectFreeVarsExpr(e.Expr, locals, free, mutated)
	case *ast.BinaryExpr:
		collectFreeVarsExpr(e.Left, locals, free, mutated)
		collectFreeVarsExpr(e.Right, locals, free, mutated)
	case *ast.CallExpr:
		if e.Callee != "" {
			if _, ok := locals[e.Callee]; !ok {
				free.add(e.Callee)
			}
		}
		for _, arg := range e.Args {
			collectFreeVarsExpr(arg, locals, free, mutated)
		}
	case *ast.MethodCallExpr:
		collectFreeVarsExpr(e.Receiver, locals, free, mutated)
		for _, arg := range e.Args {
			collectFreeVarsExpr(arg, locals, free, mutated)
		}
	case *ast.StructLit:
		for _, f := range e.Fields {
			collectFreeVarsExpr(f.Value, locals, free, mutated)
		}
	case *ast.AccessExpr:
		collectFreeVarsExpr(e.Receiver, locals, free, mutated)
	case *ast.IndexExpr:
		collectFreeVarsExpr(e.Receiver, locals, free, mutated)
		collectFreeVarsExpr(e.Index, locals, free, mutated)
	case *ast.EnumVariantExpr:
		if e.Arg != nil {
			collectFreeVarsExpr(e.Arg, locals, free, mutated)
		}
	case *ast.BlockExpr:
		collectFreeVarsBlock(e.Block, locals, free, mutated)
	case *ast.IfExpr:
		collectFreeVarsExpr(e.Cond, locals, free, mutated)
		collectFreeVarsExpr(e.Then, locals, free, mutated)
		if e.Else != nil {
			collectFreeVarsExpr(e.Else, locals, free, mutated)
		}
	case *ast.MatchExpr:
		collectFreeVarsExpr(e.Expr, locals, free, mutated)
		for _, arm := range e.Arms {
			localCopy := copyLocalSet(locals)
			declarePatternBindings(arm.Pattern, localCopy)
			if arm.Guard != nil {
				collectFreeVarsExpr(arm.Guard, localCopy, free, mutated)
			}
			collectFreeVarsBlock(arm.Body, localCopy, free, mutated)
		}
	case *ast.ClosureExpr:
		localCopy := copyLocalSet(locals)
		for _, p := range e.Params {
			localCopy[p.Name] = struct{}{}
		}
		collectFreeVarsExpr(e.Body, localCopy, free, mutated)
	}
}

func collectFreeVarsBlock(block *ast.Block, locals map[string]struct{}, free *nameSet, mutated map[string]struct{}) {
	if block == nil {
		return
	}
	localCopy := copyLocalSet(locals)
	for _, stmt := range block.Stmts {
		collectFreeVarsStmt(stmt, localCopy, free, mutated)
	}
}

func collectFreeVarsStmt(stmt ast.Stmt, locals map[string]struct{}, free *nameSet, mutated map[string]struct{}) {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		if s.Init != nil {
			collectFreeVarsExpr(s.Init, locals, free, mutated)
		}
		locals[s.Name] = struct{}{}
	case *ast.LetPatternStmt:
		if s.Init != nil {
			collectFreeVarsExpr(s.Init, locals, free, mutated)
		}
		declarePatternBindings(s.Pattern, locals)
	case *ast.AssignStmt:
		if id, ok := s.Target.(*ast.IdentExpr); ok {
			if _, ok := locals[id.Name]; !ok {
				mutated[id.Name] = struct{}{}
				free.add(id.Name)
			}
		} else {
			collectFreeVarsExpr(s.Target, locals, free, mutated)
		}
		if s.Value != nil {
			collectFreeVarsExpr(s.Value, locals, free, mutated)
		}
	case *ast.ExprStmt:
		collectFreeVarsExpr(s.Expr, locals, free, mutated)
	case *ast.ReturnStmt:
		if s.Value != nil {
			collectFreeVarsExpr(s.Value, locals, free, mutated)
		}
	case *ast.IfStmt:
		collectFreeVarsExpr(s.Cond, locals, free, mutated)
		collectFreeVarsBlock(s.Then, locals, free, mutated)
		collectFreeVarsBlock(s.Else, locals, free, mutated)
	case *ast.IfLetStmt:
		collectFreeVarsExpr(s.Expr, locals, free, mutated)
		localCopy := copyLocalSet(locals)
		declarePatternBindings(s.Pattern, localCopy)
		collectFreeVarsBlock(s.Then, localCopy, free, mutated)
		collectFreeVarsBlock(s.Else, locals, free, mutated)
	case *ast.WhileStmt:
		collectFreeVarsExpr(s.Cond, locals, free, mutated)
		collectFreeVarsBlock(s.Body, locals, free, mutated)
	case *ast.WhileLetStmt:
		collectFreeVarsExpr(s.Expr, locals, free, mutated)
		localCopy := copyLocalSet(locals)
		declarePatternBindings(s.Pattern, localCopy)
		collectFreeVarsBlock(s.Body, localCopy, free, mutated)
	case *ast.MatchStmt:
		collectFreeVarsExpr(s.Expr, locals, free, mutated)
		for _, arm := range s.Arms {
			localCopy := copyLocalSet(locals)
			declarePatternBindings(arm.Pattern, localCopy)
			if arm.Guard != nil {
				collectFreeVarsExpr(arm.Guard, localCopy, free, mutated)
			}
			collectFreeVarsBlock(arm.Body, localCopy, free, mutated)
		}
	case *ast.Block:
		collectFreeVarsBlock(s, locals, free, mutated)
	}
}

func declarePatternBindings(p ast.Pattern, locals map[string]struct{}) {
	if p == nil {
		return
	}
	switch pat := p.(type) {
	case *ast.VariantPattern:
		if pat.Binding != "" {
			locals[pat.Binding] = struct{}{}
		}
	case *ast.StructPattern:
		for _, f := range pat.Fields {
			if f.Binding != "" {
				locals[f.Binding] = struct{}{}
			}
			if f.Pattern != nil {
				declarePatternBindings(f.Pattern, locals)
			}
		}
	case *ast.OrPattern:
		for _, alt := range pat.Alts {
			declarePatternBindings(alt, locals)
		}
	}
}

func copyLocalSet(src map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for k := range src {
		out[k] = struct{}{}
	}
	return out
}

func (c *Compiler) compileClosureExpr(e *ast.ClosureExpr) int {
	closureName := fmt.Sprintf("%s$%d", c.currentFuncName, c.closureID)
	c.closureID++

	paramNames := make([]string, 0, len(e.Params))
	for _, p := range e.Params {
		paramNames = append(paramNames, p.Name)
	}
	freeVars, mutated := analyzeFreeVars(e.Body, paramNames)

	// Filter free vars to those visible in outer scope
	captures := make([]string, 0, len(freeVars))
	mutatedCaptures := map[string]struct{}{}
	for _, name := range freeVars {
		if _, ok := c.lookupVar(name); ok {
			captures = append(captures, name)
			if _, ok := mutated[name]; ok {
				mutatedCaptures[name] = struct{}{}
			}
		}
	}

	// Save outer compiler state
	savedCurrent := c.current
	savedCurBlock := c.curBlock
	savedBlockID := c.blockID
	savedTempID := c.tempID
	savedScope := c.scopeStack
	savedNameCount := c.nameCount
	savedTempTypes := c.tempTypes
	savedLoopStack := c.loopStack
	savedFuncName := c.currentFuncName
	savedClosureID := c.closureID

	// Set up closure function compilation state
	irFn := &ir.Function{Name: closureName}
	c.current = irFn
	c.curBlock = nil
	c.blockID = 0
	c.tempID = 0
	c.scopeStack = nil
	c.nameCount = map[string]int{}
	c.tempTypes = map[int]string{}
	c.loopStack = nil
	c.currentFuncName = closureName
	c.closureID = 0

	// Parameters: always include $env first
	c.pushScope()
	envTemp := c.newTemp()
	c.declareValueVar("$env", envTemp)
	irFn.Params = append(irFn.Params, ir.Var{Name: "$env", Type: "$Env"})

	for _, p := range e.Params {
		paramTemp := c.newTemp()
		c.declareValueVar(p.Name, paramTemp)
		paramType := "i64"
		if p.Type != nil {
			paramType = formatType(*p.Type)
		}
		irFn.Params = append(irFn.Params, ir.Var{Name: p.Name, Type: paramType})
	}

	if e.ReturnType != nil {
		irFn.ReturnType = formatType(*e.ReturnType)
	} else {
		irFn.ReturnType = "unit"
	}

	entry := c.newBlock("entry")
	c.setCurrentBlock(entry)

	// Unpack captured variables from env
	for _, name := range captures {
		dst := c.newTemp()
		c.setTempType(dst, "i64")
		c.emit(&ir.GetField{Dst: dst, Src: envTemp, Field: name})
		if _, ok := mutatedCaptures[name]; ok {
			c.declareRefVar(name, dst, true)
		} else {
			c.declareValueVar(name, dst)
		}
	}

	bodyOp := c.compileOperand(e.Body)
	if c.currentBlock() != nil && c.currentBlock().Term == nil {
		c.emitTerm(&ir.Return{Value: &bodyOp})
	}

	c.prog.Functions[closureName] = irFn
	c.funcRetTypes[closureName] = irFn.ReturnType

	// Restore outer compiler state
	c.current = savedCurrent
	c.curBlock = savedCurBlock
	c.blockID = savedBlockID
	c.tempID = savedTempID
	c.scopeStack = savedScope
	c.nameCount = savedNameCount
	c.tempTypes = savedTempTypes
	c.loopStack = savedLoopStack
	c.currentFuncName = savedFuncName
	c.closureID = savedClosureID

	// Ensure Closure type is declared once
	if _, ok := c.prog.TypeDecls["Closure"]; !ok {
		c.prog.TypeDecls["Closure"] = &ir.TypeDecl{
			Name: "Closure",
			Fields: []ir.Var{
				{Name: "func", Type: "String"},
				{Name: "env", Type: "$Env"},
			},
		}
	}

	// Build env struct in outer function
	envFields := make([]ir.StructFieldInit, 0, len(captures))
	for _, name := range captures {
		varInfo, ok := c.lookupVar(name)
		if !ok {
			continue
		}
		if _, ok := mutatedCaptures[name]; ok {
			if !varInfo.Mutable {
				c.diag.Add(e.Span(), fmt.Sprintf("cannot capture immutable variable '%s' by mutable closure", name))
				continue
			}
			addrTemp := c.newTemp()
			c.setTempType(addrTemp, "*i64")
			c.emit(&ir.LoadVar{Dst: addrTemp, Name: varInfo.Name, Addr: true})
			envFields = append(envFields, ir.StructFieldInit{Name: name, Src: ir.TempOperand(addrTemp)})
		} else if varInfo.Temp >= 0 {
			envFields = append(envFields, ir.StructFieldInit{Name: name, Src: ir.TempOperand(varInfo.Temp)})
		} else {
			valTemp := c.newTemp()
			c.setTempType(valTemp, "i64")
			c.emit(&ir.LoadVar{Dst: valTemp, Name: varInfo.Name})
			envFields = append(envFields, ir.StructFieldInit{Name: name, Src: ir.TempOperand(valTemp)})
		}
	}
	envTempOuter := c.newTemp()
	c.setTempType(envTempOuter, "$Env")
	c.emit(&ir.MakeStruct{Dst: envTempOuter, Name: "$Env", Fields: envFields})

	closureFields := []ir.StructFieldInit{
		{Name: "func", Src: ir.StringOperand(closureName)},
		{Name: "env", Src: ir.TempOperand(envTempOuter)},
	}
	closureTemp := c.newTemp()
	c.setTempType(closureTemp, "Closure")
	c.emit(&ir.MakeStruct{Dst: closureTemp, Name: "Closure", Fields: closureFields})
	return closureTemp
}
