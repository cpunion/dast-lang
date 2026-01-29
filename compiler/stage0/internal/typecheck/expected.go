package typecheck

import "dastlang/internal/ast"

func (c *Checker) pushExpected(t Type) {
	c.expectedStack = append(c.expectedStack, t)
}

func (c *Checker) popExpected() {
	if len(c.expectedStack) == 0 {
		return
	}
	c.expectedStack = c.expectedStack[:len(c.expectedStack)-1]
}

func (c *Checker) currentExpected() (Type, bool) {
	if len(c.expectedStack) == 0 {
		return Type{}, false
	}
	return c.expectedStack[len(c.expectedStack)-1], true
}

func (c *Checker) checkExprWithExpected(expr ast.Expr, expected Type) Type {
	c.pushExpected(expected)
	out := c.checkExpr(expr)
	c.popExpected()
	return out
}
