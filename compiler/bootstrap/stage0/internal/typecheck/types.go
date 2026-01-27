package typecheck

import (
	"strings"

	"dastlang/internal/ast"
)

type Kind int

const (
	TypeInvalid Kind = iota
	TypeParam
	TypeInt
	TypeBool
	TypeString
	TypeStr
	TypeUnit
	TypeStruct
	TypeEnum
	TypeArray
	TypeTuple
	TypeClosure
	TypeAstExpr
	TypeAstStmt
	TypeAstItem
	TypeAstBlock
)

type Type struct {
	Kind Kind
	Ref  bool
	Mut  bool
	Name string
	Elem *Type
	Args []Type
	Elems []Type
}

func (t Type) String() string {
	base := t.baseName()
	if t.Ref {
		if t.Mut {
			return "&mut " + base
		}
		return "&" + base
	}
	return base
}

func (t Type) baseName() string {
	switch t.Kind {
	case TypeParam:
		if t.Name != "" {
			return t.Name
		}
		return "T"
	case TypeInt:
		if t.Name != "" {
			return t.Name
		}
		return "int"
	case TypeBool:
		return "bool"
	case TypeString:
		if t.Name != "" {
			return t.Name
		}
		return "String"
	case TypeStr:
		return "str"
	case TypeUnit:
		return "unit"
	case TypeStruct, TypeEnum:
		if t.Name != "" {
			return t.Name + formatTypeArgs(t.Args)
		}
		return "named"
	case TypeArray:
		if t.Elem != nil {
			return "[" + t.Elem.String() + "]"
		}
		return "[]"
	case TypeTuple:
		if len(t.Elems) == 0 {
			return "()"
		}
		if len(t.Elems) == 1 {
			return "(" + t.Elems[0].String() + ",)"
		}
		parts := make([]string, 0, len(t.Elems))
		for _, e := range t.Elems {
			parts = append(parts, e.String())
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case TypeClosure:
		return "closure"
	case TypeAstExpr:
		return "AstExpr"
	case TypeAstStmt:
		return "AstStmt"
	case TypeAstItem:
		return "AstItem"
	case TypeAstBlock:
		return "AstBlock"
	default:
		if t.Name != "" {
			return t.Name + formatTypeArgs(t.Args)
		}
		return "invalid"
	}
}

func formatTypeArgs(args []Type) string {
	if len(args) == 0 {
		return ""
	}
	out := "["
	for i, a := range args {
		if i > 0 {
			out += ", "
		}
		out += a.String()
	}
	out += "]"
	return out
}

type FuncSig struct {
	Name           string
	TypeParams     []ast.TypeParam
	Params         []Type
	Return         Type
	ReturnExplicit bool
}

type MethodSig struct {
	Name           string
	FuncName       string
	TypeParams     []ast.TypeParam
	Params         []Type
	Return         Type
	ReturnExplicit bool
	HasSelf        bool
	SelfType       Type
}

type VarInfo struct {
	Type    Type
	Mutable bool
}
