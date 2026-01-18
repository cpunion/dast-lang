package ast

import "dastlang/internal/source"

type Expr interface {
	exprNode()
	Span() source.Span
}

type IdentExpr struct {
	Name     string
	SpanInfo source.Span
}

func (e *IdentExpr) exprNode()         {}
func (e *IdentExpr) Span() source.Span { return e.SpanInfo }

type IntLit struct {
	Value    int64
	SpanInfo source.Span
}

func (e *IntLit) exprNode()         {}
func (e *IntLit) Span() source.Span { return e.SpanInfo }

type BoolLit struct {
	Value    bool
	SpanInfo source.Span
}

func (e *BoolLit) exprNode()         {}
func (e *BoolLit) Span() source.Span { return e.SpanInfo }

type StringLit struct {
	Value    string
	SpanInfo source.Span
}

func (e *StringLit) exprNode()         {}
func (e *StringLit) Span() source.Span { return e.SpanInfo }

type ArrayLit struct {
	Elems    []Expr
	SpanInfo source.Span
}

func (e *ArrayLit) exprNode()         {}
func (e *ArrayLit) Span() source.Span { return e.SpanInfo }

type UnaryExpr struct {
	Op       string
	Expr     Expr
	SpanInfo source.Span
}

func (e *UnaryExpr) exprNode()         {}
func (e *UnaryExpr) Span() source.Span { return e.SpanInfo }

type RefExpr struct {
	Mutable  bool
	Expr     Expr
	SpanInfo source.Span
}

func (e *RefExpr) exprNode()         {}
func (e *RefExpr) Span() source.Span { return e.SpanInfo }

type DerefExpr struct {
	Expr     Expr
	SpanInfo source.Span
}

func (e *DerefExpr) exprNode()         {}
func (e *DerefExpr) Span() source.Span { return e.SpanInfo }

type BinaryExpr struct {
	Op       string
	Left     Expr
	Right    Expr
	SpanInfo source.Span
}

func (e *BinaryExpr) exprNode()         {}
func (e *BinaryExpr) Span() source.Span { return e.SpanInfo }

type CallExpr struct {
	Callee   string
	Args     []Expr
	SpanInfo source.Span
}

func (e *CallExpr) exprNode()         {}
func (e *CallExpr) Span() source.Span { return e.SpanInfo }

type MethodCallExpr struct {
	Receiver      Expr
	Method        string
	Args          []Expr
	ResolvedName  string
	ResolvedSelf  bool
	EnumName      string
	SpanInfo      source.Span
}

func (e *MethodCallExpr) exprNode()         {}
func (e *MethodCallExpr) Span() source.Span { return e.SpanInfo }

type StructLit struct {
	Name     string
	Fields   []FieldInit
	SpanInfo source.Span
}

func (e *StructLit) exprNode()         {}
func (e *StructLit) Span() source.Span { return e.SpanInfo }

type FieldInit struct {
	Name  string
	Value Expr
	Span  source.Span
}

type AccessExpr struct {
	Receiver Expr
	Field    string
	SpanInfo source.Span
}

func (e *AccessExpr) exprNode()         {}
func (e *AccessExpr) Span() source.Span { return e.SpanInfo }

type IndexExpr struct {
	Receiver Expr
	Index    Expr
	SpanInfo source.Span
}

func (e *IndexExpr) exprNode()         {}
func (e *IndexExpr) Span() source.Span { return e.SpanInfo }

type EnumVariantExpr struct {
	EnumName string
	Variant  string
	Arg      Expr
	SpanInfo source.Span
}

func (e *EnumVariantExpr) exprNode()         {}
func (e *EnumVariantExpr) Span() source.Span { return e.SpanInfo }
