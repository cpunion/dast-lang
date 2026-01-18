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

func (c *Checker) checkFunctionBlock(block *ast.Block) {
	allowImplicit := c.inferReturn || (c.current.ReturnExplicit && c.current.Return.Kind != TypeUnit)
	c.checkTailBlock(block, allowImplicit)
}

func (c *Checker) checkStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		c.checkLet(s)
	case *ast.AssignStmt:
		c.checkAssign(s)
	case *ast.ExprStmt:
		c.checkExpr(s.Expr)
	case *ast.ReturnStmt:
		c.checkReturn(s)
	case *ast.IfStmt:
		c.checkIf(s)
	case *ast.WhileStmt:
		c.checkWhile(s)
	case *ast.MatchStmt:
		c.checkMatch(s)
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
		if lit, ok := s.Init.(*ast.ArrayLit); ok && len(lit.Elems) == 0 && declType.Kind == TypeArray {
			// allow empty array literal with explicit type
		} else if !typesAssignable(initType, declType) && initType.Kind != TypeInvalid && declType.Kind != TypeInvalid {
			c.diag.Add(s.Span(), fmt.Sprintf("cannot assign %s to %s", initType.String(), declType.String()))
		}
	}
	c.env.declare(s.Name, VarInfo{Type: declType, Mutable: s.Mutable})
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
		valType := c.checkExpr(s.Value)
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
		valType := c.checkExpr(s.Value)
		base := derefType(refType)
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
		valType := c.checkExpr(s.Value)
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
		valType := c.checkExpr(s.Value)
		fieldType := c.fromAstType(field.Type)
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
		valType := c.checkExpr(s.Value)
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
	valType := c.checkExpr(s.Value)
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
	c.checkBlock(s.Body)
}

func (c *Checker) checkMatch(s *ast.MatchStmt) {
	scrutType := c.checkExpr(s.Expr)
	enumName := ""
	if scrutType.Kind == TypeEnum {
		enumName = scrutType.Name
	} else if scrutType.Kind != TypeInvalid {
		c.diag.Add(s.Expr.Span(), "match requires enum expression")
	}

	for _, arm := range s.Arms {
		c.env.push()
		switch p := arm.Pattern.(type) {
		case *ast.WildcardPattern:
			// no bindings
		case *ast.VariantPattern:
			patEnum := p.EnumName
			if patEnum == "" {
				patEnum = enumName
			}
			if patEnum == "" {
				c.diag.Add(p.Span(), "cannot resolve enum for pattern")
				break
			}
			if enumName != "" && patEnum != enumName {
				c.diag.Add(p.Span(), fmt.Sprintf("pattern enum '%s' does not match '%s'", patEnum, enumName))
			}
			decl, ok := c.enums[patEnum]
			if !ok {
				c.diag.Add(p.Span(), fmt.Sprintf("unknown enum '%s'", patEnum))
				break
			}
			variant := findVariant(decl, p.Variant)
			if variant == nil {
				c.diag.Add(p.Span(), fmt.Sprintf("unknown variant '%s'", p.Variant))
				break
			}
			if p.Binding != "" {
				if variant.Payload == nil {
					c.diag.Add(p.Span(), "variant has no payload to bind")
					break
				}
				payloadType := c.fromAstType(*variant.Payload)
				c.env.declare(p.Binding, VarInfo{Type: payloadType, Mutable: false})
			}
			if p.Binding == "" && variant.Payload != nil {
				// allow but no binding
			}
		default:
			c.diag.Add(p.Span(), "unsupported pattern in stage 0")
		}
		c.checkBlock(arm.Body)
		c.env.pop()
	}
}

func (c *Checker) checkTailMatch(s *ast.MatchStmt, allowImplicit bool) {
	scrutType := c.checkExpr(s.Expr)
	enumName := ""
	if scrutType.Kind == TypeEnum {
		enumName = scrutType.Name
	} else if scrutType.Kind != TypeInvalid {
		c.diag.Add(s.Expr.Span(), "match requires enum expression")
	}

	for _, arm := range s.Arms {
		c.env.push()
		switch p := arm.Pattern.(type) {
		case *ast.WildcardPattern:
		case *ast.VariantPattern:
			patEnum := p.EnumName
			if patEnum == "" {
				patEnum = enumName
			}
			if patEnum == "" {
				c.diag.Add(p.Span(), "cannot resolve enum for pattern")
				break
			}
			if enumName != "" && patEnum != enumName {
				c.diag.Add(p.Span(), fmt.Sprintf("pattern enum '%s' does not match '%s'", patEnum, enumName))
			}
			decl, ok := c.enums[patEnum]
			if !ok {
				c.diag.Add(p.Span(), fmt.Sprintf("unknown enum '%s'", patEnum))
				break
			}
			variant := findVariant(decl, p.Variant)
			if variant == nil {
				c.diag.Add(p.Span(), fmt.Sprintf("unknown variant '%s'", p.Variant))
				break
			}
			if p.Binding != "" {
				if variant.Payload == nil {
					c.diag.Add(p.Span(), "variant has no payload to bind")
					break
				}
				payloadType := c.fromAstType(*variant.Payload)
				c.env.declare(p.Binding, VarInfo{Type: payloadType, Mutable: false})
			}
		default:
			c.diag.Add(p.Span(), "unsupported pattern in stage 0")
		}
		c.checkTailBlock(arm.Body, allowImplicit)
		c.env.pop()
	}
}
