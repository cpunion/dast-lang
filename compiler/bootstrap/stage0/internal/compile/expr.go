package compile

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/ir"
)

// compileOperand compiles an expression and returns an Operand (inline constant or temp)
func (c *Compiler) compileOperand(expr ast.Expr) ir.Operand {
	// For literals, return inline constants
	switch e := expr.(type) {
	case *ast.IntLit:
		return ir.IntOperand(e.Value)
	case *ast.BoolLit:
		return ir.BoolOperand(e.Value)
	case *ast.StringLit:
		return ir.StringOperand(e.Value)
	case *ast.IdentExpr:
		if info, ok := c.consts[e.Name]; ok {
			return ir.ConstOperand(constValueToIr(info.Value, info.TypeName))
		}
	}
	// For everything else, compile to temp and wrap
	return ir.TempOperand(c.compileExpr(expr))
}

func (c *Compiler) compileExpr(expr ast.Expr) int {
	switch e := expr.(type) {
	case *ast.ArrayLit:
		elems := make([]ir.Operand, 0, len(e.Elems))
		for _, elem := range e.Elems {
			elems = append(elems, c.compileOperand(elem))
		}
		dst := c.newTemp()
		c.setTempType(dst, "[i64]") // Array type
		c.emit(&ir.MakeArray{Dst: dst, Elems: elems})
		return dst
	case *ast.IdentExpr:
		varInfo, ok := c.lookupVar(e.Name)
		if !ok {
			if _, ok := c.consts[e.Name]; ok {
				c.diag.Add(e.Span(), fmt.Sprintf("const '%s' cannot be used where a temp is required", e.Name))
				return c.constZero()
			}
			c.diag.Add(e.Span(), fmt.Sprintf("undefined variable '%s'", e.Name))
			return c.constZero()
		}
		if varInfo.Temp >= 0 {
			// Value parameter: return the temp directly
			return varInfo.Temp
		}
		// Memory variable (let or let mut): generate load
		t := c.newTemp()
		c.setTempType(t, "i64") // Default type
		c.emit(&ir.LoadVar{Dst: t, Name: varInfo.Name})
		return t
	case *ast.RefExpr:
		ident, ok := e.Expr.(*ast.IdentExpr)
		if !ok {
			c.diag.Add(e.Span(), "reference target must be identifier in stage 0")
			return c.constZero()
		}
		varInfo, ok := c.lookupVar(ident.Name)
		if !ok {
			if _, ok := c.consts[ident.Name]; ok {
				c.diag.Add(e.Span(), fmt.Sprintf("cannot take reference to const '%s'", ident.Name))
			} else {
				c.diag.Add(e.Span(), fmt.Sprintf("undefined variable '%s'", ident.Name))
			}
			return c.constZero()
		}
		// Value parameters (Temp >= 0) cannot be referenced - they have no memory address
		if varInfo.Temp >= 0 {
			c.diag.Add(e.Span(), fmt.Sprintf("cannot take reference to value parameter '%s'", ident.Name))
			return c.constZero()
		}
		// Only &mut requires mutable variable
		if e.Mutable && !varInfo.Mutable {
			c.diag.Add(e.Span(), fmt.Sprintf("cannot take &mut reference to immutable variable '%s'", ident.Name))
			return c.constZero()
		}
		t := c.newTemp()
		c.setTempType(t, "*i64") // Pointer type
		c.emit(&ir.LoadVar{Dst: t, Name: varInfo.Name, Addr: true})
		return t
	case *ast.DerefExpr:
		src := c.compileExpr(e.Expr)
		t := c.newTemp()
		c.setTempType(t, "i64") // Dereferenced type
		c.emit(&ir.LoadVar{Dst: t, Ref: true, RefTemp: src})
		return t
	case *ast.UnaryExpr:
		src := c.compileOperand(e.Expr)
		switch e.Op {
		case "-":
			t := c.newTemp()
			typ := c.inferExprType(e)
			c.setTempType(t, typ)
			c.emit(&ir.BinOp{Dst: t, Op: "-", Lhs: ir.IntOperand(0), Rhs: src})
			return t
		case "!":
			t := c.newTemp()
			c.setTempType(t, "bool")
			c.emit(&ir.BinOp{Dst: t, Op: "==", Lhs: src, Rhs: ir.BoolOperand(false)})
			return t
		default:
			return c.constZero()
		}
	case *ast.BinaryExpr:
		lhs := c.compileOperand(e.Left)
		rhs := c.compileOperand(e.Right)
		t := c.newTemp()
		typ := c.inferExprType(e)
		c.setTempType(t, typ)
		c.emit(&ir.BinOp{Dst: t, Op: e.Op, Lhs: lhs, Rhs: rhs})
		return t
	case *ast.CallExpr:
		args := make([]ir.Operand, 0, len(e.Args))
		for _, arg := range e.Args {
			args = append(args, c.compileOperand(arg))
		}
		dst := c.newTemp()
		// Look up function return type
		if retType, ok := c.funcRetTypes[e.Callee]; ok {
			c.setTempType(dst, retType)
		} else {
			c.setTempType(dst, "unit") // Default return type
		}
		c.emit(&ir.Call{Dst: dst, Callee: e.Callee, Args: args})
		return dst
	case *ast.MethodCallExpr:
		if e.EnumName != "" {
			return c.compileEnumVariant(e.EnumName, e.Method, e.Args, e.Span())
		}
		if e.ResolvedName == "" {
			c.diag.Add(e.Span(), "unresolved method call")
			return c.constZero()
		}
		args := make([]ir.Operand, 0, len(e.Args)+1)
		if e.ResolvedSelf {
			recv := c.compileOperand(e.Receiver)
			args = append(args, recv)
		}
		for _, arg := range e.Args {
			args = append(args, c.compileOperand(arg))
		}
		dst := c.newTemp()
		// Look up method return type
		if retType, ok := c.funcRetTypes[e.ResolvedName]; ok {
			c.setTempType(dst, retType)
		} else {
			c.setTempType(dst, "unit") // Default return type
		}
		c.emit(&ir.Call{Dst: dst, Callee: e.ResolvedName, Args: args})
		return dst
	case *ast.StructLit:
		fields := make([]ir.StructFieldInit, 0, len(e.Fields))
		for _, f := range e.Fields {
			src := c.compileOperand(f.Value)
			fields = append(fields, ir.StructFieldInit{Name: f.Name, Src: src})
		}
		dst := c.newTemp()
		c.setTempType(dst, e.Name)
		c.emit(&ir.MakeStruct{Dst: dst, Name: e.Name, Fields: fields})
		return dst
	case *ast.AccessExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			if _, ok := c.lookupVar(ident.Name); !ok {
				if enumDecl, ok := c.enums[ident.Name]; ok {
					if !enumHasVariant(enumDecl, e.Field) {
						c.diag.Add(e.Span(), fmt.Sprintf("unknown variant '%s'", e.Field))
						return c.constZero()
					}
					return c.compileEnumVariant(ident.Name, e.Field, nil, e.Span())
				}
			}
		}
		recv := c.compileExpr(e.Receiver)
		dst := c.newTemp()
		c.setTempType(dst, "i64") // Field type
		c.emit(&ir.GetField{Dst: dst, Src: recv, Field: e.Field})
		return dst
	case *ast.IndexExpr:
		recv := c.compileOperand(e.Receiver)
		index := c.compileOperand(e.Index)
		dst := c.newTemp()
		c.setTempType(dst, "i64") // Array element type
		c.emit(&ir.Index{Dst: dst, Array: recv, Index: index})
		return dst
	case *ast.EnumVariantExpr:
		enumDecl, ok := c.enums[e.EnumName]
		if !ok {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown enum '%s'", e.EnumName))
			return c.constZero()
		}
		variant := enumVariant(enumDecl, e.Variant)
		if variant == nil {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown variant '%s'", e.Variant))
			return c.constZero()
		}
		if e.Arg == nil && variant.Payload != nil {
			c.diag.Add(e.Span(), "missing payload for enum variant")
		}
		var args []ast.Expr
		if e.Arg != nil {
			args = []ast.Expr{e.Arg}
		}
		return c.compileEnumVariant(e.EnumName, e.Variant, args, e.Span())
	default:
		c.diag.Add(expr.Span(), "unsupported expression in stage 0")
		return c.constZero()
	}
}
