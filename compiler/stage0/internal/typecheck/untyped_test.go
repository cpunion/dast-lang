package typecheck

import (
	"testing"

	"dastlang/internal/ast"
	"dastlang/internal/parser"
)

func TestUntypedArrayDefaultsToI64(t *testing.T) {
	src := `
fn sum(xs: &[i64]) -> i64 {
    let mut i: i64 = 0
    let mut acc: i64 = 0
    while i < len(xs) {
        acc = acc + (*xs)[i]
        i = i + 1
    }
    return acc
}

fn main() {
    let nums = [1, 2, 3]
    let _ = sum(&nums)
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

func TestArrayLiteralAssignsToAlias(t *testing.T) {
	src := `
type IntList = [i32]

fn main() {
    let nums: IntList = [1, 2, 3]
    let _ = len(&nums)
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

func TestMatchVariantBindingIntLiteral(t *testing.T) {
	src := `
enum OptionI32 {
    Some(i32),
    None,
}

fn main() {
    let opt = OptionI32.Some(10)
    let mut v: i32 = 0
    match opt {
        .Some(n) => { v = n + 1 }
        .None => { v = 0 }
    }
    let _ = v
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

func TestIntBinaryTypedPlusLiteral(t *testing.T) {
	src := `
fn main() {
    let n: i32 = 5
    let _ = n + 1
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

func TestPatternBindsPayloadType(t *testing.T) {
	src := `
enum OptionI32 {
    Some(i32),
    None,
}

fn main() {
    let opt = OptionI32.Some(10)
    match opt {
        .Some(n) => { let _ = n }
        .None => {}
    }
}
`
	prog, diags := parser.Parse("test.dast", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected parse errors: %s", diags.Error())
	}
	c := newChecker()
	c.collectDecls(prog)
	if len(c.enums) == 0 {
		t.Fatalf("expected enum decls")
	}
	// Find the first match arm pattern.
	var pat ast.Pattern
	for _, item := range prog.Items {
		fn, ok := item.(*ast.Function)
		if !ok || fn.Name != "main" {
			continue
		}
		for _, stmt := range fn.Body.Stmts {
			if m, ok := stmt.(*ast.MatchStmt); ok {
				if len(m.Arms) == 0 {
					t.Fatalf("expected match arms")
				}
				pat = m.Arms[0].Pattern
				break
			}
		}
	}
	if pat == nil {
		t.Fatalf("pattern not found")
	}
	c.env = newEnv()
	c.env.push()
	c.checkPattern(Type{Kind: TypeEnum, Name: "OptionI32"}, pat)
	info, ok := c.env.lookup("n")
	c.env.pop()
	if !ok {
		t.Fatalf("expected binding for n")
	}
	if info.Type.Kind != TypeInt || info.Type.Name != "i32" {
		t.Fatalf("expected n to be i32, got %s", info.Type.String())
	}
	if c.diag.HasErrors() {
		t.Fatalf("unexpected diagnostics: %s", c.diag.Error())
	}
}

func TestPatternRangeChar(t *testing.T) {
	src := `
fn main() {
    let c = 'b'
    match c {
        'a'..='c' => {}
        _ => {}
    }
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

func TestEqualityWithUnaryIntLiteral(t *testing.T) {
	src := `
fn main() {
    let x: i64 = -6
    if x == -6 { } else { }
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

func TestUntypedBinaryAssignToInt(t *testing.T) {
	src := `
fn main() {
    let x: i32 = 1 + 2
    let _ = x
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

func TestUntypedBinaryAssignToFloat(t *testing.T) {
	src := `
fn main() {
    let x: f32 = 1 + 2
    let _ = x
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
