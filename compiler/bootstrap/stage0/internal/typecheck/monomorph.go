package typecheck

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/diag"
)

func newChecker() *Checker {
	return &Checker{
		diag:               &diag.Bag{},
		funcs:              map[string]*FuncSig{},
		funcDecls:          map[string]*ast.Function{},
		methods:            map[string]map[string]*MethodSig{},
	builtins:           map[string]struct{}{"print": {}, "println": {}, "eprint": {}, "eprintln": {}, "len": {}, "push": {}, "pop": {}, "exit": {}, "read_file": {}, "read_dir": {}, "write_file": {}, "mkdir": {}, "args": {}, "getenv": {}, "char_at": {}, "substr": {}, "string_clone": {}, "read_line": {}, "read_bytes": {}, "exec": {}, "ast_expr": {}, "ast_stmt": {}, "ast_item": {}, "ast_block": {}, "ast_to_string": {}, "gensym": {}, "bind": {}},
		consts:             map[string]ConstInfo{},
		structs:            map[string]*ast.StructDecl{},
		enums:              map[string]*ast.EnumDecl{},
		aliases:            map[string]*ast.TypeAlias{},
		traits:             map[string]*TraitSig{},
		implTraits:         []*ast.ImplTraitDecl{},
		typeParams:         map[string]ast.TypeParam{},
		traitImpls:         map[string]map[string]struct{}{},
		implTemplates:      map[string][]*ast.ImplDecl{},
		implTraitTemplates: map[string][]*ast.ImplTraitDecl{},
		funcInsts:          map[string]string{},
		structInsts:        map[string]string{},
		enumInsts:          map[string]string{},
		structInstBase:     map[string]string{},
		enumInstBase:       map[string]string{},
		structInstArgs:     map[string][]Type{},
		enumInstArgs:       map[string][]Type{},
	}
}

func Check(prog *ast.Program) *diag.Bag {
	_, diags := CheckAndMonomorph(prog)
	return diags
}

func CheckAndMonomorph(prog *ast.Program) (*ast.Program, *diag.Bag) {
	c := newChecker()
	c.collectDecls(prog)
	c.collectConsts(prog)
	c.collectSignatures(prog)
	c.collectTraitSigs()
	c.collectImplTraits(prog)
	for _, item := range prog.Items {
		fn, ok := item.(*ast.Function)
		if !ok {
			continue
		}
		c.checkFunction(fn)
	}
	for _, item := range prog.Items {
		if impl, ok := item.(*ast.ImplDecl); ok {
			restore := c.pushTypeParams(impl.TypeParams)
			selfType := c.typeFromNameAndArgs(impl.TypeName, impl.TypeArgs)
			for _, method := range impl.Methods {
				c.checkMethod(selfType, method)
			}
			c.popTypeParams(restore)
		}
		if impl, ok := item.(*ast.ImplTraitDecl); ok {
			restore := c.pushTypeParams(impl.TypeParams)
			selfType := c.typeFromNameAndArgs(impl.ForTypeName, impl.ForTypeArgs)
			for _, method := range impl.Methods {
				c.checkMethod(selfType, method)
			}
			c.popTypeParams(restore)
		}
	}
	c.processInstantiations(prog)
	c.expandAliasesInProgram(prog)
	if c.diag.HasErrors() {
		return nil, c.diag
	}
	out := c.filterMonomorphic(prog)
	return out, c.diag
}

func (c *Checker) processInstantiations(prog *ast.Program) {
	for len(c.pendingStructs) > 0 || len(c.pendingEnums) > 0 || len(c.pendingFuncs) > 0 {
		for len(c.pendingStructs) > 0 {
			inst := c.pendingStructs[0]
			c.pendingStructs = c.pendingStructs[1:]
			c.instantiateStruct(prog, inst)
		}
		for len(c.pendingEnums) > 0 {
			inst := c.pendingEnums[0]
			c.pendingEnums = c.pendingEnums[1:]
			c.instantiateEnum(prog, inst)
		}
		for len(c.pendingFuncs) > 0 {
			inst := c.pendingFuncs[0]
			c.pendingFuncs = c.pendingFuncs[1:]
			c.instantiateFunc(prog, inst)
		}
	}
}

func (c *Checker) instantiateFunc(prog *ast.Program, inst funcInst) {
	fn := c.funcDecls[inst.name]
	if fn == nil || len(fn.TypeParams) == 0 {
		return
	}
	if len(fn.TypeParams) != len(inst.args) {
		return
	}
	instName := mangleName(inst.name, inst.args)
	if _, ok := c.funcDecls[instName]; ok {
		return
	}
	subst := map[string]Type{}
	for i, p := range fn.TypeParams {
		subst[p.Name] = inst.args[i]
	}
	clone := c.cloneFunction(fn, subst)
	clone.Name = instName
	clone.TypeParams = nil
	prog.Items = append(prog.Items, clone)
	c.funcDecls[instName] = clone

	restore := c.pushTypeParams(nil)
	c.collectSignatures(&ast.Program{Items: []ast.Item{clone}})
	c.popTypeParams(restore)
	c.checkFunction(clone)
}

func (c *Checker) instantiateStruct(prog *ast.Program, inst typeInst) {
	decl := c.structs[inst.name]
	if decl == nil || len(decl.TypeParams) == 0 {
		return
	}
	if len(decl.TypeParams) != len(inst.args) {
		return
	}
	instName := mangleName(inst.name, inst.args)
	if _, ok := c.structs[instName]; ok {
		return
	}
	subst := map[string]Type{}
	for i, p := range decl.TypeParams {
		subst[p.Name] = inst.args[i]
	}
	clone := &ast.StructDecl{
		Name:     instName,
		Fields:   nil,
		Vis:      decl.Vis,
		SpanInfo: decl.SpanInfo,
	}
	for _, f := range decl.Fields {
		field := f
		field.Type = c.cloneType(f.Type, subst)
		clone.Fields = append(clone.Fields, field)
	}
	prog.Items = append(prog.Items, clone)
	c.structs[instName] = clone
	c.instantiateImplsForType(prog, inst.name, inst.args, instName)
	c.instantiateImplTraitsForType(prog, inst.name, inst.args, instName)
}

func (c *Checker) instantiateEnum(prog *ast.Program, inst typeInst) {
	decl := c.enums[inst.name]
	if decl == nil || len(decl.TypeParams) == 0 {
		return
	}
	if len(decl.TypeParams) != len(inst.args) {
		return
	}
	instName := mangleName(inst.name, inst.args)
	if _, ok := c.enums[instName]; ok {
		return
	}
	subst := map[string]Type{}
	for i, p := range decl.TypeParams {
		subst[p.Name] = inst.args[i]
	}
	clone := &ast.EnumDecl{
		Name:     instName,
		Repr:     decl.Repr,
		Variants: nil,
		Vis:      decl.Vis,
		SpanInfo: decl.SpanInfo,
	}
	for _, v := range decl.Variants {
		variant := v
		if v.Payload != nil {
			pt := c.cloneType(*v.Payload, subst)
			variant.Payload = &pt
		}
		clone.Variants = append(clone.Variants, variant)
	}
	prog.Items = append(prog.Items, clone)
	c.enums[instName] = clone
	c.instantiateImplsForType(prog, inst.name, inst.args, instName)
	c.instantiateImplTraitsForType(prog, inst.name, inst.args, instName)
}

func (c *Checker) instantiateImplsForType(prog *ast.Program, base string, args []Type, instName string) {
	impls := c.implTemplates[base]
	if len(impls) == 0 {
		return
	}
	for _, impl := range impls {
		if len(impl.TypeParams) != len(args) {
			continue
		}
		subst := map[string]Type{}
		for i, p := range impl.TypeParams {
			subst[p.Name] = args[i]
		}
		clone := &ast.ImplDecl{
			TypeName: instName,
			Methods:  nil,
			Vis:      impl.Vis,
			SpanInfo: impl.SpanInfo,
		}
		for _, method := range impl.Methods {
			mc := c.cloneFunction(method, subst)
			clone.Methods = append(clone.Methods, mc)
		}
		prog.Items = append(prog.Items, clone)
		selfType := c.typeFromNameAndArgs(instName, nil)
		for _, method := range clone.Methods {
			sig := c.methodSignature(selfType, method)
			if _, exists := c.methods[instName]; !exists {
				c.methods[instName] = map[string]*MethodSig{}
			}
			c.methods[instName][method.Name] = sig
			c.checkMethod(selfType, method)
		}
	}
}

func (c *Checker) instantiateImplTraitsForType(prog *ast.Program, base string, args []Type, instName string) {
	impls := c.implTraitTemplates[base]
	if len(impls) == 0 {
		return
	}
	selfType := c.typeFromNameAndArgs(instName, nil)
	for _, impl := range impls {
		if len(impl.TypeParams) != len(args) {
			continue
		}
		traitSig, ok := c.traits[impl.TraitName]
		if !ok {
			c.diag.Add(impl.Span(), fmt.Sprintf("unknown trait '%s'", impl.TraitName))
			continue
		}
		baseSubst := map[string]Type{}
		for i, p := range impl.TypeParams {
			baseSubst[p.Name] = args[i]
		}
		traitSubst, ok := c.traitSubstForImplTrait(impl, baseSubst, traitSig, impl.Span())
		if !ok {
			continue
		}
		methodSubst := map[string]Type{}
		for k, v := range baseSubst {
			methodSubst[k] = v
		}
		for k, v := range traitSubst {
			methodSubst[k] = v
		}
		// Ensure trait method bodies see the concrete Self type.
		methodSubst["Self"] = selfType
		clone := &ast.ImplTraitDecl{
			TraitName:   impl.TraitName,
			TraitArgs:   nil,
			ForTypeName: instName,
			ForTypeArgs: nil,
			TypeParams:  nil,
			Methods:     nil,
			Vis:         impl.Vis,
			SpanInfo:    impl.SpanInfo,
		}
		savedSelf := c.selfType
		c.selfType = &selfType
		for _, arg := range impl.TraitArgs {
			clone.TraitArgs = append(clone.TraitArgs, c.cloneType(arg, baseSubst))
		}
		for _, method := range impl.Methods {
			mc := c.cloneFunction(method, methodSubst)
			clone.Methods = append(clone.Methods, mc)
		}
		c.selfType = savedSelf
		prog.Items = append(prog.Items, clone)
		c.applyImplTrait(clone, selfType, traitSig, traitSubst)
		for _, method := range clone.Methods {
			c.checkMethod(selfType, method)
		}
	}
}

func (c *Checker) filterMonomorphic(prog *ast.Program) *ast.Program {
	out := &ast.Program{}
	for _, item := range prog.Items {
		switch t := item.(type) {
		case *ast.Function:
			if len(t.TypeParams) == 0 {
				out.Items = append(out.Items, t)
			}
		case *ast.StructDecl:
			if len(t.TypeParams) == 0 {
				out.Items = append(out.Items, t)
			}
		case *ast.EnumDecl:
			if len(t.TypeParams) == 0 {
				out.Items = append(out.Items, t)
			}
		case *ast.ImplDecl:
			if len(t.TypeParams) == 0 {
				out.Items = append(out.Items, t)
			}
		case *ast.ImplTraitDecl:
			if len(t.TypeParams) == 0 {
				out.Items = append(out.Items, t)
			}
		case *ast.TypeAlias:
			out.Items = append(out.Items, t)
		case *ast.ConstDecl:
			out.Items = append(out.Items, t)
		case *ast.ImportDecl:
			// imports already stripped; skip
		case *ast.TraitDecl:
			out.Items = append(out.Items, t)
		default:
			out.Items = append(out.Items, t)
		}
	}
	return out
}
