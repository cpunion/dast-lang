package compile

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/ir"
)

func (c *Compiler) compileStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		c.compileLet(s)
	case *ast.LetPatternStmt:
		c.compileLetPattern(s)
	case *ast.AssignStmt:
		c.compileAssign(s)
	case *ast.ExprStmt:
		op := c.compileOperandMove(s.Expr)
		typ := c.inferExprType(s.Expr)
		c.emitDropCall(op, typ)
	case *ast.ReturnStmt:
		c.compileReturn(s)
	case *ast.IfStmt:
		c.compileIf(s)
	case *ast.IfLetStmt:
		c.compileIfLet(s)
	case *ast.WhileStmt:
		c.compileWhile(s)
	case *ast.WhileLetStmt:
		c.compileWhileLet(s)
	case *ast.MatchStmt:
		c.compileMatch(s)
	case *ast.LoopStmt:
		c.compileLoop(s)
	case *ast.ForStmt:
		c.compileFor(s)
	case *ast.BreakStmt:
		c.compileBreak(s)
	case *ast.ContinueStmt:
		c.compileContinue(s)
	case *ast.Block:
		c.compileBlock(s)
	default:
		c.diag.Add(stmt.Span(), "unsupported statement in stage 0")
	}
}

func (c *Compiler) compileLet(s *ast.LetStmt) {
	if s.Init == nil {
		c.diag.Add(s.Span(), "let requires initializer in stage 0")
		return
	}
	isClosure := false
	if _, ok := s.Init.(*ast.ClosureExpr); ok {
		isClosure = true
	}
	typ := ""
	if s.Type != nil {
		typ = formatType(*s.Type)
	} else {
		typ = c.inferExprType(s.Init)
	}
	// All let variables use named storage (so they can be referenced with &)
	// The only difference between let and let mut is whether reassignment is allowed
	name := c.declareMutVar(s.Name, typ)
	// Track mutability separately for &mut checks
	if !s.Mutable {
		// Mark as immutable in scope (declareMutVar sets Mutable=true, we need to fix it)
		c.markImmutable(s.Name)
	}
	op := c.compileOperandMove(s.Init)
	borrowed := c.operandBorrowed(op)
	c.emit(&ir.StoreVar{Name: name, Src: op})
	c.updateVar(s.Name, func(v *VarInfo) {
		v.Borrowed = borrowed
		v.Moved = false
	})
	if isClosure {
		c.markClosure(s.Name)
	}
}

func (c *Compiler) compileLetPattern(s *ast.LetPatternStmt) {
	if s.Init == nil {
		c.diag.Add(s.Span(), "let requires initializer in stage 0")
		return
	}
	scrut := c.compileExpr(s.Init)
	scrutType := c.tempTypes[scrut]
	if scrutType == "" {
		scrutType = c.inferExprType(s.Init)
	}
	scrutTemp := !isLvalueExpr(s.Init)
	bindsDroppable := c.patternBindsDroppable(scrutType, s.Pattern)
	moveScrut := scrutTemp || bindsDroppable
	if !scrutTemp && bindsDroppable {
		c.markMovedExpr(s.Init)
	}
	cond, always := c.compilePatternCond(scrut, s.Pattern)
	if always {
		c.bindPattern(scrut, s.Pattern, false, moveScrut)
		if !scrutTemp && moveScrut {
			c.emitClearLvalue(s.Init)
		}
		if scrutTemp {
			if bindsDroppable {
				c.emitShallowFree(ir.TempOperand(scrut), scrutType)
			} else {
				c.emitDropCall(ir.TempOperand(scrut), scrutType)
			}
		}
		return
	}
	thenBlock := c.newBlock("let_pat")
	elseBlock := c.newBlock("let_pat_fail")
	afterBlock := c.newBlock("let_pat_after")

	c.emitTerm(&ir.Branch{Cond: cond, Then: thenBlock.Label, Else: elseBlock.Label})

	c.setCurrentBlock(thenBlock)
	c.bindPattern(scrut, s.Pattern, false, moveScrut)
	if !scrutTemp && moveScrut {
		c.emitClearLvalue(s.Init)
	}
	if scrutTemp {
		if bindsDroppable {
			c.emitShallowFree(ir.TempOperand(scrut), scrutType)
		} else {
			c.emitDropCall(ir.TempOperand(scrut), scrutType)
		}
	}
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: afterBlock.Label})
	}

	c.setCurrentBlock(elseBlock)
	if scrutTemp {
		c.emitDropCall(ir.TempOperand(scrut), scrutType)
	}
	c.emit(&ir.Call{Dst: -1, Callee: "exit", Args: []ir.Operand{ir.IntOperand(1)}})
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Return{Value: nil})
	}

	c.setCurrentBlock(afterBlock)
}

func (c *Compiler) compileAssign(s *ast.AssignStmt) {
	switch target := s.Target.(type) {
	case *ast.IdentExpr:
		varInfo, ok := c.lookupVar(target.Name)
		if !ok {
			if _, ok := c.consts[target.Name]; ok {
				c.diag.Add(s.Span(), fmt.Sprintf("cannot assign to const '%s'", target.Name))
			} else {
				c.diag.Add(s.Span(), fmt.Sprintf("undefined variable '%s'", target.Name))
			}
			return
		}
		if varInfo.RefTemp >= 0 {
			val := c.compileOperandMove(s.Value)
			c.emit(&ir.StoreVar{Ref: true, RefTemp: varInfo.RefTemp, Src: val})
			return
		}
		if !varInfo.Mutable {
			c.diag.Add(s.Span(), fmt.Sprintf("cannot assign to immutable variable '%s'", target.Name))
			return
		}
		val := c.compileOperandMove(s.Value)
		borrowed := c.operandBorrowed(val)
		if curInfo, ok := c.lookupVar(target.Name); ok {
			if !curInfo.Moved {
				c.emitDropForVar(curInfo)
			}
		}
		c.emit(&ir.StoreVar{Name: varInfo.Name, Src: val})
		c.updateVar(target.Name, func(v *VarInfo) {
			v.Moved = false
			v.Borrowed = borrowed
		})
	case *ast.DerefExpr:
		refTemp := c.compileExpr(target.Expr)
		val := c.compileOperandMove(s.Value)
		c.emit(&ir.StoreVar{Ref: true, RefTemp: refTemp, Src: val})
	case *ast.IndexExpr:
		arrayTemp := c.compileOperandBorrow(target.Receiver)
		indexTemp := c.compileOperandBorrow(target.Index)
		val := c.compileOperandMove(s.Value)
		c.emit(&ir.SetIndex{Array: arrayTemp, Index: indexTemp, Src: val})
	case *ast.AccessExpr:
		recv := c.compileExpr(target.Receiver)
		val := c.compileOperandMove(s.Value)
		c.emit(&ir.SetField{Src: recv, Field: target.Field, Value: val})
	default:
		c.diag.Add(s.Span(), "invalid assignment target")
	}
}

func (c *Compiler) compileReturn(s *ast.ReturnStmt) {
	if s.Value == nil {
		c.emitDropsFromDepth(0)
		c.emitTerm(&ir.Return{Value: nil})
		return
	}
	val := c.compileOperandMove(s.Value)
	c.emitDropsFromDepth(0)
	c.emitTerm(&ir.Return{Value: &val})
}

func (c *Compiler) compileIf(s *ast.IfStmt) {
	cond := c.compileOperandBorrow(s.Cond)
	thenBlock := c.newBlock("then")
	elseBlock := c.newBlock("else")
	mergeBlock := c.newBlock("merge")

	c.emitTerm(&ir.Branch{Cond: cond, Then: thenBlock.Label, Else: elseBlock.Label})

	c.setCurrentBlock(thenBlock)
	base := c.snapshotScopes()
	c.compileBlock(s.Then)
	thenReturns := termIsReturn(c.currentBlock().Term)
	thenState := c.captureVarState(base)
	c.restoreScopes(base)
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}

	c.setCurrentBlock(elseBlock)
	if s.Else != nil {
		c.compileBlock(s.Else)
	}
	elseReturns := termIsReturn(c.currentBlock().Term)
	elseState := c.captureVarState(base)
	c.restoreScopes(base)
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}

	// Merge variable state from branches that reach the merge block.
	mergeState := map[string]varState{}
	for i := 0; i < len(base); i++ {
		for name, info := range base[i].vars {
			key := fmt.Sprintf("%d:%s", i, name)
			moved := info.Moved
			borrowed := info.Borrowed
			if !thenReturns && !elseReturns {
				moved = thenState[key].Moved || elseState[key].Moved
				borrowed = thenState[key].Borrowed || elseState[key].Borrowed
			} else if !thenReturns {
				moved = thenState[key].Moved
				borrowed = thenState[key].Borrowed
			} else if !elseReturns {
				moved = elseState[key].Moved
				borrowed = elseState[key].Borrowed
			}
			mergeState[key] = varState{Moved: moved, Borrowed: borrowed}
		}
	}
	c.applyVarState(base, mergeState)

	c.setCurrentBlock(mergeBlock)
}

func (c *Compiler) compileIfLet(s *ast.IfLetStmt) {
	scrut := c.compileExpr(s.Expr)
	scrutType := c.tempTypes[scrut]
	if scrutType == "" {
		scrutType = c.inferExprType(s.Expr)
	}
	scrutTemp := !isLvalueExpr(s.Expr)
	armMove := c.patternBindsDroppable(scrutType, s.Pattern)
	moveScrut := scrutTemp || armMove
	if !scrutTemp && armMove {
		c.markMovedExpr(s.Expr)
	}
	thenBlock := c.newBlock("iflet_then")
	elseBlock := c.newBlock("iflet_else")
	mergeBlock := c.newBlock("iflet_merge")
	base := c.snapshotScopes()

	switch p := s.Pattern.(type) {
	case *ast.WildcardPattern:
		c.emitTerm(&ir.Jump{Target: thenBlock.Label})
	case *ast.VariantPattern:
		enumName := p.EnumName
		if enumName == "" {
			var ok bool
			enumName, ok = c.resolveEnumName(p.Variant)
			if !ok {
				c.diag.Add(p.Span(), fmt.Sprintf("ambiguous or unknown variant '%s'", p.Variant))
				return
			}
		}
		tagVal, tagType, ok := c.enumTagInfo(enumName, p.Variant)
		if !ok {
			c.diag.Add(p.Span(), fmt.Sprintf("unknown enum variant '%s.%s'", enumName, p.Variant))
			return
		}
		tag := c.newTemp()
		c.emit(&ir.GetField{Dst: tag, Src: scrut, Field: "_tag"})
		cmp := c.newTemp()
		c.emit(&ir.BinOp{Dst: cmp, Op: "==", Lhs: ir.TempOperand(tag), Rhs: ir.ConstOperand(ir.Value{Kind: ir.KindInt, Int: tagVal, IntType: tagType})})
		c.emitTerm(&ir.Branch{Cond: ir.TempOperand(cmp), Then: thenBlock.Label, Else: elseBlock.Label})
	default:
		c.diag.Add(s.Span(), "unsupported pattern in stage 0")
		return
	}

	c.setCurrentBlock(thenBlock)
	arm := ast.MatchArm{Pattern: s.Pattern, Body: s.Then, SpanInfo: s.Then.Span()}
	c.compileMatchArm(&arm, scrut, moveScrut, elseBlock.Label)
	thenReturns := termIsReturn(c.currentBlock().Term)
	thenState := c.captureVarState(base)
	c.restoreScopes(base)
	if c.currentBlock().Term == nil {
		if !scrutTemp && moveScrut {
			c.emitClearLvalue(s.Expr)
		}
		if scrutTemp {
			if armMove {
				c.emitShallowFree(ir.TempOperand(scrut), scrutType)
			} else {
				c.emitDropCall(ir.TempOperand(scrut), scrutType)
			}
		}
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}

	c.setCurrentBlock(elseBlock)
	if s.Else != nil {
		c.compileBlock(s.Else)
	}
	elseReturns := termIsReturn(c.currentBlock().Term)
	elseState := c.captureVarState(base)
	c.restoreScopes(base)
	if c.currentBlock().Term == nil {
		if !scrutTemp && moveScrut {
			c.emitClearLvalue(s.Expr)
		}
		if scrutTemp {
			c.emitDropCall(ir.TempOperand(scrut), scrutType)
		}
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}

	mergeState := map[string]varState{}
	for i := 0; i < len(base); i++ {
		for name, info := range base[i].vars {
			key := fmt.Sprintf("%d:%s", i, name)
			moved := info.Moved
			borrowed := info.Borrowed
			if !thenReturns && !elseReturns {
				moved = thenState[key].Moved || elseState[key].Moved
				borrowed = thenState[key].Borrowed || elseState[key].Borrowed
			} else if !thenReturns {
				moved = thenState[key].Moved
				borrowed = thenState[key].Borrowed
			} else if !elseReturns {
				moved = elseState[key].Moved
				borrowed = elseState[key].Borrowed
			}
			mergeState[key] = varState{Moved: moved, Borrowed: borrowed}
		}
	}
	c.applyVarState(base, mergeState)

	c.setCurrentBlock(mergeBlock)
}

func (c *Compiler) compileTailIf(s *ast.IfStmt, allowImplicit bool) {
	if s.Else == nil {
		c.compileIf(s)
		return
	}
	cond := c.compileOperandBorrow(s.Cond)
	thenBlock := c.newBlock("then")
	elseBlock := c.newBlock("else")
	mergeBlock := c.newBlock("merge")

	c.emitTerm(&ir.Branch{Cond: cond, Then: thenBlock.Label, Else: elseBlock.Label})

	c.setCurrentBlock(thenBlock)
	c.compileBlockWithTail(s.Then, allowImplicit)
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}

	c.setCurrentBlock(elseBlock)
	if s.Else != nil {
		c.compileBlockWithTail(s.Else, allowImplicit)
	}
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}

	c.setCurrentBlock(mergeBlock)
}

func (c *Compiler) compileWhile(s *ast.WhileStmt) {
	condBlock := c.newBlock("cond")
	bodyBlock := c.newBlock("body")
	afterBlock := c.newBlock("after")

	c.emitTerm(&ir.Jump{Target: condBlock.Label})

	c.setCurrentBlock(condBlock)
	cond := c.compileOperandBorrow(s.Cond)
	c.emitTerm(&ir.Branch{Cond: cond, Then: bodyBlock.Label, Else: afterBlock.Label})

	c.setCurrentBlock(bodyBlock)
	c.pushLoop(afterBlock.Label, condBlock.Label, s.Label, "")
	c.compileBlock(s.Body)
	c.popLoop()
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: condBlock.Label})
	}

	c.setCurrentBlock(afterBlock)
}

func (c *Compiler) compileWhileLet(s *ast.WhileLetStmt) {
	condBlock := c.newBlock("whilelet_cond")
	bodyBlock := c.newBlock("whilelet_body")
	afterBlock := c.newBlock("whilelet_after")

	c.emitTerm(&ir.Jump{Target: condBlock.Label})

	c.setCurrentBlock(condBlock)
	scrut := c.compileExpr(s.Expr)
	scrutType := c.tempTypes[scrut]
	if scrutType == "" {
		scrutType = c.inferExprType(s.Expr)
	}
	scrutTemp := !isLvalueExpr(s.Expr)
	armMove := c.patternBindsDroppable(scrutType, s.Pattern)
	moveScrut := scrutTemp || armMove
	if !scrutTemp && armMove {
		c.markMovedExpr(s.Expr)
	}
	switch p := s.Pattern.(type) {
	case *ast.WildcardPattern:
		c.emitTerm(&ir.Jump{Target: bodyBlock.Label})
	case *ast.VariantPattern:
		enumName := p.EnumName
		if enumName == "" {
			var ok bool
			enumName, ok = c.resolveEnumName(p.Variant)
			if !ok {
				c.diag.Add(p.Span(), fmt.Sprintf("ambiguous or unknown variant '%s'", p.Variant))
				return
			}
		}
		tagVal, tagType, ok := c.enumTagInfo(enumName, p.Variant)
		if !ok {
			c.diag.Add(p.Span(), fmt.Sprintf("unknown enum variant '%s.%s'", enumName, p.Variant))
			return
		}
		tag := c.newTemp()
		c.emit(&ir.GetField{Dst: tag, Src: scrut, Field: "_tag"})
		cmp := c.newTemp()
		c.emit(&ir.BinOp{Dst: cmp, Op: "==", Lhs: ir.TempOperand(tag), Rhs: ir.ConstOperand(ir.Value{Kind: ir.KindInt, Int: tagVal, IntType: tagType})})
		c.emitTerm(&ir.Branch{Cond: ir.TempOperand(cmp), Then: bodyBlock.Label, Else: afterBlock.Label})
	default:
		c.diag.Add(s.Span(), "unsupported pattern in stage 0")
		return
	}

	c.setCurrentBlock(bodyBlock)
	arm := ast.MatchArm{Pattern: s.Pattern, Body: s.Body, SpanInfo: s.Body.Span()}
	c.pushLoop(afterBlock.Label, condBlock.Label, s.Label, "")
	c.compileMatchArm(&arm, scrut, moveScrut, afterBlock.Label)
	c.popLoop()
	if c.currentBlock().Term == nil {
		if !scrutTemp && moveScrut {
			c.emitClearLvalue(s.Expr)
		}
		if scrutTemp {
			if armMove {
				c.emitShallowFree(ir.TempOperand(scrut), scrutType)
			} else {
				c.emitDropCall(ir.TempOperand(scrut), scrutType)
			}
		}
		c.emitTerm(&ir.Jump{Target: condBlock.Label})
	}

	c.setCurrentBlock(afterBlock)
	if !scrutTemp && moveScrut {
		c.emitClearLvalue(s.Expr)
	}
	if scrutTemp {
		c.emitDropCall(ir.TempOperand(scrut), scrutType)
	}
}

func (c *Compiler) compileLoop(s *ast.LoopStmt) {
	bodyBlock := c.newBlock("loop_body")
	afterBlock := c.newBlock("loop_after")

	c.emitTerm(&ir.Jump{Target: bodyBlock.Label})

	c.setCurrentBlock(bodyBlock)
	c.pushLoop(afterBlock.Label, bodyBlock.Label, s.Label, "")
	c.compileBlock(s.Body)
	c.popLoop()
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: bodyBlock.Label})
	}

	c.setCurrentBlock(afterBlock)
}

func (c *Compiler) compileLoopExpr(e *ast.LoopExpr) int {
	resultType := c.inferExprType(e)
	bodyBlock := c.newBlock("loop_body")
	afterBlock := c.newBlock("loop_after")

	var breakValueName string
	var breakValueKey string
	if resultType != "unit" {
		breakValueKey = fmt.Sprintf("$loop_value%d", c.tempID)
		breakValueName = c.declareMutVar(breakValueKey, resultType)
	}

	c.emitTerm(&ir.Jump{Target: bodyBlock.Label})

	c.setCurrentBlock(bodyBlock)
	c.pushLoop(afterBlock.Label, bodyBlock.Label, "", breakValueName)
	c.compileBlock(e.Body)
	c.popLoop()
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: bodyBlock.Label})
	}

	c.setCurrentBlock(afterBlock)
	if resultType == "unit" {
		return c.operandToTemp(ir.ConstOperand(ir.Value{Kind: ir.KindUnit}), "unit")
	}
	dst := c.newTemp()
	c.setTempType(dst, resultType)
	c.emit(&ir.LoadVar{Dst: dst, Name: breakValueName})
	if c.needsDropType(resultType) && !isCopyTypeName(resultType) && !isRefTypeName(resultType) {
		c.updateVar(breakValueKey, func(v *VarInfo) { v.Moved = true })
	}
	return dst
}

func (c *Compiler) compileFor(s *ast.ForStmt) {
	// Create an internal scope for loop temporaries
	c.pushScope()
	iterType := c.inferExprType(s.Expr)
	elemType := "i64"
	if isArrayTypeName(derefTypeName(iterType)) {
		if t := arrayElemTypeName(derefTypeName(iterType)); t != "" {
			elemType = t
		}
	}
	iterName := c.declareMutVar("__for_iter", iterType)
	iterOp := c.compileOperandMove(s.Expr)
	c.emit(&ir.StoreVar{Name: iterName, Src: iterOp})

	lenName := c.declareMutVar("__for_len", "int")
	lenTemp := c.newTemp()
	c.setTempType(lenTemp, "int")
	c.emit(&ir.Call{Dst: lenTemp, Callee: "len", Args: []ir.Operand{ir.TempOperand(c.compileExpr(&ast.IdentExpr{Name: "__for_iter", SpanInfo: s.Span()}))}})
	c.emit(&ir.StoreVar{Name: lenName, Src: ir.TempOperand(lenTemp)})

	idxName := c.declareMutVar("__for_i", "int")
	c.emit(&ir.StoreVar{Name: idxName, Src: ir.IntOperand(0)})

	condBlock := c.newBlock("for_cond")
	bodyBlock := c.newBlock("for_body")
	stepBlock := c.newBlock("for_step")
	afterBlock := c.newBlock("for_after")

	c.emitTerm(&ir.Jump{Target: condBlock.Label})

	c.setCurrentBlock(condBlock)
	idxTemp := c.newTemp()
	c.setTempType(idxTemp, "int")
	c.emit(&ir.LoadVar{Dst: idxTemp, Name: idxName})
	lenTemp2 := c.newTemp()
	c.setTempType(lenTemp2, "int")
	c.emit(&ir.LoadVar{Dst: lenTemp2, Name: lenName})
	condTemp := c.newTemp()
	c.setTempType(condTemp, "bool")
	c.emit(&ir.BinOp{Dst: condTemp, Op: "<", Lhs: ir.TempOperand(idxTemp), Rhs: ir.TempOperand(lenTemp2)})
	c.emitTerm(&ir.Branch{Cond: ir.TempOperand(condTemp), Then: bodyBlock.Label, Else: afterBlock.Label})

	c.setCurrentBlock(bodyBlock)
	c.pushLoop(afterBlock.Label, stepBlock.Label, s.Label, "")
	c.pushScope()
	iterTemp := c.newTemp()
	c.setTempType(iterTemp, iterType)
	c.emit(&ir.LoadVar{Dst: iterTemp, Name: iterName})
	idxTempBody := c.newTemp()
	c.setTempType(idxTempBody, "int")
	c.emit(&ir.LoadVar{Dst: idxTempBody, Name: idxName})
	elemTemp := c.newTemp()
	c.setTempType(elemTemp, elemType)
	c.emit(&ir.Index{Dst: elemTemp, Array: ir.TempOperand(iterTemp), Index: ir.TempOperand(idxTempBody)})
	c.bindPattern(elemTemp, s.Pattern, false, false)
	for _, stmt := range s.Body.Stmts {
		if c.currentBlock().Term != nil {
			break
		}
		c.compileStmt(stmt)
	}
	c.popScope()
	c.popLoop()
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: stepBlock.Label})
	}

	c.setCurrentBlock(stepBlock)
	incTemp := c.newTemp()
	c.setTempType(incTemp, "int")
	idxTempStep := c.newTemp()
	c.setTempType(idxTempStep, "int")
	c.emit(&ir.LoadVar{Dst: idxTempStep, Name: idxName})
	c.emit(&ir.BinOp{Dst: incTemp, Op: "+", Lhs: ir.TempOperand(idxTempStep), Rhs: ir.IntOperand(1)})
	c.emit(&ir.StoreVar{Name: idxName, Src: ir.TempOperand(incTemp)})
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: condBlock.Label})
	}

	c.setCurrentBlock(afterBlock)
	c.popScope()
}

func (c *Compiler) compileBreak(s *ast.BreakStmt) {
	loop, ok := c.findLoop(s.Label)
	if !ok {
		c.diag.Add(s.Span(), "break outside of loop")
		return
	}
	if s.Value != nil {
		val := c.compileOperandMove(s.Value)
		if loop.breakValueName == "" {
			c.diag.Add(s.Span(), "break value not allowed in this loop")
		} else {
			c.emit(&ir.StoreVar{Name: loop.breakValueName, Src: val})
		}
	}
	c.emitDropsFromDepth(loop.scopeDepth)
	c.emitTerm(&ir.Jump{Target: loop.breakLabel})
}

func (c *Compiler) compileContinue(s *ast.ContinueStmt) {
	loop, ok := c.findLoop(s.Label)
	if !ok {
		c.diag.Add(s.Span(), "continue outside of loop")
		return
	}
	c.emitDropsFromDepth(loop.scopeDepth)
	c.emitTerm(&ir.Jump{Target: loop.continueLabel})
}

func (c *Compiler) compileMatch(s *ast.MatchStmt) {
	scrut := c.compileExpr(s.Expr)
	scrutType := c.tempTypes[scrut]
	if scrutType == "" {
		scrutType = c.inferExprType(s.Expr)
	}
	scrutTemp := !isLvalueExpr(s.Expr)
	anyMove := false
	for _, arm := range s.Arms {
		if c.patternBindsDroppable(scrutType, arm.Pattern) {
			anyMove = true
			break
		}
	}
	moveScrut := scrutTemp || anyMove
	if !scrutTemp && anyMove {
		c.markMovedExpr(s.Expr)
	}
	after := c.newBlock("match_after")

	for i, arm := range s.Arms {
		armBlock := c.newBlock("match_arm")
		next := c.newBlock("match_next")
		cond, always := c.compilePatternCond(scrut, arm.Pattern)
		if always {
			c.emitTerm(&ir.Jump{Target: armBlock.Label})
		} else {
			c.emitTerm(&ir.Branch{Cond: cond, Then: armBlock.Label, Else: next.Label})
		}
		c.setCurrentBlock(armBlock)
		armMove := c.patternBindsDroppable(scrutType, arm.Pattern)
		c.compileMatchArm(&arm, scrut, moveScrut, next.Label)
		if c.currentBlock().Term == nil {
			if !scrutTemp && moveScrut {
				c.emitClearLvalue(s.Expr)
			}
			if scrutTemp {
				if armMove {
					c.emitShallowFree(ir.TempOperand(scrut), scrutType)
				} else {
					c.emitDropCall(ir.TempOperand(scrut), scrutType)
				}
			}
			c.emitTerm(&ir.Jump{Target: after.Label})
		}
		c.setCurrentBlock(next)
		if i == len(s.Arms)-1 {
			c.emitTerm(&ir.Jump{Target: after.Label})
		}
	}
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: after.Label})
	}
	c.setCurrentBlock(after)
}

func (c *Compiler) compileTailMatch(s *ast.MatchStmt, allowImplicit bool) {
	scrut := c.compileExpr(s.Expr)
	scrutType := c.tempTypes[scrut]
	if scrutType == "" {
		scrutType = c.inferExprType(s.Expr)
	}
	scrutTemp := !isLvalueExpr(s.Expr)
	anyMove := false
	for _, arm := range s.Arms {
		if c.patternBindsDroppable(scrutType, arm.Pattern) {
			anyMove = true
			break
		}
	}
	moveScrut := scrutTemp || anyMove
	if !scrutTemp && anyMove {
		c.markMovedExpr(s.Expr)
	}
	after := c.newBlock("match_after")

	for i, arm := range s.Arms {
		armBlock := c.newBlock("match_arm")
		next := c.newBlock("match_next")
		cond, always := c.compilePatternCond(scrut, arm.Pattern)
		if always {
			c.emitTerm(&ir.Jump{Target: armBlock.Label})
		} else {
			c.emitTerm(&ir.Branch{Cond: cond, Then: armBlock.Label, Else: next.Label})
		}
		c.setCurrentBlock(armBlock)
		armMove := c.patternBindsDroppable(scrutType, arm.Pattern)
		c.compileMatchArmTail(&arm, scrut, moveScrut, allowImplicit, next.Label)
		if c.currentBlock().Term == nil {
			if !scrutTemp && moveScrut {
				c.emitClearLvalue(s.Expr)
			}
			if scrutTemp {
				if armMove {
					c.emitShallowFree(ir.TempOperand(scrut), scrutType)
				} else {
					c.emitDropCall(ir.TempOperand(scrut), scrutType)
				}
			}
			c.emitTerm(&ir.Jump{Target: after.Label})
		}
		c.setCurrentBlock(next)
		if i == len(s.Arms)-1 {
			c.emitTerm(&ir.Jump{Target: after.Label})
		}
	}
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: after.Label})
	}
	c.setCurrentBlock(after)
}

func (c *Compiler) compileIfExpr(e *ast.IfExpr) int {
	cond := c.compileOperandBorrow(e.Cond)
	thenBlock := c.newBlock("if_then")
	elseBlock := c.newBlock("if_else")
	mergeBlock := c.newBlock("if_merge")

	dstName := c.declareMutVar("__if", c.inferExprType(e))
	base := c.snapshotScopes()
	c.emitTerm(&ir.Branch{Cond: cond, Then: thenBlock.Label, Else: elseBlock.Label})

	c.setCurrentBlock(thenBlock)
	thenOp := c.compileOperandMove(e.Then)
	if c.currentBlock().Term == nil {
		borrowed := c.operandBorrowed(thenOp)
		c.emit(&ir.StoreVar{Name: dstName, Src: thenOp})
		c.updateVar("__if", func(v *VarInfo) {
			v.Borrowed = borrowed
			v.Moved = false
		})
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}
	thenReturns := termIsReturn(c.currentBlock().Term)
	thenState := c.captureVarState(base)
	c.restoreScopes(base)

	c.setCurrentBlock(elseBlock)
	elseOp := ir.ConstOperand(ir.Value{Kind: ir.KindUnit})
	if e.Else != nil {
		elseOp = c.compileOperandMove(e.Else)
	}
	if c.currentBlock().Term == nil {
		borrowed := c.operandBorrowed(elseOp)
		c.emit(&ir.StoreVar{Name: dstName, Src: elseOp})
		c.updateVar("__if", func(v *VarInfo) {
			v.Borrowed = borrowed
			v.Moved = false
		})
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}
	elseReturns := termIsReturn(c.currentBlock().Term)
	elseState := c.captureVarState(base)
	c.restoreScopes(base)

	mergeState := map[string]varState{}
	for i := 0; i < len(base); i++ {
		for name, info := range base[i].vars {
			key := fmt.Sprintf("%d:%s", i, name)
			moved := info.Moved
			borrowed := info.Borrowed
			if !thenReturns && !elseReturns {
				moved = thenState[key].Moved || elseState[key].Moved
				borrowed = thenState[key].Borrowed || elseState[key].Borrowed
			} else if !thenReturns {
				moved = thenState[key].Moved
				borrowed = thenState[key].Borrowed
			} else if !elseReturns {
				moved = elseState[key].Moved
				borrowed = elseState[key].Borrowed
			}
			mergeState[key] = varState{Moved: moved, Borrowed: borrowed}
		}
	}
	c.applyVarState(base, mergeState)

	c.setCurrentBlock(mergeBlock)
	dst := c.newTemp()
	c.setTempType(dst, c.inferExprType(e))
	c.emit(&ir.LoadVar{Dst: dst, Name: dstName})
	if info, ok := c.lookupVar("__if"); ok {
		c.setTempBorrowed(dst, info.Borrowed)
	}
	return dst
}

func (c *Compiler) compileMatchExpr(e *ast.MatchExpr) int {
	scrut := c.compileExpr(e.Expr)
	scrutType := c.tempTypes[scrut]
	if scrutType == "" {
		scrutType = c.inferExprType(e.Expr)
	}
	scrutTemp := !isLvalueExpr(e.Expr)
	anyMove := false
	for _, arm := range e.Arms {
		if c.patternBindsDroppable(scrutType, arm.Pattern) {
			anyMove = true
			break
		}
	}
	moveScrut := scrutTemp || anyMove
	if !scrutTemp && anyMove {
		c.markMovedExpr(e.Expr)
	}
	after := c.newBlock("match_after")
	dstName := c.declareMutVar("__match", c.inferExprType(e))
	c.emit(&ir.StoreVar{Name: dstName, Src: ir.ConstOperand(ir.Value{Kind: ir.KindUnit})})

	for i, arm := range e.Arms {
		armBlock := c.newBlock("match_arm")
		next := c.newBlock("match_next")
		cond, always := c.compilePatternCond(scrut, arm.Pattern)
		if always {
			c.emitTerm(&ir.Jump{Target: armBlock.Label})
		} else {
			c.emitTerm(&ir.Branch{Cond: cond, Then: armBlock.Label, Else: next.Label})
		}
		c.setCurrentBlock(armBlock)
		armMove := c.patternBindsDroppable(scrutType, arm.Pattern)
		op := c.compileMatchArmExpr(&arm, scrut, moveScrut, next.Label)
		if c.currentBlock().Term == nil {
			c.emit(&ir.StoreVar{Name: dstName, Src: op})
			if !scrutTemp && moveScrut {
				c.emitClearLvalue(e.Expr)
			}
			if scrutTemp {
				if armMove {
					c.emitShallowFree(ir.TempOperand(scrut), scrutType)
				} else {
					c.emitDropCall(ir.TempOperand(scrut), scrutType)
				}
			}
			c.emitTerm(&ir.Jump{Target: after.Label})
		}
		c.setCurrentBlock(next)
		if i == len(e.Arms)-1 {
			c.emitTerm(&ir.Jump{Target: after.Label})
		}
	}
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: after.Label})
	}
	c.setCurrentBlock(after)
	dst := c.newTemp()
	c.setTempType(dst, c.inferExprType(e))
	c.emit(&ir.LoadVar{Dst: dst, Name: dstName})
	return dst
}

func (c *Compiler) compileMatchArmExpr(arm *ast.MatchArm, scrut int, moveScrut bool, nextLabel string) ir.Operand {
	c.pushScope()
	c.bindPattern(scrut, arm.Pattern, false, moveScrut)
	if arm.Guard != nil {
		if nextLabel == "" {
			c.diag.Add(arm.Guard.Span(), "match guard requires next arm")
		} else {
			guardBlock := c.newBlock("match_guard")
			cond := c.compileOperandBorrow(arm.Guard)
			c.emitTerm(&ir.Branch{Cond: cond, Then: guardBlock.Label, Else: nextLabel})
			c.setCurrentBlock(guardBlock)
		}
	}
	op := c.compileBlockExprOperand(arm.Body)
	c.popScope()
	return op
}

func (c *Compiler) bindPattern(scrut int, pat ast.Pattern, mutable bool, moveScrut bool) {
	switch p := pat.(type) {
	case *ast.BindingPattern:
		if _, _, ok := c.constPatternInfo(p.Name); ok {
			return
		}
		typ := c.tempTypes[scrut]
		if typ == "" {
			typ = "i64"
		}
		name := c.declareMutVar(p.Name, typ)
		if !mutable {
			c.markImmutable(p.Name)
		}
		borrowed := false
		if !moveScrut {
			borrowed = c.operandBorrowed(ir.TempOperand(scrut))
		}
		c.emit(&ir.StoreVar{Name: name, Src: ir.TempOperand(scrut)})
		c.updateVar(p.Name, func(v *VarInfo) {
			v.Borrowed = borrowed
			v.Moved = false
		})
	case *ast.VariantPattern:
		payloadPat := p.Payload
		if payloadPat == nil && p.Binding != "" {
			payloadPat = &ast.BindingPattern{Name: p.Binding, SpanInfo: p.SpanInfo}
		}
		if payloadPat != nil {
			// Borrow the scrutinee when extracting payload to avoid
			// dropping the base enum while the payload is in use.
			if !moveScrut {
				c.setTempBorrowed(scrut, true)
				c.markTempBorrowedVar(scrut)
			}
			payload := c.newTemp()
			c.emit(&ir.GetField{Dst: payload, Src: scrut, Field: "_payload"})
			typ := "unit"
			enumName := p.EnumName
			if enumName == "" {
				if resolved, ok := c.resolveEnumName(p.Variant); ok {
					enumName = resolved
				}
			}
			if enumDecl, ok := c.enums[enumName]; ok {
				if v := enumVariant(enumDecl, p.Variant); v != nil && v.Payload != nil {
					typ = formatType(*v.Payload)
				}
			}
			c.setTempType(payload, typ)
			if !moveScrut {
				c.setTempBorrowed(payload, true)
			}
			c.bindPattern(payload, payloadPat, mutable, moveScrut)
		}
	case *ast.StructPattern:
		if !moveScrut {
			c.setTempBorrowed(scrut, true)
			c.markTempBorrowedVar(scrut)
		}
		for _, f := range p.Fields {
			if f.Pattern == nil {
				fieldTemp := c.newTemp()
				c.emit(&ir.GetField{Dst: fieldTemp, Src: scrut, Field: f.Name})
				if !moveScrut {
					c.setTempBorrowed(fieldTemp, true)
				}
				binding := f.Binding
				if binding == "" {
					binding = f.Name
				}
				typ := "i64"
				structName := p.StructName
				if structName == "" {
					structName = p.StructName
				}
				if decl, ok := c.structs[structName]; ok {
					if fd := findField(decl, f.Name); fd != nil {
						typ = formatType(fd.Type)
					}
				}
				name := c.declareMutVar(binding, typ)
				if !mutable {
					c.markImmutable(binding)
				}
				borrowed := false
				if !moveScrut {
					borrowed = c.operandBorrowed(ir.TempOperand(fieldTemp))
				}
				c.emit(&ir.StoreVar{Name: name, Src: ir.TempOperand(fieldTemp)})
				c.updateVar(binding, func(v *VarInfo) {
					v.Borrowed = borrowed
					v.Moved = false
				})
				continue
			}
			if c.patternHasBinding(f.Pattern) {
				fieldTemp := c.newTemp()
				c.emit(&ir.GetField{Dst: fieldTemp, Src: scrut, Field: f.Name})
				if !moveScrut {
					c.setTempBorrowed(fieldTemp, true)
				}
				c.bindPattern(fieldTemp, f.Pattern, mutable, moveScrut)
			}
		}
	case *ast.TuplePattern:
		if !moveScrut {
			c.setTempBorrowed(scrut, true)
			c.markTempBorrowedVar(scrut)
		}
		for i, el := range p.Elems {
			if !c.patternHasBinding(el) {
				continue
			}
			fieldTemp := c.newTemp()
			c.emit(&ir.GetField{Dst: fieldTemp, Src: scrut, Field: fmt.Sprintf("%d", i)})
			if !moveScrut {
				c.setTempBorrowed(fieldTemp, true)
			}
			c.bindPattern(fieldTemp, el, mutable, moveScrut)
		}
	case *ast.ArrayPattern:
		if !moveScrut {
			c.setTempBorrowed(scrut, true)
			c.markTempBorrowedVar(scrut)
		}
		elemType := "i64"
		arrType := derefTypeName(c.tempTypes[scrut])
		if isArrayTypeName(arrType) {
			if t := arrayElemTypeName(arrType); t != "" {
				elemType = t
			}
		}
		for i, el := range p.Elems {
			if !c.patternHasBinding(el) {
				continue
			}
			elemTemp := c.newTemp()
			c.setTempType(elemTemp, elemType)
			c.emit(&ir.Index{Dst: elemTemp, Array: ir.TempOperand(scrut), Index: ir.IntOperand(int64(i))})
			if !moveScrut {
				c.setTempBorrowed(elemTemp, true)
			}
			c.bindPattern(elemTemp, el, mutable, moveScrut)
		}
	case *ast.OrPattern:
		// bindings in or-patterns are not supported
	}
}

func (c *Compiler) patternHasBinding(pat ast.Pattern) bool {
	switch p := pat.(type) {
	case *ast.BindingPattern:
		if _, _, ok := c.constPatternInfo(p.Name); ok {
			return false
		}
		return true
	case *ast.VariantPattern:
		if p.Payload != nil {
			return c.patternHasBinding(p.Payload)
		}
		return p.Binding != ""
	case *ast.StructPattern:
		for _, f := range p.Fields {
			if f.Pattern == nil {
				return f.Binding != ""
			}
			if c.patternHasBinding(f.Pattern) {
				return true
			}
		}
		return false
	case *ast.OrPattern:
		for _, alt := range p.Alts {
			if c.patternHasBinding(alt) {
				return true
			}
		}
		return false
	case *ast.TuplePattern:
		for _, el := range p.Elems {
			if c.patternHasBinding(el) {
				return true
			}
		}
		return false
	case *ast.ArrayPattern:
		for _, el := range p.Elems {
			if c.patternHasBinding(el) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (c *Compiler) patternBindsDroppable(scrutType string, pat ast.Pattern) bool {
	scrutType = derefTypeName(scrutType)
	switch p := pat.(type) {
	case *ast.WildcardPattern, *ast.LiteralPattern, *ast.RangePattern:
		return false
	case *ast.BindingPattern:
		if _, _, ok := c.constPatternInfo(p.Name); ok {
			return false
		}
		return c.needsDropType(scrutType)
	case *ast.VariantPattern:
		payloadPat := p.Payload
		if payloadPat == nil && p.Binding != "" {
			payloadPat = &ast.BindingPattern{Name: p.Binding, SpanInfo: p.SpanInfo}
		}
		if payloadPat == nil {
			return false
		}
		enumName := p.EnumName
		if enumName == "" {
			enumName = scrutType
		}
		if enumName == "" {
			if resolved, ok := c.resolveEnumName(p.Variant); ok {
				enumName = resolved
			}
		}
		payloadType := ""
		if decl, ok := c.enums[enumName]; ok {
			if v := enumVariant(decl, p.Variant); v != nil && v.Payload != nil {
				payloadType = formatType(*v.Payload)
			}
		}
		if payloadType == "" {
			return true
		}
		return c.patternBindsDroppable(payloadType, payloadPat)
	case *ast.StructPattern:
		structName := p.StructName
		if structName == "" {
			structName = scrutType
		}
		if decl, ok := c.structs[structName]; ok {
			for _, f := range p.Fields {
				fd := findField(decl, f.Name)
				if fd == nil {
					return true
				}
				ft := formatType(fd.Type)
				if f.Pattern == nil {
					if c.needsDropType(ft) {
						return true
					}
					continue
				}
				if c.patternBindsDroppable(ft, f.Pattern) {
					return true
				}
			}
			return false
		}
		return true
	case *ast.TuplePattern:
		if c.prog.TypeDecls == nil {
			return true
		}
		td, ok := c.prog.TypeDecls[scrutType]
		if !ok || td == nil {
			return true
		}
		for i, el := range p.Elems {
			if i >= len(td.Fields) {
				return true
			}
			ft := td.Fields[i].Type
			if ft == "" {
				return true
			}
			if c.patternBindsDroppable(ft, el) {
				return true
			}
		}
		return false
	case *ast.ArrayPattern:
		elemType := arrayElemTypeName(scrutType)
		if elemType == "" {
			return true
		}
		for _, el := range p.Elems {
			if c.patternBindsDroppable(elemType, el) {
				return true
			}
		}
		return false
	case *ast.OrPattern:
		for _, alt := range p.Alts {
			if c.patternBindsDroppable(scrutType, alt) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func (c *Compiler) emitShallowFree(op ir.Operand, typ string) {
	typ = normalizeTypeName(typ)
	if isArrayTypeName(typ) {
		c.emit(&ir.Call{Dst: -1, Callee: "array_free", Args: []ir.Operand{op}})
		return
	}
	if c.isStructTypeName(typ) || c.isEnumTypeName(typ) {
		c.emit(&ir.Call{Dst: -1, Callee: "struct_free", Args: []ir.Operand{op}})
		return
	}
}

func isLvalueExpr(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.IdentExpr, *ast.AccessExpr, *ast.IndexExpr, *ast.DerefExpr:
		return true
	default:
		return false
	}
}

func (c *Compiler) emitClearLvalue(expr ast.Expr) {
	zero := ir.IntOperand(0)
	switch t := expr.(type) {
	case *ast.IdentExpr:
		if info, ok := c.lookupVar(t.Name); ok {
			c.emit(&ir.StoreVar{Name: info.Name, Src: zero})
		}
	case *ast.AccessExpr:
		recv := c.compileExpr(t.Receiver)
		c.emit(&ir.SetField{Src: recv, Field: t.Field, Value: zero})
	case *ast.IndexExpr:
		arrayTemp := c.compileOperandBorrow(t.Receiver)
		indexTemp := c.compileOperandBorrow(t.Index)
		c.emit(&ir.SetIndex{Array: arrayTemp, Index: indexTemp, Src: zero})
	case *ast.DerefExpr:
		refTemp := c.compileExpr(t.Expr)
		c.emit(&ir.StoreVar{Ref: true, RefTemp: refTemp, Src: zero})
	}
}

func (c *Compiler) constPatternInfo(name string) (ast.ConstValue, string, bool) {
	if info, ok := c.consts[name]; ok {
		return info.Value, info.TypeName, true
	}
	return ast.ConstValue{}, "", false
}

func (c *Compiler) compilePatternCond(scrut int, pat ast.Pattern) (ir.Operand, bool) {
	switch p := pat.(type) {
	case *ast.WildcardPattern:
		return ir.Operand{}, true
	case *ast.BindingPattern:
		if cv, typ, ok := c.constPatternInfo(p.Name); ok {
			lit := ir.ConstOperand(constValueToIr(cv, typ))
			cmp := c.newTemp()
			c.emit(&ir.BinOp{Dst: cmp, Op: "==", Lhs: ir.TempOperand(scrut), Rhs: lit})
			return ir.TempOperand(cmp), false
		}
		return ir.Operand{}, true
	case *ast.LiteralPattern:
		lit := ir.ConstOperand(constValueToIr(p.Value, ""))
		cmp := c.newTemp()
		c.emit(&ir.BinOp{Dst: cmp, Op: "==", Lhs: ir.TempOperand(scrut), Rhs: lit})
		return ir.TempOperand(cmp), false
	case *ast.RangePattern:
		hasStart := p.Start != nil
		hasEnd := p.End != nil
		if !hasStart && !hasEnd {
			return ir.Operand{}, true
		}
		var startCond ir.Operand
		if hasStart {
			startOp := c.compileOperandBorrow(p.Start)
			ge := c.newTemp()
			c.emit(&ir.BinOp{Dst: ge, Op: ">=", Lhs: ir.TempOperand(scrut), Rhs: startOp})
			startCond = ir.TempOperand(ge)
		}
		var endCond ir.Operand
		if hasEnd {
			endOp := c.compileOperandBorrow(p.End)
			op := "<"
			if p.Inclusive {
				op = "<="
			}
			le := c.newTemp()
			c.emit(&ir.BinOp{Dst: le, Op: op, Lhs: ir.TempOperand(scrut), Rhs: endOp})
			endCond = ir.TempOperand(le)
		}
		if hasStart && hasEnd {
			both := c.newTemp()
			c.emit(&ir.BinOp{Dst: both, Op: "&&", Lhs: startCond, Rhs: endCond})
			return ir.TempOperand(both), false
		}
		if hasStart {
			return startCond, false
		}
		return endCond, false
	case *ast.VariantPattern:
		enumName := p.EnumName
		if enumName == "" {
			var ok bool
			enumName, ok = c.resolveEnumName(p.Variant)
			if !ok {
				c.diag.Add(p.Span(), fmt.Sprintf("ambiguous or unknown variant '%s'", p.Variant))
				return ir.Operand{}, true
			}
		}
		tagVal, tagType, ok := c.enumTagInfo(enumName, p.Variant)
		if !ok {
			c.diag.Add(p.Span(), fmt.Sprintf("unknown enum variant '%s.%s'", enumName, p.Variant))
			return ir.Operand{}, true
		}
		tag := c.newTemp()
		c.emit(&ir.GetField{Dst: tag, Src: scrut, Field: "_tag"})
		cmp := c.newTemp()
		c.emit(&ir.BinOp{Dst: cmp, Op: "==", Lhs: ir.TempOperand(tag), Rhs: ir.ConstOperand(ir.Value{Kind: ir.KindInt, Int: tagVal, IntType: tagType})})
		payloadPat := p.Payload
		if payloadPat == nil && p.Binding != "" {
			payloadPat = &ast.BindingPattern{Name: p.Binding, SpanInfo: p.SpanInfo}
		}
		if payloadPat != nil {
			payload := c.newTemp()
			c.emit(&ir.GetField{Dst: payload, Src: scrut, Field: "_payload"})
			cond, always := c.compilePatternCond(payload, payloadPat)
			if !always {
				both := c.newTemp()
				c.emit(&ir.BinOp{Dst: both, Op: "&&", Lhs: ir.TempOperand(cmp), Rhs: cond})
				return ir.TempOperand(both), false
			}
		}
		return ir.TempOperand(cmp), false
	case *ast.StructPattern:
		return c.compileStructPatternCond(scrut, p)
	case *ast.TuplePattern:
		return c.compileTuplePatternCond(scrut, p)
	case *ast.ArrayPattern:
		return c.compileArrayPatternCond(scrut, p)
	case *ast.OrPattern:
		return c.compileOrPatternCond(scrut, p)
	default:
		c.diag.Add(pat.Span(), "unsupported pattern in stage 0")
		return ir.Operand{}, true
	}
}

func (c *Compiler) compileTuplePatternCond(scrut int, pat *ast.TuplePattern) (ir.Operand, bool) {
	if len(pat.Elems) == 0 {
		return ir.Operand{}, true
	}
	var cond ir.Operand
	hasCond := false
	for i, el := range pat.Elems {
		if _, ok := el.(*ast.WildcardPattern); ok {
			continue
		}
		fieldTemp := c.newTemp()
		c.emit(&ir.GetField{Dst: fieldTemp, Src: scrut, Field: fmt.Sprintf("%d", i)})
		fieldCond, always := c.compilePatternCond(fieldTemp, el)
		if always {
			continue
		}
		if !hasCond {
			cond = fieldCond
			hasCond = true
			continue
		}
		combined := c.newTemp()
		c.emit(&ir.BinOp{Dst: combined, Op: "&&", Lhs: cond, Rhs: fieldCond})
		cond = ir.TempOperand(combined)
	}
	if !hasCond {
		return ir.Operand{}, true
	}
	return cond, false
}

func (c *Compiler) compileArrayPatternCond(scrut int, pat *ast.ArrayPattern) (ir.Operand, bool) {
	lenTemp := c.newTemp()
	c.setTempType(lenTemp, "int")
	c.emit(&ir.Call{Dst: lenTemp, Callee: "len", Args: []ir.Operand{ir.TempOperand(scrut)}})
	lenCond := c.newTemp()
	c.setTempType(lenCond, "bool")
	c.emit(&ir.BinOp{Dst: lenCond, Op: "==", Lhs: ir.TempOperand(lenTemp), Rhs: ir.IntOperand(int64(len(pat.Elems)))})

	cond := ir.TempOperand(lenCond)
	hasCond := true

	elemType := "i64"
	arrType := derefTypeName(c.tempTypes[scrut])
	if isArrayTypeName(arrType) {
		if t := arrayElemTypeName(arrType); t != "" {
			elemType = t
		}
	}
	for i, el := range pat.Elems {
		if _, ok := el.(*ast.WildcardPattern); ok {
			continue
		}
		elemTemp := c.newTemp()
		c.setTempType(elemTemp, elemType)
		c.emit(&ir.Index{Dst: elemTemp, Array: ir.TempOperand(scrut), Index: ir.IntOperand(int64(i))})
		elemCond, always := c.compilePatternCond(elemTemp, el)
		if always {
			continue
		}
		combined := c.newTemp()
		c.emit(&ir.BinOp{Dst: combined, Op: "&&", Lhs: cond, Rhs: elemCond})
		cond = ir.TempOperand(combined)
		hasCond = true
	}
	if !hasCond {
		return ir.Operand{}, true
	}
	return cond, false
}

func (c *Compiler) compileStructPatternCond(scrut int, pat *ast.StructPattern) (ir.Operand, bool) {
	var cond ir.Operand
	hasCond := false
	for _, f := range pat.Fields {
		if f.Pattern == nil {
			continue
		}
		if _, ok := f.Pattern.(*ast.WildcardPattern); ok {
			continue
		}
		fieldTemp := c.newTemp()
		c.emit(&ir.GetField{Dst: fieldTemp, Src: scrut, Field: f.Name})
		fieldCond, always := c.compilePatternCond(fieldTemp, f.Pattern)
		if always {
			continue
		}
		if !hasCond {
			cond = fieldCond
			hasCond = true
			continue
		}
		combined := c.newTemp()
		c.emit(&ir.BinOp{Dst: combined, Op: "&&", Lhs: cond, Rhs: fieldCond})
		cond = ir.TempOperand(combined)
	}
	if !hasCond {
		return ir.Operand{}, true
	}
	return cond, false
}

func (c *Compiler) compileOrPatternCond(scrut int, pat *ast.OrPattern) (ir.Operand, bool) {
	var cond ir.Operand
	hasCond := false
	for _, alt := range pat.Alts {
		altCond, always := c.compilePatternCond(scrut, alt)
		if always {
			return ir.Operand{}, true
		}
		if !hasCond {
			cond = altCond
			hasCond = true
			continue
		}
		combined := c.newTemp()
		c.emit(&ir.BinOp{Dst: combined, Op: "||", Lhs: cond, Rhs: altCond})
		cond = ir.TempOperand(combined)
	}
	if !hasCond {
		return ir.Operand{}, true
	}
	return cond, false
}

func (c *Compiler) compileMatchArm(arm *ast.MatchArm, scrut int, moveScrut bool, nextLabel string) {
	c.pushScope()
	c.bindPattern(scrut, arm.Pattern, false, moveScrut)
	if arm.Guard != nil {
		if nextLabel == "" {
			c.diag.Add(arm.Guard.Span(), "match guard requires next arm")
		} else {
			guardBlock := c.newBlock("match_guard")
			cond := c.compileOperandBorrow(arm.Guard)
			c.emitTerm(&ir.Branch{Cond: cond, Then: guardBlock.Label, Else: nextLabel})
			c.setCurrentBlock(guardBlock)
		}
	}
	for _, stmt := range arm.Body.Stmts {
		if c.currentBlock().Term != nil {
			break
		}
		c.compileStmt(stmt)
	}
	c.popScope()
}

func (c *Compiler) compileMatchArmTail(arm *ast.MatchArm, scrut int, moveScrut bool, allowImplicit bool, nextLabel string) {
	c.pushScope()
	c.bindPattern(scrut, arm.Pattern, false, moveScrut)
	if arm.Guard != nil {
		if nextLabel == "" {
			c.diag.Add(arm.Guard.Span(), "match guard requires next arm")
		} else {
			guardBlock := c.newBlock("match_guard")
			cond := c.compileOperandBorrow(arm.Guard)
			c.emitTerm(&ir.Branch{Cond: cond, Then: guardBlock.Label, Else: nextLabel})
			c.setCurrentBlock(guardBlock)
		}
	}
	c.compileBlockWithTail(arm.Body, allowImplicit)
	c.popScope()
}
