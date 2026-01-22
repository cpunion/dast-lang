package ir

import "fmt"

func Optimize(p *Program) *Program {
	inline := collectInlineCandidates(p)
	for _, fn := range p.Functions {
		if fn == nil {
			continue
		}
		optimizeFunction(fn, inline)
	}
	return p
}

type inlineCandidate struct {
	fn      *Function
	retOp   *Operand
}

func optimizeFunction(fn *Function, inline map[string]inlineCandidate) {
	if len(fn.Blocks) == 0 {
		return
	}
	inlineCalls(fn, inline)
	for _, blk := range fn.Blocks {
		optimizeBlock(blk)
	}
	pruneUnreachable(fn)
}

func optimizeBlock(blk *Block) {
	consts := map[int]Value{}
	varConsts := map[string]Value{}
	arrayLens, arraySafe := scanArraySafety(blk)

	// Helper to get constant value from operand
	getOperandConst := func(op Operand) (Value, bool) {
		if op.IsConst {
			return op.Const, true
		}
		val, ok := consts[op.Temp]
		return val, ok
	}
	getOperandTemp := func(op Operand) (int, bool) {
		if op.IsConst {
			return 0, false
		}
		return op.Temp, true
	}

	for _, inst := range blk.Instr {
		switch v := inst.(type) {
		case *LoadVar:
			if v.Ref {
				delete(consts, v.Dst)
				continue
			}
			if v.Addr {
				delete(consts, v.Dst)
				delete(varConsts, v.Name)
				continue
			}
			if val, ok := varConsts[v.Name]; ok {
				consts[v.Dst] = val
				continue
			}
			delete(consts, v.Dst)
		case *BinOp:
			if l, ok := getOperandConst(v.Lhs); ok {
				if r, ok := getOperandConst(v.Rhs); ok {
					if folded, ok := foldBinary(v.Op, l, r); ok {
						consts[v.Dst] = folded
						continue
					}
				}
			}
			delete(consts, v.Dst)
		case *Call:
			if v.Dst >= 0 {
				delete(consts, v.Dst)
			}
			for name := range varConsts {
				delete(varConsts, name)
			}
		case *MakeArray:
			delete(consts, v.Dst)
		case *Index:
			if idx, ok := getOperandConst(v.Index); ok && idx.Kind == KindInt {
				if arrTemp, ok := getOperandTemp(v.Array); ok {
					if length, ok := arrayLens[arrTemp]; ok && arraySafe[arrTemp] {
						if idx.Int >= 0 && idx.Int < int64(length) {
							v.Unchecked = true
						}
					}
				}
			}
			delete(consts, v.Dst)
		case *MakeStruct:
			delete(consts, v.Dst)
		case *GetField:
			delete(consts, v.Dst)
		case *SetIndex:
			if idx, ok := getOperandConst(v.Index); ok && idx.Kind == KindInt {
				if arrTemp, ok := getOperandTemp(v.Array); ok {
					if length, ok := arrayLens[arrTemp]; ok && arraySafe[arrTemp] {
						if idx.Int >= 0 && idx.Int < int64(length) {
							v.Unchecked = true
						}
					}
				}
			}
		}
		if sv, ok := inst.(*StoreVar); ok {
			if sv.Ref {
				for name := range varConsts {
					delete(varConsts, name)
				}
			} else if val, ok := getOperandConst(sv.Src); ok {
				varConsts[sv.Name] = val
			} else {
				delete(varConsts, sv.Name)
			}
		}
	}
	if br, ok := blk.Term.(*Branch); ok {
		if cond, ok := getOperandConst(br.Cond); ok && cond.Kind == KindBool {
			target := br.Then
			if !cond.Bool {
				target = br.Else
			}
			blk.Term = &Jump{Target: target}
		}
	}
}

func foldBinary(op string, a Value, b Value) (Value, bool) {
	switch op {
	case "+":
		if a.Kind == KindInt && b.Kind == KindInt {
			return Value{Kind: KindInt, Int: a.Int + b.Int, IntType: mergeIntType(a, b)}, true
		}
		if a.Kind == KindString && b.Kind == KindString {
			return Value{Kind: KindString, Str: a.Str + b.Str}, true
		}
	case "-":
		if a.Kind == KindInt && b.Kind == KindInt {
			return Value{Kind: KindInt, Int: a.Int - b.Int, IntType: mergeIntType(a, b)}, true
		}
	case "*":
		if a.Kind == KindInt && b.Kind == KindInt {
			return Value{Kind: KindInt, Int: a.Int * b.Int, IntType: mergeIntType(a, b)}, true
		}
	case "/":
		if a.Kind == KindInt && b.Kind == KindInt && b.Int != 0 {
			return Value{Kind: KindInt, Int: a.Int / b.Int, IntType: mergeIntType(a, b)}, true
		}
	case "%":
		if a.Kind == KindInt && b.Kind == KindInt && b.Int != 0 {
			return Value{Kind: KindInt, Int: a.Int % b.Int, IntType: mergeIntType(a, b)}, true
		}
	case "==":
		if a.Kind == KindInt && b.Kind == KindInt {
			return Value{Kind: KindBool, Bool: a.Int == b.Int}, true
		}
		if a.Kind == KindBool && b.Kind == KindBool {
			return Value{Kind: KindBool, Bool: a.Bool == b.Bool}, true
		}
		if a.Kind == KindString && b.Kind == KindString {
			return Value{Kind: KindBool, Bool: a.Str == b.Str}, true
		}
	case "!=":
		if a.Kind == KindInt && b.Kind == KindInt {
			return Value{Kind: KindBool, Bool: a.Int != b.Int}, true
		}
		if a.Kind == KindBool && b.Kind == KindBool {
			return Value{Kind: KindBool, Bool: a.Bool != b.Bool}, true
		}
		if a.Kind == KindString && b.Kind == KindString {
			return Value{Kind: KindBool, Bool: a.Str != b.Str}, true
		}
	case "<":
		if a.Kind == KindInt && b.Kind == KindInt {
			return Value{Kind: KindBool, Bool: a.Int < b.Int}, true
		}
	case "<=":
		if a.Kind == KindInt && b.Kind == KindInt {
			return Value{Kind: KindBool, Bool: a.Int <= b.Int}, true
		}
	case ">":
		if a.Kind == KindInt && b.Kind == KindInt {
			return Value{Kind: KindBool, Bool: a.Int > b.Int}, true
		}
	case ">=":
		if a.Kind == KindInt && b.Kind == KindInt {
			return Value{Kind: KindBool, Bool: a.Int >= b.Int}, true
		}
	case "&&":
		if a.Kind == KindBool && b.Kind == KindBool {
			return Value{Kind: KindBool, Bool: a.Bool && b.Bool}, true
		}
	case "||":
		if a.Kind == KindBool && b.Kind == KindBool {
			return Value{Kind: KindBool, Bool: a.Bool || b.Bool}, true
		}
	}
	return Value{}, false
}

func mergeIntType(a Value, b Value) string {
	if a.IntType != "" {
		return a.IntType
	}
	if b.IntType != "" {
		return b.IntType
	}
	return ""
}

func scanArraySafety(blk *Block) (map[int]int, map[int]bool) {
	lengths := map[int]int{}
	safe := map[int]bool{}
	for _, inst := range blk.Instr {
		if mk, ok := inst.(*MakeArray); ok {
			lengths[mk.Dst] = len(mk.Elems)
			safe[mk.Dst] = true
		}
	}
	for _, inst := range blk.Instr {
		switch inst.(type) {
		case *MakeArray:
		case *Index, *SetIndex:
		default:
			for _, temp := range instrTemps(inst) {
				if _, ok := safe[temp]; ok {
					safe[temp] = false
				}
			}
		}
	}
	return lengths, safe
}

func instrTemps(inst Instr) []int {
	// Helper to extract temp from operand
	getTemp := func(op Operand) []int {
		if op.IsConst {
			return nil
		}
		return []int{op.Temp}
	}

	switch v := inst.(type) {
	case *LoadVar:
		if v.Ref {
			return []int{v.Dst, v.RefTemp}
		}
		return []int{v.Dst}
	case *StoreVar:
		if v.Ref {
			out := []int{v.RefTemp}
			return append(out, getTemp(v.Src)...)
		}
		return getTemp(v.Src)
	case *BinOp:
		out := []int{v.Dst}
		out = append(out, getTemp(v.Lhs)...)
		out = append(out, getTemp(v.Rhs)...)
		return out
	case *Call:
		out := []int{}
		if v.Dst >= 0 {
			out = append(out, v.Dst)
		}
		for _, a := range v.Args {
			out = append(out, getTemp(a)...)
		}
		return out
	case *MakeArray:
		out := []int{v.Dst}
		for _, e := range v.Elems {
			out = append(out, getTemp(e)...)
		}
		return out
	case *Index:
		out := []int{v.Dst}
		out = append(out, getTemp(v.Array)...)
		out = append(out, getTemp(v.Index)...)
		return out
	case *SetIndex:
		out := []int{}
		out = append(out, getTemp(v.Array)...)
		out = append(out, getTemp(v.Index)...)
		out = append(out, getTemp(v.Src)...)
		return out
	case *MakeStruct:
		out := []int{v.Dst}
		for _, f := range v.Fields {
			out = append(out, getTemp(f.Src)...)
		}
		return out
	case *GetField:
		return []int{v.Dst, v.Src}
	case *SetField:
		out := []int{v.Src}
		return append(out, getTemp(v.Value)...)
	default:
		return nil
	}
}

func collectInlineCandidates(p *Program) map[string]inlineCandidate {
	out := map[string]inlineCandidate{}
	for name, fn := range p.Functions {
		if fn == nil || len(fn.Blocks) != 1 {
			continue
		}
		blk := fn.Blocks[0]
		ret, ok := blk.Term.(*Return)
		if !ok {
			continue
		}
		if containsCall(blk.Instr) {
			continue
		}
		var retOp *Operand
		if ret.Value != nil {
			tmp := *ret.Value
			retOp = &tmp
		}
		out[name] = inlineCandidate{fn: fn, retOp: retOp}
	}
	return out
}

func containsCall(instrs []Instr) bool {
	for _, inst := range instrs {
		if _, ok := inst.(*Call); ok {
			return true
		}
	}
	return false
}

func inlineCalls(fn *Function, inline map[string]inlineCandidate) {
	nextTemp := fn.TempCount
	inlineID := 0
	for _, blk := range fn.Blocks {
		out := make([]Instr, 0, len(blk.Instr))
		for _, inst := range blk.Instr {
			call, ok := inst.(*Call)
			if !ok {
				out = append(out, inst)
				continue
			}
			cand, ok := inline[call.Callee]
			if !ok || cand.fn == nil || call.Callee == fn.Name {
				out = append(out, inst)
				continue
			}
			if call.Dst >= 0 {
				if cand.retOp == nil || cand.retOp.IsConst {
					out = append(out, inst)
					continue
				}
			}
			inlined := inlineCall(call, cand, &nextTemp, inlineID)
			inlineID++
			out = append(out, inlined...)
		}
		blk.Instr = out
	}
	fn.TempCount = nextTemp
}

func inlineCall(call *Call, cand inlineCandidate, nextTemp *int, inlineID int) []Instr {
	blk := cand.fn.Blocks[0]
	tempMap := map[int]int{}
	retTemp := -1
	if cand.retOp != nil && !cand.retOp.IsConst {
		retTemp = cand.retOp.Temp
	}
	for t := 0; t < cand.fn.TempCount; t++ {
		if retTemp >= 0 && call.Dst >= 0 && t == retTemp {
			tempMap[t] = call.Dst
			continue
		}
		tempMap[t] = *nextTemp
		*nextTemp++
	}
	varMap := map[string]string{}
	prefix := fmt.Sprintf("_inl%d_", inlineID)
	mapVar := func(name string) string {
		if mapped, ok := varMap[name]; ok {
			return mapped
		}
		mapped := prefix + name
		varMap[name] = mapped
		return mapped
	}
	out := []Instr{}
	for i, param := range cand.fn.Params {
		if i < len(call.Args) {
			out = append(out, &StoreVar{Name: mapVar(param.Name), Src: call.Args[i]})
		}
	}
	for _, inst := range blk.Instr {
		out = append(out, remapInstr(inst, tempMap, mapVar))
	}
	return out
}

func remapInstr(inst Instr, tempMap map[int]int, mapVar func(string) string) Instr {
	remap := func(t int) int {
		if v, ok := tempMap[t]; ok {
			return v
		}
		return t
	}
	remapOperand := func(op Operand) Operand {
		if op.IsConst {
			return op
		}
		return TempOperand(remap(op.Temp))
	}
	switch v := inst.(type) {
	case *LoadVar:
		remapped := &LoadVar{Dst: remap(v.Dst), Name: mapVar(v.Name), Addr: v.Addr, Ref: v.Ref, RefTemp: v.RefTemp}
		if v.Ref {
			remapped.RefTemp = remap(v.RefTemp)
		}
		return remapped
	case *StoreVar:
		remapped := &StoreVar{Name: mapVar(v.Name), Src: remapOperand(v.Src), Ref: v.Ref, RefTemp: v.RefTemp}
		if v.Ref {
			remapped.RefTemp = remap(v.RefTemp)
		}
		return remapped
	case *BinOp:
		return &BinOp{Dst: remap(v.Dst), Op: v.Op, Lhs: remapOperand(v.Lhs), Rhs: remapOperand(v.Rhs)}
	case *Call:
		args := make([]Operand, 0, len(v.Args))
		for _, a := range v.Args {
			args = append(args, remapOperand(a))
		}
		return &Call{Dst: remap(v.Dst), Callee: v.Callee, Args: args}
	case *MakeArray:
		elems := make([]Operand, 0, len(v.Elems))
		for _, e := range v.Elems {
			elems = append(elems, remapOperand(e))
		}
		return &MakeArray{Dst: remap(v.Dst), Elems: elems}
	case *Index:
		return &Index{Dst: remap(v.Dst), Array: remapOperand(v.Array), Index: remapOperand(v.Index)}
	case *SetIndex:
		return &SetIndex{Array: remapOperand(v.Array), Index: remapOperand(v.Index), Src: remapOperand(v.Src)}

	case *MakeStruct:
		fields := make([]StructFieldInit, 0, len(v.Fields))
		for _, f := range v.Fields {
			fields = append(fields, StructFieldInit{Name: f.Name, Src: remapOperand(f.Src)})
		}
		return &MakeStruct{Dst: remap(v.Dst), Name: v.Name, Fields: fields}
	case *GetField:
		return &GetField{Dst: remap(v.Dst), Src: remap(v.Src), Field: v.Field}
	case *SetField:
		return &SetField{Src: remap(v.Src), Field: v.Field, Value: remapOperand(v.Value)}
	default:
		return inst
	}
}

func pruneUnreachable(fn *Function) {
	if len(fn.Blocks) == 0 {
		return
	}
	blocks := map[string]*Block{}
	for _, blk := range fn.Blocks {
		blocks[blk.Label] = blk
	}
	reachable := map[string]struct{}{}
	work := []string{fn.Blocks[0].Label}
	for len(work) > 0 {
		label := work[len(work)-1]
		work = work[:len(work)-1]
		if _, ok := reachable[label]; ok {
			continue
		}
		reachable[label] = struct{}{}
		blk := blocks[label]
		if blk == nil || blk.Term == nil {
			continue
		}
		switch t := blk.Term.(type) {
		case *Jump:
			work = append(work, t.Target)
		case *Branch:
			work = append(work, t.Then)
			work = append(work, t.Else)
		}
	}
	out := make([]*Block, 0, len(fn.Blocks))
	for _, blk := range fn.Blocks {
		if _, ok := reachable[blk.Label]; ok {
			out = append(out, blk)
		}
	}
	fn.Blocks = out
}
