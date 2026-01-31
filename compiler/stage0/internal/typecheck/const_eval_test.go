package typecheck

import (
	"testing"

	"dastlang/internal/ast"
	"dastlang/internal/parser"
)

func TestConstEvalBinary(t *testing.T) {
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
	out, diags := CheckAndMonomorph(prog)
	if diags.HasErrors() {
		t.Fatalf("unexpected typecheck errors: %s", diags.Error())
	}
	if len(out.Items) != 6 {
		t.Fatalf("expected 6 items, got %d", len(out.Items))
	}
	decl := func(idx int) *ast.ConstDecl {
		c, ok := out.Items[idx].(*ast.ConstDecl)
		if !ok {
			t.Fatalf("expected const decl at %d", idx)
		}
		return c
	}
	if d := decl(0); d.Value.Kind != ast.ConstInt || d.Value.Int != 7 {
		t.Fatalf("const A expected 7 got %+v", d.Value)
	}
	if d := decl(1); d.Value.Kind != ast.ConstInt || d.Value.Int != -9 {
		t.Fatalf("const B expected -9 got %+v", d.Value)
	}
	if d := decl(2); d.Value.Kind != ast.ConstBool || !d.Value.Bool {
		t.Fatalf("const C expected true got %+v", d.Value)
	}
	if d := decl(3); d.Value.Kind != ast.ConstFloat || d.Value.FloatText != "3.5" {
		t.Fatalf("const D expected 3.5 got %+v", d.Value)
	}
	if d := decl(4); d.Value.Kind != ast.ConstBool || !d.Value.Bool {
		t.Fatalf("const E expected true got %+v", d.Value)
	}
	if d := decl(5); d.Value.Kind != ast.ConstString || d.Value.Str != "ab" {
		t.Fatalf("const F expected ab got %+v", d.Value)
	}
}
