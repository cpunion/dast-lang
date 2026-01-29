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

type BindingPattern struct {
	Name     string
	SpanInfo source.Span
}

func (p *BindingPattern) patternNode()      {}
func (p *BindingPattern) Span() source.Span { return p.SpanInfo }

type VariantPattern struct {
	EnumName string
	Variant  string
	Binding  string
	Payload  Pattern
	SpanInfo source.Span
}

func (p *VariantPattern) patternNode()      {}
func (p *VariantPattern) Span() source.Span { return p.SpanInfo }

type LiteralPattern struct {
	Value    ConstValue
	SpanInfo source.Span
}

func (p *LiteralPattern) patternNode()      {}
func (p *LiteralPattern) Span() source.Span { return p.SpanInfo }

type RangePattern struct {
	Start     Expr
	End       Expr
	Inclusive bool
	SpanInfo  source.Span
}

func (p *RangePattern) patternNode()      {}
func (p *RangePattern) Span() source.Span { return p.SpanInfo }

type OrPattern struct {
	Alts     []Pattern
	SpanInfo source.Span
}

func (p *OrPattern) patternNode()      {}
func (p *OrPattern) Span() source.Span { return p.SpanInfo }

type StructFieldPattern struct {
	Name    string
	Binding string
	Pattern Pattern
	Span    source.Span
}

type StructPattern struct {
	StructName string
	Fields     []StructFieldPattern
	SpanInfo   source.Span
}

func (p *StructPattern) patternNode()      {}
func (p *StructPattern) Span() source.Span { return p.SpanInfo }

type TuplePattern struct {
	Elems    []Pattern
	SpanInfo source.Span
}

func (p *TuplePattern) patternNode()      {}
func (p *TuplePattern) Span() source.Span { return p.SpanInfo }

type ArrayPattern struct {
	Elems    []Pattern
	SpanInfo source.Span
}

func (p *ArrayPattern) patternNode()      {}
func (p *ArrayPattern) Span() source.Span { return p.SpanInfo }
