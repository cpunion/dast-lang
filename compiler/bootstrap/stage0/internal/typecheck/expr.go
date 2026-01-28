package typecheck

import (
	"fmt"
	"strconv"

	"dastlang/internal/ast"
)

func (c *Checker) checkExpr(expr ast.Expr) Type {
	switch e := expr.(type) {
	case *ast.IntLit:
		return Type{Kind: TypeInt, Name: "int"}
	case *ast.BoolLit:
		return Type{Kind: TypeBool, Name: "bool"}
	case *ast.StringLit:
		return Type{Kind: TypeString, Name: "String"}
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
	case *ast.TupleLit:
		if len(e.Elems) == 0 {
			return Type{Kind: TypeUnit}
		}
		var elems []Type
		for _, el := range e.Elems {
			elems = append(elems, c.checkExpr(el))
		}
		return Type{Kind: TypeTuple, Elems: elems}
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
	case *ast.CompileExpr:
		c.diag.Add(e.Span(), "compile! must be expanded before typecheck")
		return Type{Kind: TypeInvalid}
	case *ast.QuoteExpr:
		for _, part := range e.Parts {
			if part.Expr == nil {
				continue
			}
			t := c.checkExpr(part.Expr)
			if !isAstType(t) && t.Kind != TypeInvalid {
				c.diag.Add(part.Expr.Span(), "quote splice expects ast")
			}
		}
		switch e.Kind {
		case ast.QuoteExprKind:
			return Type{Kind: TypeAstExpr}
		case ast.QuoteStmtKind:
			return Type{Kind: TypeAstStmt}
		case ast.QuoteItemKind:
			return Type{Kind: TypeAstItem}
		case ast.QuoteBlockKind:
			return Type{Kind: TypeAstBlock}
		default:
			return Type{Kind: TypeInvalid}
		}
	case *ast.MacroCallExpr:
		c.diag.Add(e.Span(), "macro call must be expanded before typecheck")
		return Type{Kind: TypeInvalid}
	case *ast.RefExpr:
		baseType, mutable, ok := c.checkRefTarget(e.Expr)
		if !ok {
			return Type{Kind: TypeInvalid}
		}
		if baseType.Kind == TypeString || baseType.Kind == TypeStr {
			if e.Mutable {
				c.diag.Add(e.Span(), "cannot take &mut of string")
			}
			return Type{Kind: TypeStr, Name: "str", Ref: true}
		}
		if e.Mutable && !mutable {
			c.diag.Add(e.Span(), "cannot take &mut of immutable value")
		}
		refType := baseType
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
				return Type{Kind: TypeString, Name: "String"}
			}
			if lhs.Kind != TypeInvalid && rhs.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "'+' requires int or String/str operands")
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
			if !typesEqual(lhs, rhs) && !(isString(lhs) && isString(rhs)) && lhs.Kind != TypeInvalid && rhs.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "equality operands must have same type")
			}
			if !isComparable(lhs) && lhs.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "equality only supports int/bool/String/str")
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
			return c.checkCallExpr(e, sig)
		}
		if v, ok := c.env.lookup(e.Callee); ok && v.Type.Kind == TypeClosure {
			for _, arg := range e.Args {
				c.checkExpr(arg)
			}
			return Type{Kind: TypeUnit}
		}
		if _, ok := c.builtins[e.Callee]; ok {
			switch e.Callee {
			case "ast_expr":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "ast_expr expects 1 argument")
					return Type{Kind: TypeAstExpr}
				}
				argType := c.checkExpr(e.Args[0])
				if !isString(argType) && argType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "ast_expr expects String/str")
				}
				return Type{Kind: TypeAstExpr}
			case "ast_stmt":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "ast_stmt expects 1 argument")
					return Type{Kind: TypeAstStmt}
				}
				argType := c.checkExpr(e.Args[0])
				if !isString(argType) && argType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "ast_stmt expects String/str")
				}
				return Type{Kind: TypeAstStmt}
			case "ast_item":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "ast_item expects 1 argument")
					return Type{Kind: TypeAstItem}
				}
				argType := c.checkExpr(e.Args[0])
				if !isString(argType) && argType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "ast_item expects String/str")
				}
				return Type{Kind: TypeAstItem}
			case "ast_block":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "ast_block expects 1 argument")
					return Type{Kind: TypeAstBlock}
				}
				argType := c.checkExpr(e.Args[0])
				if !isString(argType) && argType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "ast_block expects String/str")
				}
				return Type{Kind: TypeAstBlock}
			case "ast_to_string":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "ast_to_string expects 1 argument")
					return Type{Kind: TypeString, Name: "String"}
				}
				argType := c.checkExpr(e.Args[0])
				switch argType.Kind {
				case TypeAstExpr, TypeAstStmt, TypeAstItem, TypeAstBlock, TypeInvalid:
				default:
					c.diag.Add(e.Args[0].Span(), "ast_to_string expects ast")
				}
				return Type{Kind: TypeString, Name: "String"}
			case "gensym", "bind":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), e.Callee+" expects 1 argument")
					return Type{Kind: TypeAstExpr}
				}
				argType := c.checkExpr(e.Args[0])
				if !isString(argType) && argType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), e.Callee+" expects String/str")
				}
				return Type{Kind: TypeAstExpr}
			case "len":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "len expects 1 argument")
					return Type{Kind: TypeInt, Name: "int"}
				}
				argType := c.checkExpr(e.Args[0])
				if !argType.Ref {
					c.diag.Add(e.Span(), "len expects reference to String/str or array")
					return Type{Kind: TypeInt, Name: "int"}
				}
				argType = derefType(argType)
				if argType.Kind != TypeString && argType.Kind != TypeStr && argType.Kind != TypeArray {
					c.diag.Add(e.Span(), "len expects reference to String/str or array")
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
					c.diag.Add(e.Args[0].Span(), "char_at expects String/str")
				}
				if !isInt(idxType) && idxType.Kind != TypeInvalid {
					c.diag.Add(e.Args[1].Span(), "char_at expects int index")
				}
				return Type{Kind: TypeInt, Name: "int"}
			case "substr":
				if len(e.Args) != 3 {
					c.diag.Add(e.Span(), "substr expects 3 arguments")
					return Type{Kind: TypeString, Name: "String"}
				}
				strType := c.checkExpr(e.Args[0])
				startType := c.checkExpr(e.Args[1])
				lenType := c.checkExpr(e.Args[2])
				if !isString(strType) && strType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "substr expects String/str")
				}
				if !isInt(startType) && startType.Kind != TypeInvalid {
					c.diag.Add(e.Args[1].Span(), "substr expects int start")
				}
				if !isInt(lenType) && lenType.Kind != TypeInvalid {
					c.diag.Add(e.Args[2].Span(), "substr expects int length")
				}
				return Type{Kind: TypeString, Name: "String"}
			case "string_clone":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "string_clone expects 1 argument")
					return Type{Kind: TypeString, Name: "String"}
				}
				argType := c.checkExpr(e.Args[0])
				if !isString(argType) && argType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "string_clone expects String/str or &str")
				}
				return Type{Kind: TypeString, Name: "String"}
			case "read_file":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "read_file expects 1 argument")
					return Type{Kind: TypeString, Name: "String"}
				}
				argType := c.checkExpr(e.Args[0])
				if !isString(argType) && argType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "read_file expects String/str path")
				}
				return Type{Kind: TypeString, Name: "String"}
			case "read_dir":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "read_dir expects 1 argument")
				}
				argType := c.checkExpr(e.Args[0])
				if !isString(argType) && argType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "read_dir expects String/str path")
				}
				elem := Type{Kind: TypeString, Name: "String"}
				return Type{Kind: TypeArray, Elem: &elem}
			case "write_file":
				if len(e.Args) != 2 {
					c.diag.Add(e.Span(), "write_file expects 2 arguments")
					return Type{Kind: TypeUnit}
				}
				pathType := c.checkExpr(e.Args[0])
				dataType := c.checkExpr(e.Args[1])
				if !isString(pathType) && pathType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "write_file expects String/str path")
				}
				if !isString(dataType) && dataType.Kind != TypeInvalid {
					c.diag.Add(e.Args[1].Span(), "write_file expects String/str data")
				}
				return Type{Kind: TypeUnit}
			case "args":
				if len(e.Args) != 0 {
					c.diag.Add(e.Span(), "args expects no arguments")
				}
				elem := Type{Kind: TypeString, Name: "String"}
				return Type{Kind: TypeArray, Elem: &elem}
			case "read_line":
				if len(e.Args) != 0 {
					c.diag.Add(e.Span(), "read_line expects no arguments")
				}
				return Type{Kind: TypeString, Name: "String"}
			case "read_bytes":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "read_bytes expects 1 argument")
					return Type{Kind: TypeString, Name: "String"}
				}
				argType := c.checkExpr(e.Args[0])
				if !isInt(argType) && argType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "read_bytes expects int count")
				}
				return Type{Kind: TypeString, Name: "String"}
			case "mkdir":
				if len(e.Args) != 1 {
					c.diag.Add(e.Span(), "mkdir expects 1 argument")
					return Type{Kind: TypeUnit}
				}
				argType := c.checkExpr(e.Args[0])
				if !isString(argType) && argType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "mkdir expects String/str path")
				}
				return Type{Kind: TypeUnit}
			case "exec":
				if len(e.Args) != 2 {
					c.diag.Add(e.Span(), "exec expects 2 arguments")
					return Type{Kind: TypeInt, Name: "int"}
				}
				cmdType := c.checkExpr(e.Args[0])
				if !isString(cmdType) && cmdType.Kind != TypeInvalid {
					c.diag.Add(e.Args[0].Span(), "exec expects String/str command")
				}
				argsType := c.checkExpr(e.Args[1])
				if argsType.Kind != TypeArray || argsType.Elem == nil || argsType.Elem.Kind != TypeString {
					if argsType.Kind != TypeInvalid {
						c.diag.Add(e.Args[1].Span(), "exec expects [String] args")
					}
				}
				return Type{Kind: TypeInt, Name: "int"}
			default:
				for _, arg := range e.Args {
					c.checkExpr(arg)
				}
				return Type{Kind: TypeUnit}
			}
		}
		c.diag.Add(e.Span(), fmt.Sprintf("undefined function '%s'", e.Callee))
		return Type{Kind: TypeInvalid}
	case *ast.ClosureExpr:
		c.env.push()
		for _, param := range e.Params {
			if param.Type == nil {
				c.diag.Add(param.Span, fmt.Sprintf("closure parameter '%s' requires type annotation", param.Name))
				c.env.declare(param.Name, VarInfo{Type: Type{Kind: TypeInvalid}, Mutable: false})
				continue
			}
			t := c.fromAstType(*param.Type)
			c.env.declare(param.Name, VarInfo{Type: t, Mutable: false})
		}
		bodyType := c.checkExpr(e.Body)
		if e.ReturnType != nil {
			rt := c.fromAstType(*e.ReturnType)
			if !typesAssignable(bodyType, rt) && bodyType.Kind != TypeInvalid && rt.Kind != TypeInvalid {
				c.diag.Add(e.Span(), fmt.Sprintf("closure expects return type %s, got %s", rt.String(), bodyType.String()))
			}
		}
		c.env.pop()
		return Type{Kind: TypeClosure}
	case *ast.MethodCallExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			if _, ok := c.env.lookup(ident.Name); !ok {
				if enumDecl, ok := c.enums[ident.Name]; ok {
					variant := findVariant(enumDecl, e.Method)
					if variant != nil {
						if len(e.Args) > 1 {
							c.diag.Add(e.Span(), "variant expects single payload")
						}
						expected, _ := c.currentExpected()
						var arg ast.Expr
						if len(e.Args) > 0 {
							arg = e.Args[0]
						}
						typ := c.checkEnumVariantCall(ident.Name, nil, e.Method, arg, e, expected)
						if typ.Kind == TypeEnum {
							e.EnumName = typ.Name
						}
						return typ
					}
				}
				if sig := c.lookupMethod(ident.Name, e.Method); sig != nil {
					ret := c.checkStaticMethodCall(sig, e.Args, e.Span())
					e.ResolvedName = ident.Name + "." + sig.Name
					e.ResolvedSelf = false
					return ret
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
			if base.Kind == TypeStruct {
				if baseName, ok := c.structInstBase[base.Name]; ok {
					sig = c.lookupMethod(baseName, e.Method)
				}
			} else if base.Kind == TypeEnum {
				if baseName, ok := c.enumInstBase[base.Name]; ok {
					sig = c.lookupMethod(baseName, e.Method)
				}
			}
		}
		if sig == nil {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown method '%s' on '%s'", e.Method, base.Name))
			return Type{Kind: TypeInvalid}
		}
		if !sig.HasSelf {
			c.diag.Add(e.Span(), "method requires no self; call as Type.method()")
			return sig.Return
		}
		ret := c.checkMethodCall(sig, recvType, e.Args, e.Span())
		resolvedType := base.Name
		switch base.Kind {
		case TypeStruct:
			if _, ok := c.structInstBase[resolvedType]; ok {
				// already an instanced name
			} else if len(base.Args) > 0 && isConcreteArgs(base.Args) {
				resolvedType = c.ensureStructInstance(base.Name, base.Args)
			}
		case TypeEnum:
			if _, ok := c.enumInstBase[resolvedType]; ok {
				// already an instanced name
			} else if len(base.Args) > 0 && isConcreteArgs(base.Args) {
				resolvedType = c.ensureEnumInstance(base.Name, base.Args)
			}
		}
		e.ResolvedName = resolvedType + "." + sig.Name
		e.ResolvedSelf = true
		return ret
	case *ast.StructLit:
		return c.checkStructLit(e)
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
						return Type{Kind: TypeInvalid}
					}
					expected, _ := c.currentExpected()
					typ := c.checkEnumVariantCall(ident.Name, nil, e.Field, nil, e, expected)
					if typ.Kind == TypeEnum && ident.Name != typ.Name {
						ident.Name = typ.Name
					}
					return typ
				}
			}
		}
		recvType := c.checkExpr(e.Receiver)
		if recvType.Ref {
			recvType = derefType(recvType)
		}
		if recvType.Kind == TypeTuple {
			idx, err := strconv.Atoi(e.Field)
			if err != nil || idx < 0 || idx >= len(recvType.Elems) {
				c.diag.Add(e.Span(), "tuple index out of range")
				return Type{Kind: TypeInvalid}
			}
			return recvType.Elems[idx]
		}
		if recvType.Kind != TypeStruct {
			if recvType.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "field access requires struct")
			}
			return Type{Kind: TypeInvalid}
		}
		decl, subst, ok := c.resolveStructDecl(recvType)
		if !ok {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown struct '%s'", recvType.Name))
			return Type{Kind: TypeInvalid}
		}
		field := findField(decl, e.Field)
		if field == nil {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown field '%s'", e.Field))
			return Type{Kind: TypeInvalid}
		}
		fieldAst := field.Type
		if len(subst) > 0 {
			fieldAst = c.cloneType(fieldAst, subst)
		}
		return c.fromAstType(fieldAst)
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
		return c.checkEnumVariantExpr(e)
	case *ast.BlockExpr:
		return c.checkBlockExpr(e.Block)
	case *ast.LoopExpr:
		return c.checkLoopExpr(e)
	case *ast.IfExpr:
		condType := c.checkExpr(e.Cond)
		if !isBool(condType) && condType.Kind != TypeInvalid {
			c.diag.Add(e.Cond.Span(), "if condition must be bool")
		}
		expected, hasExpected := c.currentExpected()
		var thenType Type
		if hasExpected {
			thenType = c.checkExprWithExpected(e.Then, expected)
		} else {
			thenType = c.checkExpr(e.Then)
		}
		elseType := Type{Kind: TypeUnit}
		if e.Else != nil {
			if hasExpected {
				elseType = c.checkExprWithExpected(e.Else, expected)
			} else {
				elseType = c.checkExpr(e.Else)
			}
		}
		if e.Else != nil && !typesEqual(thenType, elseType) && thenType.Kind != TypeInvalid && elseType.Kind != TypeInvalid {
			c.diag.Add(e.Span(), fmt.Sprintf("if branches must have same type: %s vs %s", thenType.String(), elseType.String()))
		}
		if e.Else == nil {
			return Type{Kind: TypeUnit}
		}
		return thenType
	case *ast.MatchExpr:
		scrutType := c.checkExpr(e.Expr)
		if scrutType.Ref {
			c.diag.Add(e.Expr.Span(), "match requires non-reference expression")
			scrutType = derefType(scrutType)
		}
		out := Type{Kind: TypeUnit}
		expected, hasExpected := c.currentExpected()
		for i, arm := range e.Arms {
			c.env.push()
			c.checkPattern(scrutType, arm.Pattern)
			if arm.Guard != nil {
				guardType := c.checkExpr(arm.Guard)
				if !isBool(guardType) && guardType.Kind != TypeInvalid {
					c.diag.Add(arm.Guard.Span(), "match guard must be bool")
				}
			}
			var armType Type
			if hasExpected {
				armType = c.checkBlockExprExpected(arm.Body, expected)
			} else {
				armType = c.checkBlockExpr(arm.Body)
			}
			if i == 0 {
				out = armType
			} else if !typesEqual(out, armType) && out.Kind != TypeInvalid && armType.Kind != TypeInvalid {
				c.diag.Add(arm.SpanInfo, fmt.Sprintf("match arms must have same type: %s vs %s", out.String(), armType.String()))
			}
			c.env.pop()
		}
		return out
	default:
		c.diag.Add(expr.Span(), "unsupported expression in stage 0")
		return Type{Kind: TypeInvalid}
	}
}

func (c *Checker) checkRefTarget(expr ast.Expr) (Type, bool, bool) {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		info, ok := c.env.lookup(e.Name)
		if !ok {
			if _, ok := c.consts[e.Name]; ok {
				c.diag.Add(e.Span(), fmt.Sprintf("cannot take reference to const '%s'", e.Name))
			} else {
				c.diag.Add(e.Span(), fmt.Sprintf("undefined variable '%s'", e.Name))
			}
			return Type{Kind: TypeInvalid}, false, false
		}
		return info.Type, info.Mutable, true
	case *ast.AccessExpr:
		recvType, recvMut, ok := c.checkRefTarget(e.Receiver)
		if !ok {
			return Type{Kind: TypeInvalid}, false, false
		}
		if recvType.Ref {
			recvMut = recvType.Mut
			recvType = derefType(recvType)
		}
		if recvType.Kind != TypeStruct {
			if recvType.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "field access requires struct")
			}
			return Type{Kind: TypeInvalid}, recvMut, false
		}
		decl, subst, ok := c.resolveStructDecl(recvType)
		if !ok {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown struct '%s'", recvType.Name))
			return Type{Kind: TypeInvalid}, recvMut, false
		}
		field := findField(decl, e.Field)
		if field == nil {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown field '%s'", e.Field))
			return Type{Kind: TypeInvalid}, recvMut, false
		}
		fieldAst := field.Type
		if len(subst) > 0 {
			fieldAst = c.cloneType(fieldAst, subst)
		}
		return c.fromAstType(fieldAst), recvMut, true
	case *ast.IndexExpr:
		recvType, recvMut, ok := c.checkRefTarget(e.Receiver)
		if !ok {
			return Type{Kind: TypeInvalid}, false, false
		}
		indexType := c.checkExpr(e.Index)
		if !isInt(indexType) && indexType.Kind != TypeInvalid {
			c.diag.Add(e.Index.Span(), "index requires int")
		}
		if recvType.Ref {
			recvMut = recvType.Mut
			recvType = derefType(recvType)
		}
		if recvType.Kind != TypeArray || recvType.Elem == nil {
			if recvType.Kind != TypeInvalid {
				c.diag.Add(e.Span(), "indexing requires array")
			}
			return Type{Kind: TypeInvalid}, recvMut, false
		}
		return *recvType.Elem, recvMut, true
	case *ast.DerefExpr:
		operand := c.checkExpr(e.Expr)
		if !operand.Ref {
			c.diag.Add(e.Span(), "deref requires reference")
			return Type{Kind: TypeInvalid}, false, false
		}
		return derefType(operand), operand.Mut, true
	default:
		c.diag.Add(expr.Span(), "reference target must be identifier, field, or index in stage 0")
		return Type{Kind: TypeInvalid}, false, false
	}
}
