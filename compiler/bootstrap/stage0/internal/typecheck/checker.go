package typecheck

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/diag"
)

type Checker struct {
	diag     *diag.Bag
	funcs    map[string]*FuncSig
	methods  map[string]map[string]*MethodSig
	builtins map[string]struct{}
	consts   map[string]ConstInfo
	structs  map[string]*ast.StructDecl
	enums    map[string]*ast.EnumDecl
	env      *env
	current  *FuncSig
	selfType string

	inferReturn   bool
	inferredType  Type
	hasReturn     bool
	hasBareReturn bool
}

type ConstInfo struct {
	Type  Type
	Value ast.ConstValue
}

func Check(prog *ast.Program) *diag.Bag {
	c := &Checker{
		diag:     &diag.Bag{},
		funcs:    map[string]*FuncSig{},
		methods:  map[string]map[string]*MethodSig{},
		builtins: map[string]struct{}{"print": {}, "println": {}, "len": {}, "push": {}, "read_file": {}, "write_file": {}, "args": {}, "char_at": {}, "substr": {}, "pop": {}},
		consts:   map[string]ConstInfo{},
		structs:  map[string]*ast.StructDecl{},
		enums:    map[string]*ast.EnumDecl{},
	}
	c.collectDecls(prog)
	c.collectConsts(prog)
	c.collectSignatures(prog)
	for _, item := range prog.Items {
		fn, ok := item.(*ast.Function)
		if !ok {
			continue
		}
		c.checkFunction(fn)
	}
	for _, item := range prog.Items {
		impl, ok := item.(*ast.ImplDecl)
		if !ok {
			continue
		}
		for _, method := range impl.Methods {
			c.checkMethod(impl.TypeName, method)
		}
	}
	return c.diag
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
		case *ast.ConstDecl:
			if _, exists := c.structs[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by struct", t.Name))
			}
			if _, exists := c.enums[t.Name]; exists {
				c.diag.Add(t.Span(), fmt.Sprintf("name '%s' already used by enum", t.Name))
			}
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
			if !typesEqual(finalType, valueType) && finalType.Kind != TypeInvalid && valueType.Kind != TypeInvalid {
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
		sig := &FuncSig{Name: fn.Name}
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
		c.funcs[fn.Name] = sig
	}
	for _, item := range prog.Items {
		impl, ok := item.(*ast.ImplDecl)
		if !ok {
			continue
		}
		for _, method := range impl.Methods {
			if _, exists := c.methods[impl.TypeName]; !exists {
				c.methods[impl.TypeName] = map[string]*MethodSig{}
			}
			if _, exists := c.methods[impl.TypeName][method.Name]; exists {
				c.diag.Add(method.Span(), fmt.Sprintf("duplicate method '%s' for '%s'", method.Name, impl.TypeName))
				continue
			}
			sig := &MethodSig{Name: method.Name, FuncName: impl.TypeName + "." + method.Name}
			c.selfType = impl.TypeName
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
			c.selfType = ""
		}
	}
}

func (c *Checker) checkFunction(fn *ast.Function) {
	sig, ok := c.funcs[fn.Name]
	if !ok {
		return
	}
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

func (c *Checker) checkMethod(typeName string, fn *ast.Function) {
	sig := c.lookupMethod(typeName, fn.Name)
	if sig == nil {
		return
	}
	if sig.HasSelf {
		if len(fn.Params) == 0 || fn.Params[0].Name != "self" {
			c.diag.Add(fn.Span(), "self must be first parameter")
		} else {
			selfType := sig.SelfType
			base := selfType
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

	c.current = &FuncSig{Name: sig.FuncName, Params: sig.Params, Return: sig.Return, ReturnExplicit: sig.ReturnExplicit}
	c.env = newEnv()
	c.env.push()
	for i, param := range fn.Params {
		c.env.declare(param.Name, VarInfo{Type: sig.Params[i], Mutable: false})
	}
	c.inferReturn = !sig.ReturnExplicit
	c.inferredType = Type{Kind: TypeInvalid}
	c.hasReturn = false
	c.hasBareReturn = false

	c.selfType = typeName
	c.checkFunctionBlock(fn.Body)
	c.selfType = ""

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
}
