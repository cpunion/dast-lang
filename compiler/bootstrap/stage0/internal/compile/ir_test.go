package compile_test

import (
	"testing"

	"dastlang/internal/compile"
	"dastlang/internal/parser"
	"dastlang/internal/typecheck"
)

func compileToIR(t *testing.T, src string) string {
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
	return irProg.Format()
}

// =============================================================================
// 1. 基本函数和参数
// =============================================================================

func TestValueParamsKeepNames(t *testing.T) {
	src := `fn add(a: i64, b: i64) -> i64 { a + b }`

	// 保留参数名 a, b；temp 从 t0 开始；有类型标注
	want := `ir v0
fn add(a: i64, b: i64) -> i64
  block entry0:
    t0: i64 = + a, b
    return t0

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestParamNamedT0(t *testing.T) {
	src := `fn weird(t0: i64) -> i64 { t0 + 1 }`

	// 参数叫 t0，temp 从 t1 开始
	want := `ir v0
fn weird(t0: i64) -> i64
  block entry0:
    t1: i64 = + t0, 1
    return t1

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestFunctionReturnUnit(t *testing.T) {
	src := `fn foo() { }`

	want := `ir v0
fn foo() -> unit
  block entry0:
    return

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// =============================================================================
// 2. let 绑定 (值语义)
// =============================================================================

func TestLetBindingNoStore(t *testing.T) {
	src := `fn main() { let x = 42; println(x) }`

	// let 绑定：当前实现使用 store/load
	want := `ir v0
fn main() -> unit
  block entry0:
    t0: i64 = 42
    store x, t0
    t1: i64 = load x
    t2: unit = call println(t1)
    return t2

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestLetBindingExpr(t *testing.T) {
	src := `fn calc(a: i64, b: i64) -> i64 { let sum = a + b; sum * 2 }`

	// let 绑定：当前实现使用 store/load
	want := `ir v0
fn calc(a: i64, b: i64) -> i64
  block entry0:
    t0: i64 = + a, b
    store sum, t0
    t1: i64 = load sum
    t2: i64 = * t1, 2
    return t2

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// =============================================================================
// 3. mut 变量 (需要 store/load)
// =============================================================================

func TestMutVarUsesStoreLoad(t *testing.T) {
	src := `fn counter() -> i64 { let mut x = 0; x = x + 1; x }`

	// mut 变量用 store/load
	want := `ir v0
fn counter() -> i64
  block entry0:
    t0: i64 = 0
    store x, t0
    t1: i64 = load x
    t2: i64 = + t1, 1
    store x, t2
    t3: i64 = load x
    return t3

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// =============================================================================
// 4. 结构体
// =============================================================================

func TestStructTypeDecl(t *testing.T) {
	src := `
struct Point { x: i64, y: i64 }
fn origin() -> Point { Point { x: 0, y: 0 } }
`
	// IR 中包含类型声明
	want := `ir v0
type Point = { x: i64, y: i64 }

fn origin() -> Point
  block entry0:
    t0: i64 = 0
    t1: i64 = 0
    t2: Point = struct Point { x: t0, y: t1 }
    return t2

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestFieldAccess(t *testing.T) {
	src := `
struct Point { x: i64, y: i64 }
fn get_x(p: Point) -> i64 { p.x }
`
	want := `ir v0
type Point = { x: i64, y: i64 }

fn get_x(p: Point) -> i64
  block entry0:
    t0: i64 = p.x
    return t0

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestFieldAssignment(t *testing.T) {
	src := `
struct Point { x: i64, y: i64 }
fn set_x(p: &mut Point, val: i64) { p.x = val }
`
	want := `ir v0
type Point = { x: i64, y: i64 }

fn set_x(p: *Point, val: i64) -> unit
  block entry0:
    p.x = val
    return

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// =============================================================================
// 5. 枚举
// =============================================================================

func TestEnumTypeDecl(t *testing.T) {
	src := `
enum Option { Some(i64), None }
fn none() -> Option { Option.None }
`
	// 枚举展开为 struct（tag + payload）
	want := `ir v0
fn none() -> Option
  block entry0:
    t0: Option = enum Option.None@1:i32
    return t0

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// =============================================================================
// 6. 数组
// =============================================================================

// =============================================================================
// 7. 引用
// =============================================================================

func TestRefDeref(t *testing.T) {
	src := `fn inc(x: &mut i64) { *x = *x + 1 }`

	want := `ir v0
fn inc(x: *i64) -> unit
  block entry0:
    t0: i64 = load_ref x
    t1: i64 = + t0, 1
    store_ref x, t1
    return

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// =============================================================================
// 8. 控制流
// =============================================================================

func TestIfElse(t *testing.T) {
	src := `fn abs(x: i64) -> i64 { if x < 0 { -x } else { x } }`

	want := `ir v0
fn abs(x: i64) -> i64
  block entry0:
    t0: bool = < x, 0
    branch t0, then1, else2
  block then1:
    t1: i64 = - x
    return t1
  block else2:
    return x
  block merge3:
    return

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestWhileLoop(t *testing.T) {
	src := `fn count() -> i64 { let mut i = 0; while i < 3 { i = i + 1 } i }`

	want := `ir v0
fn count() -> i64
  block entry0:
    t0: i64 = 0
    store i, t0
    jump cond1
  block cond1:
    t1: i64 = load i
    t2: bool = < t1, 3
    branch t2, body2, after3
  block body2:
    t3: i64 = load i
    t4: i64 = + t3, 1
    store i, t4
    jump cond1
  block after3:
    t5: i64 = load i
    return t5

`
	got := compileToIR(t, src)
	if got != want {
		t.Errorf("IR mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

