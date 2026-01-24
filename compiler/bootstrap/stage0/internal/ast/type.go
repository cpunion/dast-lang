package ast

import "dastlang/internal/source"

type Type struct {
	Name    string
	IsRef   bool
	IsMut   bool
	IsArray bool
	Elem    *Type
	Args    []Type
	Span    source.Span
}
