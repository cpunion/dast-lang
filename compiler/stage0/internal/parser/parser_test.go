package parser_test

import (
	"testing"

	"dastlang/internal/ast"
	"dastlang/internal/parser"
)

func TestParseCharLiterals(t *testing.T) {
	src := `fn main() {
    let a = 'a'
    let n = '\n'
    let q = '\''
    let b = '\\'
}`
	prog, diags := parser.Parse("test.dast", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected parse errors: %s", diags.Error())
	}
	if len(prog.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(prog.Items))
	}
	fn, ok := prog.Items[0].(*ast.Function)
	if !ok {
		t.Fatalf("expected function item")
	}
	if fn.Body == nil || len(fn.Body.Stmts) != 4 {
		t.Fatalf("expected 4 let statements")
	}
	expect := map[string]int64{
		"a": int64('a'),
		"n": int64('\n'),
		"q": int64('\''),
		"b": int64('\\'),
	}
	for _, stmt := range fn.Body.Stmts {
		letStmt, ok := stmt.(*ast.LetStmt)
		if !ok {
			t.Fatalf("expected let stmt, got %T", stmt)
		}
		lit, ok := letStmt.Init.(*ast.IntLit)
		if !ok {
			t.Fatalf("expected int literal init, got %T", letStmt.Init)
		}
		want, ok := expect[letStmt.Name]
		if !ok {
			t.Fatalf("unexpected let name %q", letStmt.Name)
		}
		if lit.Value != want {
			t.Fatalf("let %s expected %d got %d", letStmt.Name, want, lit.Value)
		}
	}
}
