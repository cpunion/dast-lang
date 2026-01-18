package diag

import (
	"fmt"
	"strings"

	"dastlang/internal/source"
)

type Diagnostic struct {
	Message string
	Span    source.Span
}

type Bag struct {
	Items []Diagnostic
}

func (b *Bag) Add(span source.Span, msg string) {
	b.Items = append(b.Items, Diagnostic{Message: msg, Span: span})
}

func (b *Bag) HasErrors() bool {
	return len(b.Items) > 0
}

func (b *Bag) Error() string {
	if len(b.Items) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, d := range b.Items {
		if i > 0 {
			sb.WriteString("\n")
		}
		if d.Span.Start.Filename != "" {
			sb.WriteString(fmt.Sprintf("%s:%d:%d: %s", d.Span.Start.Filename, d.Span.Start.Line, d.Span.Start.Column, d.Message))
		} else {
			sb.WriteString(d.Message)
		}
	}
	return sb.String()
}
