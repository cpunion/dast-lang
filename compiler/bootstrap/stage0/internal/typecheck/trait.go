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
		for _, m := range sig.Decl.Methods {
			sig.Methods = append(sig.Methods, c.traitMethodSig(m))
		}
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
			// Generic impl traits are not instantiated in stage0 yet.
			continue
		}
		implType := c.typeFromNameAndArgs(impl.ForTypeName, impl.ForTypeArgs)
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
			reqSig := c.substSelf(req, implType)
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

func (c *Checker) substSelf(sig TraitMethodSig, selfType Type) TraitMethodSig {
	subst := map[string]Type{"Self": selfType}
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
	if !ok {
		return false
	}
	_, ok = impls[typeKey(t)]
	return ok
}
