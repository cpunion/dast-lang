package typecheck

import (
	"os"
	"strconv"
	"strings"

	"dastlang/internal/ast"
)

func (c *Checker) fromAstType(t ast.Type) Type {
	if t.IsArray {
		if t.Elem == nil {
			return Type{Kind: TypeInvalid}
		}
		elem := c.fromAstType(*t.Elem)
		arrayType := Type{Kind: TypeArray, Elem: &elem}
		if t.IsRef {
			arrayType.Ref = true
			arrayType.Mut = t.IsMut
		}
		return arrayType
	}
	if t.IsTuple {
		if len(t.TupleElems) == 0 {
			unit := Type{Kind: TypeUnit}
			if t.IsRef {
				unit.Ref = true
				unit.Mut = t.IsMut
			}
			return unit
		}
		var elems []Type
		for _, e := range t.TupleElems {
			elems = append(elems, c.fromAstType(e))
		}
		tupleType := Type{Kind: TypeTuple, Elems: elems}
		if t.IsRef {
			tupleType.Ref = true
			tupleType.Mut = t.IsMut
		}
		return tupleType
	}
	name := t.Name
	if name == "Self" {
		if c.selfType != nil {
			base := *c.selfType
			if t.IsRef {
				base.Ref = true
				base.Mut = t.IsMut
			}
			return base
		}
		c.diag.Add(t.Span, "unknown type 'Self'")
		return Type{Kind: TypeInvalid}
	}
	if param, ok := c.typeParams[name]; ok {
		_ = param
		if len(t.Args) > 0 {
			c.diag.Add(t.Span, "type arguments not allowed on type parameter")
			return Type{Kind: TypeInvalid}
		}
		base := Type{Kind: TypeParam, Name: name}
		if t.IsRef {
			base.Ref = true
			base.Mut = t.IsMut
		}
		return base
	}
	if alias, ok := c.aliases[name]; ok {
		expanded, ok := c.expandAlias(alias, t)
		if !ok {
			return Type{Kind: TypeInvalid}
		}
		return c.fromAstType(expanded)
	}
	var args []Type
	if len(t.Args) > 0 {
		for _, arg := range t.Args {
			args = append(args, c.fromAstType(arg))
		}
	}
	if name == "string" {
		c.diag.Add(t.Span, "use 'String' or 'str' instead of 'string'")
		return Type{Kind: TypeInvalid}
	}
	base := Type{Kind: TypeInvalid, Name: name}
	switch name {
	case "int", "i8", "i16", "i32", "i64", "i128", "u8", "u16", "u32", "u64", "u128", "isize", "usize", "char", "untyped-int":
		base.Kind = TypeInt
		base.Name = name
	case "f32", "f64", "untyped-float":
		base.Kind = TypeFloat
		base.Name = name
	case "bool":
		base.Kind = TypeBool
	case "String":
		base.Kind = TypeString
		base.Name = "String"
	case "str":
		if !t.IsRef {
			c.diag.Add(t.Span, "type 'str' must be used as '&str'")
			return Type{Kind: TypeInvalid}
		}
		base.Kind = TypeStr
		base.Name = "str"
	case "unit", "()":
		base.Kind = TypeUnit
	case "Closure", "closure":
		base.Kind = TypeClosure
	case "AstExpr":
		base.Kind = TypeAstExpr
	case "AstStmt":
		base.Kind = TypeAstStmt
	case "AstItem":
		base.Kind = TypeAstItem
	case "AstBlock":
		base.Kind = TypeAstBlock
	default:
		if _, ok := c.structs[name]; ok {
			base.Kind = TypeStruct
			base.Name = name
			base.Args = args
			if len(t.Args) == 0 && len(c.structs[name].TypeParams) > 0 {
				c.diag.Add(t.Span, "missing type arguments for '"+name+"'")
			}
			if len(t.Args) > 0 && len(c.structs[name].TypeParams) == 0 {
				c.diag.Add(t.Span, "type arguments not allowed for '"+name+"'")
			}
			if len(c.structs[name].TypeParams) > 0 && isConcreteArgs(args) {
				base.Name = c.ensureStructInstance(name, args)
				base.Args = nil
			}
			break
		}
		if _, ok := c.enums[name]; ok {
			base.Kind = TypeEnum
			base.Name = name
			base.Args = args
			if len(t.Args) == 0 && len(c.enums[name].TypeParams) > 0 {
				c.diag.Add(t.Span, "missing type arguments for '"+name+"'")
			}
			if len(t.Args) > 0 && len(c.enums[name].TypeParams) == 0 {
				c.diag.Add(t.Span, "type arguments not allowed for '"+name+"'")
			}
			if len(c.enums[name].TypeParams) > 0 && isConcreteArgs(args) {
				base.Name = c.ensureEnumInstance(name, args)
				base.Args = nil
			}
			break
		}
		c.diag.Add(t.Span, "unknown type '"+name+"'")
	}
	if len(t.Args) > 0 && base.Kind != TypeStruct && base.Kind != TypeEnum && base.Kind != TypeInvalid {
		c.diag.Add(t.Span, "type arguments not allowed on '"+name+"'")
	}
	if t.IsRef {
		if base.Ref {
			c.diag.Add(t.Span, "double reference type is not allowed")
			return Type{Kind: TypeInvalid}
		}
		base.Ref = true
		base.Mut = t.IsMut
	}
	return base
}

func typesEqual(a, b Type) bool {
	if a.Kind == TypeInvalid || b.Kind == TypeInvalid {
		return false
	}
	if a.Kind != b.Kind {
		return false
	}
	if a.Kind == TypeParam {
		return a.Name == b.Name && a.Ref == b.Ref && a.Mut == b.Mut
	}
	if a.Kind == TypeInt {
		return canonicalIntName(a.Name) == canonicalIntName(b.Name) && a.Ref == b.Ref && a.Mut == b.Mut
	}
	if a.Kind == TypeFloat {
		return a.Name == b.Name && a.Ref == b.Ref && a.Mut == b.Mut
	}
	if (a.Kind == TypeStruct || a.Kind == TypeEnum) && a.Name != b.Name {
		return false
	}
	if a.Kind == TypeArray {
		if a.Elem == nil || b.Elem == nil {
			return false
		}
		return typesEqual(*a.Elem, *b.Elem) && a.Ref == b.Ref && a.Mut == b.Mut
	}
	if a.Kind == TypeTuple {
		if len(a.Elems) != len(b.Elems) {
			return false
		}
		for i := range a.Elems {
			if !typesEqual(a.Elems[i], b.Elems[i]) {
				return false
			}
		}
		return a.Ref == b.Ref && a.Mut == b.Mut
	}
	if len(a.Args) != len(b.Args) {
		return false
	}
	for i := range a.Args {
		if !typesEqual(a.Args[i], b.Args[i]) {
			return false
		}
	}
	return a.Ref == b.Ref && a.Mut == b.Mut
}

func isAstType(t Type) bool {
	switch t.Kind {
	case TypeAstExpr, TypeAstStmt, TypeAstItem, TypeAstBlock:
		return true
	default:
		return false
	}
}

func typesAssignable(actual, expected Type) bool {
	if typesEqual(actual, expected) {
		return true
	}
	if isUntypedInt(actual) && isInt(expected) {
		return true
	}
	if isUntypedInt(actual) && isFloat(expected) {
		return true
	}
	if isUntypedFloat(actual) && isFloat(expected) {
		return true
	}
	if isString(actual) && isString(expected) {
		if actual.Kind == TypeString && expected.Kind == TypeStr {
			if actual.Ref && expected.Ref && !expected.Mut {
				return true
			}
			return false
		}
		if actual.Kind == TypeStr && expected.Kind == TypeStr {
			return actual.Ref == expected.Ref && actual.Mut == expected.Mut
		}
		return false
	}
	if !actual.Ref && expected.Ref && !expected.Mut && actual.Kind == TypeArray && expected.Kind == TypeArray {
		if actual.Elem != nil && expected.Elem != nil {
			// Allow [T] -> &[U] only when element types are compatible.
			return typesAssignable(*actual.Elem, *expected.Elem)
		}
		return false
	}
	if expected.Ref && !expected.Mut && actual.Ref && actual.Mut {
		a := actual
		b := expected
		a.Mut = false
		b.Mut = false
		return typesEqual(a, b)
	}
	return false
}

func isInt(t Type) bool {
	return t.Kind == TypeInt && !t.Ref && t.Name != "untyped-int" && t.Name != "char"
}

func isUntypedInt(t Type) bool {
	return t.Kind == TypeInt && !t.Ref && t.Name == "untyped-int"
}

func isChar(t Type) bool {
	return t.Kind == TypeInt && !t.Ref && t.Name == "char"
}

func isFloat(t Type) bool {
	return t.Kind == TypeFloat && !t.Ref && t.Name != "untyped-float"
}

func isUntypedFloat(t Type) bool {
	return t.Kind == TypeFloat && !t.Ref && t.Name == "untyped-float"
}

func isBool(t Type) bool {
	return t.Kind == TypeBool && !t.Ref
}

func isString(t Type) bool {
	return t.Kind == TypeString || t.Kind == TypeStr
}

func ptrWidthBytes() int64 {
	if v := os.Getenv("DAST_TARGET_PTR_WIDTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n == 32 {
				return 4
			}
			if n == 64 {
				return 8
			}
		}
	}
	return 8
}

func intTypeName(t Type) string {
	if t.Name != "" {
		return t.Name
	}
	return "int"
}

func intTypeWidth(name string) int64 {
	switch strings.TrimSpace(name) {
	case "untyped-int":
		return 0
	case "i8", "u8":
		return 8
	case "i16", "u16":
		return 16
	case "i32", "u32", "char":
		return 32
	case "i64", "u64", "int":
		return 64
	case "isize", "usize":
		return ptrWidthBytes() * 8
	}
	return 0
}

func isSignedIntName(name string) bool {
	switch strings.TrimSpace(name) {
	case "i8", "i16", "i32", "i64", "int", "isize":
		return true
	}
	return false
}

func isUnsignedIntName(name string) bool {
	switch strings.TrimSpace(name) {
	case "u8", "u16", "u32", "u64", "usize":
		return true
	}
	return false
}

func canonicalIntName(name string) string {
	n := strings.TrimSpace(name)
	if n == "int" {
		return "i64"
	}
	return n
}

func i64MaxValue() int64 { return 9223372036854775807 }

func i64MinValue() int64 { return -9223372036854775807 - 1 }

func intTypeMin(name string) int64 {
	if !isSignedIntName(name) && !isUnsignedIntName(name) {
		return 0
	}
	if isUnsignedIntName(name) {
		return 0
	}
	bits := intTypeWidth(name)
	if bits >= 64 {
		return i64MinValue()
	}
	shift := bits - 1
	var v int64 = 1
	for i := int64(0); i < shift; i++ {
		v *= 2
	}
	return -v
}

func intTypeMax(name string) int64 {
	if !isSignedIntName(name) && !isUnsignedIntName(name) {
		return 0
	}
	bits := intTypeWidth(name)
	if isUnsignedIntName(name) {
		if bits >= 63 {
			return i64MaxValue()
		}
		var v int64 = 1
		for i := int64(0); i < bits; i++ {
			v *= 2
		}
		return v - 1
	}
	if bits >= 64 {
		return i64MaxValue()
	}
	shift := bits - 1
	var v int64 = 1
	for i := int64(0); i < shift; i++ {
		v *= 2
	}
	return v - 1
}

func intValueFitsType(val int64, name string) bool {
	if !isSignedIntName(name) && !isUnsignedIntName(name) {
		return false
	}
	return val >= intTypeMin(name) && val <= intTypeMax(name)
}

func intPeerTypeName(a, b string) string {
	la := canonicalIntName(a)
	lb := canonicalIntName(b)
	if intTypeWidth(la) == 0 || intTypeWidth(lb) == 0 {
		return ""
	}
	if la == lb {
		return la
	}
	return ""
}

func constValueType(v ast.ConstValue) Type {
	switch v.Kind {
	case ast.ConstInt:
		return Type{Kind: TypeInt, Name: "untyped-int"}
	case ast.ConstBool:
		return Type{Kind: TypeBool, Name: "bool"}
	case ast.ConstString:
		return Type{Kind: TypeString, Name: "String"}
	case ast.ConstChar:
		return Type{Kind: TypeInt, Name: "char"}
	case ast.ConstFloat:
		return Type{Kind: TypeFloat, Name: "untyped-float"}
	default:
		return Type{Kind: TypeInvalid}
	}
}

func isValidEnumRepr(name string) bool {
	switch name {
	case "i8", "i16", "i32", "i64", "i128",
		"u8", "u16", "u32", "u64", "u128",
		"isize", "usize":
		return true
	default:
		return false
	}
}

func isComparable(t Type) bool {
	return isInt(t) || isChar(t) || isFloat(t) || isBool(t) || isString(t) || t.Kind == TypeEnum
}

func derefType(t Type) Type {
	return Type{Kind: t.Kind, Name: t.Name, Elem: t.Elem, Args: t.Args}
}

func (c *Checker) resolveStructDecl(t Type) (*ast.StructDecl, map[string]Type, bool) {
	decl, ok := c.structs[t.Name]
	args := t.Args
	if !ok {
		if base, okBase := c.structInstBase[t.Name]; okBase {
			decl, ok = c.structs[base]
			if mapped, okMapped := c.structInstArgs[t.Name]; okMapped {
				args = mapped
			} else {
				args = nil
			}
		}
	}
	if !ok || decl == nil {
		return nil, nil, false
	}
	subst := map[string]Type{}
	if len(decl.TypeParams) > 0 && len(args) == len(decl.TypeParams) {
		for i, p := range decl.TypeParams {
			subst[p.Name] = args[i]
		}
	}
	return decl, subst, true
}

func findField(decl *ast.StructDecl, name string) *ast.FieldDef {
	for i := range decl.Fields {
		if decl.Fields[i].Name == name {
			return &decl.Fields[i]
		}
	}
	return nil
}

func findVariant(decl *ast.EnumDecl, name string) *ast.VariantDef {
	for i := range decl.Variants {
		if decl.Variants[i].Name == name {
			return &decl.Variants[i]
		}
	}
	return nil
}
