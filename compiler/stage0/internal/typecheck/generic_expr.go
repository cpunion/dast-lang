package typecheck

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/source"
)

func isBorrowableExpr(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.IdentExpr, *ast.AccessExpr, *ast.IndexExpr:
		return true
	default:
		return false
	}
}

func (c *Checker) tryAutoBorrow(arg ast.Expr, argType Type, expect Type) (ast.Expr, Type) {
	if argType.Kind == TypeInvalid || expect.Kind == TypeInvalid {
		return arg, argType
	}
	if argType.Ref || !expect.Ref || expect.Mut || !isBorrowableExpr(arg) {
		return arg, argType
	}
	baseExpect := derefType(expect)
	if baseExpect.Kind == TypeStr && argType.Kind == TypeString {
		// allow auto-borrow String -> &str
	} else if !typesAssignable(argType, baseExpect) {
		return arg, argType
	}
	borrow := &ast.RefExpr{Mutable: false, Expr: arg, SpanInfo: arg.Span()}
	borrowType := c.checkExprWithExpected(borrow, expect)
	if typesAssignable(borrowType, expect) {
		return borrow, borrowType
	}
	return arg, argType
}

func (c *Checker) checkCallExpr(e *ast.CallExpr, sig *FuncSig) Type {
	if len(sig.TypeParams) == 0 {
		if len(e.Args) != len(sig.Params) {
			c.diag.Add(e.Span(), fmt.Sprintf("function '%s' expects %d args, got %d", e.Callee, len(sig.Params), len(e.Args)))
			return sig.Return
		}
		for i, arg := range e.Args {
			expect := sig.Params[i]
			argType := c.checkExprWithExpected(arg, expect)
			argExpr, borrowedType := c.tryAutoBorrow(arg, argType, expect)
			if argExpr != arg {
				e.Args[i] = argExpr
				argType = borrowedType
			}
			if !typesAssignable(argType, sig.Params[i]) && argType.Kind != TypeInvalid && sig.Params[i].Kind != TypeInvalid {
				c.diag.Add(arg.Span(), fmt.Sprintf("argument %d expects %s, got %s", i+1, sig.Params[i].String(), argType.String()))
				break
			}
		}
		return sig.Return
	}
	if len(e.Args) != len(sig.Params) {
		c.diag.Add(e.Span(), fmt.Sprintf("function '%s' expects %d args, got %d", e.Callee, len(sig.Params), len(e.Args)))
		return Type{Kind: TypeInvalid}
	}
	subst := map[string]Type{}
	var typeArgs []Type
	if len(e.TypeArgs) > 0 {
		if len(e.TypeArgs) != len(sig.TypeParams) {
			c.diag.Add(e.Span(), fmt.Sprintf("function '%s' expects %d type args, got %d", e.Callee, len(sig.TypeParams), len(e.TypeArgs)))
			return Type{Kind: TypeInvalid}
		}
		for _, arg := range e.TypeArgs {
			typeArgs = append(typeArgs, c.fromAstType(arg))
		}
		for i, p := range sig.TypeParams {
			subst[p.Name] = typeArgs[i]
		}
	} else {
		for i, arg := range e.Args {
			argType := c.checkExprWithExpected(arg, sig.Params[i])
			if !c.unifyType(sig.Params[i], argType, subst) {
				if argType.Kind != TypeInvalid && sig.Params[i].Kind != TypeInvalid {
					c.diag.Add(arg.Span(), fmt.Sprintf("argument %d expects %s, got %s", i+1, sig.Params[i].String(), argType.String()))
				}
				return Type{Kind: TypeInvalid}
			}
		}
		if expected, ok := c.currentExpected(); ok && expected.Kind != TypeInvalid {
			_ = c.unifyType(sig.Return, expected, subst)
		}
		for _, p := range sig.TypeParams {
			if _, ok := subst[p.Name]; !ok {
				c.diag.Add(e.Span(), fmt.Sprintf("cannot infer type parameter '%s' for '%s'", p.Name, sig.Name))
				return Type{Kind: TypeInvalid}
			}
		}
		for _, p := range sig.TypeParams {
			typeArgs = append(typeArgs, subst[p.Name])
		}
	}
	c.checkTypeParamBounds(sig.TypeParams, subst, e.Span())
	instRet := c.applySubst(sig.Return, subst)
	for i, arg := range e.Args {
		expect := c.applySubst(sig.Params[i], subst)
		argType := c.checkExprWithExpected(arg, expect)
		argExpr, borrowedType := c.tryAutoBorrow(arg, argType, expect)
		if argExpr != arg {
			e.Args[i] = argExpr
			argType = borrowedType
		}
		if !typesAssignable(argType, expect) && argType.Kind != TypeInvalid && expect.Kind != TypeInvalid {
			c.diag.Add(arg.Span(), fmt.Sprintf("argument %d expects %s, got %s", i+1, expect.String(), argType.String()))
			break
		}
	}
	if isConcreteArgs(typeArgs) {
		instName := c.ensureFuncInstance(sig.Name, typeArgs)
		if instName != sig.Name {
			e.Callee = instName
			e.TypeArgs = nil
		}
	}
	return instRet
}

func (c *Checker) checkStructLit(e *ast.StructLit) Type {
	decl, ok := c.structs[e.Name]
	if !ok {
		if alias, okAlias := c.aliases[e.Name]; okAlias {
			use := ast.Type{Name: e.Name, Args: e.TypeArgs, Span: e.SpanInfo}
			if expanded, okExp := c.expandAlias(alias, use); okExp {
				expanded = c.expandAliasType(expanded)
				if expanded.Name != "" && !expanded.IsArray && !expanded.IsTuple {
					e.Name = expanded.Name
					e.TypeArgs = expanded.Args
					decl, ok = c.structs[e.Name]
				}
			}
		}
		if !ok {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown struct '%s'", e.Name))
			return Type{Kind: TypeInvalid}
		}
	}
	restore := c.pushTypeParams(decl.TypeParams)
	defer c.popTypeParams(restore)
	subst := map[string]Type{}
	var typeArgs []Type
	if len(decl.TypeParams) > 0 {
		if len(e.TypeArgs) > 0 {
			if len(e.TypeArgs) != len(decl.TypeParams) {
				c.diag.Add(e.Span(), fmt.Sprintf("struct '%s' expects %d type args, got %d", e.Name, len(decl.TypeParams), len(e.TypeArgs)))
				return Type{Kind: TypeInvalid}
			}
			for _, arg := range e.TypeArgs {
				typeArgs = append(typeArgs, c.fromAstType(arg))
			}
			for i, p := range decl.TypeParams {
				subst[p.Name] = typeArgs[i]
			}
		}
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
		fieldType := c.fromAstType(field.Type)
		if len(decl.TypeParams) > 0 && len(e.TypeArgs) == 0 {
			argType := c.checkExprWithExpected(f.Value, fieldType)
			_ = c.unifyType(fieldType, argType, subst)
		}
	}
	if len(decl.TypeParams) > 0 && len(e.TypeArgs) == 0 {
		if expected, ok := c.currentExpected(); ok && expected.Kind == TypeStruct {
			exp := c.expandInstType(expected)
			if exp.Kind == TypeStruct && exp.Name == e.Name && len(exp.Args) == len(decl.TypeParams) {
				for i, p := range decl.TypeParams {
					if _, ok := subst[p.Name]; !ok {
						subst[p.Name] = exp.Args[i]
					}
				}
			}
		}
		for _, p := range decl.TypeParams {
			if _, ok := subst[p.Name]; !ok {
				c.diag.Add(e.Span(), fmt.Sprintf("cannot infer type parameter '%s' for '%s'", p.Name, e.Name))
				return Type{Kind: TypeInvalid}
			}
		}
		for _, p := range decl.TypeParams {
			typeArgs = append(typeArgs, subst[p.Name])
		}
	}
	if len(decl.TypeParams) == 0 && len(e.TypeArgs) > 0 {
		c.diag.Add(e.Span(), fmt.Sprintf("struct '%s' does not accept type args", e.Name))
	}
	c.checkTypeParamBounds(decl.TypeParams, subst, e.Span())
	for _, f := range e.Fields {
		field := findField(decl, f.Name)
		if field == nil {
			continue
		}
		fieldType := c.applySubst(c.fromAstType(field.Type), subst)
		valType := c.checkExprWithExpected(f.Value, fieldType)
		if !typesAssignable(valType, fieldType) && valType.Kind != TypeInvalid && fieldType.Kind != TypeInvalid {
			c.diag.Add(f.Span, fmt.Sprintf("field '%s' expects %s, got %s", f.Name, fieldType.String(), valType.String()))
		}
	}
	for _, field := range decl.Fields {
		if _, ok := seen[field.Name]; !ok {
			c.diag.Add(e.Span(), fmt.Sprintf("missing field '%s'", field.Name))
		}
	}
	name := e.Name
	if len(decl.TypeParams) > 0 && isConcreteArgs(typeArgs) {
		name = c.ensureStructInstance(e.Name, typeArgs)
		e.Name = name
		e.TypeArgs = nil
	}
	typ := Type{Kind: TypeStruct, Name: name}
	if len(decl.TypeParams) > 0 && !isConcreteArgs(typeArgs) {
		typ.Args = typeArgs
	}
	return typ
}

func (c *Checker) checkEnumVariantExpr(e *ast.EnumVariantExpr) Type {
	expected, _ := c.currentExpected()
	return c.checkEnumVariantCall(e.EnumName, e.TypeArgs, e.Variant, e.Arg, e, expected)
}

func (c *Checker) checkEnumVariantCall(enumName string, typeArgsAst []ast.Type, variantName string, arg ast.Expr, span ast.Expr, expected Type) Type {
	if enumName == "" && expected.Kind == TypeEnum {
		enumName = expected.Name
	}
	if enumName != "" && expected.Kind == TypeEnum && expected.Name != "" && expected.Name != enumName {
		if base, ok := c.enumInstBase[expected.Name]; ok && base == enumName {
			enumName = expected.Name
		}
	}
	if enumName == "" {
		c.diag.Add(span.Span(), "cannot resolve enum for variant")
		return Type{Kind: TypeInvalid}
	}
	if alias, okAlias := c.aliases[enumName]; okAlias {
		use := ast.Type{Name: enumName, Args: typeArgsAst, Span: span.Span()}
		if expanded, okExp := c.expandAlias(alias, use); okExp {
			expanded = c.expandAliasType(expanded)
			if expanded.Name != "" && !expanded.IsArray && !expanded.IsTuple {
				enumName = expanded.Name
				typeArgsAst = expanded.Args
			}
		}
	}
	decl, ok := c.enums[enumName]
	if !ok {
		c.diag.Add(span.Span(), fmt.Sprintf("unknown enum '%s'", enumName))
		return Type{Kind: TypeInvalid}
	}
	restore := c.pushTypeParams(decl.TypeParams)
	defer c.popTypeParams(restore)
	variant := findVariant(decl, variantName)
	if variant == nil {
		c.diag.Add(span.Span(), fmt.Sprintf("unknown variant '%s'", variantName))
		return Type{Kind: TypeInvalid}
	}
	subst := map[string]Type{}
	var typeArgs []Type
	if len(decl.TypeParams) > 0 {
		if len(typeArgsAst) > 0 {
			if len(typeArgsAst) != len(decl.TypeParams) {
				c.diag.Add(span.Span(), fmt.Sprintf("enum '%s' expects %d type args, got %d", enumName, len(decl.TypeParams), len(typeArgsAst)))
				return Type{Kind: TypeInvalid}
			}
			for _, ta := range typeArgsAst {
				typeArgs = append(typeArgs, c.fromAstType(ta))
			}
			for i, p := range decl.TypeParams {
				subst[p.Name] = typeArgs[i]
			}
		} else if expected.Kind == TypeEnum && expected.Name == enumName && len(expected.Args) == len(decl.TypeParams) {
			for i, p := range decl.TypeParams {
				subst[p.Name] = expected.Args[i]
			}
			typeArgs = append(typeArgs, expected.Args...)
		}
	}
	if variant.Payload == nil {
		if arg != nil {
			c.diag.Add(span.Span(), "variant has no payload")
		}
	} else {
		if arg == nil {
			c.diag.Add(span.Span(), "variant expects payload")
		} else {
			payloadType := c.fromAstType(*variant.Payload)
			argType := c.checkExprWithExpected(arg, payloadType)
			if len(decl.TypeParams) > 0 && len(typeArgsAst) == 0 {
				_ = c.unifyType(payloadType, argType, subst)
			}
			if !typesAssignable(argType, payloadType) && argType.Kind != TypeInvalid && payloadType.Kind != TypeInvalid {
				c.diag.Add(span.Span(), fmt.Sprintf("variant expects %s, got %s", payloadType.String(), argType.String()))
			}
		}
	}
	if len(decl.TypeParams) > 0 {
		for _, p := range decl.TypeParams {
			if _, ok := subst[p.Name]; !ok {
				c.diag.Add(span.Span(), fmt.Sprintf("cannot infer type parameter '%s' for '%s'", p.Name, enumName))
				return Type{Kind: TypeInvalid}
			}
		}
		if len(typeArgs) == 0 {
			for _, p := range decl.TypeParams {
				typeArgs = append(typeArgs, subst[p.Name])
			}
		}
		c.checkTypeParamBounds(decl.TypeParams, subst, span.Span())
	}
	name := enumName
	if len(decl.TypeParams) > 0 && isConcreteArgs(typeArgs) {
		name = c.ensureEnumInstance(enumName, typeArgs)
	}
	if variantExpr, ok := span.(*ast.EnumVariantExpr); ok {
		variantExpr.EnumName = name
		variantExpr.TypeArgs = nil
	}
	typ := Type{Kind: TypeEnum, Name: name}
	if len(decl.TypeParams) > 0 && !isConcreteArgs(typeArgs) {
		typ.Args = typeArgs
	}
	return typ
}

func (c *Checker) checkMethodCall(sig *MethodSig, recvType Type, args []ast.Expr, span source.Span) Type {
	if !sig.HasSelf {
		c.diag.Add(span, "method requires no self; call as Type.method()")
		return sig.Return
	}
	if len(sig.Params) == 0 {
		c.diag.Add(span, "method missing self parameter")
		return Type{Kind: TypeInvalid}
	}
	expectedArgs := len(sig.Params) - 1
	if len(args) != expectedArgs {
		c.diag.Add(span, fmt.Sprintf("method '%s' expects %d args, got %d", sig.Name, expectedArgs, len(args)))
	}
	recvForUnify := recvType
	if sig.Params[0].Ref && !recvForUnify.Ref {
		recvForUnify.Ref = true
		recvForUnify.Mut = sig.Params[0].Mut
	}
	if len(sig.TypeParams) == 0 {
		if !typesAssignable(recvForUnify, sig.Params[0]) {
			c.diag.Add(span, "receiver type mismatch")
		}
		for i, arg := range args {
			if i+1 >= len(sig.Params) {
				break
			}
			argType := c.checkExprWithExpected(arg, sig.Params[i+1])
			if !typesAssignable(argType, sig.Params[i+1]) && argType.Kind != TypeInvalid && sig.Params[i+1].Kind != TypeInvalid {
				c.diag.Add(arg.Span(), fmt.Sprintf("argument %d expects %s, got %s", i+1, sig.Params[i+1].String(), argType.String()))
				break
			}
		}
		return sig.Return
	}
	subst := map[string]Type{}
	_ = c.unifyType(sig.Params[0], recvForUnify, subst)
	for i, arg := range args {
		if i+1 >= len(sig.Params) {
			break
		}
		argType := c.checkExprWithExpected(arg, sig.Params[i+1])
		if !c.unifyType(sig.Params[i+1], argType, subst) {
			if argType.Kind != TypeInvalid && sig.Params[i+1].Kind != TypeInvalid {
				c.diag.Add(arg.Span(), fmt.Sprintf("argument %d expects %s, got %s", i+1, sig.Params[i+1].String(), argType.String()))
			}
			return Type{Kind: TypeInvalid}
		}
	}
	if expected, ok := c.currentExpected(); ok && expected.Kind != TypeInvalid {
		_ = c.unifyType(sig.Return, expected, subst)
	}
	for _, p := range sig.TypeParams {
		if _, ok := subst[p.Name]; !ok {
			c.diag.Add(span, fmt.Sprintf("cannot infer type parameter '%s' for '%s'", p.Name, sig.Name))
			return Type{Kind: TypeInvalid}
		}
	}
	c.checkTypeParamBounds(sig.TypeParams, subst, span)
	instRet := c.applySubst(sig.Return, subst)
	expectedSelf := c.applySubst(sig.Params[0], subst)
	if !typesAssignable(recvForUnify, expectedSelf) && recvForUnify.Kind != TypeInvalid && expectedSelf.Kind != TypeInvalid {
		c.diag.Add(span, "receiver type mismatch")
	}
	for i, arg := range args {
		if i+1 >= len(sig.Params) {
			break
		}
		expect := c.applySubst(sig.Params[i+1], subst)
		argType := c.checkExprWithExpected(arg, expect)
		if !typesAssignable(argType, expect) && argType.Kind != TypeInvalid && expect.Kind != TypeInvalid {
			c.diag.Add(arg.Span(), fmt.Sprintf("argument %d expects %s, got %s", i+1, expect.String(), argType.String()))
			break
		}
	}
	return instRet
}

func (c *Checker) checkStaticMethodCall(sig *MethodSig, args []ast.Expr, span source.Span) Type {
	if sig.HasSelf {
		c.diag.Add(span, "static call to method that requires self")
		return sig.Return
	}
	if len(args) != len(sig.Params) {
		c.diag.Add(span, fmt.Sprintf("method '%s' expects %d args, got %d", sig.Name, len(sig.Params), len(args)))
	}
	if len(sig.TypeParams) == 0 {
		for i, arg := range args {
			if i >= len(sig.Params) {
				break
			}
			argType := c.checkExprWithExpected(arg, sig.Params[i])
			if !typesAssignable(argType, sig.Params[i]) && argType.Kind != TypeInvalid && sig.Params[i].Kind != TypeInvalid {
				c.diag.Add(arg.Span(), fmt.Sprintf("argument %d expects %s, got %s", i+1, sig.Params[i].String(), argType.String()))
				break
			}
		}
		return sig.Return
	}
	subst := map[string]Type{}
	for i, arg := range args {
		if i >= len(sig.Params) {
			break
		}
		argType := c.checkExprWithExpected(arg, sig.Params[i])
		if !c.unifyType(sig.Params[i], argType, subst) {
			if argType.Kind != TypeInvalid && sig.Params[i].Kind != TypeInvalid {
				c.diag.Add(arg.Span(), fmt.Sprintf("argument %d expects %s, got %s", i+1, sig.Params[i].String(), argType.String()))
			}
			return Type{Kind: TypeInvalid}
		}
	}
	if expected, ok := c.currentExpected(); ok && expected.Kind != TypeInvalid {
		_ = c.unifyType(sig.Return, expected, subst)
	}
	for _, p := range sig.TypeParams {
		if _, ok := subst[p.Name]; !ok {
			c.diag.Add(span, fmt.Sprintf("cannot infer type parameter '%s' for '%s'", p.Name, sig.Name))
			return Type{Kind: TypeInvalid}
		}
	}
	c.checkTypeParamBounds(sig.TypeParams, subst, span)
	instRet := c.applySubst(sig.Return, subst)
	for i, arg := range args {
		if i >= len(sig.Params) {
			break
		}
		expect := c.applySubst(sig.Params[i], subst)
		argType := c.checkExprWithExpected(arg, expect)
		if !typesAssignable(argType, expect) && argType.Kind != TypeInvalid && expect.Kind != TypeInvalid {
			c.diag.Add(arg.Span(), fmt.Sprintf("argument %d expects %s, got %s", i+1, expect.String(), argType.String()))
			break
		}
	}
	return instRet
}
