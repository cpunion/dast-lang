package typecheck

import (
	"strings"
	"testing"

	"dastlang/internal/parser"
)

func TestReturnRefLifetimeOrigins(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		wantError bool
	}{
		{
			name: "A_single_param_ok",
			src:  `fn id(x: &i64) -> &i64 { x }`,
		},
		{
			name: "B_field_from_ref_param_ok",
			src:  `struct S { x: i64 } fn get(s: &S) -> &i64 { &s.x }`,
		},
		{
			name: "C_string_literal_ok",
			src:  `fn hello() -> &str { "hi" }`,
		},
		{
			name:      "D_return_ref_to_local_err",
			src:       `fn bad() -> &i64 { let x = 1; &x }`,
			wantError: true,
		},
		{
			name:      "E_return_ref_from_value_param_err",
			src:       `fn bad2(x: i64) -> &i64 { &x }`,
			wantError: true,
		},
		{
			name:      "F_return_ref_from_const_err",
			src:       `const X: i64 = 1; fn bad3(x: &i64) -> &i64 { &X }`,
			wantError: true,
		},
		{
			name:      "G_ambiguous_if_err",
			src:       `fn choose(x: &i64, y: &i64, flag: bool) -> &i64 { return if flag { x } else { y } }`,
			wantError: true,
		},
		{
			name: "H_match_same_origin_ok",
			src:  `fn pick(x: &i64) -> &i64 { return match true { true => x, false => x } }`,
		},
		{
			name:      "I_match_mixed_origin_err",
			src:       `fn pick(x: &i64, y: &i64) -> &i64 { return match true { true => x, false => y } }`,
			wantError: true,
		},
		{
			name: "J_block_tail_ok",
			src:  `fn tail(x: &i64) -> &i64 { { x } }`,
		},
		{
			name:      "K_tuple_refs_mixed_origin_err",
			src:       `fn pair(x: &i64, y: &i64) -> (&i64, &i64) { (x, y) }`,
			wantError: true,
		},
		{
			name: "L_tuple_refs_same_origin_ok",
			src:  `fn pair(x: &i64) -> (&i64, &i64) { (x, x) }`,
		},
		{
			name: "M_reborrow_ok",
			src:  `fn reborrow(x: &i64) -> &i64 { let r = &*x; r }`,
		},
		{
			name:      "N_method_self_param_ambiguous_err",
			src:       `struct S { x: i64 } impl S { fn pick(self: &Self, y: &i64, flag: bool) -> &i64 { return if flag { &self.x } else { y } } }`,
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog, diags := parser.Parse("test.dast", tc.src)
			if diags.HasErrors() {
				t.Fatalf("parse error: %s", diags.Error())
			}
			_, diags = CheckAndMonomorph(prog)
			if tc.wantError {
				if diags == nil || !diags.HasErrors() {
					t.Fatalf("expected type errors")
				}
				if !(strings.Contains(diags.Error(), "returning reference") || strings.Contains(diags.Error(), "cannot take reference")) {
					t.Fatalf("unexpected error: %s", diags.Error())
				}
				return
			}
			if diags != nil && diags.HasErrors() {
				t.Fatalf("unexpected type errors: %s", diags.Error())
			}
		})
	}
}
