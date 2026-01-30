package compile

import "testing"

func TestIntPeerTypeNameZip(t *testing.T) {
	types := []string{"i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64", "isize", "usize", "int"}
	for _, a := range types {
		for _, b := range types {
			got := intPeerTypeName(a, b)
			ra := a
			rb := b
			if ra == "int" {
				ra = "i64"
			}
			if rb == "int" {
				rb = "i64"
			}
			if ra == rb {
				if got != ra {
					t.Fatalf("peer type mismatch for %s/%s: got %q", a, b, got)
				}
				continue
			}
			if got != "" {
				t.Fatalf("expected no peer type for %s/%s, got %q", a, b, got)
			}
		}
	}
}
