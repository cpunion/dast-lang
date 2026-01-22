package ir

import (
	"fmt"
	"strconv"
	"strings"
)

// parseContext holds parameter name to temp index mapping for current function
type parseContext struct {
	paramToTemp map[string]int
	paramCount  int // Number of parameters - body temps are offset by this
}

func newParseContext(params []Var) *parseContext {
	ctx := &parseContext{paramToTemp: map[string]int{}, paramCount: len(params)}
	for i, p := range params {
		ctx.paramToTemp[p.Name] = i
	}
	return ctx
}

func Parse(text string) (*Program, error) {
	lines := strings.Split(text, "\n")
	i := 0
	header := ""
	for i < len(lines) && header == "" {
		header = strings.TrimSpace(lines[i])
		if header == "" {
			i++
		}
	}
	if header == "" {
		return nil, fmt.Errorf("ir parse error (line 1): empty ir")
	}
	if !strings.HasPrefix(header, "ir ") {
		return nil, fmt.Errorf("ir parse error (line %d): missing ir header", i+1)
	}
	version := strings.TrimSpace(strings.TrimPrefix(header, "ir "))
	if version == "" {
		version = "v0"
	}
	prog := &Program{Version: version, Functions: map[string]*Function{}}
	i++
	var curFn *Function
	var curBlk *Block
	var ctx *parseContext
	maxTemp := -1
	flushBlock := func() {
		if curBlk != nil && curFn != nil {
			curFn.Blocks = append(curFn.Blocks, curBlk)
		}
		curBlk = nil
	}
	flushFn := func() error {
		if curFn == nil {
			return nil
		}
		if maxTemp < 0 {
			curFn.TempCount = 0
		} else {
			curFn.TempCount = maxTemp + 1
		}
		if _, exists := prog.Functions[curFn.Name]; exists {
			return fmt.Errorf("duplicate function '%s'", curFn.Name)
		}
		prog.Functions[curFn.Name] = curFn
		curFn = nil
		ctx = nil
		return nil
	}
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			flushBlock()
			if err := flushFn(); err != nil {
				return nil, err
			}
			i++
			continue
		}
		// Skip type declarations
		if strings.HasPrefix(line, "type ") {
			i++
			continue
		}
		if strings.HasPrefix(line, "fn ") {
			flushBlock()
			if err := flushFn(); err != nil {
				return nil, err
			}
			name, params, retType, err := parseFnHeader(line)
			if err != nil {
				return nil, fmt.Errorf("ir parse error (line %d): %w", i+1, err)
			}
			curFn = &Function{Name: name, Params: params, ReturnType: retType}
			ctx = newParseContext(params)
			maxTemp = -1
			i++
			continue
		}
		if strings.HasPrefix(line, "block ") {
			if curFn == nil {
				return nil, fmt.Errorf("ir parse error (line %d): block outside function", i+1)
			}
			if curBlk != nil {
				curFn.Blocks = append(curFn.Blocks, curBlk)
			}
			label, err := parseBlockLabel(line)
			if err != nil {
				return nil, fmt.Errorf("ir parse error (line %d): %w", i+1, err)
			}
			curBlk = &Block{Label: label}
			i++
			continue
		}
		if curBlk == nil {
			return nil, fmt.Errorf("ir parse error (line %d): instruction outside block", i+1)
		}
		if curBlk.Term != nil {
			return nil, fmt.Errorf("ir parse error (line %d): instruction after terminator", i+1)
		}
		lp, err := parseIRLineWithCtx(line, ctx)
		if err != nil {
			return nil, fmt.Errorf("ir parse error (line %d): %w", i+1, err)
		}
		if lp.isTerm {
			curBlk.Term = lp.term
		} else {
			curBlk.Instr = append(curBlk.Instr, lp.instr)
		}
		if lp.maxTemp > maxTemp {
			maxTemp = lp.maxTemp
		}
		i++
	}
	flushBlock()
	if err := flushFn(); err != nil {
		return nil, err
	}
	if _, ok := prog.Functions["main"]; ok {
		prog.Entry = "main"
	} else if len(prog.Functions) == 1 {
		for name := range prog.Functions {
			prog.Entry = name
		}
	}
	return prog, nil
}

type lineParse struct {
	instr   Instr
	term    Term
	isTerm  bool
	maxTemp int
}

func parseIRLine(line string) (lineParse, error) {
	return parseIRLineWithCtx(line, nil)
}

func parseIRLineWithCtx(line string, ctx *parseContext) (lineParse, error) {
	if strings.HasPrefix(line, "jump ") {
		target := strings.TrimSpace(strings.TrimPrefix(line, "jump "))
		return lineParse{term: &Jump{Target: target}, isTerm: true, maxTemp: -1}, nil
	}
	if strings.HasPrefix(line, "branch ") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
		parts := splitComma(rest, 3)
		if len(parts) != 3 {
			return lineParse{}, fmt.Errorf("invalid branch syntax")
		}
		cond, err := parseTempWithCtx(parts[0], ctx)
		if err != nil {
			return lineParse{}, err
		}
		return lineParse{term: &Branch{Cond: cond, Then: parts[1], Else: parts[2]}, isTerm: true, maxTemp: cond}, nil
	}
	if strings.HasPrefix(line, "return") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "return"))
		if rest == "" {
			return lineParse{term: &Return{Value: nil}, isTerm: true, maxTemp: -1}, nil
		}
		val, err := parseTempWithCtx(rest, ctx)
		if err != nil {
			return lineParse{}, err
		}
		return lineParse{term: &Return{Value: &val}, isTerm: true, maxTemp: val}, nil
	}
	if strings.HasPrefix(line, "store_ref ") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "store_ref "))
		parts := splitComma(rest, 2)
		if len(parts) != 2 {
			return lineParse{}, fmt.Errorf("invalid store_ref syntax")
		}
		ref, err := parseTempWithCtx(parts[0], ctx)
		if err != nil {
			return lineParse{}, err
		}
		src, err := parseTempWithCtx(parts[1], ctx)
		if err != nil {
			return lineParse{}, err
		}
		max := maxTempIdx(ref, src)
		return lineParse{instr: &StoreRef{Ref: ref, Src: src}, maxTemp: max}, nil
	}
	if strings.HasPrefix(line, "store ") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "store "))
		parts := splitComma(rest, 2)
		if len(parts) != 2 {
			return lineParse{}, fmt.Errorf("invalid store syntax")
		}
		src, err := parseTempWithCtx(parts[1], ctx)
		if err != nil {
			return lineParse{}, err
		}
		return lineParse{instr: &StoreVar{Name: parts[0], Src: src}, maxTemp: src}, nil
	}
	if strings.HasPrefix(line, "set_index_unchecked ") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "set_index_unchecked "))
		eq := strings.Index(rest, "=")
		if eq < 0 {
			return lineParse{}, fmt.Errorf("invalid set_index_unchecked syntax")
		}
		left := strings.TrimSpace(rest[:eq])
		right := strings.TrimSpace(rest[eq+1:])
		array, index, err := parseIndexExprWithCtx(left, ctx)
		if err != nil {
			return lineParse{}, err
		}
		src, err := parseTempWithCtx(right, ctx)
		if err != nil {
			return lineParse{}, err
		}
		max := maxTempIdx(array, index, src)
		return lineParse{instr: &SetIndex{Unchecked: true, Array: array, Index: index, Src: src}, maxTemp: max}, nil
	}
	if strings.HasPrefix(line, "set_index ") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "set_index "))
		eq := strings.Index(rest, "=")
		if eq < 0 {
			return lineParse{}, fmt.Errorf("invalid set_index syntax")
		}
		left := strings.TrimSpace(rest[:eq])
		right := strings.TrimSpace(rest[eq+1:])
		unchecked := false
		if strings.HasSuffix(right, "@unchecked") {
			unchecked = true
			right = strings.TrimSpace(right[:len(right)-10])
		}
		array, index, err := parseIndexExprWithCtx(left, ctx)
		if err != nil {
			return lineParse{}, err
		}
		src, err := parseTempWithCtx(right, ctx)
		if err != nil {
			return lineParse{}, err
		}
		max := maxTempIdx(array, index, src)
		return lineParse{instr: &SetIndex{Unchecked: unchecked, Array: array, Index: index, Src: src}, maxTemp: max}, nil
	}
	if strings.HasPrefix(line, "set_field ") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "set_field "))
		eq := strings.Index(rest, "=")
		if eq < 0 {
			return lineParse{}, fmt.Errorf("invalid set_field syntax")
		}
		left := strings.TrimSpace(rest[:eq])
		right := strings.TrimSpace(rest[eq+1:])
		recv, field, err := parseFieldExprWithCtx(left, ctx)
		if err != nil {
			return lineParse{}, err
		}
		val, err := parseTempWithCtx(right, ctx)
		if err != nil {
			return lineParse{}, err
		}
		max := maxTempIdx(recv, val)
		return lineParse{instr: &SetField{Src: recv, Field: field, Value: val}, maxTemp: max}, nil
	}
	if strings.HasPrefix(line, "call ") {
		call, max, err := parseCallWithCtx(strings.TrimSpace(strings.TrimPrefix(line, "call ")), -1, ctx)
		if err != nil {
			return lineParse{}, err
		}
		return lineParse{instr: call, maxTemp: max}, nil
	}
	if strings.HasPrefix(line, "t") {
		eq := strings.Index(line, "=")
		if eq < 0 {
			return lineParse{}, fmt.Errorf("invalid assignment syntax")
		}
		left := strings.TrimSpace(line[:eq])
		right := strings.TrimSpace(line[eq+1:])
		// Check for t0.field = t1 (SetField)
		if dot := strings.Index(left, "."); dot > 0 {
			recv, err := parseTempWithCtx(left[:dot], ctx)
			if err != nil {
				return lineParse{}, err
			}
			field := strings.TrimSpace(left[dot+1:])
			val, err := parseTempWithCtx(right, ctx)
			if err != nil {
				return lineParse{}, err
			}
			max := maxTempIdx(recv, val)
			return lineParse{instr: &SetField{Src: recv, Field: field, Value: val}, maxTemp: max}, nil
		}
		dst, err := parseTempWithCtx(left, ctx)
		if err != nil {
			return lineParse{}, err
		}
		switch {
		case strings.HasPrefix(right, "const "):
			val, err := parseValue(strings.TrimSpace(strings.TrimPrefix(right, "const ")))
			if err != nil {
				return lineParse{}, err
			}
			return lineParse{instr: &Const{Dst: dst, Value: val}, maxTemp: dst}, nil
		case strings.HasPrefix(right, "load_addr "):
			name := strings.TrimSpace(strings.TrimPrefix(right, "load_addr "))
			return lineParse{instr: &LoadVar{Dst: dst, Name: name, Addr: true}, maxTemp: dst}, nil
		case strings.HasPrefix(right, "load "):
			name := strings.TrimSpace(strings.TrimPrefix(right, "load "))
			return lineParse{instr: &LoadVar{Dst: dst, Name: name}, maxTemp: dst}, nil
		case strings.HasPrefix(right, "load_ref "):
			src, err := parseTempWithCtx(strings.TrimSpace(strings.TrimPrefix(right, "load_ref ")), ctx)
			if err != nil {
				return lineParse{}, err
			}
			return lineParse{instr: &LoadRef{Dst: dst, Src: src}, maxTemp: maxTempIdx(dst, src)}, nil
		case strings.HasPrefix(right, "call "):
			call, max, err := parseCallWithCtx(strings.TrimSpace(strings.TrimPrefix(right, "call ")), dst, ctx)
			if err != nil {
				return lineParse{}, err
			}
			return lineParse{instr: call, maxTemp: max}, nil
		case strings.HasPrefix(right, "array "):
			rest := strings.TrimSpace(strings.TrimPrefix(right, "array "))
			open := strings.Index(rest, "[")
			close := strings.Index(rest, "]")
			if open < 0 || close < 0 || close < open {
				return lineParse{}, fmt.Errorf("invalid array syntax")
			}
			inside := strings.TrimSpace(rest[open+1 : close])
			elems := []int{}
			max := dst
			if inside != "" {
				for _, part := range splitComma(inside, -1) {
					t, err := parseTempWithCtx(part, ctx)
					if err != nil {
						return lineParse{}, err
					}
					elems = append(elems, t)
					if t > max {
						max = t
					}
				}
			}
			return lineParse{instr: &MakeArray{Dst: dst, Elems: elems}, maxTemp: max}, nil
		case strings.HasPrefix(right, "index_unchecked "):
			rest := strings.TrimSpace(strings.TrimPrefix(right, "index_unchecked "))
			array, index, err := parseIndexExprWithCtx(rest, ctx)
			if err != nil {
				return lineParse{}, err
			}
			max := maxTempIdx(dst, array, index)
			return lineParse{instr: &Index{Unchecked: true, Dst: dst, Array: array, Index: index}, maxTemp: max}, nil
		case strings.HasPrefix(right, "index "):
			rest := strings.TrimSpace(strings.TrimPrefix(right, "index "))
			unchecked := false
			if strings.HasSuffix(rest, "@unchecked") {
				unchecked = true
				rest = strings.TrimSpace(rest[:len(rest)-10])
			}
			array, index, err := parseIndexExprWithCtx(rest, ctx)
			if err != nil {
				return lineParse{}, err
			}
			max := maxTempIdx(dst, array, index)
			return lineParse{instr: &Index{Unchecked: unchecked, Dst: dst, Array: array, Index: index}, maxTemp: max}, nil
		case strings.HasPrefix(right, "struct "):
			name, fields, max, err := parseStructInitWithCtx(strings.TrimSpace(strings.TrimPrefix(right, "struct ")), dst, ctx)
			if err != nil {
				return lineParse{}, err
			}
			return lineParse{instr: &MakeStruct{Dst: dst, Name: name, Fields: fields}, maxTemp: max}, nil
		case strings.HasPrefix(right, "get_field "):
			rest := strings.TrimSpace(strings.TrimPrefix(right, "get_field "))
			src, field, err := parseFieldExprWithCtx(rest, ctx)
			if err != nil {
				return lineParse{}, err
			}
			return lineParse{instr: &GetField{Dst: dst, Src: src, Field: field}, maxTemp: maxTempIdx(dst, src)}, nil
		case isFieldAccessExpr(right, ctx):
			// Field access: t0.field or param.field
			src, field, err := parseFieldExprWithCtx(right, ctx)
			if err != nil {
				return lineParse{}, err
			}
			return lineParse{instr: &GetField{Dst: dst, Src: src, Field: field}, maxTemp: maxTempIdx(dst, src)}, nil
		case right == "unit", right == "true", right == "false",
			strings.HasPrefix(right, "\""), strings.HasPrefix(right, "i8 "),
			strings.HasPrefix(right, "i16 "), strings.HasPrefix(right, "i32 "),
			strings.HasPrefix(right, "i64 "), strings.HasPrefix(right, "u8 "),
			strings.HasPrefix(right, "u16 "), strings.HasPrefix(right, "u32 "),
			strings.HasPrefix(right, "u64 "), isIntLiteral(right):
			// Bare constant value without "const" prefix
			val, err := parseValue(right)
			if err != nil {
				return lineParse{}, err
			}
			return lineParse{instr: &Const{Dst: dst, Value: val}, maxTemp: dst}, nil
		default:
			op, rest, ok := splitOp(right)
			if !ok {
				return lineParse{}, fmt.Errorf("invalid op syntax")
			}
			parts := splitComma(rest, -1)
			if len(parts) == 1 {
				srcOp, srcMax, err := parseOperandWithCtx(parts[0], ctx)
				if err != nil {
					return lineParse{}, err
				}
				return lineParse{instr: &UnaryOp{Dst: dst, Op: op, Src: srcOp}, maxTemp: maxTempIdx(dst, srcMax)}, nil
			}
			if len(parts) == 2 {
				lhsOp, lhsMax, err := parseOperandWithCtx(parts[0], ctx)
				if err != nil {
					return lineParse{}, err
				}
				rhsOp, rhsMax, err := parseOperandWithCtx(parts[1], ctx)
				if err != nil {
					return lineParse{}, err
				}
				return lineParse{instr: &BinOp{Dst: dst, Op: op, Lhs: lhsOp, Rhs: rhsOp}, maxTemp: maxTempIdx(dst, lhsMax, rhsMax)}, nil
			}
			return lineParse{}, fmt.Errorf("invalid op syntax")
		}
	}
	// Check for param.field = temp (SetField with parameter name)
	if eq := strings.Index(line, "="); eq > 0 {
		left := strings.TrimSpace(line[:eq])
		right := strings.TrimSpace(line[eq+1:])
		if dot := strings.Index(left, "."); dot > 0 {
			recv, err := parseTempWithCtx(left[:dot], ctx)
			if err == nil {
				field := strings.TrimSpace(left[dot+1:])
				val, err := parseTempWithCtx(right, ctx)
				if err == nil {
					max := maxTempIdx(recv, val)
					return lineParse{instr: &SetField{Src: recv, Field: field, Value: val}, maxTemp: max}, nil
				}
			}
		}
	}
	return lineParse{}, fmt.Errorf("unknown instruction")
}

func parseFnHeader(line string) (string, []Var, string, error) {
	retType := ""
	header := strings.TrimSpace(line)
	if arrow := strings.Index(header, "->"); arrow >= 0 {
		retType = strings.TrimSpace(header[arrow+2:])
		header = strings.TrimSpace(header[:arrow])
	}
	open := strings.Index(header, "(")
	close := strings.LastIndex(header, ")")
	if open < 0 || close < 0 || close < open {
		return "", nil, "", fmt.Errorf("invalid fn syntax")
	}
	name := strings.TrimSpace(header[3:open])
	paramsText := strings.TrimSpace(header[open+1 : close])
	params := []Var{}
	if paramsText != "" {
		for _, part := range splitComma(paramsText, -1) {
			if strings.Contains(part, ":") {
				chunks := strings.SplitN(part, ":", 2)
				pname := strings.TrimSpace(chunks[0])
				ptype := ""
				if len(chunks) > 1 {
					ptype = strings.TrimSpace(chunks[1])
				}
				params = append(params, Var{Name: pname, Type: ptype})
			} else {
				params = append(params, Var{Name: part, Type: ""})
			}
		}
	}
	return name, params, retType, nil
}

func parseBlockLabel(line string) (string, error) {
	if !strings.HasSuffix(line, ":") {
		return "", fmt.Errorf("invalid block syntax")
	}
	label := strings.TrimSpace(line[len("block ") : len(line)-1])
	if label == "" {
		return "", fmt.Errorf("invalid block syntax")
	}
	return label, nil
}

func parseTemp(s string) (int, error) {
	return parseTempWithCtx(s, nil)
}

func parseTempWithCtx(s string, ctx *parseContext) (int, error) {
	s = strings.TrimSpace(s)
	// Strip type annotation if present (e.g., "t0: String" -> "t0")
	if colon := strings.Index(s, ":"); colon > 0 {
		s = strings.TrimSpace(s[:colon])
	}
	// Check if it's a parameter name
	if ctx != nil {
		if idx, ok := ctx.paramToTemp[s]; ok {
			return idx, nil
		}
	}
	if !strings.HasPrefix(s, "t") {
		return 0, fmt.Errorf("invalid temp")
	}
	n, err := strconv.Atoi(strings.TrimPrefix(s, "t"))
	if err != nil {
		return 0, fmt.Errorf("invalid temp")
	}
	// Body temps in formatted IR start at 0, but need to be offset by paramCount
	if ctx != nil {
		n += ctx.paramCount
	}
	return n, nil
}

// parseOperandWithCtx parses an operand which can be either a temp or a constant value
// Returns (operand, maxTemp, error) where maxTemp is -1 for constants
func parseOperandWithCtx(s string, ctx *parseContext) (Operand, int, error) {
	s = strings.TrimSpace(s)
	// First try to parse as a temp
	temp, err := parseTempWithCtx(s, ctx)
	if err == nil {
		return TempOperand(temp), temp, nil
	}
	// Try to parse as a constant value
	val, err := parseValue(s)
	if err == nil {
		return Operand{IsConst: true, Const: val}, -1, nil
	}
	return Operand{}, 0, fmt.Errorf("invalid operand: %s", s)
}

// isFieldAccessExpr checks if s looks like a field access expression (e.g., "t0.field" or "param.field")
func isFieldAccessExpr(s string, ctx *parseContext) bool {
	dot := strings.Index(s, ".")
	if dot < 0 {
		return false
	}
	before := strings.TrimSpace(s[:dot])
	after := strings.TrimSpace(s[dot+1:])
	if after == "" {
		return false
	}
	// Check if before is a valid temp (t0, t1...) or a parameter name
	if strings.HasPrefix(before, "t") {
		rest := strings.TrimPrefix(before, "t")
		if _, err := strconv.Atoi(rest); err == nil {
			return true
		}
	}
	// Check if it's a parameter name
	if ctx != nil {
		if _, ok := ctx.paramToTemp[before]; ok {
			return true
		}
	}
	return false
}

func isIntLiteral(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	// Allow negative numbers
	if s[0] == '-' {
		s = s[1:]
	}
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func parseValue(s string) (Value, error) {
	s = strings.TrimSpace(s)
	switch s {
	case "unit":
		return Value{Kind: KindUnit}, nil
	case "true":
		return Value{Kind: KindBool, Bool: true}, nil
	case "false":
		return Value{Kind: KindBool, Bool: false}, nil
	}
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") && len(s) >= 2 {
		str, err := unescapeString(s[1 : len(s)-1])
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: KindString, Str: str}, nil
	}
	if strings.HasPrefix(s, "&") {
		n, err := strconv.Atoi(strings.TrimPrefix(s, "&"))
		if err != nil {
			return Value{}, fmt.Errorf("invalid ref")
		}
		return Value{Kind: KindRef, Ref: n}, nil
	}
	if strings.HasPrefix(s, "struct#") {
		n, _ := strconv.Atoi(strings.TrimPrefix(s, "struct#"))
		return Value{Kind: KindStruct, Struct: &StructValue{Name: fmt.Sprintf("struct#%d", n), Fields: map[string]Value{}}}, nil
	}
	if strings.HasPrefix(s, "enum#") {
		n, _ := strconv.Atoi(strings.TrimPrefix(s, "enum#"))
		return Value{Kind: KindEnum, Enum: &EnumValue{Name: fmt.Sprintf("enum#%d", n), Variant: ""}}, nil
	}
	if strings.HasPrefix(s, "array#") {
		n, _ := strconv.Atoi(strings.TrimPrefix(s, "array#"))
		return Value{Kind: KindArray, Array: &ArrayValue{Elems: make([]Value, 0, n)}}, nil
	}
	if name, rest, ok := parseIntTypePrefix(s); ok {
		n, err := strconv.ParseInt(rest, 10, 64)
		if err != nil {
			return Value{}, fmt.Errorf("invalid const value")
		}
		return Value{Kind: KindInt, Int: n, IntType: name}, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return Value{}, fmt.Errorf("invalid const value")
	}
	return Value{Kind: KindInt, Int: n}, nil
}

func unescapeString(s string) (string, error) {
	var out []rune
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch != '\\' {
			out = append(out, rune(ch))
			continue
		}
		if i+1 >= len(s) {
			return "", fmt.Errorf("invalid escape")
		}
		n := s[i+1]
		switch n {
		case 'n':
			out = append(out, '\n')
		case 't':
			out = append(out, '\t')
		case 'r':
			out = append(out, '\r')
		case '\\':
			out = append(out, '\\')
		case '"':
			out = append(out, '"')
		default:
			out = append(out, rune(n))
		}
		i++
	}
	return string(out), nil
}

func parseIndexExpr(s string) (int, int, error) {
	return parseIndexExprWithCtx(s, nil)
}

func parseIndexExprWithCtx(s string, ctx *parseContext) (int, int, error) {
	open := strings.Index(s, "[")
	close := strings.Index(s, "]")
	if open < 0 || close < 0 || close < open {
		return 0, 0, fmt.Errorf("invalid index syntax")
	}
	array, err := parseTempWithCtx(s[:open], ctx)
	if err != nil {
		return 0, 0, err
	}
	index, err := parseTempWithCtx(s[open+1:close], ctx)
	if err != nil {
		return 0, 0, err
	}
	return array, index, nil
}

func parseFieldExpr(s string) (int, string, error) {
	return parseFieldExprWithCtx(s, nil)
}

func parseFieldExprWithCtx(s string, ctx *parseContext) (int, string, error) {
	dot := strings.Index(s, ".")
	if dot < 0 {
		return 0, "", fmt.Errorf("invalid field syntax")
	}
	recv, err := parseTempWithCtx(s[:dot], ctx)
	if err != nil {
		return 0, "", err
	}
	field := strings.TrimSpace(s[dot+1:])
	if field == "" {
		return 0, "", fmt.Errorf("invalid field syntax")
	}
	return recv, field, nil
}

func parseStructInit(s string, dst int) (string, []StructFieldInit, int, error) {
	return parseStructInitWithCtx(s, dst, nil)
}

func parseStructInitWithCtx(s string, dst int, ctx *parseContext) (string, []StructFieldInit, int, error) {
	open := strings.Index(s, "{")
	close := strings.LastIndex(s, "}")
	if open < 0 || close < 0 || close < open {
		return "", nil, 0, fmt.Errorf("invalid struct syntax")
	}
	name := strings.TrimSpace(s[:open])
	fieldsText := strings.TrimSpace(s[open+1 : close])
	fields := []StructFieldInit{}
	max := dst
	if fieldsText != "" {
		for _, part := range splitComma(fieldsText, -1) {
			colon := strings.Index(part, ":")
			if colon < 0 {
				return "", nil, 0, fmt.Errorf("invalid struct field")
			}
			fname := strings.TrimSpace(part[:colon])
			fval := strings.TrimSpace(part[colon+1:])
			fsrc, err := parseTempWithCtx(fval, ctx)
			if err != nil {
				return "", nil, 0, err
			}
			fields = append(fields, StructFieldInit{Name: fname, Src: fsrc})
			if fsrc > max {
				max = fsrc
			}
		}
	}
	return name, fields, max, nil
}

func parseIntTypePrefix(s string) (string, string, bool) {
	parts := strings.SplitN(s, " ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	name := strings.TrimSpace(parts[0])
	rest := strings.TrimSpace(parts[1])
	if !isIntTypeName(name) {
		return "", "", false
	}
	if rest == "" {
		return "", "", false
	}
	return name, rest, true
}

func isIntTypeName(name string) bool {
	switch name {
	case "i8", "i16", "i32", "i64", "i128",
		"u8", "u16", "u32", "u64", "u128",
		"isize", "usize", "char":
		return true
	default:
		return false
	}
}

func parseCall(text string, dst int) (*Call, int, error) {
	return parseCallWithCtx(text, dst, nil)
}

func parseCallWithCtx(text string, dst int, ctx *parseContext) (*Call, int, error) {
	open := strings.Index(text, "(")
	close := strings.LastIndex(text, ")")
	if open < 0 || close < 0 || close < open {
		return nil, 0, fmt.Errorf("invalid call syntax")
	}
	callee := strings.TrimSpace(text[:open])
	argsText := strings.TrimSpace(text[open+1 : close])
	args := []int{}
	max := dst
	if argsText != "" {
		for _, part := range splitComma(argsText, -1) {
			t, err := parseTempWithCtx(part, ctx)
			if err != nil {
				return nil, 0, err
			}
			args = append(args, t)
			if t > max {
				max = t
			}
		}
	}
	return &Call{Dst: dst, Callee: callee, Args: args}, max, nil
}

func splitOp(s string) (string, string, bool) {
	idx := strings.Index(s, " ")
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+1:]), true
}

func splitComma(s string, limit int) []string {
	out := make([]string, 0)
	current := ""
	inString := false
	depth := 0 // for nested braces/brackets
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' && (i == 0 || s[i-1] != '\\') {
			inString = !inString
		}
		if !inString {
			if ch == '{' || ch == '[' || ch == '(' {
				depth++
			} else if ch == '}' || ch == ']' || ch == ')' {
				depth--
			} else if ch == ',' && depth == 0 {
				part := strings.TrimSpace(current)
				if part != "" {
					out = append(out, part)
					if limit > 0 && len(out) >= limit {
						return out
					}
				}
				current = ""
				continue
			}
		}
		current += string(ch)
	}
	part := strings.TrimSpace(current)
	if part != "" {
		out = append(out, part)
	}
	return out
}

func maxTempIdx(vals ...int) int {
	max := -1
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	return max
}
