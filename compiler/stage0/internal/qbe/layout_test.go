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
	if got := typeSize(e64, p, "i16"); got != 2 {
		t.Fatalf("i16 size = %d, want 2", got)
	}
	if got := typeAlign(e64, p, "i16"); got != 2 {
		t.Fatalf("i16 align = %d, want 2", got)
	}
	if got := typeSize(e64, p, "i32"); got != 4 {
		t.Fatalf("i32 size = %d, want 4", got)
	}
	if got := typeAlign(e64, p, "i32"); got != 4 {
		t.Fatalf("i32 align = %d, want 4", got)
	}
	if got := typeSize(e64, p, "i64"); got != 8 {
		t.Fatalf("i64 size = %d, want 8", got)
	}
	if got := typeAlign(e64, p, "i64"); got != 8 {
		t.Fatalf("i64 align = %d, want 8", got)
	}
	if got := typeSize(e64, p, "isize"); got != 8 {
		t.Fatalf("isize size = %d, want 8", got)
	}
	if got := typeAlign(e64, p, "isize"); got != 8 {
		t.Fatalf("isize align = %d, want 8", got)
	}
	e32 := &emitter{ptrSize: 4}
	if got := typeSize(e32, p, "isize"); got != 4 {
		t.Fatalf("isize size (32-bit) = %d, want 4", got)
	}
	if got := typeAlign(e32, p, "isize"); got != 4 {
		t.Fatalf("isize align (32-bit) = %d, want 4", got)
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
}
