package ir

import (
	"fmt"
	"sort"
	"strings"
)

type Kind int

const (
	KindInt Kind = iota
	KindBool
	KindString
	KindUnit
	KindRef
	KindStruct
	KindEnum
	KindArray
)

type Value struct {
	Kind   Kind
	Int    int64
	Bool   bool
	Str    string
	Ref    int
	Struct *StructValue
	Enum   *EnumValue
	Array  *ArrayValue
}

type StructValue struct {
	Name   string
	Fields map[string]Value
}

type EnumValue struct {
	Name    string
	Variant string
	Payload *Value
}

type ArrayValue struct {
	Elems []Value
}

func (v Value) String() string {
	switch v.Kind {
	case KindInt:
		return fmt.Sprintf("%d", v.Int)
	case KindBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case KindString:
		return fmt.Sprintf("\"%s\"", v.Str)
	case KindRef:
		return fmt.Sprintf("&%d", v.Ref)
	case KindStruct:
		return fmt.Sprintf("%s{...}", v.Struct.Name)
	case KindEnum:
		return fmt.Sprintf("%s.%s", v.Enum.Name, v.Enum.Variant)
	case KindArray:
		if v.Array == nil {
			return "[]"
		}
		return fmt.Sprintf("[len=%d]", len(v.Array.Elems))
	default:
		return "unit"
	}
}

type Program struct {
	Version   string
	Features  []string
	Functions map[string]*Function
	Entry     string
}

type Function struct {
	Name      string
	Params    []string
	Blocks    []*Block
	TempCount int
}

type Block struct {
	Label string
	Instr []Instr
	Term  Term
}

type Instr interface {
	instrNode()
	String() string
}

type Const struct {
	Dst   int
	Value Value
}

func (i *Const) instrNode() {}
func (i *Const) String() string {
	return fmt.Sprintf("t%d = const %s", i.Dst, i.Value.String())
}

type LoadVar struct {
	Dst  int
	Name string
}

func (i *LoadVar) instrNode() {}
func (i *LoadVar) String() string {
	return fmt.Sprintf("t%d = load %s", i.Dst, i.Name)
}

type StoreVar struct {
	Name string
	Src  int
}

func (i *StoreVar) instrNode() {}
func (i *StoreVar) String() string {
	return fmt.Sprintf("store %s, t%d", i.Name, i.Src)
}

type AddrOf struct {
	Dst  int
	Name string
}

func (i *AddrOf) instrNode() {}
func (i *AddrOf) String() string {
	return fmt.Sprintf("t%d = addr_of %s", i.Dst, i.Name)
}

type LoadRef struct {
	Dst int
	Src int
}

func (i *LoadRef) instrNode() {}
func (i *LoadRef) String() string {
	return fmt.Sprintf("t%d = load_ref t%d", i.Dst, i.Src)
}

type StoreRef struct {
	Ref int
	Src int
}

func (i *StoreRef) instrNode() {}
func (i *StoreRef) String() string {
	return fmt.Sprintf("store_ref t%d, t%d", i.Ref, i.Src)
}

type BinOp struct {
	Dst int
	Op  string
	Lhs int
	Rhs int
}

func (i *BinOp) instrNode() {}
func (i *BinOp) String() string {
	return fmt.Sprintf("t%d = %s t%d, t%d", i.Dst, i.Op, i.Lhs, i.Rhs)
}

type UnaryOp struct {
	Dst int
	Op  string
	Src int
}

func (i *UnaryOp) instrNode() {}
func (i *UnaryOp) String() string {
	return fmt.Sprintf("t%d = %s t%d", i.Dst, i.Op, i.Src)
}

type Call struct {
	Dst    int
	Callee string
	Args   []int
}

func (i *Call) instrNode() {}
func (i *Call) String() string {
	args := make([]string, 0, len(i.Args))
	for _, a := range i.Args {
		args = append(args, fmt.Sprintf("t%d", a))
	}
	if i.Dst >= 0 {
		return fmt.Sprintf("t%d = call %s(%s)", i.Dst, i.Callee, strings.Join(args, ", "))
	}
	return fmt.Sprintf("call %s(%s)", i.Callee, strings.Join(args, ", "))
}

type MakeArray struct {
	Dst   int
	Elems []int
}

func (i *MakeArray) instrNode() {}
func (i *MakeArray) String() string {
	parts := make([]string, 0, len(i.Elems))
	for _, e := range i.Elems {
		parts = append(parts, fmt.Sprintf("t%d", e))
	}
	return fmt.Sprintf("t%d = array [%s]", i.Dst, strings.Join(parts, ", "))
}

type Index struct {
	Dst   int
	Array int
	Index int
}

func (i *Index) instrNode() {}
func (i *Index) String() string {
	return fmt.Sprintf("t%d = index t%d[t%d]", i.Dst, i.Array, i.Index)
}

type SetIndex struct {
	Array int
	Index int
	Src   int
}

func (i *SetIndex) instrNode() {}
func (i *SetIndex) String() string {
	return fmt.Sprintf("set_index t%d[t%d] = t%d", i.Array, i.Index, i.Src)
}

type StructFieldInit struct {
	Name string
	Src  int
}

type MakeStruct struct {
	Dst    int
	Name   string
	Fields []StructFieldInit
}

func (i *MakeStruct) instrNode() {}
func (i *MakeStruct) String() string {
	parts := make([]string, 0, len(i.Fields))
	for _, f := range i.Fields {
		parts = append(parts, fmt.Sprintf("%s: t%d", f.Name, f.Src))
	}
	return fmt.Sprintf("t%d = struct %s { %s }", i.Dst, i.Name, strings.Join(parts, ", "))
}

type GetField struct {
	Dst   int
	Src   int
	Field string
}

func (i *GetField) instrNode() {}
func (i *GetField) String() string {
	return fmt.Sprintf("t%d = get_field t%d.%s", i.Dst, i.Src, i.Field)
}

type SetField struct {
	Src   int
	Field string
	Value int
}

func (i *SetField) instrNode() {}
func (i *SetField) String() string {
	return fmt.Sprintf("set_field t%d.%s = t%d", i.Src, i.Field, i.Value)
}

type MakeEnum struct {
	Dst     int
	Name    string
	Variant string
	Payload int
}

func (i *MakeEnum) instrNode() {}
func (i *MakeEnum) String() string {
	if i.Payload >= 0 {
		return fmt.Sprintf("t%d = enum %s.%s(t%d)", i.Dst, i.Name, i.Variant, i.Payload)
	}
	return fmt.Sprintf("t%d = enum %s.%s", i.Dst, i.Name, i.Variant)
}

type EnumTag struct {
	Dst int
	Src int
}

func (i *EnumTag) instrNode() {}
func (i *EnumTag) String() string {
	return fmt.Sprintf("t%d = enum_tag t%d", i.Dst, i.Src)
}

type EnumPayload struct {
	Dst int
	Src int
}

func (i *EnumPayload) instrNode() {}
func (i *EnumPayload) String() string {
	return fmt.Sprintf("t%d = enum_payload t%d", i.Dst, i.Src)
}

type Term interface {
	termNode()
	String() string
}

type Jump struct {
	Target string
}

func (t *Jump) termNode() {}
func (t *Jump) String() string {
	return fmt.Sprintf("jump %s", t.Target)
}

type Branch struct {
	Cond int
	Then string
	Else string
}

func (t *Branch) termNode() {}
func (t *Branch) String() string {
	return fmt.Sprintf("branch t%d, %s, %s", t.Cond, t.Then, t.Else)
}

type Return struct {
	Value *int
}

func (t *Return) termNode() {}
func (t *Return) String() string {
	if t.Value == nil {
		return "return"
	}
	return fmt.Sprintf("return t%d", *t.Value)
}

func (p *Program) Format() string {
	var sb strings.Builder
	version := p.Version
	if version == "" {
		version = "v0"
	}
	sb.WriteString(fmt.Sprintf("ir %s\n", version))
	ordered := make([]string, 0, len(p.Functions))
	for name := range p.Functions {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	for _, name := range ordered {
		fn := p.Functions[name]
		sb.WriteString(fmt.Sprintf("fn %s(%s)\n", fn.Name, strings.Join(fn.Params, ", ")))
		for _, b := range fn.Blocks {
			sb.WriteString(fmt.Sprintf("  block %s:\n", b.Label))
			for _, inst := range b.Instr {
				sb.WriteString("    ")
				sb.WriteString(inst.String())
				sb.WriteString("\n")
			}
			if b.Term != nil {
				sb.WriteString("    ")
				sb.WriteString(b.Term.String())
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (p *Program) Validate() error {
	version := p.Version
	if version == "" {
		version = "v0"
	}
	if version != "v0" {
		return fmt.Errorf("unsupported IR version '%s'", version)
	}
	if len(p.Features) > 0 {
		return fmt.Errorf("unsupported IR feature '%s'", p.Features[0])
	}
	if p.Entry != "" {
		if _, ok := p.Functions[p.Entry]; !ok {
			return fmt.Errorf("entry function '%s' not found", p.Entry)
		}
	}
	return nil
}
