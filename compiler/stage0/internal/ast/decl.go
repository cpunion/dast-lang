package ast

import "dastlang/internal/source"

type Function struct {
	Name       string
	TypeParams []TypeParam
	Params     []Param
	ReturnType *Type
	Body       *Block
	Vis        Visibility
	IsMacro    bool
	SpanInfo   source.Span
}

func (f *Function) itemNode() {}
func (f *Function) Span() source.Span {
	return f.SpanInfo
}

type ImportDecl struct {
	Path     string
	Alias    string
	SpanInfo source.Span
}

func (i *ImportDecl) itemNode() {}
func (i *ImportDecl) Span() source.Span {
	return i.SpanInfo
}

type StructDecl struct {
	Name     string
	TypeParams []TypeParam
	Fields   []FieldDef
	Vis      Visibility
	SpanInfo source.Span
}

func (s *StructDecl) itemNode() {}
func (s *StructDecl) Span() source.Span {
	return s.SpanInfo
}

type ConstKind int

const (
	ConstInt ConstKind = iota
	ConstBool
	ConstString
)

type ConstValue struct {
	Kind ConstKind
	Int  int64
	Bool bool
	Str  string
	Span source.Span
}

type ConstDecl struct {
	Name     string
	Type     *Type
	Value    ConstValue
	Vis      Visibility
	SpanInfo source.Span
}

func (c *ConstDecl) itemNode() {}
func (c *ConstDecl) Span() source.Span {
	return c.SpanInfo
}

type EnumDecl struct {
	Name     string
	TypeParams []TypeParam
	Repr     string
	Variants []VariantDef
	Vis      Visibility
	SpanInfo source.Span
}

func (e *EnumDecl) itemNode() {}
func (e *EnumDecl) Span() source.Span {
	return e.SpanInfo
}

type ImplDecl struct {
	TypeName string
	TypeArgs []Type
	TypeParams []TypeParam
	Methods  []*Function
	Vis      Visibility
	SpanInfo source.Span
}

func (i *ImplDecl) itemNode() {}
func (i *ImplDecl) Span() source.Span {
	return i.SpanInfo
}

type VariantDef struct {
	Name      string
	Payload   *Type
	HasValue  bool
	Value     int64
	ValueSpan source.Span
	SpanInfo  source.Span
}

type FieldDef struct {
	Name string
	Type Type
	Span source.Span
}

type Param struct {
	Name string
	Type Type
	Span source.Span
}
