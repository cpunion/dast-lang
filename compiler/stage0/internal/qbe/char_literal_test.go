package qbe

import (
	"strings"
	"testing"

	"dastlang/internal/compile"
	"dastlang/internal/parser"
	"dastlang/internal/typecheck"
)

func compileToQBE(t *testing.T, src string) string {
	prog, diag := parser.Parse("test.dast", src)
	if diag.HasErrors() {
		t.Fatalf("parse error: %v", diag.Items)
	}
	checkDiag := typecheck.Check(prog)
	if checkDiag.HasErrors() {
		t.Fatalf("typecheck error: %v", checkDiag.Items)
	}
	irProg, compileDiag := compile.Compile(prog)
	if compileDiag.HasErrors() {
		t.Fatalf("compile error: %v", compileDiag.Items)
	}
	return EmitProgram(irProg)
}

func TestCharLiteralEscapesQBE(t *testing.T) {
	src := `fn main() {
    let a = 'a'
    let n = '\n'
    let q = '\''
    let b = '\\'
    println(a)
    println(n)
    println(q)
    println(b)
}`
	got := compileToQBE(t, src)
	want := []string{
		"storew 97, %v_a",
		"storew 10, %v_n",
		"storew 39, %v_q",
		"storew 92, %v_b",
	}
	for _, needle := range want {
		if !strings.Contains(got, needle) {
			t.Fatalf("expected %q in QBE:\n%s", needle, got)
		}
	}
}
