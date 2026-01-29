package compile

import (
	"os"
	"strconv"
	"strings"

	"dastlang/internal/ast"
)

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
		lt := normalizeTypeName(c.inferExprType(e.Left))
		rt := normalizeTypeName(c.inferExprType(e.Right))
		lhsLit, lhsIsLit := e.Left.(*ast.IntLit)
		rhsLit, rhsIsLit := e.Right.(*ast.IntLit)
		intBinaryResult := func() string {
			if !isIntTypeName(lt) || !isIntTypeName(rt) {
				return ""
			}
			if lhsIsLit && !rhsIsLit {
				if intValueFitsType(lhsLit.Value, rt) {
					return rt
				}
			}
			if rhsIsLit && !lhsIsLit {
				if intValueFitsType(rhsLit.Value, lt) {
					return lt
				}
			}
			pt := intPeerTypeName(lt, rt)
			if pt == "" {
				return ""
			}
			if lhsIsLit && !intValueFitsType(lhsLit.Value, pt) {
				return ""
			}
			if rhsIsLit && !intValueFitsType(rhsLit.Value, pt) {
				return ""
			}
			return pt
		}
		shiftResult := func() string {
			if !isIntTypeName(lt) || !isIntTypeName(rt) {
				return ""
			}
			if rhsIsLit {
				if rhsLit.Value < 0 {
					return ""
				}
				return lt
			}
			if !isUnsignedIntName(rt) {
				return ""
			}
			return lt
		}
		switch e.Op {
		case "&&", "||":
			return "bool"
		case "==", "!=", "<", "<=", ">", ">=":
			return "bool"
		case "+":
			if isStringTypeName(lt) && isStringTypeName(rt) {
				return "String"
			}
			if t := intBinaryResult(); t != "" {
				return t
			}
			return lt
		case "-", "*", "/", "%", "&", "|", "^":
			if t := intBinaryResult(); t != "" {
				return t
			}
			return lt
		case "<<", ">>":
			if t := shiftResult(); t != "" {
				return t
			}
			return lt
		default:
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

func ptrWidthBytes() int64 {
	if v := os.Getenv("DAST_TARGET_PTR_WIDTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n == 32 {
				return 4
			}
			if n == 64 {
				return 8
			}
		}
	}
	return 8
}

func intTypeWidth(name string) int64 {
	switch strings.TrimSpace(name) {
	case "i8", "u8":
		return 8
	case "i16", "u16":
		return 16
	case "i32", "u32", "char":
		return 32
	case "i64", "u64", "int":
		return 64
	case "isize", "usize":
		return ptrWidthBytes() * 8
	}
	return 0
}

func isSignedIntName(name string) bool {
	switch strings.TrimSpace(name) {
	case "i8", "i16", "i32", "i64", "int", "isize":
		return true
	}
	return false
}

func isUnsignedIntName(name string) bool {
	switch strings.TrimSpace(name) {
	case "u8", "u16", "u32", "u64", "usize", "char":
		return true
	}
	return false
}

func i64MaxValue() int64 { return 9223372036854775807 }

func i64MinValue() int64 { return -9223372036854775807 - 1 }

func intTypeMin(name string) int64 {
	if !isSignedIntName(name) && !isUnsignedIntName(name) {
		return 0
	}
	if isUnsignedIntName(name) {
		return 0
	}
	bits := intTypeWidth(name)
	if bits >= 64 {
		return i64MinValue()
	}
	shift := bits - 1
	var v int64 = 1
	for i := int64(0); i < shift; i++ {
		v *= 2
	}
	return -v
}

func intTypeMax(name string) int64 {
	if !isSignedIntName(name) && !isUnsignedIntName(name) {
		return 0
	}
	bits := intTypeWidth(name)
	if isUnsignedIntName(name) {
		if bits >= 63 {
			return i64MaxValue()
		}
		var v int64 = 1
		for i := int64(0); i < bits; i++ {
			v *= 2
		}
		return v - 1
	}
	if bits >= 64 {
		return i64MaxValue()
	}
	shift := bits - 1
	var v int64 = 1
	for i := int64(0); i < shift; i++ {
		v *= 2
	}
	return v - 1
}

func intValueFitsType(val int64, name string) bool {
	if !isSignedIntName(name) && !isUnsignedIntName(name) {
		return false
	}
	return val >= intTypeMin(name) && val <= intTypeMax(name)
}

func intPeerTypeName(a, b string) string {
	la := strings.TrimSpace(a)
	lb := strings.TrimSpace(b)
	if intTypeWidth(la) == 0 || intTypeWidth(lb) == 0 {
		return ""
	}
	abits := intTypeWidth(la)
	bbits := intTypeWidth(lb)
	if isUnsignedIntName(la) && abits == 64 && intTypeMin(lb) < 0 {
		return ""
	}
	if isUnsignedIntName(lb) && bbits == 64 && intTypeMin(la) < 0 {
		return ""
	}
	if la == lb {
		return la
	}
	minAll := intTypeMin(la)
	if intTypeMin(lb) < minAll {
		minAll = intTypeMin(lb)
	}
	maxAll := intTypeMax(la)
	if intTypeMax(lb) > maxAll {
		maxAll = intTypeMax(lb)
	}
	if la == "int" || lb == "int" {
		t := la
		if t != "int" {
			t = lb
		}
		if intTypeMin(t) <= minAll && intTypeMax(t) >= maxAll {
			return t
		}
	}
	if la == "isize" || lb == "isize" {
		t := la
		if t != "isize" {
			t = lb
		}
		if intTypeMin(t) <= minAll && intTypeMax(t) >= maxAll {
			return t
		}
	}
	if la == "usize" || lb == "usize" {
		t := la
		if t != "usize" {
			t = lb
		}
		if intTypeMin(t) <= minAll && intTypeMax(t) >= maxAll {
			return t
		}
	}
	if la == "char" || lb == "char" {
		t := la
		if t != "char" {
			t = lb
		}
		if intTypeMin(t) <= minAll && intTypeMax(t) >= maxAll {
			return t
		}
	}
	signed := []string{"i8", "i16", "i32"}
	if ptrWidthBytes() == 4 {
		signed = append(signed, "isize")
	}
	signed = append(signed, "i64")
	if ptrWidthBytes() == 8 {
		signed = append(signed, "isize")
	}
	signed = append(signed, "int")
	unsigned := []string{"u8", "u16", "u32", "char"}
	if ptrWidthBytes() == 4 {
		unsigned = append(unsigned, "usize")
	}
	unsigned = append(unsigned, "u64")
	if ptrWidthBytes() == 8 {
		unsigned = append(unsigned, "usize")
	}
	if minAll < 0 {
		for _, t := range signed {
			if intTypeMin(t) <= minAll && intTypeMax(t) >= maxAll {
				return t
			}
		}
		return ""
	}
	for _, t := range unsigned {
		if intTypeMin(t) <= minAll && intTypeMax(t) >= maxAll {
			return t
		}
	}
	for _, t := range signed {
		if intTypeMin(t) <= minAll && intTypeMax(t) >= maxAll {
			return t
		}
	}
	return ""
}
