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
	retTemp *int
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
	for i, inst := range blk.Instr {
		switch v := inst.(type) {
		case *Const:
			consts[v.Dst] = v.Value
		case *LoadVar:
			if val, ok := varConsts[v.Name]; ok {
				blk.Instr[i] = &Const{Dst: v.Dst, Value: val}
				consts[v.Dst] = val
				continue
			}
			delete(consts, v.Dst)
		case *AddrOf:
			delete(varConsts, v.Name)
			delete(consts, v.Dst)
		case *LoadRef:
			delete(consts, v.Dst)
		case *UnaryOp:
			if c, ok := consts[v.Src]; ok {
				if folded, ok := foldUnary(v.Op, c); ok {
					blk.Instr[i] = &Const{Dst: v.Dst, Value: folded}
					consts[v.Dst] = folded
					continue
				}
			}
			delete(consts, v.Dst)
		case *BinOp:
			if l, ok := consts[v.Lhs]; ok {
				if r, ok := consts[v.Rhs]; ok {
					if folded, ok := foldBinary(v.Op, l, r); ok {
						blk.Instr[i] = &Const{Dst: v.Dst, Value: folded}
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
			if idx, ok := consts[v.Index]; ok && idx.Kind == KindInt {
				if length, ok := arrayLens[v.Array]; ok && arraySafe[v.Array] {
					if idx.Int >= 0 && idx.Int < int64(length) {
						blk.Instr[i] = &IndexUnchecked{Dst: v.Dst, Array: v.Array, Index: v.Index}
					}
				}
			}
			delete(consts, v.Dst)
		case *MakeStruct:
			delete(consts, v.Dst)
		case *GetField:
			delete(consts, v.Dst)
		case *MakeEnum:
			delete(consts, v.Dst)
		case *EnumTag:
			delete(consts, v.Dst)
		case *EnumPayload:
			delete(consts, v.Dst)
		case *SetIndex:
			if idx, ok := consts[v.Index]; ok && idx.Kind == KindInt {
				if length, ok := arrayLens[v.Array]; ok && arraySafe[v.Array] {
					if idx.Int >= 0 && idx.Int < int64(length) {
						blk.Instr[i] = &SetIndexUnchecked{Array: v.Array, Index: v.Index, Src: v.Src}
					}
				}
			}
		case *SetIndexUnchecked:
		case *IndexUnchecked:
		}
		if sv, ok := inst.(*StoreVar); ok {
			if val, ok := consts[sv.Src]; ok {
				varConsts[sv.Name] = val
			} else {
				delete(varConsts, sv.Name)
			}
		}
		if _, ok := inst.(*StoreRef); ok {
			for name := range varConsts {
				delete(varConsts, name)
			}
		}
	}
	if br, ok := blk.Term.(*Branch); ok {
		if cond, ok := consts[br.Cond]; ok && cond.Kind == KindBool {
			target := br.Then
			if !cond.Bool {
				target = br.Else
			}
			blk.Term = &Jump{Target: target}
		}
	}
}

func foldUnary(op string, v Value) (Value, bool) {
	switch op {
	case "-":
		if v.Kind == KindInt {
			return Value{Kind: KindInt, Int: -v.Int, IntType: v.IntType}, true
		}
	case "!":
		if v.Kind == KindBool {
			return Value{Kind: KindBool, Bool: !v.Bool}, true
		}
	}
	return Value{}, false
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
		case *Index, *IndexUnchecked, *SetIndex, *SetIndexUnchecked:
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
	switch v := inst.(type) {
	case *Const:
		return []int{v.Dst}
	case *LoadVar:
		return []int{v.Dst}
	case *StoreVar:
		return []int{v.Src}
	case *AddrOf:
		return []int{v.Dst}
	case *LoadRef:
		return []int{v.Dst, v.Src}
	case *StoreRef:
		return []int{v.Ref, v.Src}
	case *BinOp:
		return []int{v.Dst, v.Lhs, v.Rhs}
	case *UnaryOp:
		return []int{v.Dst, v.Src}
	case *Call:
		out := []int{}
		if v.Dst >= 0 {
			out = append(out, v.Dst)
		}
		out = append(out, v.Args...)
		return out
	case *MakeArray:
		out := []int{v.Dst}
		return append(out, v.Elems...)
	case *Index:
		return []int{v.Dst, v.Array, v.Index}
	case *SetIndex:
		return []int{v.Array, v.Index, v.Src}
	case *IndexUnchecked:
		return []int{v.Dst, v.Array, v.Index}
	case *SetIndexUnchecked:
		return []int{v.Array, v.Index, v.Src}
	case *MakeStruct:
		out := []int{v.Dst}
		for _, f := range v.Fields {
			out = append(out, f.Src)
		}
		return out
	case *GetField:
		return []int{v.Dst, v.Src}
	case *SetField:
		return []int{v.Src, v.Value}
	case *MakeEnum:
		out := []int{v.Dst}
		if v.Payload >= 0 {
			out = append(out, v.Payload)
		}
		return out
	case *EnumTag:
		return []int{v.Dst, v.Src}
	case *EnumPayload:
		return []int{v.Dst, v.Src}
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
		var retTemp *int
		if ret.Value != nil {
			tmp := *ret.Value
			retTemp = &tmp
		}
		out[name] = inlineCandidate{fn: fn, retTemp: retTemp}
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
	for t := 0; t < cand.fn.TempCount; t++ {
		if cand.retTemp != nil && call.Dst >= 0 && t == *cand.retTemp {
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
			out = append(out, &StoreVar{Name: mapVar(param), Src: call.Args[i]})
		}
	}
	for _, inst := range blk.Instr {
		out = append(out, remapInstr(inst, tempMap, mapVar))
	}
	if cand.retTemp == nil && call.Dst >= 0 {
		out = append(out, &Const{Dst: call.Dst, Value: Value{Kind: KindUnit}})
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
	switch v := inst.(type) {
	case *Const:
		return &Const{Dst: remap(v.Dst), Value: v.Value}
	case *LoadVar:
		return &LoadVar{Dst: remap(v.Dst), Name: mapVar(v.Name)}
	case *StoreVar:
		return &StoreVar{Name: mapVar(v.Name), Src: remap(v.Src)}
	case *AddrOf:
		return &AddrOf{Dst: remap(v.Dst), Name: mapVar(v.Name)}
	case *LoadRef:
		return &LoadRef{Dst: remap(v.Dst), Src: remap(v.Src)}
	case *StoreRef:
		return &StoreRef{Ref: remap(v.Ref), Src: remap(v.Src)}
	case *BinOp:
		return &BinOp{Dst: remap(v.Dst), Op: v.Op, Lhs: remap(v.Lhs), Rhs: remap(v.Rhs)}
	case *UnaryOp:
		return &UnaryOp{Dst: remap(v.Dst), Op: v.Op, Src: remap(v.Src)}
	case *Call:
		args := make([]int, 0, len(v.Args))
		for _, a := range v.Args {
			args = append(args, remap(a))
		}
		return &Call{Dst: remap(v.Dst), Callee: v.Callee, Args: args}
	case *MakeArray:
		elems := make([]int, 0, len(v.Elems))
		for _, e := range v.Elems {
			elems = append(elems, remap(e))
		}
		return &MakeArray{Dst: remap(v.Dst), Elems: elems}
	case *Index:
		return &Index{Dst: remap(v.Dst), Array: remap(v.Array), Index: remap(v.Index)}
	case *SetIndex:
		return &SetIndex{Array: remap(v.Array), Index: remap(v.Index), Src: remap(v.Src)}
	case *IndexUnchecked:
		return &IndexUnchecked{Dst: remap(v.Dst), Array: remap(v.Array), Index: remap(v.Index)}
	case *SetIndexUnchecked:
		return &SetIndexUnchecked{Array: remap(v.Array), Index: remap(v.Index), Src: remap(v.Src)}
	case *MakeStruct:
		fields := make([]StructFieldInit, 0, len(v.Fields))
		for _, f := range v.Fields {
			fields = append(fields, StructFieldInit{Name: f.Name, Src: remap(f.Src)})
		}
		return &MakeStruct{Dst: remap(v.Dst), Name: v.Name, Fields: fields}
	case *GetField:
		return &GetField{Dst: remap(v.Dst), Src: remap(v.Src), Field: v.Field}
	case *SetField:
		return &SetField{Src: remap(v.Src), Field: v.Field, Value: remap(v.Value)}
	case *MakeEnum:
		return &MakeEnum{Dst: remap(v.Dst), Name: v.Name, Variant: v.Variant, Tag: v.Tag, TagType: v.TagType, Payload: remap(v.Payload)}
	case *EnumTag:
		return &EnumTag{Dst: remap(v.Dst), Src: remap(v.Src)}
	case *EnumPayload:
		return &EnumPayload{Dst: remap(v.Dst), Src: remap(v.Src)}
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
