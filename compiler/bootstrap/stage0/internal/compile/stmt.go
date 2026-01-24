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
		c.compileOperand(s.Expr)
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
	// All let variables use named storage (so they can be referenced with &)
	// The only difference between let and let mut is whether reassignment is allowed
	name := c.declareMutVar(s.Name)
	// Track mutability separately for &mut checks
	if !s.Mutable {
		// Mark as immutable in scope (declareMutVar sets Mutable=true, we need to fix it)
		c.markImmutable(s.Name)
	}
	c.emit(&ir.StoreVar{Name: name, Src: c.compileOperand(s.Init)})
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
	cond, always := c.compilePatternCond(scrut, s.Pattern)
	if always {
		c.bindPattern(scrut, s.Pattern, false)
		return
	}
	thenBlock := c.newBlock("let_pat")
	elseBlock := c.newBlock("let_pat_fail")
	afterBlock := c.newBlock("let_pat_after")

	c.emitTerm(&ir.Branch{Cond: cond, Then: thenBlock.Label, Else: elseBlock.Label})

	c.setCurrentBlock(thenBlock)
	c.bindPattern(scrut, s.Pattern, false)
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: afterBlock.Label})
	}

	c.setCurrentBlock(elseBlock)
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
			val := c.compileOperand(s.Value)
			c.emit(&ir.StoreVar{Ref: true, RefTemp: varInfo.RefTemp, Src: val})
			return
		}
		if !varInfo.Mutable {
			c.diag.Add(s.Span(), fmt.Sprintf("cannot assign to immutable variable '%s'", target.Name))
			return
		}
		val := c.compileOperand(s.Value)
		c.emit(&ir.StoreVar{Name: varInfo.Name, Src: val})
	case *ast.DerefExpr:
		refTemp := c.compileExpr(target.Expr)
		val := c.compileOperand(s.Value)
		c.emit(&ir.StoreVar{Ref: true, RefTemp: refTemp, Src: val})
	case *ast.IndexExpr:
		arrayTemp := c.compileOperand(target.Receiver)
		indexTemp := c.compileOperand(target.Index)
		val := c.compileOperand(s.Value)
		c.emit(&ir.SetIndex{Array: arrayTemp, Index: indexTemp, Src: val})
	case *ast.AccessExpr:
		recv := c.compileExpr(target.Receiver)
		val := c.compileOperand(s.Value)
		c.emit(&ir.SetField{Src: recv, Field: target.Field, Value: val})
	default:
		c.diag.Add(s.Span(), "invalid assignment target")
	}
}

func (c *Compiler) compileReturn(s *ast.ReturnStmt) {
	if s.Value == nil {
		c.emitTerm(&ir.Return{Value: nil})
		return
	}
	val := c.compileOperand(s.Value)
	c.emitTerm(&ir.Return{Value: &val})
}

func (c *Compiler) compileIf(s *ast.IfStmt) {
	cond := c.compileOperand(s.Cond)
	thenBlock := c.newBlock("then")
	elseBlock := c.newBlock("else")
	mergeBlock := c.newBlock("merge")

	c.emitTerm(&ir.Branch{Cond: cond, Then: thenBlock.Label, Else: elseBlock.Label})

	c.setCurrentBlock(thenBlock)
	c.compileBlock(s.Then)
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}

	c.setCurrentBlock(elseBlock)
	if s.Else != nil {
		c.compileBlock(s.Else)
	}
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}

	c.setCurrentBlock(mergeBlock)
}

func (c *Compiler) compileIfLet(s *ast.IfLetStmt) {
	scrut := c.compileExpr(s.Expr)
	thenBlock := c.newBlock("iflet_then")
	elseBlock := c.newBlock("iflet_else")
	mergeBlock := c.newBlock("iflet_merge")

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
	c.compileMatchArm(&arm, scrut, elseBlock.Label)
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}

	c.setCurrentBlock(elseBlock)
	if s.Else != nil {
		c.compileBlock(s.Else)
	}
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}

	c.setCurrentBlock(mergeBlock)
}

func (c *Compiler) compileTailIf(s *ast.IfStmt, allowImplicit bool) {
	if s.Else == nil {
		c.compileIf(s)
		return
	}
	cond := c.compileOperand(s.Cond)
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
	cond := c.compileOperand(s.Cond)
	c.emitTerm(&ir.Branch{Cond: cond, Then: bodyBlock.Label, Else: afterBlock.Label})

	c.setCurrentBlock(bodyBlock)
	c.pushLoop(afterBlock.Label, condBlock.Label)
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
	c.pushLoop(afterBlock.Label, condBlock.Label)
	c.compileMatchArm(&arm, scrut, afterBlock.Label)
	c.popLoop()
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: condBlock.Label})
	}

	c.setCurrentBlock(afterBlock)
}

func (c *Compiler) compileLoop(s *ast.LoopStmt) {
	bodyBlock := c.newBlock("loop_body")
	afterBlock := c.newBlock("loop_after")

	c.emitTerm(&ir.Jump{Target: bodyBlock.Label})

	c.setCurrentBlock(bodyBlock)
	c.pushLoop(afterBlock.Label, bodyBlock.Label)
	c.compileBlock(s.Body)
	c.popLoop()
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: bodyBlock.Label})
	}

	c.setCurrentBlock(afterBlock)
}

func (c *Compiler) compileBreak(s *ast.BreakStmt) {
	loop, ok := c.currentLoop()
	if !ok {
		c.diag.Add(s.Span(), "break outside of loop")
		return
	}
	c.emitTerm(&ir.Jump{Target: loop.breakLabel})
}

func (c *Compiler) compileContinue(s *ast.ContinueStmt) {
	loop, ok := c.currentLoop()
	if !ok {
		c.diag.Add(s.Span(), "continue outside of loop")
		return
	}
	c.emitTerm(&ir.Jump{Target: loop.continueLabel})
}

func (c *Compiler) compileMatch(s *ast.MatchStmt) {
	scrut := c.compileExpr(s.Expr)
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
		c.compileMatchArm(&arm, scrut, next.Label)
		if c.currentBlock().Term == nil {
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
		c.compileMatchArmTail(&arm, scrut, allowImplicit, next.Label)
		if c.currentBlock().Term == nil {
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
	cond := c.compileOperand(e.Cond)
	thenBlock := c.newBlock("if_then")
	elseBlock := c.newBlock("if_else")
	mergeBlock := c.newBlock("if_merge")

	dstName := c.declareMutVar("__if")
	c.emitTerm(&ir.Branch{Cond: cond, Then: thenBlock.Label, Else: elseBlock.Label})

	c.setCurrentBlock(thenBlock)
	thenOp := c.compileOperand(e.Then)
	if c.currentBlock().Term == nil {
		c.emit(&ir.StoreVar{Name: dstName, Src: thenOp})
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}

	c.setCurrentBlock(elseBlock)
	elseOp := ir.ConstOperand(ir.Value{Kind: ir.KindUnit})
	if e.Else != nil {
		elseOp = c.compileOperand(e.Else)
	}
	if c.currentBlock().Term == nil {
		c.emit(&ir.StoreVar{Name: dstName, Src: elseOp})
		c.emitTerm(&ir.Jump{Target: mergeBlock.Label})
	}

	c.setCurrentBlock(mergeBlock)
	dst := c.newTemp()
	c.setTempType(dst, c.inferExprType(e))
	c.emit(&ir.LoadVar{Dst: dst, Name: dstName})
	return dst
}

func (c *Compiler) compileMatchExpr(e *ast.MatchExpr) int {
	scrut := c.compileExpr(e.Expr)
	after := c.newBlock("match_after")
	dstName := c.declareMutVar("__match")
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
		op := c.compileMatchArmExpr(&arm, scrut, next.Label)
		if c.currentBlock().Term == nil {
			c.emit(&ir.StoreVar{Name: dstName, Src: op})
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

func (c *Compiler) compileMatchArmExpr(arm *ast.MatchArm, scrut int, nextLabel string) ir.Operand {
	c.pushScope()
	c.bindPattern(scrut, arm.Pattern, false)
	if arm.Guard != nil {
		if nextLabel == "" {
			c.diag.Add(arm.Guard.Span(), "match guard requires next arm")
		} else {
			guardBlock := c.newBlock("match_guard")
			cond := c.compileOperand(arm.Guard)
			c.emitTerm(&ir.Branch{Cond: cond, Then: guardBlock.Label, Else: nextLabel})
			c.setCurrentBlock(guardBlock)
		}
	}
	op := c.compileBlockExprOperand(arm.Body)
	c.popScope()
	return op
}

func (c *Compiler) bindPattern(scrut int, pat ast.Pattern, mutable bool) {
	switch p := pat.(type) {
	case *ast.VariantPattern:
		if p.Binding != "" {
			payload := c.newTemp()
			c.emit(&ir.GetField{Dst: payload, Src: scrut, Field: "_payload"})
			name := c.declareMutVar(p.Binding)
			if !mutable {
				c.markImmutable(p.Binding)
			}
			c.emit(&ir.StoreVar{Name: name, Src: ir.TempOperand(payload)})
		}
	case *ast.StructPattern:
		for _, f := range p.Fields {
			if f.Pattern == nil {
				fieldTemp := c.newTemp()
				c.emit(&ir.GetField{Dst: fieldTemp, Src: scrut, Field: f.Name})
				binding := f.Binding
				if binding == "" {
					binding = f.Name
				}
				name := c.declareMutVar(binding)
				if !mutable {
					c.markImmutable(binding)
				}
				c.emit(&ir.StoreVar{Name: name, Src: ir.TempOperand(fieldTemp)})
				continue
			}
			if c.patternHasBinding(f.Pattern) {
				fieldTemp := c.newTemp()
				c.emit(&ir.GetField{Dst: fieldTemp, Src: scrut, Field: f.Name})
				c.bindPattern(fieldTemp, f.Pattern, mutable)
			}
		}
	case *ast.OrPattern:
		// bindings in or-patterns are not supported
	}
}

func (c *Compiler) patternHasBinding(pat ast.Pattern) bool {
	switch p := pat.(type) {
	case *ast.VariantPattern:
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
	default:
		return false
	}
}

func (c *Compiler) compilePatternCond(scrut int, pat ast.Pattern) (ir.Operand, bool) {
	switch p := pat.(type) {
	case *ast.WildcardPattern:
		return ir.Operand{}, true
	case *ast.LiteralPattern:
		lit := ir.ConstOperand(constValueToIr(p.Value, ""))
		cmp := c.newTemp()
		c.emit(&ir.BinOp{Dst: cmp, Op: "==", Lhs: ir.TempOperand(scrut), Rhs: lit})
		return ir.TempOperand(cmp), false
	case *ast.RangePattern:
		ge := c.newTemp()
		c.emit(&ir.BinOp{Dst: ge, Op: ">=", Lhs: ir.TempOperand(scrut), Rhs: ir.IntOperand(p.Start)})
		op := "<"
		if p.Inclusive {
			op = "<="
		}
		le := c.newTemp()
		c.emit(&ir.BinOp{Dst: le, Op: op, Lhs: ir.TempOperand(scrut), Rhs: ir.IntOperand(p.End)})
		both := c.newTemp()
		c.emit(&ir.BinOp{Dst: both, Op: "&&", Lhs: ir.TempOperand(ge), Rhs: ir.TempOperand(le)})
		return ir.TempOperand(both), false
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
		return ir.TempOperand(cmp), false
	case *ast.StructPattern:
		return c.compileStructPatternCond(scrut, p)
	case *ast.OrPattern:
		return c.compileOrPatternCond(scrut, p)
	default:
		c.diag.Add(pat.Span(), "unsupported pattern in stage 0")
		return ir.Operand{}, true
	}
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

func (c *Compiler) compileMatchArm(arm *ast.MatchArm, scrut int, nextLabel string) {
	c.pushScope()
	c.bindPattern(scrut, arm.Pattern, false)
	if arm.Guard != nil {
		if nextLabel == "" {
			c.diag.Add(arm.Guard.Span(), "match guard requires next arm")
		} else {
			guardBlock := c.newBlock("match_guard")
			cond := c.compileOperand(arm.Guard)
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

func (c *Compiler) compileMatchArmTail(arm *ast.MatchArm, scrut int, allowImplicit bool, nextLabel string) {
	c.pushScope()
	c.bindPattern(scrut, arm.Pattern, false)
	if arm.Guard != nil {
		if nextLabel == "" {
			c.diag.Add(arm.Guard.Span(), "match guard requires next arm")
		} else {
			guardBlock := c.newBlock("match_guard")
			cond := c.compileOperand(arm.Guard)
			c.emitTerm(&ir.Branch{Cond: cond, Then: guardBlock.Label, Else: nextLabel})
			c.setCurrentBlock(guardBlock)
		}
	}
	c.compileBlockWithTail(arm.Body, allowImplicit)
	c.popScope()
}
