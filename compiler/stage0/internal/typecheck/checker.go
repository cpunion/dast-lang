package typecheck

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/diag"
)

type Checker struct {
	diag               *diag.Bag
	funcs              map[string]*FuncSig
	funcDecls          map[string]*ast.Function
	methods            map[string]map[string]*MethodSig
	builtins           map[string]struct{}
	consts             map[string]ConstInfo
	structs            map[string]*ast.StructDecl
	enums              map[string]*ast.EnumDecl
	aliases            map[string]*ast.TypeAlias
	traits             map[string]*TraitSig
	implTraits         []*ast.ImplTraitDecl
	traitImpls         map[string]map[string]struct{}
	implTemplates      map[string][]*ast.ImplDecl
	implTraitTemplates map[string][]*ast.ImplTraitDecl
	env                *env
	current            *FuncSig
	selfType           *Type
	typeParams         map[string]ast.TypeParam

	inferReturn   bool
	inferredType  Type
	hasReturn     bool
	hasBareReturn bool
	loopStack     []loopContext

	funcInsts      map[string]string
	structInsts    map[string]string
	enumInsts      map[string]string
	structInstBase map[string]string
	enumInstBase   map[string]string
	structInstArgs map[string][]Type
	enumInstArgs   map[string][]Type
	pendingFuncs   []funcInst
	pendingStructs []typeInst
	pendingEnums   []typeInst

	expectedStack []Type
}

type loopContext struct {
	label      string
	allowValue bool
	expected   Type
	valueType  Type
	hasValue   bool
}

type ConstInfo struct {
	Type  Type
	Value ast.ConstValue
}

func (c *Checker) collectDecls(prog *ast.Program) {
	for _, item := range prog.Items {
		switch t := item.(type) {
		case *ast.StructDecl:
			if _, exists := c.structs[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("duplicate struct '%s'", t.Name))
				continue
			}
			if _, exists := c.enums[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by enum", t.Name))
				continue
			}
			if _, exists := c.aliases[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by type alias", t.Name))
				continue
			}
			if _, exists := c.traits[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by trait", t.Name))
				continue
			}
			if t.Repr != "" && t.Repr != "C" {
				c.diag.Add(t.Span(), fmt.Sprintf("unsupported repr '%s'", t.Repr))
			}
			c.structs[t.Name] = t
		case *ast.EnumDecl:
			if _, exists := c.enums[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("duplicate enum '%s'", t.Name))
				continue
			}
			if _, exists := c.structs[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by struct", t.Name))
				continue
			}
			if _, exists := c.aliases[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by type alias", t.Name))
				continue
			}
			if _, exists := c.traits[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by trait", t.Name))
				continue
			}
			if t.Repr != "" && !isValidEnumRepr(t.Repr) {
				c.diag.Add(t.Span(), fmt.Sprintf("unsupported repr '%s'", t.Repr))
			}
			c.enums[t.Name] = t
		case *ast.ImplDecl:
			if _, ok := c.structs[t.TypeName]; !ok {
				if _, ok := c.enums[t.TypeName]; !ok {
					c.diag.Add(t.Span(), fmt.Sprintf("unknown type '%s' in impl", t.TypeName))
				}
			}
			if len(t.TypeParams) > 0 {
				c.implTemplates[t.TypeName] = append(c.implTemplates[t.TypeName], t)
			}
		case *ast.ImplTraitDecl:
			if _, ok := c.structs[t.ForTypeName]; !ok {
				if _, ok := c.enums[t.ForTypeName]; !ok {
					c.diag.Add(t.Span(), fmt.Sprintf("unknown type '%s' in impl", t.ForTypeName))
				}
			}
			c.implTraits = append(c.implTraits, t)
			if len(t.TypeParams) > 0 {
				c.implTraitTemplates[t.ForTypeName] = append(c.implTraitTemplates[t.ForTypeName], t)
			}
		case *ast.TypeAlias:
			if _, exists := c.structs[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by struct", t.Name))
				continue
			}
			if _, exists := c.enums[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by enum", t.Name))
				continue
			}
			if _, exists := c.traits[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by trait", t.Name))
				continue
			}
			if _, exists := c.aliases[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("duplicate type alias '%s'", t.Name))
				continue
			}
			c.aliases[t.Name] = t
		case *ast.TraitDecl:
			if _, exists := c.structs[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by struct", t.Name))
				continue
			}
			if _, exists := c.enums[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by enum", t.Name))
				continue
			}
			if _, exists := c.aliases[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by type alias", t.Name))
				continue
			}
			if _, exists := c.traits[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("duplicate trait '%s'", t.Name))
				continue
			}
			c.traits[t.Name] = &TraitSig{Name: t.Name, Decl: t}
		case *ast.ConstDecl:
			if _, exists := c.structs[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by struct", t.Name))
			}
			if _, exists := c.enums[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by enum", t.Name))
			}
			if _, exists := c.aliases[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by type alias", t.Name))
			}
			if _, exists := c.traits[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by trait", t.Name))
			}
		case *ast.Function:
			if _, exists := c.funcDecls[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("duplicate function '%s'", t.Name))
				continue
			}
			c.funcDecls[t.Name] = t
		}
	}
}

func (c *Checker) collectConsts(prog *ast.Program) {
	for _, item := range prog.Items {
		decl, ok := item.(*ast.ConstDecl)
		if !ok {
			continue
		}
		if _, exists := c.consts[decl.Name]; exists {
			c.diag.Add(decl.Span(), fmt.Sprintf("duplicate const '%s'", decl.Name))
			continue
		}
		if _, exists := c.structs[decl.Name]; exists {
			c.diag.Add(decl.Span(), fmt.Sprintf("name '%s' already used by struct", decl.Name))
		}
		if _, exists := c.enums[decl.Name]; exists {
			c.diag.Add(decl.Span(), fmt.Sprintf("name '%s' already used by enum", decl.Name))
		}
		valueType := constValueType(decl.Value)
		finalType := valueType
		if decl.Type != nil {
			finalType = c.fromAstType(*decl.Type)
			if !typesAssignable(valueType, finalType) && finalType.Kind != TypeInvalid && valueType.Kind != TypeInvalid {
				c.diag.Add(decl.Span(), fmt.Sprintf("const '%s' expects %s, got %s", decl.Name, finalType.String(), valueType.String()))
			}
		}
		c.consts[decl.Name] = ConstInfo{Type: finalType, Value: decl.Value}
	}
}

func (c *Checker) collectSignatures(prog *ast.Program) {
	for _, item := range prog.Items {
		fn, ok := item.(*ast.Function)
		if !ok {
			continue
		}
		if _, exists := c.consts[fn.Name]; exists {
			c.diag.Add(fn.Span(), fmt.Sprintf("name '%s' already used by const", fn.Name))
			continue
		}
		if _, exists := c.funcs[fn.Name]; exists {
			c.diag.Add(fn.Span(), fmt.Sprintf("duplicate function '%s'", fn.Name))
			continue
		}
		sig := &FuncSig{Name: fn.Name, TypeParams: fn.TypeParams}
		restore := c.pushTypeParams(fn.TypeParams)
		for _, param := range fn.Params {
			t := c.fromAstType(param.Type)
			sig.Params = append(sig.Params, t)
		}
		if fn.ReturnType != nil {
			sig.Return = c.fromAstType(*fn.ReturnType)
			sig.ReturnExplicit = true
		} else {
			sig.Return = Type{Kind: TypeInvalid}
			sig.ReturnExplicit = false
		}
		c.popTypeParams(restore)
		c.funcs[fn.Name] = sig
	}
	for _, item := range prog.Items {
		impl, ok := item.(*ast.ImplDecl)
		if !ok {
			continue
		}
		restoreImpl := c.pushTypeParams(impl.TypeParams)
		selfType := c.typeFromNameAndArgs(impl.TypeName, impl.TypeArgs)
		for _, method := range impl.Methods {
			if _, exists := c.methods[impl.TypeName]; !exists {
				c.methods[impl.TypeName] = map[string]*MethodSig{}
			}
			if _, exists := c.methods[impl.TypeName][method.Name]; exists {
				c.diag.Add(method.Span(), fmt.Sprintf("duplicate method '%s' for '%s'", method.Name, impl.TypeName))
				continue
			}
			combinedParams := append([]ast.TypeParam{}, impl.TypeParams...)
			for _, p := range method.TypeParams {
				dup := false
				for _, ep := range combinedParams {
					if ep.Name == p.Name {
						dup = true
						break
					}
				}
				if dup {
					c.diag.Add(method.Span(), fmt.Sprintf("duplicate type parameter '%s' in method '%s'", p.Name, method.Name))
					continue
				}
				combinedParams = append(combinedParams, p)
			}
			sig := &MethodSig{Name: method.Name, FuncName: impl.TypeName + "." + method.Name, TypeParams: combinedParams}
			c.selfType = &selfType
			restoreMethod := c.pushTypeParams(method.TypeParams)
			for i, param := range method.Params {
				if param.Name == "self" && i != 0 {
					c.diag.Add(param.Span, "self must be first parameter")
				}
				t := c.fromAstType(param.Type)
				sig.Params = append(sig.Params, t)
			}
			if len(method.Params) > 0 && method.Params[0].Name == "self" {
				sig.HasSelf = true
				sig.SelfType = sig.Params[0]
			}
			if method.ReturnType != nil {
				sig.Return = c.fromAstType(*method.ReturnType)
				sig.ReturnExplicit = true
			} else {
				sig.Return = Type{Kind: TypeInvalid}
				sig.ReturnExplicit = false
			}
			c.methods[impl.TypeName][method.Name] = sig
			c.popTypeParams(restoreMethod)
			c.selfType = nil
		}
		c.popTypeParams(restoreImpl)
	}
	for _, item := range prog.Items {
		impl, ok := item.(*ast.ImplTraitDecl)
		if !ok {
			continue
		}
		restoreImpl := c.pushTypeParams(impl.TypeParams)
		selfType := c.typeFromNameAndArgs(impl.ForTypeName, impl.ForTypeArgs)
		for _, method := range impl.Methods {
			if _, exists := c.methods[impl.ForTypeName]; !exists {
				c.methods[impl.ForTypeName] = map[string]*MethodSig{}
			}
			if _, exists := c.methods[impl.ForTypeName][method.Name]; exists {
				c.diag.Add(method.Span(), fmt.Sprintf("duplicate method '%s' for '%s'", method.Name, impl.ForTypeName))
				continue
			}
			combinedParams := append([]ast.TypeParam{}, impl.TypeParams...)
			for _, p := range method.TypeParams {
				dup := false
				for _, ep := range combinedParams {
					if ep.Name == p.Name {
						dup = true
						break
					}
				}
				if dup {
					c.diag.Add(method.Span(), fmt.Sprintf("duplicate type parameter '%s' in method '%s'", p.Name, method.Name))
					continue
				}
				combinedParams = append(combinedParams, p)
			}
			sig := &MethodSig{Name: method.Name, FuncName: impl.ForTypeName + "." + method.Name, TypeParams: combinedParams}
			c.selfType = &selfType
			restoreMethod := c.pushTypeParams(method.TypeParams)
			for i, param := range method.Params {
				if param.Name == "self" && i != 0 {
					c.diag.Add(param.Span, "self must be first parameter")
				}
				t := c.fromAstType(param.Type)
				sig.Params = append(sig.Params, t)
			}
			if len(method.Params) > 0 && method.Params[0].Name == "self" {
				sig.HasSelf = true
				sig.SelfType = sig.Params[0]
			}
			if method.ReturnType != nil {
				sig.Return = c.fromAstType(*method.ReturnType)
				sig.ReturnExplicit = true
			} else {
				sig.Return = Type{Kind: TypeInvalid}
				sig.ReturnExplicit = false
			}
			c.methods[impl.ForTypeName][method.Name] = sig
			c.popTypeParams(restoreMethod)
			c.selfType = nil
		}
		c.popTypeParams(restoreImpl)
	}
}

func (c *Checker) checkFunction(fn *ast.Function) {
	sig, ok := c.funcs[fn.Name]
	if !ok {
		return
	}
	restore := c.pushTypeParams(fn.TypeParams)
	c.current = sig
	c.env = newEnv()
	c.env.push()
	for i, param := range fn.Params {
		c.env.declare(param.Name, VarInfo{Type: sig.Params[i], Mutable: false})
	}
	c.inferReturn = !sig.ReturnExplicit
	c.inferredType = Type{Kind: TypeInvalid}
	c.hasReturn = false
	c.hasBareReturn = false

	c.checkFunctionBlock(fn.Body)

	if c.inferReturn {
		if c.hasReturn {
			if c.inferredType.Kind != TypeInvalid {
				sig.Return = c.inferredType
			}
			if c.hasBareReturn && sig.Return.Kind != TypeUnit {
				c.diag.Add(fn.Span(), "mixing 'return' and 'return value' in inferred function")
			}
		} else {
			sig.Return = Type{Kind: TypeUnit}
		}
	}

	c.env.pop()
	c.current = nil
	c.popTypeParams(restore)
}

func (c *Checker) lookupMethod(typeName string, method string) *MethodSig {
	if typeName == "" {
		return nil
	}
	if c.methods == nil {
		return nil
	}
	if byType, ok := c.methods[typeName]; ok {
		if sig, ok := byType[method]; ok {
			return sig
		}
	}
	return nil
}

func (c *Checker) checkMethod(selfType Type, fn *ast.Function) {
	typeName := selfType.Name
	sig := c.lookupMethod(typeName, fn.Name)
	if sig == nil {
		return
	}
	if sig.HasSelf {
		if len(fn.Params) == 0 || fn.Params[0].Name != "self" {
			c.diag.Add(fn.Span(), "self must be first parameter")
		} else {
			base := sig.SelfType
			if base.Ref {
				base = derefType(base)
			}
			if base.Kind != TypeStruct && base.Kind != TypeEnum {
				c.diag.Add(fn.Params[0].Span, "self must be Self, &Self, or &mut Self")
			} else if base.Name != typeName {
				c.diag.Add(fn.Params[0].Span, "self type must be Self")
			}
		}
	}

	c.current = &FuncSig{Name: sig.FuncName, Params: sig.Params, Return: sig.Return, ReturnExplicit: sig.ReturnExplicit, TypeParams: sig.TypeParams}
	restore := c.pushTypeParams(sig.TypeParams)
	c.env = newEnv()
	c.env.push()
	for i, param := range fn.Params {
		c.env.declare(param.Name, VarInfo{Type: sig.Params[i], Mutable: false})
	}
	c.inferReturn = !sig.ReturnExplicit
	c.inferredType = Type{Kind: TypeInvalid}
	c.hasReturn = false
	c.hasBareReturn = false

	c.selfType = &selfType
	c.checkFunctionBlock(fn.Body)
	c.selfType = nil

	if c.inferReturn {
		if c.hasReturn {
			if c.inferredType.Kind != TypeInvalid {
				sig.Return = c.inferredType
			}
			if c.hasBareReturn && sig.Return.Kind != TypeUnit {
				c.diag.Add(fn.Span(), "mixing 'return' and 'return value' in inferred function")
			}
		} else {
			sig.Return = Type{Kind: TypeUnit}
		}
	}

	c.env.pop()
	c.current = nil
	c.popTypeParams(restore)
}
