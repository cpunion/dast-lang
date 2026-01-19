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
	Kind    Kind
	Int     int64
	IntType string
	Bool    bool
	Str     string
	Ref     int
	Struct  *StructValue
	Enum    *EnumValue
	Array   *ArrayValue
}

type StructValue struct {
	Name   string
	Fields map[string]Value
}

type EnumValue struct {
	Name    string
	Variant string
	Tag     int64
	TagType string
	Payload *Value
}

type ArrayValue struct {
	Elems []Value
}

func (v Value) String() string {
	switch v.Kind {
	case KindInt:
		if v.IntType != "" {
			return fmt.Sprintf("%s %d", v.IntType, v.Int)
		}
		return fmt.Sprintf("%d", v.Int)
	case KindBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case KindString:
		return fmt.Sprintf("\"%s\"", escapeString(v.Str))
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

func escapeString(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '\n':
			sb.WriteString("\\n")
		case '\t':
			sb.WriteString("\\t")
		case '\r':
			sb.WriteString("\\r")
		case '\\':
			sb.WriteString("\\\\")
		case '"':
			sb.WriteString("\\\"")
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

type Program struct {
	Version   string
	Features  []string
	Functions map[string]*Function
	Entry     string
}

type Function struct {
	Name       string
	Params     []string
	ParamTypes []string
	ReturnType string
	Blocks     []*Block
	TempCount  int
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

type IndexUnchecked struct {
	Dst   int
	Array int
	Index int
}

func (i *IndexUnchecked) instrNode() {}
func (i *IndexUnchecked) String() string {
	return fmt.Sprintf("t%d = index_unchecked t%d[t%d]", i.Dst, i.Array, i.Index)
}

type SetIndexUnchecked struct {
	Array int
	Index int
	Src   int
}

func (i *SetIndexUnchecked) instrNode() {}
func (i *SetIndexUnchecked) String() string {
	return fmt.Sprintf("set_index_unchecked t%d[t%d] = t%d", i.Array, i.Index, i.Src)
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
	Tag     int64
	TagType string
	Payload int
}

func (i *MakeEnum) instrNode() {}
func (i *MakeEnum) String() string {
	tag := ""
	if i.TagType != "" {
		tag = fmt.Sprintf("@%d:%s", i.Tag, i.TagType)
	}
	if i.Payload >= 0 {
		return fmt.Sprintf("t%d = enum %s.%s%s(t%d)", i.Dst, i.Name, i.Variant, tag, i.Payload)
	}
	return fmt.Sprintf("t%d = enum %s.%s%s", i.Dst, i.Name, i.Variant, tag)
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
		sb.WriteString(fmt.Sprintf("fn %s(%s)", fn.Name, formatParams(fn.Params, fn.ParamTypes)))
		if fn.ReturnType != "" && fn.ReturnType != "unit" {
			sb.WriteString(" -> ")
			sb.WriteString(fn.ReturnType)
		}
		sb.WriteString("\n")
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
	for name, fn := range p.Functions {
		if fn == nil {
			return fmt.Errorf("function '%s' is nil", name)
		}
		if fn.Name == "" {
			return fmt.Errorf("function name is empty")
		}
		if fn.Name != name {
			return fmt.Errorf("function name mismatch: key '%s' vs name '%s'", name, fn.Name)
		}
	}
	for _, fn := range p.Functions {
		if err := validateFunction(fn); err != nil {
			return fmt.Errorf("function '%s': %w", fn.Name, err)
		}
	}
	return nil
}

func validateFunction(fn *Function) error {
	if fn.TempCount < 0 {
		return fmt.Errorf("invalid temp_count %d", fn.TempCount)
	}
	if len(fn.Blocks) == 0 {
		return fmt.Errorf("function has no blocks")
	}
	labels := map[string]struct{}{}
	for _, blk := range fn.Blocks {
		if blk.Label == "" {
			return fmt.Errorf("block label is empty")
		}
		if _, ok := labels[blk.Label]; ok {
			return fmt.Errorf("duplicate block label '%s'", blk.Label)
		}
		labels[blk.Label] = struct{}{}
	}
	declared, err := collectDeclaredVars(fn)
	if err != nil {
		return err
	}
	for _, blk := range fn.Blocks {
		if err := validateBlock(blk, labels, fn.TempCount, declared); err != nil {
			return fmt.Errorf("block '%s': %w", blk.Label, err)
		}
	}
	if err := validateDefiniteAssignment(fn); err != nil {
		return err
	}
	return nil
}

func validateBlock(blk *Block, labels map[string]struct{}, tempCount int, declared map[string]struct{}) error {
	for _, inst := range blk.Instr {
		if err := validateInstr(inst, tempCount, declared); err != nil {
			return err
		}
	}
	if blk.Term == nil {
		return fmt.Errorf("missing terminator")
	}
	switch t := blk.Term.(type) {
	case *Jump:
		if t.Target == "" {
			return fmt.Errorf("empty jump target")
		}
		if _, ok := labels[t.Target]; !ok {
			return fmt.Errorf("jump target '%s' not found", t.Target)
		}
	case *Branch:
		if err := validateTemp(t.Cond, tempCount, false); err != nil {
			return err
		}
		if t.Then == "" || t.Else == "" {
			return fmt.Errorf("branch target is empty")
		}
		if _, ok := labels[t.Then]; !ok {
			return fmt.Errorf("branch target '%s' not found", t.Then)
		}
		if _, ok := labels[t.Else]; !ok {
			return fmt.Errorf("branch target '%s' not found", t.Else)
		}
	case *Return:
		if t.Value != nil {
			if err := validateTemp(*t.Value, tempCount, false); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown terminator")
	}
	return nil
}

func validateInstr(inst Instr, tempCount int, declared map[string]struct{}) error {
	switch i := inst.(type) {
	case *Const:
		return validateTemp(i.Dst, tempCount, false)
	case *LoadVar:
		if i.Name == "" {
			return fmt.Errorf("load var name is empty")
		}
		if _, ok := declared[i.Name]; !ok {
			return fmt.Errorf("undefined variable '%s'", i.Name)
		}
		return validateTemp(i.Dst, tempCount, false)
	case *StoreVar:
		if i.Name == "" {
			return fmt.Errorf("store var name is empty")
		}
		return validateTemp(i.Src, tempCount, false)
	case *AddrOf:
		if i.Name == "" {
			return fmt.Errorf("addr_of name is empty")
		}
		if _, ok := declared[i.Name]; !ok {
			return fmt.Errorf("undefined variable '%s'", i.Name)
		}
		return validateTemp(i.Dst, tempCount, false)
	case *LoadRef:
		if err := validateTemp(i.Dst, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Src, tempCount, false)
	case *StoreRef:
		if err := validateTemp(i.Ref, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Src, tempCount, false)
	case *BinOp:
		if !isValidBinOp(i.Op) {
			return fmt.Errorf("invalid binop '%s'", i.Op)
		}
		if err := validateTemp(i.Dst, tempCount, false); err != nil {
			return err
		}
		if err := validateTemp(i.Lhs, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Rhs, tempCount, false)
	case *UnaryOp:
		if !isValidUnaryOp(i.Op) {
			return fmt.Errorf("invalid unary op '%s'", i.Op)
		}
		if err := validateTemp(i.Dst, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Src, tempCount, false)
	case *Call:
		if i.Callee == "" {
			return fmt.Errorf("call callee is empty")
		}
		if err := validateTemp(i.Dst, tempCount, true); err != nil {
			return err
		}
		for _, arg := range i.Args {
			if err := validateTemp(arg, tempCount, false); err != nil {
				return err
			}
		}
		return nil
	case *MakeArray:
		if err := validateTemp(i.Dst, tempCount, false); err != nil {
			return err
		}
		for _, e := range i.Elems {
			if err := validateTemp(e, tempCount, false); err != nil {
				return err
			}
		}
		return nil
	case *Index:
		if err := validateTemp(i.Dst, tempCount, false); err != nil {
			return err
		}
		if err := validateTemp(i.Array, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Index, tempCount, false)
	case *SetIndex:
		if err := validateTemp(i.Array, tempCount, false); err != nil {
			return err
		}
		if err := validateTemp(i.Index, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Src, tempCount, false)
	case *IndexUnchecked:
		if err := validateTemp(i.Dst, tempCount, false); err != nil {
			return err
		}
		if err := validateTemp(i.Array, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Index, tempCount, false)
	case *SetIndexUnchecked:
		if err := validateTemp(i.Array, tempCount, false); err != nil {
			return err
		}
		if err := validateTemp(i.Index, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Src, tempCount, false)
	case *MakeStruct:
		if i.Name == "" {
			return fmt.Errorf("struct name is empty")
		}
		if err := validateTemp(i.Dst, tempCount, false); err != nil {
			return err
		}
		fields := map[string]struct{}{}
		for _, f := range i.Fields {
			if f.Name == "" {
				return fmt.Errorf("struct field name is empty")
			}
			if _, ok := fields[f.Name]; ok {
				return fmt.Errorf("duplicate struct field '%s'", f.Name)
			}
			fields[f.Name] = struct{}{}
			if err := validateTemp(f.Src, tempCount, false); err != nil {
				return err
			}
		}
		return nil
	case *GetField:
		if i.Field == "" {
			return fmt.Errorf("get_field name is empty")
		}
		if err := validateTemp(i.Dst, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Src, tempCount, false)
	case *SetField:
		if i.Field == "" {
			return fmt.Errorf("set_field name is empty")
		}
		if err := validateTemp(i.Src, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Value, tempCount, false)
	case *MakeEnum:
		if i.Name == "" || i.Variant == "" {
			return fmt.Errorf("enum name or variant is empty")
		}
		if err := validateTemp(i.Dst, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Payload, tempCount, true)
	case *EnumTag:
		if err := validateTemp(i.Dst, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Src, tempCount, false)
	case *EnumPayload:
		if err := validateTemp(i.Dst, tempCount, false); err != nil {
			return err
		}
		return validateTemp(i.Src, tempCount, false)
	default:
		return fmt.Errorf("unknown instruction")
	}
}

func collectDeclaredVars(fn *Function) (map[string]struct{}, error) {
	declared := map[string]struct{}{}
	for _, p := range fn.Params {
		if p == "" {
			return nil, fmt.Errorf("param name is empty")
		}
		if _, ok := declared[p]; ok {
			return nil, fmt.Errorf("duplicate param '%s'", p)
		}
		declared[p] = struct{}{}
	}
	for _, blk := range fn.Blocks {
		for _, inst := range blk.Instr {
			if sv, ok := inst.(*StoreVar); ok {
				if sv.Name != "" {
					if _, ok := declared[sv.Name]; !ok {
						declared[sv.Name] = struct{}{}
					}
				}
			}
		}
	}
	return declared, nil
}

func validateDefiniteAssignment(fn *Function) error {
	if len(fn.Blocks) == 0 {
		return nil
	}
	params := map[string]struct{}{}
	for _, p := range fn.Params {
		params[p] = struct{}{}
	}
	blocks := map[string]*Block{}
	for _, blk := range fn.Blocks {
		blocks[blk.Label] = blk
	}
	reachable := reachableBlocks(fn, blocks)
	storeSets := map[string]map[string]struct{}{}
	for _, blk := range fn.Blocks {
		set := map[string]struct{}{}
		for _, inst := range blk.Instr {
			if sv, ok := inst.(*StoreVar); ok && sv.Name != "" {
				set[sv.Name] = struct{}{}
			}
		}
		storeSets[blk.Label] = set
	}
	allVars := copySet(params)
	for _, set := range storeSets {
		for name := range set {
			allVars[name] = struct{}{}
		}
	}
	inSets := map[string]map[string]struct{}{}
	outSets := map[string]map[string]struct{}{}
	for label := range reachable {
		inSets[label] = copySet(allVars)
		outSets[label] = copySet(allVars)
	}
	entry := fn.Blocks[0].Label
	changed := true
	for changed {
		changed = false
		for _, blk := range fn.Blocks {
			label := blk.Label
			if _, ok := reachable[label]; !ok {
				continue
			}
			var inSet map[string]struct{}
			if label == entry {
				inSet = copySet(params)
			} else {
				preds := predecessors(fn, label)
				first := true
				for _, p := range preds {
					if _, ok := reachable[p]; !ok {
						continue
					}
					if first {
						inSet = copySet(outSets[p])
						first = false
					} else {
						inSet = intersectSets(inSet, outSets[p])
					}
				}
				if first {
					inSet = map[string]struct{}{}
				}
			}
			outSet := unionSets(inSet, storeSets[label])
			if !setEqual(outSets[label], outSet) {
				outSets[label] = outSet
				changed = true
			}
			inSets[label] = inSet
		}
	}
	for _, blk := range fn.Blocks {
		label := blk.Label
		if _, ok := reachable[label]; !ok {
			continue
		}
		cur := copySet(inSets[label])
		for _, inst := range blk.Instr {
			switch v := inst.(type) {
			case *LoadVar:
				if _, ok := cur[v.Name]; !ok {
					return fmt.Errorf("use of possibly uninitialized variable '%s'", v.Name)
				}
			case *AddrOf:
				if _, ok := cur[v.Name]; !ok {
					return fmt.Errorf("use of possibly uninitialized variable '%s'", v.Name)
				}
			case *StoreVar:
				if v.Name != "" {
					cur[v.Name] = struct{}{}
				}
			}
		}
	}
	return nil
}

func reachableBlocks(fn *Function, blocks map[string]*Block) map[string]struct{} {
	reached := map[string]struct{}{}
	if len(fn.Blocks) == 0 {
		return reached
	}
	stack := []string{fn.Blocks[0].Label}
	for len(stack) > 0 {
		label := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := reached[label]; ok {
			continue
		}
		reached[label] = struct{}{}
		blk := blocks[label]
		if blk == nil || blk.Term == nil {
			continue
		}
		switch t := blk.Term.(type) {
		case *Jump:
			stack = append(stack, t.Target)
		case *Branch:
			stack = append(stack, t.Then, t.Else)
		}
	}
	return reached
}

func predecessors(fn *Function, label string) []string {
	preds := []string{}
	for _, blk := range fn.Blocks {
		if blk.Term == nil {
			continue
		}
		switch t := blk.Term.(type) {
		case *Jump:
			if t.Target == label {
				preds = append(preds, blk.Label)
			}
		case *Branch:
			if t.Then == label || t.Else == label {
				preds = append(preds, blk.Label)
			}
		}
	}
	return preds
}

func copySet(src map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for k := range src {
		out[k] = struct{}{}
	}
	return out
}

func unionSets(a map[string]struct{}, b map[string]struct{}) map[string]struct{} {
	out := copySet(a)
	for k := range b {
		out[k] = struct{}{}
	}
	return out
}

func intersectSets(a map[string]struct{}, b map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for k := range a {
		if _, ok := b[k]; ok {
			out[k] = struct{}{}
		}
	}
	return out
}

func setEqual(a map[string]struct{}, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func validateTemp(idx int, tempCount int, allowNeg1 bool) error {
	if allowNeg1 && idx == -1 {
		return nil
	}
	if idx < 0 {
		return fmt.Errorf("invalid temp t%d", idx)
	}
	if idx >= tempCount {
		return fmt.Errorf("temp t%d out of range (temp_count=%d)", idx, tempCount)
	}
	return nil
}

func isValidBinOp(op string) bool {
	switch op {
	case "+", "-", "*", "/", "%", "==", "!=", "<", "<=", ">", ">=", "&&", "||":
		return true
	default:
		return false
	}
}

func isValidUnaryOp(op string) bool {
	return op == "-" || op == "!"
}

func formatParams(params []string, types []string) string {
	if len(params) == 0 {
		return ""
	}
	hasTypes := false
	if len(types) == len(params) {
		for _, t := range types {
			if strings.TrimSpace(t) != "" {
				hasTypes = true
				break
			}
		}
	}
	parts := make([]string, 0, len(params))
	for i, p := range params {
		if hasTypes {
			t := ""
			if i < len(types) {
				t = strings.TrimSpace(types[i])
			}
			if t != "" {
				parts = append(parts, fmt.Sprintf("%s: %s", p, t))
			} else {
				parts = append(parts, p)
			}
			continue
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, ", ")
}
