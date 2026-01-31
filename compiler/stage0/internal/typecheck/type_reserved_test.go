package typecheck

import "testing"

func TestReservedIntTypesRejected(t *testing.T) {
	checkTypeErr(t, `
fn main() {
    let _x: i128 = 1
}
`, "reserved but not supported")
	checkTypeErr(t, `
fn main() {
    let _x: u128 = 1
}
`, "reserved but not supported")
}
