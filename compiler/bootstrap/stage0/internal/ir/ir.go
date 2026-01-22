package ir

import (
	"fmt"
	"regexp"
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

// Operand represents either a temp variable or an inline constant
type Operand struct {
	IsConst bool
	Temp    int
	Const   Value
}

// TempOperand creates an operand from a temp ID
func TempOperand(temp int) Operand {
	return Operand{IsConst: false, Temp: temp}
}

// ConstOperand creates an operand from a constant value
func ConstOperand(val Value) Operand {
	return Operand{IsConst: true, Const: val}
}

// IntOperand creates an integer constant operand
func IntOperand(n int64) Operand {
	return ConstOperand(Value{Kind: KindInt, Int: n})
}

// BoolOperand creates a boolean constant operand
func BoolOperand(b bool) Operand {
	return ConstOperand(Value{Kind: KindBool, Bool: b})
}

// StringOperand creates a string constant operand
func StringOperand(s string) Operand {
	return ConstOperand(Value{Kind: KindString, Str: s})
}

func (o Operand) String() string {
	if o.IsConst {
		return o.Const.String()
	}
	return fmt.Sprintf("t%d", o.Temp)
}

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

type TypeDecl struct {
	Name   string
	Fields []Var // For struct types
}

type Program struct {
	Version   string
	Features  []string
	TypeDecls map[string]*TypeDecl // Type declarations (structs, enums)
	Functions map[string]*Function
	Entry     string
}

// Var represents a typed variable (parameter, local, or temp)
type Var struct {
	Name string
	Type string
}

type Function struct {
	Name       string
	Params     []Var // Parameters with types
	ReturnType string
	Locals     []Var    // Local variables with types
	TempTypes  []string // Types for each temp (indexed by temp number)
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
	Dst     int
	Name    string
	Addr    bool
	Ref     bool
	RefTemp int
}

func (i *LoadVar) instrNode() {}
func (i *LoadVar) String() string {
	if i.Ref {
		return fmt.Sprintf("t%d = load_ref t%d", i.Dst, i.RefTemp)
	}
	if i.Addr {
		return fmt.Sprintf("t%d = load_addr %s", i.Dst, i.Name)
	}
	return fmt.Sprintf("t%d = load %s", i.Dst, i.Name)
}

type StoreVar struct {
	Name    string
	Src     int
	Ref     bool
	RefTemp int
}

func (i *StoreVar) instrNode() {}
func (i *StoreVar) String() string {
	if i.Ref {
		return fmt.Sprintf("store_ref t%d, t%d", i.RefTemp, i.Src)
	}
	return fmt.Sprintf("store %s, t%d", i.Name, i.Src)
}

type BinOp struct {
	Dst int
	Op  string
	Lhs Operand
	Rhs Operand
}

func (i *BinOp) instrNode() {}
func (i *BinOp) String() string {
	return fmt.Sprintf("t%d = %s %s, %s", i.Dst, i.Op, i.Lhs.String(), i.Rhs.String())
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
	Unchecked bool
	Dst   int
	Array int
	Index int
}

func (i *Index) instrNode() {}
func (i *Index) String() string {
	if i.Unchecked { return fmt.Sprintf("t%d = index t%d[t%d] @unchecked", i.Dst, i.Array, i.Index) }; return fmt.Sprintf("t%d = index t%d[t%d]", i.Dst, i.Array, i.Index)
}

type SetIndex struct {
	Unchecked bool
	Array int
	Index int
	Src   int
}

func (i *SetIndex) instrNode() {}
func (i *SetIndex) String() string {
	if i.Unchecked { return fmt.Sprintf("set_index t%d[t%d] = t%d @unchecked", i.Array, i.Index, i.Src) }; return fmt.Sprintf("set_index t%d[t%d] = t%d", i.Array, i.Index, i.Src)
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
	return fmt.Sprintf("t%d = t%d.%s", i.Dst, i.Src, i.Field)
}

type SetField struct {
	Src   int
	Field string
	Value int
}

func (i *SetField) instrNode() {}
func (i *SetField) String() string {
	return fmt.Sprintf("t%d.%s = t%d", i.Src, i.Field, i.Value)
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

	// Output type declarations
	if len(p.TypeDecls) > 0 {
		typeNames := make([]string, 0, len(p.TypeDecls))
		for name := range p.TypeDecls {
			typeNames = append(typeNames, name)
		}
		sort.Strings(typeNames)
		for _, name := range typeNames {
			td := p.TypeDecls[name]
			if len(td.Fields) > 0 {
				fields := make([]string, 0, len(td.Fields))
				for _, f := range td.Fields {
					fields = append(fields, fmt.Sprintf("%s: %s", f.Name, f.Type))
				}
				sb.WriteString(fmt.Sprintf("type %s = { %s }\n", td.Name, strings.Join(fields, ", ")))
			}
		}
		sb.WriteString("\n")
	}

	ordered := make([]string, 0, len(p.Functions))
	for name := range p.Functions {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	for _, name := range ordered {
		fn := p.Functions[name]
		retType := fn.ReturnType
		if retType == "" {
			retType = "unit"
		}
		sb.WriteString(fmt.Sprintf("fn %s(%s) -> %s\n", fn.Name, formatParams(fn.Params), retType))

		// Smart temp renumbering using intermediate names to avoid cascading
		paramCount := len(fn.Params)

		// Collect param names that look like temps (tN)
		reservedTempNames := make(map[string]bool)
		paramRemap := make(map[string]string)
		for i, param := range fn.Params {
			paramRemap[fmt.Sprintf("t%d", i)] = param.Name
			if regexp.MustCompile(`^t\d+$`).MatchString(param.Name) {
				reservedTempNames[param.Name] = true
			}
		}

		// Build renumbering map using intermediate names
		bodyToIntermediate := make(map[string]string)
		intermediateToFinal := make(map[string]string)
		finalToOriginal := make(map[string]int) // Map final temp name -> original temp ID
		nextAvailableTemp := 0
		for i := paramCount; i < paramCount+100; i++ {
			for reservedTempNames[fmt.Sprintf("t%d", nextAvailableTemp)] {
				nextAvailableTemp++
			}
			intermediate := fmt.Sprintf("__T%d__", i-paramCount)
			bodyToIntermediate[fmt.Sprintf("t%d", i)] = intermediate
			finalName := fmt.Sprintf("t%d", nextAvailableTemp)
			intermediateToFinal[intermediate] = finalName
			finalToOriginal[finalName] = i // Store mapping
			nextAvailableTemp++
		}

		// 3-pass replacement to avoid cascading
		replaceTempsFn := func(s string) string {
			// Pass 1: body temps -> intermediate
			for old, inter := range bodyToIntermediate {
				re := regexp.MustCompile(`\b` + regexp.QuoteMeta(old) + `\b`)
				s = re.ReplaceAllString(s, inter)
			}
			// Pass 2: param IDs -> param names
			for old, name := range paramRemap {
				re := regexp.MustCompile(`\b` + regexp.QuoteMeta(old) + `\b`)
				s = re.ReplaceAllString(s, name)
			}
			// Pass 3: intermediate -> final temps
			for inter, final := range intermediateToFinal {
				s = strings.ReplaceAll(s, inter, final)
			}
			return s
		}

		// Helper to add type annotations to instructions
		addTypeAnnotations := func(s string) string {
			// Match pattern: tN = ...
			re := regexp.MustCompile(`^(t\d+) = `)
			matches := re.FindStringSubmatch(s)
			if len(matches) > 1 {
				tempName := matches[1]
				// Look up original temp ID from final name
				if originalID, ok := finalToOriginal[tempName]; ok {
					if originalID < len(fn.TempTypes) && fn.TempTypes[originalID] != "" {
						typ := fn.TempTypes[originalID]
						return re.ReplaceAllString(s, fmt.Sprintf("${1}: %s = ", typ))
					}
				}
			}
			return s
		}

		for _, b := range fn.Blocks {
			sb.WriteString(fmt.Sprintf("  block %s:\n", b.Label))
			for _, inst := range b.Instr {
				sb.WriteString("    ")
				instStr := replaceTempsFn(inst.String())
				instStr = addTypeAnnotations(instStr)
				sb.WriteString(instStr)
				sb.WriteString("\n")
			}
			if b.Term != nil {
				sb.WriteString("    ")
				termStr := replaceTempsFn(b.Term.String())
				sb.WriteString(termStr)
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
		if i.Ref {
			if err := validateTemp(i.RefTemp, tempCount, false); err != nil {
				return err
			}
			return validateTemp(i.Dst, tempCount, false)
		}
		if i.Name == "" {
			return fmt.Errorf("load var name is empty")
		}
		if _, ok := declared[i.Name]; !ok {
			return fmt.Errorf("undefined variable '%s'", i.Name)
		}
		return validateTemp(i.Dst, tempCount, false)
	case *StoreVar:
		if i.Ref {
			if err := validateTemp(i.RefTemp, tempCount, false); err != nil {
				return err
			}
			return validateTemp(i.Src, tempCount, false)
		}
		if i.Name == "" {
			return fmt.Errorf("store var name is empty")
		}
		return validateTemp(i.Src, tempCount, false)
	case *BinOp:
		if !isValidBinOp(i.Op) {
			return fmt.Errorf("invalid binop '%s'", i.Op)
		}
		if err := validateTemp(i.Dst, tempCount, false); err != nil {
			return err
		}
		if err := validateOperand(i.Lhs, tempCount); err != nil {
			return err
		}
		return validateOperand(i.Rhs, tempCount)
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
	default:
		return fmt.Errorf("unknown instruction")
	}
}

func collectDeclaredVars(fn *Function) (map[string]struct{}, error) {
	declared := map[string]struct{}{}
	for _, p := range fn.Params {
		if p.Name == "" {
			return nil, fmt.Errorf("param name is empty")
		}
		if _, ok := declared[p.Name]; ok {
			return nil, fmt.Errorf("duplicate param '%s'", p.Name)
		}
		declared[p.Name] = struct{}{}
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
		params[p.Name] = struct{}{}
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
				if v.Ref {
					continue
				}
				if _, ok := cur[v.Name]; !ok {
					return fmt.Errorf("use of possibly uninitialized variable '%s'", v.Name)
				}
			case *StoreVar:
				if v.Ref {
					continue
				}
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

func validateTemp(t int, tempCount int, allowNegative bool) error {
	if allowNegative && t < 0 {
		return nil
	}
	if t < 0 || t >= tempCount {
		return fmt.Errorf("temp t%d out of range [0, %d)", t, tempCount)
	}
	return nil
}

func validateOperand(op Operand, tempCount int) error {
	if op.IsConst {
		return nil // Constants are always valid
	}
	return validateTemp(op.Temp, tempCount, false)
}

func isValidBinOp(op string) bool {
	switch op {
	case "+", "-", "*", "/", "%", "==", "!=", "<", "<=", ">", ">=", "&&", "||":
		return true
	default:
		return false
	}
}

func formatParams(params []Var) string {
	if len(params) == 0 {
		return ""
	}
	parts := make([]string, 0, len(params))
	for _, p := range params {
		if p.Type != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", p.Name, p.Type))
		} else {
			parts = append(parts, p.Name)
		}
	}
	return strings.Join(parts, ", ")
}
