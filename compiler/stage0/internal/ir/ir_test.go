package ir

import (
	"strings"
	"testing"
)

func TestFormatParamNameWithDollar(t *testing.T) {
	fn := &Function{
		Name:       "main$0",
		Params:     []Var{{Name: "$env", Type: "$Env"}},
		ReturnType: "unit",
		TempCount:  2,
		TempTypes:  []string{"$Env", "i64"},
	}
	blk := &Block{Label: "entry0"}
	blk.Instr = []Instr{
		&GetField{Dst: 1, Src: 0, Field: "count"},
	}
	blk.Term = &Return{Value: nil}
	fn.Blocks = []*Block{blk}

	prog := &Program{
		Version:   "v0",
		Functions: map[string]*Function{fn.Name: fn},
	}
	out := prog.Format()
	if strings.Contains(out, " = .count") {
		t.Fatalf("expected $env prefix in GetField output, got:\n%s", out)
	}
	if !strings.Contains(out, "$env.count") {
		t.Fatalf("expected $env.count in output, got:\n%s", out)
	}
}
