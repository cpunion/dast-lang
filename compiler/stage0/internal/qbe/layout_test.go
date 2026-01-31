package qbe

import (
	"testing"

	"dastlang/internal/ir"
)

func TestTypeSizeAlignBasic(t *testing.T) {
	p := &ir.Program{TypeDecls: map[string]*ir.TypeDecl{}}
	e64 := &emitter{ptrSize: 8}
	if got := typeSize(e64, p, "bool"); got != 1 {
		t.Fatalf("bool size = %d, want 1", got)
	}
	if got := typeAlign(e64, p, "bool"); got != 1 {
		t.Fatalf("bool align = %d, want 1", got)
	}
	if got := typeSize(e64, p, "char"); got != 4 {
		t.Fatalf("char size = %d, want 4", got)
	}
	if got := typeAlign(e64, p, "char"); got != 4 {
		t.Fatalf("char align = %d, want 4", got)
	}
	if got := typeSize(e64, p, "u8"); got != 1 {
		t.Fatalf("u8 size = %d, want 1", got)
	}
	if got := typeAlign(e64, p, "u8"); got != 1 {
		t.Fatalf("u8 align = %d, want 1", got)
	}
	if got := typeSize(e64, p, "i16"); got != 2 {
		t.Fatalf("i16 size = %d, want 2", got)
	}
	if got := typeAlign(e64, p, "i16"); got != 2 {
		t.Fatalf("i16 align = %d, want 2", got)
	}
	if got := typeSize(e64, p, "u16"); got != 2 {
		t.Fatalf("u16 size = %d, want 2", got)
	}
	if got := typeAlign(e64, p, "u16"); got != 2 {
		t.Fatalf("u16 align = %d, want 2", got)
	}
	if got := typeSize(e64, p, "i32"); got != 4 {
		t.Fatalf("i32 size = %d, want 4", got)
	}
	if got := typeAlign(e64, p, "i32"); got != 4 {
		t.Fatalf("i32 align = %d, want 4", got)
	}
	if got := typeSize(e64, p, "u32"); got != 4 {
		t.Fatalf("u32 size = %d, want 4", got)
	}
	if got := typeAlign(e64, p, "u32"); got != 4 {
		t.Fatalf("u32 align = %d, want 4", got)
	}
	if got := typeSize(e64, p, "i64"); got != 8 {
		t.Fatalf("i64 size = %d, want 8", got)
	}
	if got := typeAlign(e64, p, "i64"); got != 8 {
		t.Fatalf("i64 align = %d, want 8", got)
	}
	if got := typeSize(e64, p, "u64"); got != 8 {
		t.Fatalf("u64 size = %d, want 8", got)
	}
	if got := typeAlign(e64, p, "u64"); got != 8 {
		t.Fatalf("u64 align = %d, want 8", got)
	}
	if got := typeSize(e64, p, "f32"); got != 4 {
		t.Fatalf("f32 size = %d, want 4", got)
	}
	if got := typeAlign(e64, p, "f32"); got != 4 {
		t.Fatalf("f32 align = %d, want 4", got)
	}
	if got := typeSize(e64, p, "f64"); got != 8 {
		t.Fatalf("f64 size = %d, want 8", got)
	}
	if got := typeAlign(e64, p, "f64"); got != 8 {
		t.Fatalf("f64 align = %d, want 8", got)
	}
	if got := typeSize(e64, p, "isize"); got != 8 {
		t.Fatalf("isize size = %d, want 8", got)
	}
	if got := typeAlign(e64, p, "isize"); got != 8 {
		t.Fatalf("isize align = %d, want 8", got)
	}
	if got := typeSize(e64, p, "usize"); got != 8 {
		t.Fatalf("usize size = %d, want 8", got)
	}
	if got := typeAlign(e64, p, "usize"); got != 8 {
		t.Fatalf("usize align = %d, want 8", got)
	}
	if got := typeSize(e64, p, "*i32"); got != 8 {
		t.Fatalf("*i32 size = %d, want 8", got)
	}
	if got := typeAlign(e64, p, "*i32"); got != 8 {
		t.Fatalf("*i32 align = %d, want 8", got)
	}
	if got := typeSize(e64, p, "[i32]"); got != 8 {
		t.Fatalf("[i32] size = %d, want 8", got)
	}
	if got := typeAlign(e64, p, "[i32]"); got != 8 {
		t.Fatalf("[i32] align = %d, want 8", got)
	}
	if got := typeSize(e64, p, "String"); got != 8 {
		t.Fatalf("String size = %d, want 8", got)
	}
	if got := typeAlign(e64, p, "String"); got != 8 {
		t.Fatalf("String align = %d, want 8", got)
	}
	if got := typeSize(e64, p, "str"); got != 8 {
		t.Fatalf("str size = %d, want 8", got)
	}
	if got := typeAlign(e64, p, "str"); got != 8 {
		t.Fatalf("str align = %d, want 8", got)
	}
	e32 := &emitter{ptrSize: 4}
	if got := typeSize(e32, p, "isize"); got != 4 {
		t.Fatalf("isize size (32-bit) = %d, want 4", got)
	}
	if got := typeAlign(e32, p, "isize"); got != 4 {
		t.Fatalf("isize align (32-bit) = %d, want 4", got)
	}
	if got := typeSize(e32, p, "usize"); got != 4 {
		t.Fatalf("usize size (32-bit) = %d, want 4", got)
	}
	if got := typeAlign(e32, p, "usize"); got != 4 {
		t.Fatalf("usize align (32-bit) = %d, want 4", got)
	}
	if got := typeSize(e32, p, "*i32"); got != 4 {
		t.Fatalf("*i32 size (32-bit) = %d, want 4", got)
	}
	if got := typeAlign(e32, p, "*i32"); got != 4 {
		t.Fatalf("*i32 align (32-bit) = %d, want 4", got)
	}
	if got := typeSize(e32, p, "String"); got != 4 {
		t.Fatalf("String size (32-bit) = %d, want 4", got)
	}
	if got := typeAlign(e32, p, "String"); got != 4 {
		t.Fatalf("String align (32-bit) = %d, want 4", got)
	}
	if got := typeSize(e32, p, "str"); got != 4 {
		t.Fatalf("str size (32-bit) = %d, want 4", got)
	}
	if got := typeAlign(e32, p, "str"); got != 4 {
		t.Fatalf("str align (32-bit) = %d, want 4", got)
	}
}

func TestStructLayoutC(t *testing.T) {
	p := &ir.Program{TypeDecls: map[string]*ir.TypeDecl{}}
	p.TypeDecls["S"] = &ir.TypeDecl{
		Name: "S",
		Fields: []ir.Var{
			{Name: "a", Type: "i8"},
			{Name: "b", Type: "i32"},
		},
	}
	p.TypeDecls["T"] = &ir.TypeDecl{
		Name: "T",
		Fields: []ir.Var{
			{Name: "s", Type: "S"},
			{Name: "c", Type: "u16"},
		},
	}
	e := &emitter{ptrSize: 8}
	if got := structLayoutAlign(e, p, "S"); got != 4 {
		t.Fatalf("S align = %d, want 4", got)
	}
	if got := structLayoutSize(e, p, "S"); got != 8 {
		t.Fatalf("S size = %d, want 8", got)
	}
	if got := structLayoutAlign(e, p, "T"); got != 4 {
		t.Fatalf("T align = %d, want 4", got)
	}
	if got := structLayoutSize(e, p, "T"); got != 12 {
		t.Fatalf("T size = %d, want 12", got)
	}

	p.TypeDecls["P"] = &ir.TypeDecl{
		Name: "P",
		Fields: []ir.Var{
			{Name: "a", Type: "i8"},
			{Name: "b", Type: "*i32"},
			{Name: "c", Type: "u16"},
		},
	}
	if got := structLayoutAlign(e, p, "P"); got != 8 {
		t.Fatalf("P align = %d, want 8", got)
	}
	if got := structLayoutSize(e, p, "P"); got != 24 {
		t.Fatalf("P size = %d, want 24", got)
	}

	e32 := &emitter{ptrSize: 4}
	if got := structLayoutAlign(e32, p, "P"); got != 4 {
		t.Fatalf("P align (32-bit) = %d, want 4", got)
	}
	if got := structLayoutSize(e32, p, "P"); got != 12 {
		t.Fatalf("P size (32-bit) = %d, want 12", got)
	}
}
