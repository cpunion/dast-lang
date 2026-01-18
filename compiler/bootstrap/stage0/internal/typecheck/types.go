package typecheck

type Kind int

const (
	TypeInvalid Kind = iota
	TypeInt
	TypeBool
	TypeString
	TypeUnit
	TypeStruct
	TypeEnum
	TypeArray
)

type Type struct {
	Kind Kind
	Ref  bool
	Mut  bool
	Name string
	Elem *Type
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
	case TypeInt:
		return "int"
	case TypeBool:
		return "bool"
	case TypeString:
		return "string"
	case TypeUnit:
		return "unit"
	case TypeStruct, TypeEnum:
		if t.Name != "" {
			return t.Name
		}
		return "named"
	case TypeArray:
		if t.Elem != nil {
			return "[" + t.Elem.String() + "]"
		}
		return "[]"
	default:
		if t.Name != "" {
			return t.Name
		}
		return "invalid"
	}
}

type FuncSig struct {
	Name           string
	Params         []Type
	Return         Type
	ReturnExplicit bool
}

type MethodSig struct {
	Name           string
	FuncName       string
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
