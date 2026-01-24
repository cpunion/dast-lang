package typecheck

import (
	"fmt"

	"dastlang/internal/ast"
)

func (c *Checker) checkBlock(block *ast.Block) {
	c.env.push()
	for _, stmt := range block.Stmts {
		c.checkStmt(stmt)
	}
	c.env.pop()
}

func (c *Checker) checkBlockExpr(block *ast.Block) Type {
	if block == nil {
		return Type{Kind: TypeUnit}
	}
	c.env.push()
	out := Type{Kind: TypeUnit}
	for i, stmt := range block.Stmts {
		if i == len(block.Stmts)-1 {
			if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
				out = c.checkExpr(exprStmt.Expr)
				break
			}
		}
		c.checkStmt(stmt)
	}
	c.env.pop()
	return out
}

func (c *Checker) checkBlockExprExpected(block *ast.Block, expected Type) Type {
	c.pushExpected(expected)
	out := c.checkBlockExpr(block)
	c.popExpected()
	return out
}

func (c *Checker) checkFunctionBlock(block *ast.Block) {
	allowImplicit := c.inferReturn || (c.current.ReturnExplicit && c.current.Return.Kind != TypeUnit)
	c.checkTailBlock(block, allowImplicit)
}

func (c *Checker) checkStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		c.checkLet(s)
	case *ast.LetPatternStmt:
		c.checkLetPattern(s)
	case *ast.AssignStmt:
		c.checkAssign(s)
	case *ast.ExprStmt:
		c.checkExpr(s.Expr)
	case *ast.ReturnStmt:
		c.checkReturn(s)
	case *ast.IfStmt:
		c.checkIf(s)
	case *ast.IfLetStmt:
		c.checkIfLet(s)
	case *ast.WhileStmt:
		c.checkWhile(s)
	case *ast.WhileLetStmt:
		c.checkWhileLet(s)
	case *ast.MatchStmt:
		c.checkMatch(s)
	case *ast.LoopStmt:
		c.checkLoop(s)
	case *ast.BreakStmt:
		c.checkBreak(s)
	case *ast.ContinueStmt:
		c.checkContinue(s)
	case *ast.Block:
		c.checkBlock(s)
	default:
		c.diag.Add(stmt.Span(), "unsupported statement in stage 0")
	}
}

func (c *Checker) checkTailBlock(block *ast.Block, allowImplicit bool) {
	if !allowImplicit {
		c.checkBlock(block)
		return
	}
	c.env.push()
	for i, stmt := range block.Stmts {
		if i == len(block.Stmts)-1 {
			c.checkTailStmt(stmt, allowImplicit)
		} else {
			c.checkStmt(stmt)
		}
	}
	c.env.pop()
}

func (c *Checker) checkTailStmt(stmt ast.Stmt, allowImplicit bool) {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		ret := &ast.ReturnStmt{Value: s.Expr, SpanInfo: s.Span()}
		c.checkReturn(ret)
	case *ast.ReturnStmt:
		c.checkReturn(s)
	case *ast.IfStmt:
		c.checkTailIf(s, allowImplicit)
	case *ast.MatchStmt:
		c.checkTailMatch(s, allowImplicit)
	default:
		c.checkStmt(stmt)
	}
}

func (c *Checker) checkLet(s *ast.LetStmt) {
	if s.Init == nil {
		c.diag.Add(s.Span(), "let requires initializer in stage 0")
		return
	}
	initType := c.checkExpr(s.Init)
	declType := initType
	if s.Type != nil {
		declType = c.fromAstType(*s.Type)
		initType = c.checkExprWithExpected(s.Init, declType)
		if lit, ok := s.Init.(*ast.ArrayLit); ok && len(lit.Elems) == 0 && declType.Kind == TypeArray {
			// allow empty array literal with explicit type
		} else if !typesAssignable(initType, declType) && initType.Kind != TypeInvalid && declType.Kind != TypeInvalid {
			c.diag.Add(s.Span(), fmt.Sprintf("cannot assign %s to %s", initType.String(), declType.String()))
		}
	}
	c.env.declare(s.Name, VarInfo{Type: declType, Mutable: s.Mutable})
}

func (c *Checker) checkLetPattern(s *ast.LetPatternStmt) {
	if s.Init == nil {
		c.diag.Add(s.Span(), "let requires initializer in stage 0")
		return
	}
	initType := c.checkExpr(s.Init)
	if initType.Ref {
		c.diag.Add(s.Span(), "let pattern requires non-reference value")
		initType = derefType(initType)
	}
	c.checkPattern(initType, s.Pattern)
}

func (c *Checker) checkAssign(s *ast.AssignStmt) {
	switch target := s.Target.(type) {
	case *ast.IdentExpr:
		info, ok := c.env.lookup(target.Name)
		if !ok {
			if _, ok := c.consts[target.Name]; ok {
				c.diag.Add(s.Span(), fmt.Sprintf("cannot assign to const '%s'", target.Name))
			} else {
				c.diag.Add(s.Span(), fmt.Sprintf("undefined variable '%s'", target.Name))
			}
			return
		}
		if !info.Mutable {
			c.diag.Add(s.Span(), fmt.Sprintf("cannot assign to immutable variable '%s'", target.Name))
		}
		valType := c.checkExprWithExpected(s.Value, info.Type)
		if !typesAssignable(valType, info.Type) && valType.Kind != TypeInvalid && info.Type.Kind != TypeInvalid {
			c.diag.Add(s.Span(), fmt.Sprintf("cannot assign %s to %s", valType.String(), info.Type.String()))
		}
	case *ast.DerefExpr:
		refType := c.checkExpr(target.Expr)
		if !refType.Ref {
			c.diag.Add(target.Span(), "assignment requires mutable reference")
			return
		}
		if !refType.Mut {
			c.diag.Add(target.Span(), "assignment through & requires &mut")
		}
		base := derefType(refType)
		valType := c.checkExprWithExpected(s.Value, base)
		if !typesAssignable(valType, base) && valType.Kind != TypeInvalid && base.Kind != TypeInvalid {
			c.diag.Add(s.Span(), fmt.Sprintf("cannot assign %s to %s", valType.String(), base.String()))
		}
	case *ast.IndexExpr:
		recvType := c.checkExpr(target.Receiver)
		if ident, ok := target.Receiver.(*ast.IdentExpr); ok {
			if info, ok := c.env.lookup(ident.Name); ok && !info.Mutable {
				c.diag.Add(target.Span(), fmt.Sprintf("cannot assign through immutable '%s'", ident.Name))
			}
		}
		if recvType.Ref && !recvType.Mut {
			c.diag.Add(target.Span(), "assignment through & requires &mut")
		}
		if recvType.Ref {
			recvType = derefType(recvType)
		}
		if recvType.Kind != TypeArray || recvType.Elem == nil {
			c.diag.Add(target.Span(), "index assignment requires array")
			return
		}
		indexType := c.checkExpr(target.Index)
		if !isInt(indexType) && indexType.Kind != TypeInvalid {
			c.diag.Add(target.Index.Span(), "index requires int")
		}
		valType := c.checkExprWithExpected(s.Value, *recvType.Elem)
		if !typesAssignable(valType, *recvType.Elem) && valType.Kind != TypeInvalid && recvType.Elem.Kind != TypeInvalid {
			c.diag.Add(s.Span(), fmt.Sprintf("cannot assign %s to %s", valType.String(), recvType.Elem.String()))
		}
	case *ast.AccessExpr:
		recvType := c.checkExpr(target.Receiver)
		if recvType.Ref {
			if !recvType.Mut {
				c.diag.Add(target.Span(), "assignment through & requires &mut")
			}
			recvType = derefType(recvType)
		} else if ident, ok := target.Receiver.(*ast.IdentExpr); ok {
			if info, ok := c.env.lookup(ident.Name); ok && !info.Mutable {
				c.diag.Add(target.Span(), fmt.Sprintf("cannot assign through immutable '%s'", ident.Name))
			}
		} else {
			c.diag.Add(target.Span(), "assignment target must be mutable")
		}
		if recvType.Kind != TypeStruct {
			if recvType.Kind != TypeInvalid {
				c.diag.Add(target.Span(), "field assignment requires struct")
			}
			return
		}
		decl, ok := c.structs[recvType.Name]
		if !ok {
			c.diag.Add(target.Span(), fmt.Sprintf("unknown struct '%s'", recvType.Name))
			return
		}
		field := findField(decl, target.Field)
		if field == nil {
			c.diag.Add(target.Span(), fmt.Sprintf("unknown field '%s'", target.Field))
			return
		}
		fieldType := c.fromAstType(field.Type)
		valType := c.checkExprWithExpected(s.Value, fieldType)
		if !typesAssignable(valType, fieldType) && valType.Kind != TypeInvalid && fieldType.Kind != TypeInvalid {
			c.diag.Add(s.Span(), fmt.Sprintf("cannot assign %s to %s", valType.String(), fieldType.String()))
		}
	default:
		c.diag.Add(s.Span(), "invalid assignment target")
	}
}

func (c *Checker) checkReturn(s *ast.ReturnStmt) {
	if c.current == nil {
		return
	}
	if c.current.ReturnExplicit {
		retType := c.current.Return
		if s.Value == nil {
			if retType.Kind != TypeUnit {
				c.diag.Add(s.Span(), fmt.Sprintf("returning unit from %s function", retType.String()))
			}
			return
		}
		valType := c.checkExprWithExpected(s.Value, retType)
		if retType.Kind == TypeUnit {
			if valType.Kind != TypeUnit && valType.Kind != TypeInvalid {
				c.diag.Add(s.Span(), "returning value from unit function")
			}
			return
		}
		if !typesAssignable(valType, retType) && valType.Kind != TypeInvalid && retType.Kind != TypeInvalid {
			c.diag.Add(s.Span(), fmt.Sprintf("return type mismatch: %s vs %s", valType.String(), retType.String()))
		}
		return
	}

	if s.Value == nil {
		c.hasBareReturn = true
		return
	}
	valType := c.checkExprWithExpected(s.Value, c.inferredType)
	if valType.Kind == TypeInvalid {
		return
	}
	if !c.hasReturn {
		c.inferredType = valType
		c.hasReturn = true
		return
	}
	if !typesAssignable(valType, c.inferredType) {
		c.diag.Add(s.Span(), fmt.Sprintf("return type mismatch: %s vs %s", valType.String(), c.inferredType.String()))
	}
}

func (c *Checker) checkIf(s *ast.IfStmt) {
	cond := c.checkExpr(s.Cond)
	if !isBool(cond) && cond.Kind != TypeInvalid {
		c.diag.Add(s.Cond.Span(), "if condition must be bool")
	}
	c.checkBlock(s.Then)
	if s.Else != nil {
		c.checkBlock(s.Else)
	}
}

func (c *Checker) checkTailIf(s *ast.IfStmt, allowImplicit bool) {
	cond := c.checkExpr(s.Cond)
	if !isBool(cond) && cond.Kind != TypeInvalid {
		c.diag.Add(s.Cond.Span(), "if condition must be bool")
	}
	if s.Else == nil {
		c.checkBlock(s.Then)
		if allowImplicit && c.current.ReturnExplicit && c.current.Return.Kind != TypeUnit {
			c.diag.Add(s.Span(), "tail if requires else to return value")
		}
		return
	}
	c.checkTailBlock(s.Then, allowImplicit)
	c.checkTailBlock(s.Else, allowImplicit)
}

func (c *Checker) checkWhile(s *ast.WhileStmt) {
	cond := c.checkExpr(s.Cond)
	if !isBool(cond) && cond.Kind != TypeInvalid {
		c.diag.Add(s.Cond.Span(), "while condition must be bool")
	}
	c.loopDepth++
	c.checkBlock(s.Body)
	c.loopDepth--
}

func (c *Checker) checkIfLet(s *ast.IfLetStmt) {
	scrutType := c.checkExpr(s.Expr)
	if scrutType.Ref {
		c.diag.Add(s.Expr.Span(), "if let requires non-reference enum expression")
		scrutType = derefType(scrutType)
	}
	if scrutType.Kind != TypeEnum && scrutType.Kind != TypeInvalid {
		c.diag.Add(s.Expr.Span(), "if let requires enum expression")
	}
	c.env.push()
	c.checkPattern(scrutType, s.Pattern)
	c.checkBlock(s.Then)
	c.env.pop()
	if s.Else != nil {
		c.checkBlock(s.Else)
	}
}

func (c *Checker) checkWhileLet(s *ast.WhileLetStmt) {
	scrutType := c.checkExpr(s.Expr)
	if scrutType.Ref {
		c.diag.Add(s.Expr.Span(), "while let requires non-reference enum expression")
		scrutType = derefType(scrutType)
	}
	if scrutType.Kind != TypeEnum && scrutType.Kind != TypeInvalid {
		c.diag.Add(s.Expr.Span(), "while let requires enum expression")
	}
	c.loopDepth++
	c.env.push()
	c.checkPattern(scrutType, s.Pattern)
	c.checkBlock(s.Body)
	c.env.pop()
	c.loopDepth--
}

func (c *Checker) checkLoop(s *ast.LoopStmt) {
	c.loopDepth++
	c.checkBlock(s.Body)
	c.loopDepth--
}

func (c *Checker) checkBreak(s *ast.BreakStmt) {
	if c.loopDepth == 0 {
		c.diag.Add(s.Span(), "break outside of loop")
	}
}

func (c *Checker) checkContinue(s *ast.ContinueStmt) {
	if c.loopDepth == 0 {
		c.diag.Add(s.Span(), "continue outside of loop")
	}
}

func (c *Checker) checkMatch(s *ast.MatchStmt) {
	scrutType := c.checkExpr(s.Expr)
	if scrutType.Ref {
		c.diag.Add(s.Expr.Span(), "match requires non-reference expression")
		scrutType = derefType(scrutType)
	}

	for _, arm := range s.Arms {
		c.env.push()
		c.checkPattern(scrutType, arm.Pattern)
		if arm.Guard != nil {
			guardType := c.checkExpr(arm.Guard)
			if !isBool(guardType) && guardType.Kind != TypeInvalid {
				c.diag.Add(arm.Guard.Span(), "match guard must be bool")
			}
		}
		c.checkBlock(arm.Body)
		c.env.pop()
	}
}

func (c *Checker) checkTailMatch(s *ast.MatchStmt, allowImplicit bool) {
	scrutType := c.checkExpr(s.Expr)
	if scrutType.Ref {
		c.diag.Add(s.Expr.Span(), "match requires non-reference expression")
		scrutType = derefType(scrutType)
	}

	for _, arm := range s.Arms {
		c.env.push()
		c.checkPattern(scrutType, arm.Pattern)
		if arm.Guard != nil {
			guardType := c.checkExpr(arm.Guard)
			if !isBool(guardType) && guardType.Kind != TypeInvalid {
				c.diag.Add(arm.Guard.Span(), "match guard must be bool")
			}
		}
		c.checkTailBlock(arm.Body, allowImplicit)
		c.env.pop()
	}
}

func (c *Checker) checkPattern(scrut Type, pat ast.Pattern) {
	switch p := pat.(type) {
	case *ast.WildcardPattern:
		// no bindings
	case *ast.LiteralPattern:
		litType := constValueType(p.Value)
		if !typesEqual(litType, scrut) && litType.Kind != TypeInvalid && scrut.Kind != TypeInvalid {
			c.diag.Add(p.Span(), fmt.Sprintf("literal pattern expects %s, got %s", litType.String(), scrut.String()))
		}
	case *ast.RangePattern:
		if !isInt(scrut) && scrut.Kind != TypeInvalid {
			c.diag.Add(p.Span(), "range pattern requires int scrutinee")
		}
	case *ast.OrPattern:
		if c.patternHasBinding(p) {
			c.diag.Add(p.Span(), "or-patterns with bindings are not supported in stage 0")
		}
		for _, alt := range p.Alts {
			c.checkPattern(scrut, alt)
		}
	case *ast.VariantPattern:
		if scrut.Kind != TypeEnum && scrut.Kind != TypeInvalid {
			c.diag.Add(p.Span(), "variant pattern requires enum scrutinee")
			return
		}
		patEnum := p.EnumName
		if patEnum == "" {
			patEnum = scrut.Name
		}
		if patEnum == "" {
			c.diag.Add(p.Span(), "cannot resolve enum for pattern")
			return
		}
		if scrut.Name != "" && patEnum != scrut.Name {
			c.diag.Add(p.Span(), fmt.Sprintf("pattern enum '%s' does not match '%s'", patEnum, scrut.Name))
		}
		decl, ok := c.enums[patEnum]
		if !ok {
			c.diag.Add(p.Span(), fmt.Sprintf("unknown enum '%s'", patEnum))
			return
		}
		variant := findVariant(decl, p.Variant)
		if variant == nil {
			c.diag.Add(p.Span(), fmt.Sprintf("unknown variant '%s'", p.Variant))
			return
		}
		if p.Binding != "" {
			if variant.Payload == nil {
				c.diag.Add(p.Span(), "variant has no payload to bind")
				return
			}
			payloadType := c.fromAstType(*variant.Payload)
			c.env.declare(p.Binding, VarInfo{Type: payloadType, Mutable: false})
		}
	case *ast.StructPattern:
		if scrut.Kind != TypeStruct && scrut.Kind != TypeInvalid {
			c.diag.Add(p.Span(), "struct pattern requires struct scrutinee")
			return
		}
		if p.StructName != "" && scrut.Name != "" && p.StructName != scrut.Name {
			c.diag.Add(p.Span(), fmt.Sprintf("pattern struct '%s' does not match '%s'", p.StructName, scrut.Name))
		}
		decl, ok := c.structs[p.StructName]
		if !ok {
			c.diag.Add(p.Span(), fmt.Sprintf("unknown struct '%s'", p.StructName))
			return
		}
		for _, f := range p.Fields {
			field := findField(decl, f.Name)
			if field == nil {
				c.diag.Add(f.Span, fmt.Sprintf("unknown field '%s'", f.Name))
				continue
			}
			fieldType := c.fromAstType(field.Type)
			if f.Pattern == nil {
				binding := f.Binding
				if binding == "" {
					binding = f.Name
				}
				c.env.declare(binding, VarInfo{Type: fieldType, Mutable: false})
				continue
			}
			c.checkPattern(fieldType, f.Pattern)
		}
	default:
		c.diag.Add(p.Span(), "unsupported pattern in stage 0")
	}
}

func (c *Checker) patternHasBinding(pat ast.Pattern) bool {
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
