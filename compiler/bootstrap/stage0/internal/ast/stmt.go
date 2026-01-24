package ast

import "dastlang/internal/source"

type Stmt interface {
	stmtNode()
	Span() source.Span
}

type Block struct {
	Stmts    []Stmt
	SpanInfo source.Span
}

func (b *Block) stmtNode() {}
func (b *Block) Span() source.Span {
	return b.SpanInfo
}

type LetStmt struct {
	Name     string
	Mutable  bool
	Type     *Type
	Init     Expr
	SpanInfo source.Span
}

func (s *LetStmt) stmtNode()         {}
func (s *LetStmt) Span() source.Span { return s.SpanInfo }

type LetPatternStmt struct {
	Pattern  Pattern
	Init     Expr
	SpanInfo source.Span
}

func (s *LetPatternStmt) stmtNode()         {}
func (s *LetPatternStmt) Span() source.Span { return s.SpanInfo }

type AssignStmt struct {
	Target   Expr
	Value    Expr
	SpanInfo source.Span
}

func (s *AssignStmt) stmtNode()         {}
func (s *AssignStmt) Span() source.Span { return s.SpanInfo }

type ExprStmt struct {
	Expr     Expr
	SpanInfo source.Span
}

func (s *ExprStmt) stmtNode()         {}
func (s *ExprStmt) Span() source.Span { return s.SpanInfo }

type ReturnStmt struct {
	Value    Expr
	SpanInfo source.Span
}

func (s *ReturnStmt) stmtNode()         {}
func (s *ReturnStmt) Span() source.Span { return s.SpanInfo }

type IfStmt struct {
	Cond     Expr
	Then     *Block
	Else     *Block
	SpanInfo source.Span
}

func (s *IfStmt) stmtNode()         {}
func (s *IfStmt) Span() source.Span { return s.SpanInfo }

type IfLetStmt struct {
	Pattern  Pattern
	Expr     Expr
	Then     *Block
	Else     *Block
	SpanInfo source.Span
}

func (s *IfLetStmt) stmtNode()         {}
func (s *IfLetStmt) Span() source.Span { return s.SpanInfo }

type WhileStmt struct {
	Cond     Expr
	Body     *Block
	SpanInfo source.Span
}

func (s *WhileStmt) stmtNode()         {}
func (s *WhileStmt) Span() source.Span { return s.SpanInfo }

type WhileLetStmt struct {
	Pattern  Pattern
	Expr     Expr
	Body     *Block
	SpanInfo source.Span
}

func (s *WhileLetStmt) stmtNode()         {}
func (s *WhileLetStmt) Span() source.Span { return s.SpanInfo }

type LoopStmt struct {
	Body     *Block
	SpanInfo source.Span
}

func (s *LoopStmt) stmtNode()         {}
func (s *LoopStmt) Span() source.Span { return s.SpanInfo }

type BreakStmt struct {
	SpanInfo source.Span
}

func (s *BreakStmt) stmtNode()         {}
func (s *BreakStmt) Span() source.Span { return s.SpanInfo }

type ContinueStmt struct {
	SpanInfo source.Span
}

func (s *ContinueStmt) stmtNode()         {}
func (s *ContinueStmt) Span() source.Span { return s.SpanInfo }

type MatchStmt struct {
	Expr     Expr
	Arms     []MatchArm
	SpanInfo source.Span
}

func (s *MatchStmt) stmtNode()         {}
func (s *MatchStmt) Span() source.Span { return s.SpanInfo }

type MatchArm struct {
	Pattern  Pattern
	Guard    Expr
	Body     *Block
	SpanInfo source.Span
}
