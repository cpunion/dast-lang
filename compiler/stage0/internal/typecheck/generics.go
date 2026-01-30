package typecheck

import (
	"strings"

	"dastlang/internal/ast"
)

type TraitSig struct {
	Name       string
	TypeParams []ast.TypeParam
	Decl       *ast.TraitDecl
	Methods    []TraitMethodSig
}

type TraitMethodSig struct {
	Name           string
	Params         []Type
	Return         Type
	ReturnExplicit bool
	HasSelf        bool
	SelfType       Type
}

type funcInst struct {
	name string
	args []Type
}

type typeInst struct {
	name string
	args []Type
}

func (c *Checker) pushTypeParams(params []ast.TypeParam) map[string]ast.TypeParam {
	prev := c.typeParams
	if len(params) == 0 {
		return prev
	}
	next := map[string]ast.TypeParam{}
	for k, v := range prev {
		next[k] = v
	}
	for _, p := range params {
		next[p.Name] = p
	}
	c.typeParams = next
	return prev
}

func (c *Checker) popTypeParams(prev map[string]ast.TypeParam) {
	c.typeParams = prev
}

func (c *Checker) typeFromNameAndArgs(name string, args []ast.Type) Type {
	t := ast.Type{Name: name, Args: args}
	return c.fromAstType(t)
}

func isConcreteType(t Type) bool {
	if t.Kind == TypeInvalid || t.Kind == TypeParam {
		return false
	}
	if t.Kind == TypeArray {
		if t.Elem == nil {
			return false
		}
		return isConcreteType(*t.Elem)
	}
	for _, arg := range t.Args {
		if !isConcreteType(arg) {
			return false
		}
	}
	return true
}

func isConcreteArgs(args []Type) bool {
	for _, a := range args {
		if !isConcreteType(a) {
			return false
		}
	}
	return true
}

func applySubst(t Type, subst map[string]Type) Type {
	if t.Kind == TypeParam {
		if v, ok := subst[t.Name]; ok {
			out := v
			if t.Ref {
				out.Ref = true
				out.Mut = t.Mut
			}
			return out
		}
		return t
	}
	out := t
	if t.Elem != nil {
		elem := applySubst(*t.Elem, subst)
		out.Elem = &elem
	}
	if len(t.Args) > 0 {
		out.Args = make([]Type, 0, len(t.Args))
		for _, a := range t.Args {
			out.Args = append(out.Args, applySubst(a, subst))
		}
	}
	return out
}

func (c *Checker) applySubst(t Type, subst map[string]Type) Type {
	out := applySubst(t, subst)
	return c.canonicalizeType(out)
}

func (c *Checker) canonicalizeType(t Type) Type {
	out := t
	if out.Elem != nil {
		elem := c.canonicalizeType(*out.Elem)
		out.Elem = &elem
	}
	if len(out.Args) > 0 {
		out.Args = append([]Type{}, out.Args...)
		for i := range out.Args {
			out.Args[i] = c.canonicalizeType(out.Args[i])
		}
	}
	switch out.Kind {
	case TypeStruct:
		if len(out.Args) > 0 && isConcreteArgs(out.Args) {
			out.Name = c.ensureStructInstance(out.Name, out.Args)
			out.Args = nil
		}
	case TypeEnum:
		if len(out.Args) > 0 && isConcreteArgs(out.Args) {
			out.Name = c.ensureEnumInstance(out.Name, out.Args)
			out.Args = nil
		}
	}
	return out
}

func (c *Checker) unifyType(pattern Type, actual Type, subst map[string]Type) bool {
	if pattern.Kind == TypeInvalid || actual.Kind == TypeInvalid {
		return false
	}
	// Allow &String (and String) to match &str in generic inference.
	if pattern.Kind == TypeStr && pattern.Ref && !pattern.Mut && actual.Kind == TypeString {
		return true
	}
	if pattern.Kind == TypeParam {
		if bound, ok := subst[pattern.Name]; ok {
			if isUntypedInt(bound) && isInt(actual) {
				subst[pattern.Name] = actual
				return true
			}
			if isUntypedFloat(bound) && isFloat(actual) {
				subst[pattern.Name] = actual
				return true
			}
			if isInt(bound) && isUntypedInt(actual) {
				return true
			}
			if isFloat(bound) && isUntypedFloat(actual) {
				return true
			}
			return typesEqual(bound, actual)
		}
		subst[pattern.Name] = actual
		return true
	}
	if pattern.Kind != actual.Kind {
		return false
	}
	if pattern.Ref != actual.Ref || pattern.Mut != actual.Mut {
		return false
	}
	if pattern.Kind == TypeStruct || pattern.Kind == TypeEnum {
		if pattern.Name != actual.Name {
			if pattern.Kind == TypeStruct {
				if base, ok := c.structInstBase[actual.Name]; ok && base == pattern.Name {
					if args, ok := c.structInstArgs[actual.Name]; ok {
						if len(pattern.Args) != len(args) {
							return false
						}
						for i := range pattern.Args {
							if !c.unifyType(pattern.Args[i], args[i], subst) {
								return false
							}
						}
						return true
					}
				}
			}
			if pattern.Kind == TypeEnum {
				if base, ok := c.enumInstBase[actual.Name]; ok && base == pattern.Name {
					if args, ok := c.enumInstArgs[actual.Name]; ok {
						if len(pattern.Args) != len(args) {
							return false
						}
						for i := range pattern.Args {
							if !c.unifyType(pattern.Args[i], args[i], subst) {
								return false
							}
						}
						return true
					}
				}
			}
			return false
		}
	}
	if pattern.Kind == TypeArray {
		if pattern.Elem == nil || actual.Elem == nil {
			return false
		}
		return c.unifyType(*pattern.Elem, *actual.Elem, subst)
	}
	if pattern.Kind == TypeTuple {
		if len(pattern.Elems) != len(actual.Elems) {
			return false
		}
		for i := range pattern.Elems {
			if !c.unifyType(pattern.Elems[i], actual.Elems[i], subst) {
				return false
			}
		}
		return true
	}
	if len(pattern.Args) != len(actual.Args) {
		return false
	}
	for i := range pattern.Args {
		if !c.unifyType(pattern.Args[i], actual.Args[i], subst) {
			return false
		}
	}
	return true
}

func typeKey(t Type) string {
	return t.String()
}

func (c *Checker) expandInstType(t Type) Type {
	out := t
	switch out.Kind {
	case TypeStruct:
		if base, ok := c.structInstBase[out.Name]; ok {
			if args, ok := c.structInstArgs[out.Name]; ok {
				out.Name = base
				out.Args = args
			}
		}
	case TypeEnum:
		if base, ok := c.enumInstBase[out.Name]; ok {
			if args, ok := c.enumInstArgs[out.Name]; ok {
				out.Name = base
				out.Args = args
			}
		}
	}
	if out.Elem != nil {
		elem := c.expandInstType(*out.Elem)
		out.Elem = &elem
	}
	if len(out.Args) > 0 {
		out.Args = append([]Type{}, out.Args...)
		for i := range out.Args {
			out.Args[i] = c.expandInstType(out.Args[i])
		}
	}
	return out
}

func mangleType(t Type) string {
	raw := typeKey(t)
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "t"
	}
	return out
}

func mangleName(base string, args []Type) string {
	if len(args) == 0 {
		return base
	}
	parts := []string{base}
	for _, a := range args {
		parts = append(parts, mangleType(a))
	}
	return strings.Join(parts, "__")
}

func (c *Checker) ensureFuncInstance(name string, args []Type) string {
	sig, ok := c.funcs[name]
	if !ok || len(sig.TypeParams) == 0 {
		return name
	}
	if !isConcreteArgs(args) {
		return name
	}
	key := name + "[" + typeArgsKey(args) + "]"
	if instName, ok := c.funcInsts[key]; ok {
		return instName
	}
	instName := mangleName(name, args)
	c.funcInsts[key] = instName
	c.pendingFuncs = append(c.pendingFuncs, funcInst{name: name, args: args})
	return instName
}

func (c *Checker) ensureStructInstance(name string, args []Type) string {
	decl, ok := c.structs[name]
	if !ok || len(decl.TypeParams) == 0 {
		return name
	}
	if !isConcreteArgs(args) {
		return name
	}
	key := name + "[" + typeArgsKey(args) + "]"
	if instName, ok := c.structInsts[key]; ok {
		return instName
	}
	instName := mangleName(name, args)
	c.structInsts[key] = instName
	c.structInstBase[instName] = name
	if _, ok := c.structInstArgs[instName]; !ok {
		copied := make([]Type, len(args))
		copy(copied, args)
		c.structInstArgs[instName] = copied
	}
	c.pendingStructs = append(c.pendingStructs, typeInst{name: name, args: args})
	return instName
}

func (c *Checker) ensureEnumInstance(name string, args []Type) string {
	decl, ok := c.enums[name]
	if !ok || len(decl.TypeParams) == 0 {
		return name
	}
	if !isConcreteArgs(args) {
		return name
	}
	key := name + "[" + typeArgsKey(args) + "]"
	if instName, ok := c.enumInsts[key]; ok {
		return instName
	}
	instName := mangleName(name, args)
	c.enumInsts[key] = instName
	c.enumInstBase[instName] = name
	if _, ok := c.enumInstArgs[instName]; !ok {
		copied := make([]Type, len(args))
		copy(copied, args)
		c.enumInstArgs[instName] = copied
	}
	c.pendingEnums = append(c.pendingEnums, typeInst{name: name, args: args})
	return instName
}

func typeArgsKey(args []Type) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for _, a := range args {
		parts = append(parts, typeKey(a))
	}
	return strings.Join(parts, ",")
}
