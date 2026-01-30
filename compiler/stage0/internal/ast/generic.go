package ast

import "dastlang/internal/source"

type TypeParam struct {
	Name   string
	Bounds []Type
	Span   source.Span
}
