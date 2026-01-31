package typecheck

import (
	"testing"

	"dastlang/internal/parser"
)

func TestStringLiteralArrayExpectedString(t *testing.T) {
	src := `
fn main() {
    let xs: [String] = ["a", "b"]
    let _ = len(&xs)
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

func TestStringAutoBorrowToStrParam(t *testing.T) {
	src := `
fn takes(s: &str) -> i64 {
    return len(s)
}

fn main() {
    let s = string_clone("hi")
    let _ = takes(s)
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
