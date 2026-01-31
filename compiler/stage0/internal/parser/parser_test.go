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
		lit, ok := letStmt.Init.(*ast.CharLit)
		if !ok {
			t.Fatalf("expected char literal init, got %T", letStmt.Init)
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

func TestParseWhereBounds(t *testing.T) {
	src := `fn f[T: Clone](x: T) where T: Display + Iter[i32] {}`
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
	if len(fn.TypeParams) != 1 {
		t.Fatalf("expected 1 type param, got %d", len(fn.TypeParams))
	}
	bounds := fn.TypeParams[0].Bounds
	if len(bounds) != 3 {
		t.Fatalf("expected 3 bounds, got %d", len(bounds))
	}
	if bounds[0].Name != "Clone" || bounds[1].Name != "Display" {
		t.Fatalf("unexpected bounds: %v", bounds)
	}
	if bounds[2].Name != "Iter" || len(bounds[2].Args) != 1 || bounds[2].Args[0].Name != "i32" {
		t.Fatalf("unexpected trait args in bound: %v", bounds[2])
	}
}

func TestParseWhereUnknownParam(t *testing.T) {
	src := `fn f[T](x: T) where U: Clone {}`
	_, diags := parser.Parse("test.dast", src)
	if !diags.HasErrors() {
		t.Fatalf("expected parse errors for unknown where param")
	}
}

func TestParseWhereNonNominalBound(t *testing.T) {
	src := `fn f[T](x: T) where T: &str {}`
	_, diags := parser.Parse("test.dast", src)
	if !diags.HasErrors() {
		t.Fatalf("expected parse errors for non-nominal bound")
	}
}

func TestParseConstExprBinary(t *testing.T) {
	src := `const A = 1 + 2 * 3
const B = -(4 + 5)
const C = !false
const D = 1.5 + 2.0
const E = 1 < 2
const F = "a" + "b"`
	prog, diags := parser.Parse("test.dast", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected parse errors: %s", diags.Error())
	}
	if len(prog.Items) != 6 {
		t.Fatalf("expected 6 items, got %d", len(prog.Items))
	}
	decl := func(idx int) *ast.ConstDecl {
		c, ok := prog.Items[idx].(*ast.ConstDecl)
		if !ok {
			t.Fatalf("expected const decl at %d", idx)
		}
		return c
	}
	if d := decl(0); d.Value.Int != 7 {
		t.Fatalf("const A expected 7 got %d", d.Value.Int)
	}
	if d := decl(1); d.Value.Int != -9 {
		t.Fatalf("const B expected -9 got %d", d.Value.Int)
	}
	if d := decl(2); !d.Value.Bool {
		t.Fatalf("const C expected true")
	}
	if d := decl(3); d.Value.FloatText != "3.5" {
		t.Fatalf("const D expected 3.5 got %s", d.Value.FloatText)
	}
	if d := decl(4); !d.Value.Bool {
		t.Fatalf("const E expected true")
	}
	if d := decl(5); d.Value.Str != "ab" {
		t.Fatalf("const F expected ab got %s", d.Value.Str)
	}
}

func TestParseStructReprC(t *testing.T) {
	src := `@repr(C)
struct Point { x: i32, y: i32 }`
	prog, diags := parser.Parse("test.dast", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected parse errors: %s", diags.Error())
	}
	if len(prog.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(prog.Items))
	}
	s, ok := prog.Items[0].(*ast.StructDecl)
	if !ok {
		t.Fatalf("expected struct decl")
	}
	if s.Repr != "C" {
		t.Fatalf("expected struct repr C, got %q", s.Repr)
	}
}

func TestParseRangePatternExprStart(t *testing.T) {
	src := `fn main() {
    let x = 3
    match x {
        1 + 2 ..= 10 => {}
        _ => {}
    }
}`
	prog, diags := parser.Parse("test.dast", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected parse errors: %s", diags.Error())
	}
	fn, ok := prog.Items[0].(*ast.Function)
	if !ok {
		t.Fatalf("expected function item")
	}
	if len(fn.Body.Stmts) < 2 {
		t.Fatalf("expected match statement")
	}
	matchStmt, ok := fn.Body.Stmts[1].(*ast.MatchStmt)
	if !ok || len(matchStmt.Arms) == 0 {
		t.Fatalf("expected match statement with arms")
	}
	rp, ok := matchStmt.Arms[0].Pattern.(*ast.RangePattern)
	if !ok {
		t.Fatalf("expected range pattern")
	}
	if _, ok := rp.Start.(*ast.BinaryExpr); !ok {
		t.Fatalf("expected binary expr range start, got %T", rp.Start)
	}
}

func TestParseStructPatternRest(t *testing.T) {
	src := `struct Point { x: i64, y: i64 }
fn main() {
    let p = Point { x: 1, y: 2 }
    match p {
        Point { x, .. } => {}
        _ => {}
    }
}`
	prog, diags := parser.Parse("test.dast", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected parse errors: %s", diags.Error())
	}
	if len(prog.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(prog.Items))
	}
	fn, ok := prog.Items[1].(*ast.Function)
	if !ok {
		t.Fatalf("expected function item")
	}
	matchStmt, ok := fn.Body.Stmts[1].(*ast.MatchStmt)
	if !ok || len(matchStmt.Arms) == 0 {
		t.Fatalf("expected match statement with arms")
	}
	sp, ok := matchStmt.Arms[0].Pattern.(*ast.StructPattern)
	if !ok {
		t.Fatalf("expected struct pattern")
	}
	if len(sp.Fields) != 1 || sp.Fields[0].Name != "x" {
		t.Fatalf("expected single field binding for x")
	}
}
