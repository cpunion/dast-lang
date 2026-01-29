package ast

import "dastlang/internal/source"

type TraitDecl struct {
	Name       string
	TypeParams []TypeParam
	Methods    []TraitMethod
	Vis        Visibility
	SpanInfo   source.Span
}

func (t *TraitDecl) itemNode() {}
func (t *TraitDecl) Span() source.Span {
	return t.SpanInfo
}

type TraitMethod struct {
	Name       string
	Params     []Param
	ReturnType *Type
	SpanInfo   source.Span
}

type ImplTraitDecl struct {
	TraitName   string
	TraitArgs   []Type
	ForTypeName string
	ForTypeArgs []Type
	TypeParams  []TypeParam
	Methods     []*Function
	Vis         Visibility
	SpanInfo    source.Span
}

func (i *ImplTraitDecl) itemNode() {}
func (i *ImplTraitDecl) Span() source.Span {
	return i.SpanInfo
}

type TypeAlias struct {
	Name       string
	TypeParams []TypeParam
	Value      Type
	Vis        Visibility
	SpanInfo   source.Span
}

func (t *TypeAlias) itemNode() {}
func (t *TypeAlias) Span() source.Span {
	return t.SpanInfo
}
