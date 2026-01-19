package compile

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/ir"
)

func (c *Compiler) compileExpr(expr ast.Expr) int {
	switch e := expr.(type) {
	case *ast.IntLit:
		t := c.newTemp()
		c.emit(&ir.Const{Dst: t, Value: ir.Value{Kind: ir.KindInt, Int: e.Value}})
		return t
	case *ast.BoolLit:
		t := c.newTemp()
		c.emit(&ir.Const{Dst: t, Value: ir.Value{Kind: ir.KindBool, Bool: e.Value}})
		return t
	case *ast.StringLit:
		t := c.newTemp()
		c.emit(&ir.Const{Dst: t, Value: ir.Value{Kind: ir.KindString, Str: e.Value}})
		return t
	case *ast.ArrayLit:
		elems := make([]int, 0, len(e.Elems))
		for _, elem := range e.Elems {
			elems = append(elems, c.compileExpr(elem))
		}
		dst := c.newTemp()
		c.emit(&ir.MakeArray{Dst: dst, Elems: elems})
		return dst
	case *ast.IdentExpr:
		name, ok := c.lookupVar(e.Name)
		if !ok {
			if info, ok := c.consts[e.Name]; ok {
				t := c.newTemp()
				c.emit(&ir.Const{Dst: t, Value: constValueToIr(info.Value, info.TypeName)})
				return t
			}
			c.diag.Add(e.Span(), fmt.Sprintf("undefined variable '%s'", e.Name))
			return c.constZero()
		}
		t := c.newTemp()
		c.emit(&ir.LoadVar{Dst: t, Name: name})
		return t
	case *ast.RefExpr:
		ident, ok := e.Expr.(*ast.IdentExpr)
		if !ok {
			c.diag.Add(e.Span(), "reference target must be identifier in stage 0")
			return c.constZero()
		}
		name, ok := c.lookupVar(ident.Name)
		if !ok {
			if _, ok := c.consts[ident.Name]; ok {
				c.diag.Add(e.Span(), fmt.Sprintf("cannot take reference to const '%s'", ident.Name))
			} else {
				c.diag.Add(e.Span(), fmt.Sprintf("undefined variable '%s'", ident.Name))
			}
			return c.constZero()
		}
		t := c.newTemp()
		c.emit(&ir.AddrOf{Dst: t, Name: name})
		return t
	case *ast.DerefExpr:
		src := c.compileExpr(e.Expr)
		t := c.newTemp()
		c.emit(&ir.LoadRef{Dst: t, Src: src})
		return t
	case *ast.UnaryExpr:
		src := c.compileExpr(e.Expr)
		t := c.newTemp()
		c.emit(&ir.UnaryOp{Dst: t, Op: e.Op, Src: src})
		return t
	case *ast.BinaryExpr:
		lhs := c.compileExpr(e.Left)
		rhs := c.compileExpr(e.Right)
		t := c.newTemp()
		c.emit(&ir.BinOp{Dst: t, Op: e.Op, Lhs: lhs, Rhs: rhs})
		return t
	case *ast.CallExpr:
		args := make([]int, 0, len(e.Args))
		for _, arg := range e.Args {
			args = append(args, c.compileExpr(arg))
		}
		dst := c.newTemp()
		c.emit(&ir.Call{Dst: dst, Callee: e.Callee, Args: args})
		return dst
	case *ast.MethodCallExpr:
		if e.EnumName != "" {
			payload := -1
			if len(e.Args) > 0 {
				payload = c.compileExpr(e.Args[0])
			}
			tag, tagType, ok := c.enumTagInfo(e.EnumName, e.Method)
			if !ok {
				c.diag.Add(e.Span(), fmt.Sprintf("unknown enum variant '%s.%s'", e.EnumName, e.Method))
				return c.constZero()
			}
			dst := c.newTemp()
			c.emit(&ir.MakeEnum{Dst: dst, Name: e.EnumName, Variant: e.Method, Tag: tag, TagType: tagType, Payload: payload})
			return dst
		}
		if e.ResolvedName == "" {
			c.diag.Add(e.Span(), "unresolved method call")
			return c.constZero()
		}
		args := make([]int, 0, len(e.Args)+1)
		if e.ResolvedSelf {
			recv := c.compileExpr(e.Receiver)
			args = append(args, recv)
		}
		for _, arg := range e.Args {
			args = append(args, c.compileExpr(arg))
		}
		dst := c.newTemp()
		c.emit(&ir.Call{Dst: dst, Callee: e.ResolvedName, Args: args})
		return dst
	case *ast.StructLit:
		fields := make([]ir.StructFieldInit, 0, len(e.Fields))
		for _, f := range e.Fields {
			src := c.compileExpr(f.Value)
			fields = append(fields, ir.StructFieldInit{Name: f.Name, Src: src})
		}
		dst := c.newTemp()
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
					tag, tagType, ok := c.enumTagInfo(ident.Name, e.Field)
					if !ok {
						c.diag.Add(e.Span(), fmt.Sprintf("unknown enum variant '%s.%s'", ident.Name, e.Field))
						return c.constZero()
					}
					dst := c.newTemp()
					c.emit(&ir.MakeEnum{Dst: dst, Name: ident.Name, Variant: e.Field, Tag: tag, TagType: tagType, Payload: -1})
					return dst
				}
			}
		}
		recv := c.compileExpr(e.Receiver)
		dst := c.newTemp()
		c.emit(&ir.GetField{Dst: dst, Src: recv, Field: e.Field})
		return dst
	case *ast.IndexExpr:
		recv := c.compileExpr(e.Receiver)
		index := c.compileExpr(e.Index)
		dst := c.newTemp()
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
		payload := -1
		if e.Arg != nil {
			payload = c.compileExpr(e.Arg)
		} else if variant.Payload != nil {
			c.diag.Add(e.Span(), "missing payload for enum variant")
		}
		tag, tagType, ok := c.enumTagInfo(e.EnumName, e.Variant)
		if !ok {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown enum variant '%s.%s'", e.EnumName, e.Variant))
			return c.constZero()
		}
		dst := c.newTemp()
		c.emit(&ir.MakeEnum{Dst: dst, Name: e.EnumName, Variant: e.Variant, Tag: tag, TagType: tagType, Payload: payload})
		return dst
	default:
		c.diag.Add(expr.Span(), "unsupported expression in stage 0")
		return c.constZero()
	}
}
