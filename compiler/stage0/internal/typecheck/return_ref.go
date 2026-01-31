package typecheck

import "dastlang/internal/ast"

func (c *Checker) isRefParam(name string) bool {
	if c.currentParamTypes == nil {
		return false
	}
	t, ok := c.currentParamTypes[name]
	return ok && t.Ref
}

func (c *Checker) returnRefOrigin(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		if c.isRefParam(e.Name) {
			return e.Name, true
		}
		if c.env != nil {
			if info, ok := c.env.lookup(e.Name); ok {
				if info.RefOrigin != "" {
					return info.RefOrigin, true
				}
			}
		}
		return "", false
	case *ast.StringLit:
		return "static", true
	case *ast.RefExpr:
		return c.refOriginFromLvalue(e.Expr)
	case *ast.TupleLit:
		if len(e.Elems) == 0 {
			return "", false
		}
		var origin string
		for i, elem := range e.Elems {
			o, ok := c.returnRefOrigin(elem)
			if !ok {
				return "", false
			}
			if i == 0 {
				origin = o
				continue
			}
			if origin != o {
				return "", false
			}
		}
		return origin, true
	case *ast.IfExpr:
		if e.Else == nil {
			return "", false
		}
		originThen, okThen := c.returnRefOrigin(e.Then)
		if !okThen {
			return "", false
		}
		originElse, okElse := c.returnRefOrigin(e.Else)
		if !okElse {
			return "", false
		}
		if originThen != originElse {
			return "", false
		}
		return originThen, true
	case *ast.MatchExpr:
		var origin string
		for i := range e.Arms {
			armOrigin, ok := c.blockReturnOrigin(e.Arms[i].Body)
			if !ok {
				return "", false
			}
			if i == 0 {
				origin = armOrigin
				continue
			}
			if origin != armOrigin {
				return "", false
			}
		}
		if origin == "" {
			return "", false
		}
		return origin, true
	case *ast.BlockExpr:
		return c.blockReturnOrigin(e.Block)
	default:
		return "", false
	}
}

func (c *Checker) refOriginFromLvalue(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		if c.isRefParam(e.Name) {
			return e.Name, true
		}
		return "", false
	case *ast.AccessExpr:
		return c.refOriginFromLvalue(e.Receiver)
	case *ast.IndexExpr:
		return c.refOriginFromLvalue(e.Receiver)
	case *ast.DerefExpr:
		return c.refOriginFromLvalue(e.Expr)
	default:
		return "", false
	}
}

func (c *Checker) blockReturnOrigin(block *ast.Block) (string, bool) {
	if block == nil || len(block.Stmts) == 0 {
		return "", false
	}
	last := block.Stmts[len(block.Stmts)-1]
	switch s := last.(type) {
	case *ast.ExprStmt:
		return c.returnRefOrigin(s.Expr)
	case *ast.ReturnStmt:
		if s.Value == nil {
			return "", false
		}
		return c.returnRefOrigin(s.Value)
	default:
		return "", false
	}
}

func typeContainsRef(t Type) bool {
	if t.Ref {
		return true
	}
	switch t.Kind {
	case TypeTuple:
		for _, e := range t.Elems {
			if typeContainsRef(e) {
				return true
			}
		}
	}
	return false
}
