package typecheck

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/source"
)

func (c *Checker) collectTraitSigs() {
	for _, sig := range c.traits {
		if sig == nil || sig.Decl == nil {
			continue
		}
		if len(sig.Methods) > 0 {
			continue
		}
		sig.TypeParams = append([]ast.TypeParam{}, sig.Decl.TypeParams...)
		restore := c.pushTypeParams(sig.Decl.TypeParams)
		for _, m := range sig.Decl.Methods {
			sig.Methods = append(sig.Methods, c.traitMethodSig(m))
		}
		c.popTypeParams(restore)
	}
}

func (c *Checker) traitMethodSig(m ast.TraitMethod) TraitMethodSig {
	sig := TraitMethodSig{Name: m.Name}
	saved := c.selfType
	self := Type{Kind: TypeParam, Name: "Self"}
	c.selfType = &self
	for i, param := range m.Params {
		if param.Name == "self" && i != 0 {
			c.diag.Add(param.Span, "self must be first parameter")
		}
		t := c.fromAstType(param.Type)
		sig.Params = append(sig.Params, t)
	}
	if len(m.Params) > 0 && m.Params[0].Name == "self" {
		sig.HasSelf = true
		sig.SelfType = sig.Params[0]
	}
	if m.ReturnType != nil {
		sig.Return = c.fromAstType(*m.ReturnType)
		sig.ReturnExplicit = true
	} else {
		sig.Return = Type{Kind: TypeUnit}
		sig.ReturnExplicit = false
	}
	c.selfType = saved
	return sig
}

func (c *Checker) collectImplTraits(prog *ast.Program) {
	c.collectTraitSigs()
	for _, item := range prog.Items {
		impl, ok := item.(*ast.ImplTraitDecl)
		if !ok {
			continue
		}
		traitSig, ok := c.traits[impl.TraitName]
		if !ok {
			c.diag.Add(impl.Span(), fmt.Sprintf("unknown trait '%s'", impl.TraitName))
			continue
		}
		if len(impl.TypeParams) > 0 {
			// Generic impl traits are instantiated during monomorphization.
			continue
		}
		implType := c.typeFromNameAndArgs(impl.ForTypeName, impl.ForTypeArgs)
		traitSubst, ok := c.traitSubstForImplTrait(impl, nil, traitSig, impl.Span())
		if !ok {
			continue
		}
		c.applyImplTrait(impl, implType, traitSig, traitSubst)
	}
}

func (c *Checker) traitSubstForImplTrait(impl *ast.ImplTraitDecl, baseSubst map[string]Type, traitSig *TraitSig, span source.Span) (map[string]Type, bool) {
	subst := map[string]Type{}
	if traitSig == nil || len(traitSig.TypeParams) == 0 {
		return subst, true
	}
	if len(impl.TraitArgs) != len(traitSig.TypeParams) {
		c.diag.Add(span, fmt.Sprintf("trait '%s' expects %d type arguments, got %d", traitSig.Name, len(traitSig.TypeParams), len(impl.TraitArgs)))
		return subst, false
	}
	for i, p := range traitSig.TypeParams {
		argAst := impl.TraitArgs[i]
		if len(baseSubst) > 0 {
			argAst = c.cloneType(argAst, baseSubst)
		}
		argType := c.fromAstType(argAst)
		subst[p.Name] = argType
	}
	return subst, true
}

func (c *Checker) applyImplTrait(impl *ast.ImplTraitDecl, implType Type, traitSig *TraitSig, traitSubst map[string]Type) {
	implType = c.canonicalizeType(implType)
	typeKey := typeKey(implType)
	if _, ok := c.traitImpls[traitSig.Name]; !ok {
		c.traitImpls[traitSig.Name] = map[string]struct{}{}
	}
	if _, exists := c.traitImpls[traitSig.Name][typeKey]; exists {
		c.diag.Add(impl.Span(), fmt.Sprintf("duplicate impl of trait '%s' for '%s'", impl.TraitName, impl.ForTypeName))
	}
	c.traitImpls[traitSig.Name][typeKey] = struct{}{}

	implMethods := map[string]*ast.Function{}
	for _, m := range impl.Methods {
		implMethods[m.Name] = m
	}

	for _, req := range traitSig.Methods {
		implMethod, ok := implMethods[req.Name]
		if !ok {
			c.diag.Add(impl.Span(), fmt.Sprintf("missing method '%s' required by trait '%s'", req.Name, impl.TraitName))
			continue
		}
		reqSig := c.substTraitAndSelf(req, implType, traitSubst)
		implSig := c.methodSignature(implType, implMethod)
		if len(reqSig.Params) != len(implSig.Params) {
			c.diag.Add(implMethod.Span(), fmt.Sprintf("method '%s' has wrong parameter count", req.Name))
			continue
		}
		for i := range reqSig.Params {
			if !typesEqual(reqSig.Params[i], implSig.Params[i]) {
				c.diag.Add(implMethod.Span(), fmt.Sprintf("method '%s' parameter %d type mismatch", req.Name, i+1))
				break
			}
		}
		if !typesEqual(reqSig.Return, implSig.Return) && reqSig.Return.Kind != TypeInvalid && implSig.Return.Kind != TypeInvalid {
			c.diag.Add(implMethod.Span(), fmt.Sprintf("method '%s' return type mismatch", req.Name))
		}
	}

	// Register methods for method call resolution
	for _, method := range impl.Methods {
		if _, exists := c.methods[implType.Name]; !exists {
			c.methods[implType.Name] = map[string]*MethodSig{}
		}
		if _, exists := c.methods[implType.Name][method.Name]; exists {
			continue
		}
		sig := c.methodSignature(implType, method)
		c.methods[implType.Name][method.Name] = sig
	}
}

func (c *Checker) methodSignature(selfType Type, fn *ast.Function) *MethodSig {
	sig := &MethodSig{Name: fn.Name, FuncName: selfType.Name + "." + fn.Name, TypeParams: fn.TypeParams}
	saved := c.selfType
	c.selfType = &selfType
	restore := c.pushTypeParams(fn.TypeParams)
	for i, param := range fn.Params {
		if param.Name == "self" && i != 0 {
			c.diag.Add(param.Span, "self must be first parameter")
		}
		t := c.fromAstType(param.Type)
		sig.Params = append(sig.Params, t)
	}
	if len(fn.Params) > 0 && fn.Params[0].Name == "self" {
		sig.HasSelf = true
		sig.SelfType = sig.Params[0]
	}
	if fn.ReturnType != nil {
		sig.Return = c.fromAstType(*fn.ReturnType)
		sig.ReturnExplicit = true
	} else {
		sig.Return = Type{Kind: TypeUnit}
		sig.ReturnExplicit = false
	}
	c.popTypeParams(restore)
	c.selfType = saved
	return sig
}

func (c *Checker) substTraitAndSelf(sig TraitMethodSig, selfType Type, traitSubst map[string]Type) TraitMethodSig {
	subst := map[string]Type{"Self": selfType}
	for k, v := range traitSubst {
		subst[k] = v
	}
	out := sig
	out.Params = nil
	for _, p := range sig.Params {
		out.Params = append(out.Params, c.applySubst(p, subst))
	}
	out.Return = c.applySubst(sig.Return, subst)
	out.SelfType = c.applySubst(sig.SelfType, subst)
	return out
}

func (c *Checker) checkTypeParamBounds(params []ast.TypeParam, subst map[string]Type, span source.Span) {
	for _, p := range params {
		arg, ok := subst[p.Name]
		if !ok {
			continue
		}
		if arg.Kind == TypeParam {
			continue
		}
		for _, bound := range p.Bounds {
			if _, ok := c.traits[bound]; !ok {
				c.diag.Add(span, fmt.Sprintf("unknown trait '%s'", bound))
				continue
			}
			if !c.typeImplementsTrait(arg, bound) {
				c.diag.Add(span, fmt.Sprintf("type '%s' does not implement trait '%s'", arg.String(), bound))
			}
		}
	}
}

func (c *Checker) typeImplementsTrait(t Type, trait string) bool {
	impls, ok := c.traitImpls[trait]
	if ok {
		if _, exists := impls[typeKey(t)]; exists {
			return true
		}
	}
	expanded := c.expandInstType(t)
	if ok {
		if _, exists := impls[typeKey(expanded)]; exists {
			return true
		}
	}
	if expanded.Kind != TypeStruct && expanded.Kind != TypeEnum {
		return false
	}
	if len(expanded.Args) == 0 {
		return false
	}
	return c.hasImplTraitTemplate(expanded.Name, expanded.Args, trait)
}

func (c *Checker) hasImplTraitTemplate(base string, args []Type, trait string) bool {
	templates := c.implTraitTemplates[base]
	if len(templates) == 0 {
		return false
	}
	traitSig, ok := c.traits[trait]
	if !ok {
		return false
	}
	for _, tmpl := range templates {
		if tmpl.TraitName != trait {
			continue
		}
		if len(tmpl.TypeParams) != len(args) {
			continue
		}
		if len(traitSig.TypeParams) != len(tmpl.TraitArgs) {
			continue
		}
		return true
	}
	return false
}
