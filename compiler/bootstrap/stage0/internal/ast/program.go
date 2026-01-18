package ast

import "dastlang/internal/source"

type Program struct {
	Items []Item
}

type Item interface {
	itemNode()
	Span() source.Span
}
