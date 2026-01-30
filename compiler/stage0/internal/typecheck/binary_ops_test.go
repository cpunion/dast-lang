package typecheck

import (
	"strings"
	"testing"

	"dastlang/internal/parser"
)

func checkTypeOK(t *testing.T, src string) {
	t.Helper()
	prog, diags := parser.Parse("test.dast", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected parse errors: %s", diags.Error())
	}
	_, diags = CheckAndMonomorph(prog)
	if diags.HasErrors() {
		t.Fatalf("unexpected type errors: %s", diags.Error())
	}
}

func checkTypeErr(t *testing.T, src string, wantSub string) {
	t.Helper()
	prog, diags := parser.Parse("test.dast", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected parse errors: %s", diags.Error())
	}
	_, diags = CheckAndMonomorph(prog)
	if !diags.HasErrors() {
		t.Fatalf("expected type errors")
	}
	if wantSub != "" && !strings.Contains(diags.Error(), wantSub) {
		t.Fatalf("expected error to contain %q, got %s", wantSub, diags.Error())
	}
}

func TestCharArithmeticRejected(t *testing.T) {
	checkTypeErr(t, `
fn main() {
    let _ = 'a' + 'b'
}
`, "compatible int")
}

func TestCharComparisonAllowed(t *testing.T) {
	checkTypeOK(t, `
fn main() {
    let _ = 'a' < 'b'
}
`)
}

func TestFloatMixedRejected(t *testing.T) {
	checkTypeErr(t, `
fn main() {
    let a: f32 = 1.0
    let b: f64 = 2.0
    let _ = a + b
}
`, "compatible int or float")
}

func TestFloatIntLiteralAllowed(t *testing.T) {
	checkTypeOK(t, `
fn main() {
    let a: f32 = 1.0
    let _ = a + 1
}
`)
}

func TestShiftRequiresUnsigned(t *testing.T) {
	checkTypeErr(t, `
fn main() {
    let a: i32 = 1
    let b: i32 = 1
    let _ = a << b
}
`, "unsigned")
}

func TestShiftUnsignedOK(t *testing.T) {
	checkTypeOK(t, `
fn main() {
    let a: i32 = 1
    let b: u32 = 1
    let _ = a << b
}
`)
}

func TestIntSameTypeRequired(t *testing.T) {
	checkTypeErr(t, `
fn main() {
    let a: i16 = 1
    let b: i32 = 2
    let _ = a + b
}
`, "compatible int")
}

func TestIntOpsSameTypeMatrix(t *testing.T) {
	intTypes := []string{"i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64", "isize", "usize", "int"}
	intOps := []string{"+", "-", "*", "/", "%", "&", "|", "^"}
	cmpOps := []string{"==", "!=", "<", "<=", ">", ">="}
	for _, tname := range intTypes {
		for _, op := range intOps {
			src := `
fn main() {
    let a: ` + tname + ` = 1
    let b: ` + tname + ` = 2
    let _ = a ` + op + ` b
}
`
			checkTypeOK(t, src)
		}
		for _, op := range cmpOps {
			src := `
fn main() {
    let a: ` + tname + ` = 1
    let b: ` + tname + ` = 2
    let _ = a ` + op + ` b
}
`
			checkTypeOK(t, src)
		}
	}
}

func TestIntOpsCrossTypeRejected(t *testing.T) {
	intTypes := []string{"i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64", "isize", "usize", "int"}
	intOps := []string{"+", "-", "*", "/", "%", "&", "|", "^"}
	cmpOps := []string{"==", "!=", "<", "<=", ">", ">="}
	for i, lt := range intTypes {
		for j, rt := range intTypes {
			if i == j || canonicalIntName(lt) == canonicalIntName(rt) {
				continue
			}
			for _, op := range intOps {
				src := `
fn main() {
    let a: ` + lt + ` = 1
    let b: ` + rt + ` = 2
    let _ = a ` + op + ` b
}
`
				checkTypeErr(t, src, "compatible int")
			}
			for _, op := range cmpOps {
				src := `
fn main() {
    let a: ` + lt + ` = 1
    let b: ` + rt + ` = 2
    let _ = a ` + op + ` b
}
`
				checkTypeErr(t, src, "compatible")
			}
		}
	}
}

func TestShiftRulesMatrix(t *testing.T) {
	signed := []string{"i8", "i16", "i32", "i64", "isize", "int"}
	unsigned := []string{"u8", "u16", "u32", "u64", "usize"}
	allInts := append([]string{}, signed...)
	allInts = append(allInts, unsigned...)
	for _, lhs := range allInts {
		for _, rhs := range unsigned {
			src := `
fn main() {
    let a: ` + lhs + ` = 1
    let b: ` + rhs + ` = 1
    let _ = a << b
    let _ = a >> b
}
`
			checkTypeOK(t, src)
		}
		for _, rhs := range signed {
			src := `
fn main() {
    let a: ` + lhs + ` = 1
    let b: ` + rhs + ` = 1
    let _ = a << b
}
`
			checkTypeErr(t, src, "unsigned")
		}
	}
}

func TestBoolOpsMatrix(t *testing.T) {
	checkTypeOK(t, `
fn main() {
    let a: bool = true
    let b: bool = false
    let _ = a && b
    let _ = a || b
    let _ = a == b
    let _ = a != b
}
`)
	checkTypeErr(t, `
fn main() {
    let a: bool = true
    let b: bool = false
    let _ = a + b
}
`, "compatible")
}

func TestStringOpsMatrix(t *testing.T) {
	checkTypeOK(t, `
fn main() {
    let a: String = "a"
    let b: String = "b"
    let _ = a + b
    let _ = a == b
    let _ = a != b
}
`)
	checkTypeErr(t, `
fn main() {
    let a: String = "a"
    let b: String = "b"
    let _ = a < b
}
`, "comparison")
}
