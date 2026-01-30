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

func TestRefCoerceMutToImm(t *testing.T) {
	src := `
fn main() {
    let mut x: i32 = 1
    let r: &mut i32 = &mut x
    let s: &i32 = r
}
`
	prog, diags := parser.Parse("test.dast", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected parse errors: %s", diags.Error())
	}
	_, diags = CheckAndMonomorph(prog)
	if diags.HasErrors() {
		t.Fatalf("unexpected type errors: %s", diags.Error())
	}
}

func TestAutoBorrowStringToStrInLet(t *testing.T) {
	src := `
fn main() {
    let s = string_clone("hi")
    let r: &str = s
}
`
	prog, diags := parser.Parse("test.dast", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected parse errors: %s", diags.Error())
	}
	_, diags = CheckAndMonomorph(prog)
	if diags.HasErrors() {
		t.Fatalf("unexpected type errors: %s", diags.Error())
	}
}
