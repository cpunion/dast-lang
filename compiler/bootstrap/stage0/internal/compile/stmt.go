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
	case *ast.AssignStmt:
		c.compileAssign(s)
	case *ast.ExprStmt:
		c.compileOperand(s.Expr)
	case *ast.ReturnStmt:
		c.compileReturn(s)
	case *ast.IfStmt:
		c.compileIf(s)
	case *ast.WhileStmt:
		c.compileWhile(s)
	case *ast.MatchStmt:
		c.compileMatch(s)
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
	// All let variables use named storage (so they can be referenced with &)
	// The only difference between let and let mut is whether reassignment is allowed
	name := c.declareMutVar(s.Name)
	// Track mutability separately for &mut checks
	if !s.Mutable {
		// Mark as immutable in scope (declareMutVar sets Mutable=true, we need to fix it)
		c.markImmutable(s.Name)
	}
	c.emit(&ir.StoreVar{Name: name, Src: c.compileOperand(s.Init)})
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
	c.compileBlock(s.Body)
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: condBlock.Label})
	}

	c.setCurrentBlock(afterBlock)
}

func (c *Compiler) compileMatch(s *ast.MatchStmt) {
	scrut := c.compileExpr(s.Expr)
	after := c.newBlock("match_after")

	for i, arm := range s.Arms {
		armBlock := c.newBlock("match_arm")
		pat := arm.Pattern
		switch p := pat.(type) {
		case *ast.WildcardPattern:
			c.emitTerm(&ir.Jump{Target: armBlock.Label})
			c.setCurrentBlock(armBlock)
			c.compileMatchArm(&arm, scrut)
			if c.currentBlock().Term == nil {
				c.emitTerm(&ir.Jump{Target: after.Label})
			}
			c.setCurrentBlock(after)
			return
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
			next := c.newBlock("match_next")
			c.emitTerm(&ir.Branch{Cond: cmp, Then: armBlock.Label, Else: next.Label})
			c.setCurrentBlock(armBlock)
			c.compileMatchArm(&arm, scrut)
			if c.currentBlock().Term == nil {
				c.emitTerm(&ir.Jump{Target: after.Label})
			}
			c.setCurrentBlock(next)
			if i == len(s.Arms)-1 {
				c.emitTerm(&ir.Jump{Target: after.Label})
			}
		default:
			c.diag.Add(pat.Span(), "unsupported pattern in stage 0")
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
		pat := arm.Pattern
		switch p := pat.(type) {
		case *ast.WildcardPattern:
			c.emitTerm(&ir.Jump{Target: armBlock.Label})
			c.setCurrentBlock(armBlock)
			c.compileMatchArmTail(&arm, scrut, allowImplicit)
			if c.currentBlock().Term == nil {
				c.emitTerm(&ir.Jump{Target: after.Label})
			}
			c.setCurrentBlock(after)
			return
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
			next := c.newBlock("match_next")
			c.emitTerm(&ir.Branch{Cond: cmp, Then: armBlock.Label, Else: next.Label})
			c.setCurrentBlock(armBlock)
			c.compileMatchArmTail(&arm, scrut, allowImplicit)
			if c.currentBlock().Term == nil {
				c.emitTerm(&ir.Jump{Target: after.Label})
			}
			c.setCurrentBlock(next)
			if i == len(s.Arms)-1 {
				c.emitTerm(&ir.Jump{Target: after.Label})
			}
		default:
			c.diag.Add(pat.Span(), "unsupported pattern in stage 0")
		}
	}
	if c.currentBlock().Term == nil {
		c.emitTerm(&ir.Jump{Target: after.Label})
	}
	c.setCurrentBlock(after)
}

func (c *Compiler) compileMatchArm(arm *ast.MatchArm, scrut int) {
	c.pushScope()
	if vp, ok := arm.Pattern.(*ast.VariantPattern); ok {
		if vp.Binding != "" {
			// Get payload using GetField instead of EnumPayload
			payload := c.newTemp()
			c.emit(&ir.GetField{Dst: payload, Src: scrut, Field: "_payload"})
			name := c.declareMutVar(vp.Binding)
			c.emit(&ir.StoreVar{Name: name, Src: ir.TempOperand(payload)})
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

func (c *Compiler) compileMatchArmTail(arm *ast.MatchArm, scrut int, allowImplicit bool) {
	c.pushScope()
	if vp, ok := arm.Pattern.(*ast.VariantPattern); ok {
		if vp.Binding != "" {
			// Get payload using GetField instead of EnumPayload
			payload := c.newTemp()
			c.emit(&ir.GetField{Dst: payload, Src: scrut, Field: "_payload"})
			name := c.declareMutVar(vp.Binding)
			c.emit(&ir.StoreVar{Name: name, Src: ir.TempOperand(payload)})
		}
	}
	c.compileBlockWithTail(arm.Body, allowImplicit)
	c.popScope()
}
