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

type TupleLit struct {
	Elems    []Expr
	SpanInfo source.Span
}

func (e *TupleLit) exprNode()         {}
func (e *TupleLit) Span() source.Span { return e.SpanInfo }

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

type CompileExpr struct {
	ID       int
	Expr     Expr
	SpanInfo source.Span
}

func (e *CompileExpr) exprNode()         {}
func (e *CompileExpr) Span() source.Span { return e.SpanInfo }

type QuoteKind int

const (
	QuoteExprKind QuoteKind = iota
	QuoteStmtKind
	QuoteItemKind
	QuoteBlockKind
)

type QuotePart struct {
	Text string
	Expr Expr
}

type QuoteExpr struct {
	Kind     QuoteKind
	Parts    []QuotePart
	SpanInfo source.Span
}

func (e *QuoteExpr) exprNode()         {}
func (e *QuoteExpr) Span() source.Span { return e.SpanInfo }

type MacroCallExpr struct {
	Callee   Expr
	Args     []Expr
	SpanInfo source.Span
}

func (e *MacroCallExpr) exprNode()         {}
func (e *MacroCallExpr) Span() source.Span { return e.SpanInfo }

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
	TypeArgs []Type
	Args     []Expr
	SpanInfo source.Span
}

func (e *CallExpr) exprNode()         {}
func (e *CallExpr) Span() source.Span { return e.SpanInfo }

type MethodCallExpr struct {
	Receiver     Expr
	Method       string
	Args         []Expr
	ResolvedName string
	ResolvedSelf bool
	EnumName     string
	SpanInfo     source.Span
}

func (e *MethodCallExpr) exprNode()         {}
func (e *MethodCallExpr) Span() source.Span { return e.SpanInfo }

type ClosureParam struct {
	Name string
	Type *Type
	Span source.Span
}

type ClosureExpr struct {
	Params     []ClosureParam
	ReturnType *Type
	Body       Expr
	SpanInfo   source.Span
}

func (e *ClosureExpr) exprNode()         {}
func (e *ClosureExpr) Span() source.Span { return e.SpanInfo }

type StructLit struct {
	Name     string
	TypeArgs []Type
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
	TypeArgs []Type
	Variant  string
	Arg      Expr
	SpanInfo source.Span
}

func (e *EnumVariantExpr) exprNode()         {}
func (e *EnumVariantExpr) Span() source.Span { return e.SpanInfo }

type BlockExpr struct {
	Block    *Block
	SpanInfo source.Span
}

func (e *BlockExpr) exprNode()         {}
func (e *BlockExpr) Span() source.Span { return e.SpanInfo }

type LoopExpr struct {
	Body     *Block
	SpanInfo source.Span
}

func (e *LoopExpr) exprNode()         {}
func (e *LoopExpr) Span() source.Span { return e.SpanInfo }

type IfExpr struct {
	Cond     Expr
	Then     Expr
	Else     Expr
	SpanInfo source.Span
}

func (e *IfExpr) exprNode()         {}
func (e *IfExpr) Span() source.Span { return e.SpanInfo }

type MatchExpr struct {
	Expr     Expr
	Arms     []MatchArm
	SpanInfo source.Span
}

func (e *MatchExpr) exprNode()         {}
func (e *MatchExpr) Span() source.Span { return e.SpanInfo }
