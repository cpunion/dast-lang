package typecheck

import (
	"fmt"

	"dastlang/internal/ast"
)

func (c *Checker) checkExpr(expr ast.Expr) Type {
	switch e := expr.(type) {
	case *ast.IntLit:
		return Type{Kind: TypeInt, Name: "int"}
	case *ast.BoolLit:
		return Type{Kind: TypeBool, Name: "bool"}
	case *ast.StringLit:
		return Type{Kind: TypeString, Name: "string"}
	case *ast.ArrayLit:
		if len(e.Elems) == 0 {
			return Type{Kind: TypeArray, Elem: &Type{Kind: TypeInvalid}}
		}
		elemType := c.checkExpr(e.Elems[0])
		for _, elem := range e.Elems[1:] {
			t := c.checkExpr(elem)
			if !typesEqual(t, elemType) && t.Kind != TypeInvalid && elemType.Kind != TypeInvalid {
				c.diag.Add(elem.Span(), fmt.Sprintf("array element expects %s, got %s", elemType.String(), t.String()))
			}
		}
		return Type{Kind: TypeArray, Elem: &elemType}
	case *ast.IdentExpr:
		if info, ok := c.env.lookup(e.Name); ok {
			return info.Type
		}
		if cinfo, ok := c.consts[e.Name]; ok {
			return cinfo.Type
		}
		if _, ok := c.enums[e.Name]; ok {
			return Type{Kind: TypeEnum, Name: e.Name}
		}
		if _, ok := c.structs[e.Name]; ok {
			return Type{Kind: TypeStruct, Name: e.Name}
		}
		c.diag.Add(e.Span(), fmt.Sprintf("undefined variable '%s'", e.Name))
		return Type{Kind: TypeInvalid}
	case *ast.RefExpr:
		ident, ok := e.Expr.(*ast.IdentExpr)
		if !ok {
			c.diag.Add(e.Span(), "reference target must be identifier in stage 0")
			return Type{Kind: TypeInvalid}
		}
		info, ok := c.env.lookup(ident.Name)
		if !ok {
			if _, ok := c.consts[ident.Name]; ok {
				c.diag.Add(e.Span(), fmt.Sprintf("cannot take reference to const '%s'", ident.Name))
			} else {
				c.diag.Add(e.Span(), fmt.Sprintf("undefined variable '%s'", ident.Name))
			}
			return Type{Kind: TypeInvalid}
		}
		if e.Mutable && !info.Mutable {
			c.diag.Add(e.Span(), fmt.Sprintf("cannot take &mut of immutable '%s'", ident.Name))
		}
		refType := info.Type
		refType.Ref = true
		refType.Mut = e.Mutable
		return refType
	case *ast.DerefExpr:
		operand := c.checkExpr(e.Expr)
		if !operand.Ref {
			c.diag.Add(e.Span(), "deref requires reference")
			return Type{Kind: TypeInvalid}
		}
		return derefType(operand)
	case *ast.UnaryExpr:
		operand := c.checkExpr(e.Expr)
		switch e.Op {
		case "-":
			if !isInt(operand) && operand.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "unary '-' requires int")
			}
			return Type{Kind: TypeInt, Name: "int"}
		case "!":
			if !isBool(operand) && operand.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "unary '!' requires bool")
			}
			return Type{Kind: TypeBool, Name: "bool"}
		default:
			c.diag.Add(e.Span(), fmt.Sprintf("unknown unary op '%s'", e.Op))
			return Type{Kind: TypeInvalid}
		}
	case *ast.BinaryExpr:
		lhs := c.checkExpr(e.Left)
		rhs := c.checkExpr(e.Right)
		switch e.Op {
		case "+":
			if isInt(lhs) && isInt(rhs) {
				return Type{Kind: TypeInt, Name: "int"}
			}
			if isString(lhs) && isString(rhs) {
				return Type{Kind: TypeString, Name: "string"}
			}
			if lhs.Kind != TypeInvalid && rhs.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "'+' requires int or string operands")
			}
			return Type{Kind: TypeInvalid}
		case "-", "*", "/", "%":
			if !isInt(lhs) || !isInt(rhs) {
				if lhs.Kind != TypeInvalid && rhs.Kind != TypeInvalid {
					c.diag.Add(e.Span(), fmt.Sprintf("'%s' requires int operands", e.Op))
				}
				return Type{Kind: TypeInvalid}
			}
			return Type{Kind: TypeInt, Name: "int"}
		case "==", "!=":
			if !typesEqual(lhs, rhs) && lhs.Kind != TypeInvalid && rhs.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "equality operands must have same type")
			}
			if !isComparable(lhs) && lhs.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "equality only supports int/bool/string")
			}
			return Type{Kind: TypeBool, Name: "bool"}
		case "<", "<=", ">", ">=":
			if !isInt(lhs) || !isInt(rhs) {
				if lhs.Kind != TypeInvalid && rhs.Kind != TypeInvalid {
					c.diag.Add(e.Span(), "comparison requires int operands")
				}
				return Type{Kind: TypeInvalid}
			}
			return Type{Kind: TypeBool, Name: "bool"}
		case "&&", "||":
			if !isBool(lhs) || !isBool(rhs) {
				if lhs.Kind != TypeInvalid && rhs.Kind != TypeInvalid {
					c.diag.Add(e.Span(), "logical op requires bool operands")
				}
				return Type{Kind: TypeInvalid}
			}
			return Type{Kind: TypeBool, Name: "bool"}
		default:
			c.diag.Add(e.Span(), fmt.Sprintf("unknown binary op '%s'", e.Op))
			return Type{Kind: TypeInvalid}
		}
	case *ast.CallExpr:
		if sig, ok := c.funcs[e.Callee]; ok {
			if len(e.Args) != len(sig.Params) {
				c.diag.Add(e.Span(), fmt.Sprintf("function '%s' expects %d args, got %d", e.Callee, len(sig.Params), len(e.Args)))
				return sig.Return
			}
			for i, arg := range e.Args {
				argType := c.checkExpr(arg)
				if !typesAssignable(argType, sig.Params[i]) && argType.Kind != TypeInvalid && sig.Params[i].Kind != TypeInvalid {
					c.diag.Add(arg.Span(), fmt.Sprintf("argument %d expects %s, got %s", i+1, sig.Params[i].String(), argType.String()))
					break
				}
			}
			return sig.Return
		}
		if _, ok := c.builtins[e.Callee]; ok {
			switch e.Callee {
			case "len":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "len expects 1 argument")
					return Type{Kind: TypeInt, Name: "int"}
				}
				argType := c.checkExpr(e.Args[0])
				if argType.Ref {
					argType = derefType(argType)
				}
				if argType.Kind != TypeString && argType.Kind != TypeArray {
					c.diag.Add(e.Span(), "len expects string or array")
				}
				return Type{Kind: TypeInt, Name: "int"}
			case "push":
				if len(e.Args) != 2 {
					c.diag.Add(e.Span(), "push expects 2 arguments")
					return Type{Kind: TypeUnit}
				}
				targetType := c.checkExpr(e.Args[0])
				if !targetType.Ref || !targetType.Mut {
					c.diag.Add(e.Args[0].Span(), "push expects &mut array")
					return Type{Kind: TypeUnit}
				}
				base := derefType(targetType)
				if base.Kind != TypeArray || base.Elem == nil {
					c.diag.Add(e.Args[0].Span(), "push expects &mut [T]")
					return Type{Kind: TypeUnit}
				}
				valType := c.checkExpr(e.Args[1])
				if !typesAssignable(valType, *base.Elem) && valType.Kind != TypeInvalid && base.Elem.Kind != TypeInvalid {
					c.diag.Add(e.Args[1].Span(), fmt.Sprintf("push expects %s, got %s", base.Elem.String(), valType.String()))
				}
				return Type{Kind: TypeUnit}
			case "pop":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "pop expects 1 argument")
					return Type{Kind: TypeUnit}
				}
				targetType := c.checkExpr(e.Args[0])
				if !targetType.Ref || !targetType.Mut {
					c.diag.Add(e.Args[0].Span(), "pop expects &mut array")
					return Type{Kind: TypeUnit}
				}
				base := derefType(targetType)
				if base.Kind != TypeArray || base.Elem == nil {
					c.diag.Add(e.Args[0].Span(), "pop expects &mut [T]")
					return Type{Kind: TypeUnit}
				}
				return *base.Elem
			case "char_at":
				if len(e.Args) != 2 {
					c.diag.Add(e.Span(), "char_at expects 2 arguments")
					return Type{Kind: TypeInt, Name: "int"}
				}
				strType := c.checkExpr(e.Args[0])
				idxType := c.checkExpr(e.Args[1])
				if !isString(strType) && strType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "char_at expects string")
				}
				if !isInt(idxType) && idxType.Kind != TypeInvalid {
					c.diag.Add(e.Args[1].Span(), "char_at expects int index")
				}
				return Type{Kind: TypeInt, Name: "int"}
			case "substr":
				if len(e.Args) != 3 {
					c.diag.Add(e.Span(), "substr expects 3 arguments")
					return Type{Kind: TypeString, Name: "string"}
				}
				strType := c.checkExpr(e.Args[0])
				startType := c.checkExpr(e.Args[1])
				lenType := c.checkExpr(e.Args[2])
				if !isString(strType) && strType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "substr expects string")
				}
				if !isInt(startType) && startType.Kind != TypeInvalid {
					c.diag.Add(e.Args[1].Span(), "substr expects int start")
				}
				if !isInt(lenType) && lenType.Kind != TypeInvalid {
					c.diag.Add(e.Args[2].Span(), "substr expects int length")
				}
				return Type{Kind: TypeString, Name: "string"}
			case "read_file":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "read_file expects 1 argument")
					return Type{Kind: TypeString, Name: "string"}
				}
				argType := c.checkExpr(e.Args[0])
				if !isString(argType) && argType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "read_file expects string path")
				}
				return Type{Kind: TypeString, Name: "string"}
			case "read_dir":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "read_dir expects 1 argument")
				}
				argType := c.checkExpr(e.Args[0])
				if !isString(argType) && argType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "read_dir expects string path")
				}
				elem := Type{Kind: TypeString, Name: "string"}
				return Type{Kind: TypeArray, Elem: &elem}
			case "write_file":
				if len(e.Args) != 2 {
					c.diag.Add(e.Span(), "write_file expects 2 arguments")
					return Type{Kind: TypeUnit}
				}
				pathType := c.checkExpr(e.Args[0])
				dataType := c.checkExpr(e.Args[1])
				if !isString(pathType) && pathType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "write_file expects string path")
				}
				if !isString(dataType) && dataType.Kind != TypeInvalid {
					c.diag.Add(e.Args[1].Span(), "write_file expects string data")
				}
				return Type{Kind: TypeUnit}
			case "args":
				if len(e.Args) != 0 {
					c.diag.Add(e.Span(), "args expects no arguments")
				}
				elem := Type{Kind: TypeString, Name: "string"}
				return Type{Kind: TypeArray, Elem: &elem}
			default:
				for _, arg := range e.Args {
					c.checkExpr(arg)
				}
				return Type{Kind: TypeUnit}
			}
		}
		c.diag.Add(e.Span(), fmt.Sprintf("undefined function '%s'", e.Callee))
		return Type{Kind: TypeInvalid}
	case *ast.MethodCallExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			if _, ok := c.env.lookup(ident.Name); !ok {
				if enumDecl, ok := c.enums[ident.Name]; ok {
					variant := findVariant(enumDecl, e.Method)
					if variant != nil {
						if variant.Payload == nil && len(e.Args) > 0 {
							c.diag.Add(e.Span(), "variant has no payload")
						}
						if variant.Payload != nil {
							if len(e.Args) != 1 {
								c.diag.Add(e.Span(), "variant expects payload")
							} else {
								argType := c.checkExpr(e.Args[0])
								payloadType := c.fromAstType(*variant.Payload)
								if !typesAssignable(argType, payloadType) && argType.Kind != TypeInvalid && payloadType.Kind != TypeInvalid {
									c.diag.Add(e.Span(), fmt.Sprintf("variant expects %s, got %s", payloadType.String(), argType.String()))
								}
							}
						}
						e.EnumName = ident.Name
						return Type{Kind: TypeEnum, Name: ident.Name}
					}
				}
				if sig := c.lookupMethod(ident.Name, e.Method); sig != nil {
					if sig.HasSelf {
						c.diag.Add(e.Span(), "static call to method that requires self")
						return sig.Return
					}
					if len(e.Args) != len(sig.Params) {
						c.diag.Add(e.Span(), fmt.Sprintf("method '%s' expects %d args, got %d", e.Method, len(sig.Params), len(e.Args)))
					}
					for i, arg := range e.Args {
						argType := c.checkExpr(arg)
						if i < len(sig.Params) && !typesAssignable(argType, sig.Params[i]) && argType.Kind != TypeInvalid && sig.Params[i].Kind != TypeInvalid {
							c.diag.Add(arg.Span(), fmt.Sprintf("argument %d expects %s, got %s", i+1, sig.Params[i].String(), argType.String()))
							break
						}
					}
					e.ResolvedName = sig.FuncName
					e.ResolvedSelf = false
					return sig.Return
				}
			}
		}
		recvType := c.checkExpr(e.Receiver)
		base := recvType
		if base.Ref {
			base = derefType(base)
		}
		if base.Kind != TypeStruct && base.Kind != TypeEnum {
			if base.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "method call requires struct or enum receiver")
			}
			return Type{Kind: TypeInvalid}
		}
		sig := c.lookupMethod(base.Name, e.Method)
		if sig == nil {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown method '%s' on '%s'", e.Method, base.Name))
			return Type{Kind: TypeInvalid}
		}
		if !sig.HasSelf {
			c.diag.Add(e.Span(), "method requires no self; call as Type.method()")
			return sig.Return
		}
		if len(sig.Params) == 0 || !typesAssignable(recvType, sig.Params[0]) {
			c.diag.Add(e.Span(), "receiver type mismatch")
		}
		expectedArgs := len(sig.Params) - 1
		if len(e.Args) != expectedArgs {
			c.diag.Add(e.Span(), fmt.Sprintf("method '%s' expects %d args, got %d", e.Method, expectedArgs, len(e.Args)))
		}
		for i, arg := range e.Args {
			if i+1 >= len(sig.Params) {
				break
			}
			argType := c.checkExpr(arg)
			if !typesAssignable(argType, sig.Params[i+1]) && argType.Kind != TypeInvalid && sig.Params[i+1].Kind != TypeInvalid {
				c.diag.Add(arg.Span(), fmt.Sprintf("argument %d expects %s, got %s", i+1, sig.Params[i+1].String(), argType.String()))
				break
			}
		}
		e.ResolvedName = sig.FuncName
		e.ResolvedSelf = true
		return sig.Return
	case *ast.StructLit:
		decl, ok := c.structs[e.Name]
		if !ok {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown struct '%s'", e.Name))
			return Type{Kind: TypeInvalid}
		}
		seen := map[string]struct{}{}
		for _, f := range e.Fields {
			field := findField(decl, f.Name)
			if field == nil {
				c.diag.Add(f.Span, fmt.Sprintf("unknown field '%s'", f.Name))
				continue
			}
			if _, exists := seen[f.Name]; exists {
				c.diag.Add(f.Span, fmt.Sprintf("duplicate field '%s'", f.Name))
				continue
			}
			seen[f.Name] = struct{}{}
			valType := c.checkExpr(f.Value)
			fieldType := c.fromAstType(field.Type)
			if !typesAssignable(valType, fieldType) && valType.Kind != TypeInvalid && fieldType.Kind != TypeInvalid {
				c.diag.Add(f.Span, fmt.Sprintf("field '%s' expects %s, got %s", f.Name, fieldType.String(), valType.String()))
			}
		}
		for _, field := range decl.Fields {
			if _, ok := seen[field.Name]; !ok {
				c.diag.Add(e.Span(), fmt.Sprintf("missing field '%s'", field.Name))
			}
		}
		return Type{Kind: TypeStruct, Name: decl.Name}
	case *ast.AccessExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			if _, ok := c.env.lookup(ident.Name); !ok {
				if enumDecl, ok := c.enums[ident.Name]; ok {
					variant := findVariant(enumDecl, e.Field)
					if variant == nil {
						c.diag.Add(e.Span(), fmt.Sprintf("unknown variant '%s'", e.Field))
						return Type{Kind: TypeInvalid}
					}
					if variant.Payload != nil {
						c.diag.Add(e.Span(), "variant requires payload")
					}
					return Type{Kind: TypeEnum, Name: ident.Name}
				}
			}
		}
		recvType := c.checkExpr(e.Receiver)
		if recvType.Ref {
			recvType = derefType(recvType)
		}
		if recvType.Kind != TypeStruct {
			if recvType.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "field access requires struct")
			}
			return Type{Kind: TypeInvalid}
		}
		decl, ok := c.structs[recvType.Name]
		if !ok {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown struct '%s'", recvType.Name))
			return Type{Kind: TypeInvalid}
		}
		field := findField(decl, e.Field)
		if field == nil {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown field '%s'", e.Field))
			return Type{Kind: TypeInvalid}
		}
		return c.fromAstType(field.Type)
	case *ast.IndexExpr:
		recvType := c.checkExpr(e.Receiver)
		indexType := c.checkExpr(e.Index)
		if !isInt(indexType) && indexType.Kind != TypeInvalid {
			c.diag.Add(e.Index.Span(), "index requires int")
		}
		if recvType.Ref {
			recvType = derefType(recvType)
		}
		if recvType.Kind != TypeArray || recvType.Elem == nil {
			if recvType.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "indexing requires array")
			}
			return Type{Kind: TypeInvalid}
		}
		return *recvType.Elem
	case *ast.EnumVariantExpr:
		decl, ok := c.enums[e.EnumName]
		if !ok {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown enum '%s'", e.EnumName))
			return Type{Kind: TypeInvalid}
		}
		variant := findVariant(decl, e.Variant)
		if variant == nil {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown variant '%s'", e.Variant))
			return Type{Kind: TypeInvalid}
		}
		if variant.Payload == nil && e.Arg != nil {
			c.diag.Add(e.Span(), "variant has no payload")
		}
		if variant.Payload != nil && e.Arg == nil {
			c.diag.Add(e.Span(), "missing payload for enum variant")
		}
		if variant.Payload != nil && e.Arg != nil {
			argType := c.checkExpr(e.Arg)
			payloadType := c.fromAstType(*variant.Payload)
			if !typesAssignable(argType, payloadType) && argType.Kind != TypeInvalid && payloadType.Kind != TypeInvalid {
				c.diag.Add(e.Span(), fmt.Sprintf("payload expects %s, got %s", payloadType.String(), argType.String()))
			}
		}
		return Type{Kind: TypeEnum, Name: decl.Name}
	default:
		c.diag.Add(expr.Span(), "unsupported expression in stage 0")
		return Type{Kind: TypeInvalid}
	}
}
