package compile

import "dastlang/internal/ast"

// inferExprType attempts to infer the type of an expression
// This is a simplified version - full type info would come from typechecker
func (c *Compiler) inferExprType(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.IntLit:
		return "i64" // Default int type
	case *ast.BoolLit:
		return "bool"
	case *ast.StringLit:
		return "String"
	case *ast.BinaryExpr:
		// Comparison ops return bool
		switch e.Op {
		case "==", "!=", "<", "<=", ">", ">=":
			return "bool"
		default:
			// Arithmetic ops return same type as operands
			return c.inferExprType(e.Left)
		}
	case *ast.UnaryExpr:
		if e.Op == "!" {
			return "bool"
		}
		return c.inferExprType(e.Expr)
	case *ast.BlockExpr:
		if e.Block == nil || len(e.Block.Stmts) == 0 {
			return "unit"
		}
		if exprStmt, ok := e.Block.Stmts[len(e.Block.Stmts)-1].(*ast.ExprStmt); ok {
			return c.inferExprType(exprStmt.Expr)
		}
		return "unit"
	case *ast.IfExpr:
		if e.Else == nil {
			return "unit"
		}
		return c.inferExprType(e.Then)
	case *ast.MatchExpr:
		if len(e.Arms) == 0 {
			return "unit"
		}
		if e.Arms[0].Body != nil && len(e.Arms[0].Body.Stmts) > 0 {
			if exprStmt, ok := e.Arms[0].Body.Stmts[len(e.Arms[0].Body.Stmts)-1].(*ast.ExprStmt); ok {
				return c.inferExprType(exprStmt.Expr)
			}
		}
		return "unit"
	case *ast.IdentExpr:
		// Look up variable type
		if varInfo, ok := c.lookupVar(e.Name); ok {
			// Try to get type from tempTypes map
			if varInfo.Temp >= 0 {
				if typ, ok := c.tempTypes[varInfo.Temp]; ok && typ != "" {
					return typ
				}
			}
		}
		return "i64" // Default
	case *ast.ClosureExpr:
		return "Closure"
	default:
		return "i64" // Default fallback
	}
}
