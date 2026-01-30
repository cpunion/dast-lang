package typecheck

import "testing"

func TestTraitBoundUnknownTrait(t *testing.T) {
	checkTypeErr(t, `
trait Iter[T] { fn next(self: &Self) -> T; }
struct Foo {}
impl Iter[i32] for Foo { fn next(self: &Self) -> i32 { 0 } }
fn f[T: Missing](x: T) { }
fn main() {
    let v = Foo{}
    f(v)
}
`, "unknown trait")
}

func TestTraitBoundArgCountMismatch(t *testing.T) {
	checkTypeErr(t, `
trait Iter[T] { fn next(self: &Self) -> T; }
struct Foo {}
impl Iter[i32] for Foo { fn next(self: &Self) -> i32 { 0 } }
fn f[T: Iter[i32, i64]](x: T) { }
fn main() {
    let v = Foo{}
    f(v)
}
`, "expects 1 type arguments")
}

func TestTraitBoundMissingImpl(t *testing.T) {
	checkTypeErr(t, `
trait Eq[T] { fn eq(self: &Self, other: T) -> bool; }
struct Foo {}
impl Eq[i32] for Foo { fn eq(self: &Self, other: i32) -> bool { true } }
fn f[T: Eq[i64]](x: T) { }
fn main() {
    let v = Foo{}
    f(v)
}
`, "does not implement trait")
}
