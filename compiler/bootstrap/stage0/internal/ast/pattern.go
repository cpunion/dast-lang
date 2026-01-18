package ast

import "dastlang/internal/source"

type Pattern interface {
	patternNode()
	Span() source.Span
}

type WildcardPattern struct {
	SpanInfo source.Span
}

func (p *WildcardPattern) patternNode()      {}
func (p *WildcardPattern) Span() source.Span { return p.SpanInfo }

type VariantPattern struct {
	EnumName string
	Variant  string
	Binding  string
	SpanInfo source.Span
}

func (p *VariantPattern) patternNode()      {}
func (p *VariantPattern) Span() source.Span { return p.SpanInfo }
