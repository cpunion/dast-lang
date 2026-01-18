package typecheck

import (
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
	name := t.Name
	if name == "Self" && c.selfType != "" {
		name = c.selfType
	}
	base := Type{Kind: TypeInvalid, Name: name}
	switch name {
	case "i8", "i16", "i32", "i64", "i128", "u8", "u16", "u32", "u64", "u128", "isize", "usize":
		base.Kind = TypeInt
	case "bool":
		base.Kind = TypeBool
	case "String", "str":
		base.Kind = TypeString
	case "unit", "()":
		base.Kind = TypeUnit
	default:
		if _, ok := c.structs[name]; ok {
			base.Kind = TypeStruct
			base.Name = name
			break
		}
		if _, ok := c.enums[name]; ok {
			base.Kind = TypeEnum
			base.Name = name
			break
		}
		c.diag.Add(t.Span, "unknown type '"+name+"'")
	}
	if t.IsRef {
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
	if (a.Kind == TypeStruct || a.Kind == TypeEnum) && a.Name != b.Name {
		return false
	}
	if a.Kind == TypeArray {
		if a.Elem == nil || b.Elem == nil {
			return false
		}
		return typesEqual(*a.Elem, *b.Elem) && a.Ref == b.Ref && a.Mut == b.Mut
	}
	return a.Ref == b.Ref && a.Mut == b.Mut
}

func typesAssignable(actual, expected Type) bool {
	if typesEqual(actual, expected) {
		return true
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
	return t.Kind == TypeInt && !t.Ref
}

func isBool(t Type) bool {
	return t.Kind == TypeBool && !t.Ref
}

func isString(t Type) bool {
	return t.Kind == TypeString && !t.Ref
}

func constValueType(v ast.ConstValue) Type {
	switch v.Kind {
	case ast.ConstInt:
		return Type{Kind: TypeInt, Name: "int"}
	case ast.ConstBool:
		return Type{Kind: TypeBool, Name: "bool"}
	case ast.ConstString:
		return Type{Kind: TypeString, Name: "string"}
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
	return isInt(t) || isBool(t) || isString(t)
}

func derefType(t Type) Type {
	return Type{Kind: t.Kind, Name: t.Name, Elem: t.Elem}
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
