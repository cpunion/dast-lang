package ast

import "dastlang/internal/source"

type Function struct {
	Name       string
	Params     []Param
	ReturnType *Type
	Body       *Block
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
	Fields   []FieldDef
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
	SpanInfo source.Span
}

func (c *ConstDecl) itemNode() {}
func (c *ConstDecl) Span() source.Span {
	return c.SpanInfo
}

type EnumDecl struct {
	Name     string
	Repr     string
	Variants []VariantDef
	SpanInfo source.Span
}

func (e *EnumDecl) itemNode() {}
func (e *EnumDecl) Span() source.Span {
	return e.SpanInfo
}

type ImplDecl struct {
	TypeName string
	Methods  []*Function
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
