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
	case *ast.ArrayLit:
		elemType := "i64"
		if len(e.Elems) > 0 {
			if t := c.inferExprType(e.Elems[0]); t != "" {
				elemType = t
			}
		}
		return "[" + elemType + "]"
	case *ast.StructLit:
		if e.Name != "" {
			return e.Name
		}
		return "i64"
	case *ast.EnumVariantExpr:
		if e.EnumName != "" {
			return e.EnumName
		}
		if name, ok := c.resolveEnumName(e.Variant); ok {
			return name
		}
		return "i64"
	case *ast.TupleLit:
		if len(e.Elems) == 0 {
			return "unit"
		}
		elemTypes := make([]string, 0, len(e.Elems))
		for _, el := range e.Elems {
			elemTypes = append(elemTypes, c.inferExprType(el))
		}
		return tupleTypeName(elemTypes)
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
	case *ast.RefExpr:
		inner := c.inferExprType(e.Expr)
		if inner == "" {
			inner = "i64"
		}
		if inner == "String" || inner == "str" {
			return "str"
		}
		return "*" + inner
	case *ast.DerefExpr:
		return derefTypeName(c.inferExprType(e.Expr))
	case *ast.BlockExpr:
		if e.Block == nil || len(e.Block.Stmts) == 0 {
			return "unit"
		}
		if exprStmt, ok := e.Block.Stmts[len(e.Block.Stmts)-1].(*ast.ExprStmt); ok {
			return c.inferExprType(exprStmt.Expr)
		}
		return "unit"
	case *ast.LoopExpr:
		return c.inferLoopExprType(e)
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
			if varInfo.Type != "" {
				return varInfo.Type
			}
			// Try to get type from tempTypes map
			if varInfo.Temp >= 0 {
				if typ, ok := c.tempTypes[varInfo.Temp]; ok && typ != "" {
					return typ
				}
			}
		}
		return "i64" // Default
	case *ast.CallExpr:
		if ret, ok := c.funcRetTypes[e.Callee]; ok {
			return ret
		}
		return "unit"
	case *ast.MethodCallExpr:
		if e.EnumName != "" {
			return e.EnumName
		}
		if e.ResolvedName != "" {
			if ret, ok := c.funcRetTypes[e.ResolvedName]; ok {
				return ret
			}
		}
		return "unit"
	case *ast.ClosureExpr:
		return "Closure"
	case *ast.AccessExpr:
		// Enum variant access like TokenKind.Fn should infer to the enum type.
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			if decl, ok := c.enums[ident.Name]; ok && enumHasVariant(decl, e.Field) {
				return ident.Name
			}
		}
		recvType := derefTypeName(c.inferExprType(e.Receiver))
		if recvType != "" {
			if decl, ok := c.structs[recvType]; ok {
				if fd := findField(decl, e.Field); fd != nil {
					return formatType(fd.Type)
				}
			} else if c.prog.TypeDecls != nil {
				if td, ok := c.prog.TypeDecls[recvType]; ok {
					for _, f := range td.Fields {
						if f.Name == e.Field {
							if f.Type != "" {
								return f.Type
							}
							break
						}
					}
				}
			}
		}
		return "i64"
	case *ast.IndexExpr:
		recvType := derefTypeName(c.inferExprType(e.Receiver))
		if isArrayTypeName(recvType) {
			if t := arrayElemTypeName(recvType); t != "" {
				return t
			}
		}
		return "i64"
	default:
		return "i64" // Default fallback
	}
}

func (c *Compiler) inferLoopExprType(e *ast.LoopExpr) string {
	if e.Body == nil {
		return "unit"
	}
	found := ""
	var scanBlock func(b *ast.Block)
	var scanStmt func(s ast.Stmt)
	scanBlock = func(b *ast.Block) {
		if b == nil || found != "" {
			return
		}
		for _, stmt := range b.Stmts {
			if found != "" {
				return
			}
			scanStmt(stmt)
		}
	}
	scanStmt = func(s ast.Stmt) {
		switch v := s.(type) {
		case *ast.BreakStmt:
			if v.Label == "" && v.Value != nil {
				found = c.inferExprType(v.Value)
			}
		case *ast.Block:
			scanBlock(v)
		case *ast.IfStmt:
			scanBlock(v.Then)
			scanBlock(v.Else)
		case *ast.IfLetStmt:
			scanBlock(v.Then)
			scanBlock(v.Else)
		case *ast.MatchStmt:
			for i := range v.Arms {
				scanBlock(v.Arms[i].Body)
			}
		case *ast.LetStmt:
			if v.Init != nil {
				_ = c.inferExprType(v.Init)
			}
		case *ast.LetPatternStmt:
			if v.Init != nil {
				_ = c.inferExprType(v.Init)
			}
		case *ast.ExprStmt:
			_ = c.inferExprType(v.Expr)
		case *ast.WhileStmt, *ast.WhileLetStmt, *ast.LoopStmt, *ast.ForStmt:
			// nested loops: ignore break values inside
			return
		}
	}
	scanBlock(e.Body)
	if found == "" {
		return "unit"
	}
	return found
}
