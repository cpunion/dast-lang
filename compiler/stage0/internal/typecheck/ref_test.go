package typecheck

import (
	"strings"
	"testing"

	"dastlang/internal/parser"
)

func TestRefRejectsDoubleRef(t *testing.T) {
	src := `fn main() { let a = 1; let r = &a; let rr = &r; }`
	prog, diags := parser.Parse("test.dast", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected parse errors: %s", diags.Error())
	}
	_, diags = CheckAndMonomorph(prog)
	if diags == nil || !diags.HasErrors() {
		t.Fatalf("expected type error for reference to reference")
	}
	if !strings.Contains(diags.Error(), "cannot take reference to reference") {
		t.Fatalf("unexpected error: %s", diags.Error())
	}
}
