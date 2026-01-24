package ast

import "dastlang/internal/source"

type TypeParam struct {
	Name   string
	Bounds []string
	Span   source.Span
}
