package ir

func Optimize(p *Program) *Program {
	for _, fn := range p.Functions {
		if fn == nil {
			continue
		}
		optimizeFunction(fn)
	}
	return p
}

func optimizeFunction(fn *Function) {
	if len(fn.Blocks) == 0 {
		return
	}
	for _, blk := range fn.Blocks {
		optimizeBlock(blk)
	}
	pruneUnreachable(fn)
}

func optimizeBlock(blk *Block) {
	consts := map[int]Value{}
	for i, inst := range blk.Instr {
		switch v := inst.(type) {
		case *Const:
			consts[v.Dst] = v.Value
		case *LoadVar:
			delete(consts, v.Dst)
		case *AddrOf:
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
		case *MakeArray:
			delete(consts, v.Dst)
		case *Index:
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
			return Value{Kind: KindInt, Int: -v.Int}, true
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
			return Value{Kind: KindInt, Int: a.Int + b.Int}, true
		}
		if a.Kind == KindString && b.Kind == KindString {
			return Value{Kind: KindString, Str: a.Str + b.Str}, true
		}
	case "-":
		if a.Kind == KindInt && b.Kind == KindInt {
			return Value{Kind: KindInt, Int: a.Int - b.Int}, true
		}
	case "*":
		if a.Kind == KindInt && b.Kind == KindInt {
			return Value{Kind: KindInt, Int: a.Int * b.Int}, true
		}
	case "/":
		if a.Kind == KindInt && b.Kind == KindInt && b.Int != 0 {
			return Value{Kind: KindInt, Int: a.Int / b.Int}, true
		}
	case "%":
		if a.Kind == KindInt && b.Kind == KindInt && b.Int != 0 {
			return Value{Kind: KindInt, Int: a.Int % b.Int}, true
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
