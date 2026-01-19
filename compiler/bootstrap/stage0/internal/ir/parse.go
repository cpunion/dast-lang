package ir

import (
	"fmt"
	"strconv"
	"strings"
)

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
	maxTemp := -1
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			if curBlk != nil {
				curFn.Blocks = append(curFn.Blocks, curBlk)
				curBlk = nil
			}
			if curFn != nil {
				if maxTemp < 0 {
					curFn.TempCount = 0
				} else {
					curFn.TempCount = maxTemp + 1
				}
				prog.Functions[curFn.Name] = curFn
				curFn = nil
			}
			i++
			continue
		}
		if strings.HasPrefix(line, "fn ") {
			if curBlk != nil {
				curFn.Blocks = append(curFn.Blocks, curBlk)
				curBlk = nil
			}
			if curFn != nil {
				if maxTemp < 0 {
					curFn.TempCount = 0
				} else {
					curFn.TempCount = maxTemp + 1
				}
				prog.Functions[curFn.Name] = curFn
				curFn = nil
			}
			name, params, err := parseFnHeader(line)
			if err != nil {
				return nil, fmt.Errorf("ir parse error (line %d): %w", i+1, err)
			}
			curFn = &Function{Name: name, Params: params}
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
		lp, err := parseIRLine(line)
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
	if curBlk != nil && curFn != nil {
		curFn.Blocks = append(curFn.Blocks, curBlk)
	}
	if curFn != nil {
		if maxTemp < 0 {
			curFn.TempCount = 0
		} else {
			curFn.TempCount = maxTemp + 1
		}
		prog.Functions[curFn.Name] = curFn
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
		cond, err := parseTemp(parts[0])
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
		val, err := parseTemp(rest)
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
		ref, err := parseTemp(parts[0])
		if err != nil {
			return lineParse{}, err
		}
		src, err := parseTemp(parts[1])
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
		src, err := parseTemp(parts[1])
		if err != nil {
			return lineParse{}, err
		}
		return lineParse{instr: &StoreVar{Name: parts[0], Src: src}, maxTemp: src}, nil
	}
	if strings.HasPrefix(line, "set_index ") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "set_index "))
		eq := strings.Index(rest, "=")
		if eq < 0 {
			return lineParse{}, fmt.Errorf("invalid set_index syntax")
		}
		left := strings.TrimSpace(rest[:eq])
		right := strings.TrimSpace(rest[eq+1:])
		array, index, err := parseIndexExpr(left)
		if err != nil {
			return lineParse{}, err
		}
		src, err := parseTemp(right)
		if err != nil {
			return lineParse{}, err
		}
		max := maxTempIdx(array, index, src)
		return lineParse{instr: &SetIndex{Array: array, Index: index, Src: src}, maxTemp: max}, nil
	}
	if strings.HasPrefix(line, "set_field ") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "set_field "))
		eq := strings.Index(rest, "=")
		if eq < 0 {
			return lineParse{}, fmt.Errorf("invalid set_field syntax")
		}
		left := strings.TrimSpace(rest[:eq])
		right := strings.TrimSpace(rest[eq+1:])
		recv, field, err := parseFieldExpr(left)
		if err != nil {
			return lineParse{}, err
		}
		val, err := parseTemp(right)
		if err != nil {
			return lineParse{}, err
		}
		max := maxTempIdx(recv, val)
		return lineParse{instr: &SetField{Src: recv, Field: field, Value: val}, maxTemp: max}, nil
	}
	if strings.HasPrefix(line, "call ") {
		call, max, err := parseCall(strings.TrimSpace(strings.TrimPrefix(line, "call ")), -1)
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
		dst, err := parseTemp(left)
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
		case strings.HasPrefix(right, "load "):
			name := strings.TrimSpace(strings.TrimPrefix(right, "load "))
			return lineParse{instr: &LoadVar{Dst: dst, Name: name}, maxTemp: dst}, nil
		case strings.HasPrefix(right, "addr_of "):
			name := strings.TrimSpace(strings.TrimPrefix(right, "addr_of "))
			return lineParse{instr: &AddrOf{Dst: dst, Name: name}, maxTemp: dst}, nil
		case strings.HasPrefix(right, "load_ref "):
			src, err := parseTemp(strings.TrimSpace(strings.TrimPrefix(right, "load_ref ")))
			if err != nil {
				return lineParse{}, err
			}
			return lineParse{instr: &LoadRef{Dst: dst, Src: src}, maxTemp: maxTempIdx(dst, src)}, nil
		case strings.HasPrefix(right, "call "):
			call, max, err := parseCall(strings.TrimSpace(strings.TrimPrefix(right, "call ")), dst)
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
					t, err := parseTemp(part)
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
		case strings.HasPrefix(right, "index "):
			rest := strings.TrimSpace(strings.TrimPrefix(right, "index "))
			array, index, err := parseIndexExpr(rest)
			if err != nil {
				return lineParse{}, err
			}
			max := maxTempIdx(dst, array, index)
			return lineParse{instr: &Index{Dst: dst, Array: array, Index: index}, maxTemp: max}, nil
		case strings.HasPrefix(right, "struct "):
			name, fields, max, err := parseStructInit(strings.TrimSpace(strings.TrimPrefix(right, "struct ")), dst)
			if err != nil {
				return lineParse{}, err
			}
			return lineParse{instr: &MakeStruct{Dst: dst, Name: name, Fields: fields}, maxTemp: max}, nil
		case strings.HasPrefix(right, "get_field "):
			rest := strings.TrimSpace(strings.TrimPrefix(right, "get_field "))
			src, field, err := parseFieldExpr(rest)
			if err != nil {
				return lineParse{}, err
			}
			return lineParse{instr: &GetField{Dst: dst, Src: src, Field: field}, maxTemp: maxTempIdx(dst, src)}, nil
		case strings.HasPrefix(right, "enum "):
			name, variant, payload, err := parseEnumInit(strings.TrimSpace(strings.TrimPrefix(right, "enum ")))
			if err != nil {
				return lineParse{}, err
			}
			max := maxTempIdx(dst, payload)
			return lineParse{instr: &MakeEnum{Dst: dst, Name: name, Variant: variant, Payload: payload}, maxTemp: max}, nil
		case strings.HasPrefix(right, "enum_tag "):
			src, err := parseTemp(strings.TrimSpace(strings.TrimPrefix(right, "enum_tag ")))
			if err != nil {
				return lineParse{}, err
			}
			return lineParse{instr: &EnumTag{Dst: dst, Src: src}, maxTemp: maxTempIdx(dst, src)}, nil
		case strings.HasPrefix(right, "enum_payload "):
			src, err := parseTemp(strings.TrimSpace(strings.TrimPrefix(right, "enum_payload ")))
			if err != nil {
				return lineParse{}, err
			}
			return lineParse{instr: &EnumPayload{Dst: dst, Src: src}, maxTemp: maxTempIdx(dst, src)}, nil
		default:
			op, rest, ok := splitOp(right)
			if !ok {
				return lineParse{}, fmt.Errorf("invalid op syntax")
			}
			parts := splitComma(rest, -1)
			if len(parts) == 1 {
				src, err := parseTemp(parts[0])
				if err != nil {
					return lineParse{}, err
				}
				return lineParse{instr: &UnaryOp{Dst: dst, Op: op, Src: src}, maxTemp: maxTempIdx(dst, src)}, nil
			}
			if len(parts) == 2 {
				lhs, err := parseTemp(parts[0])
				if err != nil {
					return lineParse{}, err
				}
				rhs, err := parseTemp(parts[1])
				if err != nil {
					return lineParse{}, err
				}
				return lineParse{instr: &BinOp{Dst: dst, Op: op, Lhs: lhs, Rhs: rhs}, maxTemp: maxTempIdx(dst, lhs, rhs)}, nil
			}
			return lineParse{}, fmt.Errorf("invalid op syntax")
		}
	}
	return lineParse{}, fmt.Errorf("unknown instruction")
}

func parseFnHeader(line string) (string, []string, error) {
	open := strings.Index(line, "(")
	close := strings.LastIndex(line, ")")
	if open < 0 || close < 0 || close < open {
		return "", nil, fmt.Errorf("invalid fn syntax")
	}
	name := strings.TrimSpace(line[3:open])
	paramsText := strings.TrimSpace(line[open+1 : close])
	params := []string{}
	if paramsText != "" {
		for _, part := range splitComma(paramsText, -1) {
			params = append(params, part)
		}
	}
	return name, params, nil
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
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "t") {
		return 0, fmt.Errorf("invalid temp")
	}
	n, err := strconv.Atoi(strings.TrimPrefix(s, "t"))
	if err != nil {
		return 0, fmt.Errorf("invalid temp")
	}
	return n, nil
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
		return Value{Kind: KindString, Str: s[1 : len(s)-1]}, nil
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
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return Value{}, fmt.Errorf("invalid const value")
	}
	return Value{Kind: KindInt, Int: n}, nil
}

func parseIndexExpr(s string) (int, int, error) {
	open := strings.Index(s, "[")
	close := strings.Index(s, "]")
	if open < 0 || close < 0 || close < open {
		return 0, 0, fmt.Errorf("invalid index syntax")
	}
	array, err := parseTemp(s[:open])
	if err != nil {
		return 0, 0, err
	}
	index, err := parseTemp(s[open+1 : close])
	if err != nil {
		return 0, 0, err
	}
	return array, index, nil
}

func parseFieldExpr(s string) (int, string, error) {
	dot := strings.Index(s, ".")
	if dot < 0 {
		return 0, "", fmt.Errorf("invalid field syntax")
	}
	recv, err := parseTemp(s[:dot])
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
			fsrc, err := parseTemp(part[colon+1:])
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

func parseEnumInit(s string) (string, string, int, error) {
	payload := -1
	nameVariant := s
	open := strings.Index(s, "(")
	if open >= 0 {
		close := strings.LastIndex(s, ")")
		if close < 0 {
			return "", "", 0, fmt.Errorf("invalid enum syntax")
		}
		nameVariant = strings.TrimSpace(s[:open])
		temp, err := parseTemp(s[open+1 : close])
		if err != nil {
			return "", "", 0, err
		}
		payload = temp
	}
	dot := strings.Index(nameVariant, ".")
	if dot < 0 {
		return "", "", 0, fmt.Errorf("invalid enum syntax")
	}
	name := strings.TrimSpace(nameVariant[:dot])
	variant := strings.TrimSpace(nameVariant[dot+1:])
	return name, variant, payload, nil
}

func parseCall(text string, dst int) (*Call, int, error) {
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
			t, err := parseTemp(part)
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
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
		if limit > 0 && len(out) >= limit {
			break
		}
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
