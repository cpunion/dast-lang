package ast

import "dastlang/internal/source"

// CompileItem represents a top-level compile! invocation.
type CompileItem struct {
	Expr     Expr
	SpanInfo source.Span
}

func (c *CompileItem) itemNode() {}
func (c *CompileItem) Span() source.Span {
	return c.SpanInfo
}

