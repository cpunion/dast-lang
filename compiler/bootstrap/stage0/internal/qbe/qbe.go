package qbe

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"dastlang/internal/ir"
)

type emitter struct {
	strIDs    map[string]int
	strOrder  []string
	tempID    int
	ptrSize   int
}

func newEmitter() *emitter {
	ptrSize := 8
	if v := os.Getenv("DAST_TARGET_PTR_WIDTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n == 32 {
				ptrSize = 4
			} else if n == 64 {
				ptrSize = 8
			}
		}
	}
	return &emitter{
		strIDs:   map[string]int{},
		strOrder: []string{},
		ptrSize:  ptrSize,
	}
}

func EmitProgram(p *ir.Program) string {
	e := newEmitter()
	e.collectStrings(p)

	var sb strings.Builder
	if len(e.strOrder) > 0 {
		for _, s := range e.strOrder {
			dataLabel := e.stringDataLabel(s)
			structLabel := e.stringStructLabel(s)
			sb.WriteString("data $")
			sb.WriteString(dataLabel)
			sb.WriteString(" = { b \"")
			sb.WriteString(escapeQBEString(s))
			sb.WriteString("\", b 0 }\n")
			sb.WriteString("data $")
			sb.WriteString(structLabel)
			sb.WriteString(" = { l $")
			sb.WriteString(dataLabel)
			sb.WriteString(", l ")
			sb.WriteString(fmt.Sprintf("%d", len(s)))
			sb.WriteString(", l ")
			sb.WriteString(fmt.Sprintf("%d", len(s)+1))
			sb.WriteString(" }\n")
		}
		sb.WriteString("\n")
	}

	fnNames := make([]string, 0, len(p.Functions))
	for name := range p.Functions {
		fnNames = append(fnNames, name)
	}
	sort.Strings(fnNames)
	for _, name := range fnNames {
		fn := p.Functions[name]
		sb.WriteString(e.emitFunction(p, fn))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (e *emitter) emitFunction(p *ir.Program, fn *ir.Function) string {
	ti := inferTypes(p, fn)
	e.tempID = fn.TempCount

	var sb strings.Builder
	retType := qbeType(ti.fnReturnType(fn))
	sb.WriteString("export function ")
	sb.WriteString(retType)
	sb.WriteString(" $")
	sb.WriteString(mangleFunc(fn.Name))
	sb.WriteString("(")
	for i, param := range fn.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		pt := param.Type
		if pt == "" && i < len(ti.tempTypes) {
			pt = ti.tempTypes[i]
		}
		sb.WriteString(qbeType(pt))
		sb.WriteString(" %t")
		sb.WriteString(fmt.Sprintf("%d", i))
	}
	sb.WriteString(") {\n")

	vars := collectVars(fn)
	varNames := make([]string, 0, len(vars))
	for name := range vars {
		varNames = append(varNames, name)
	}
	sort.Strings(varNames)

	sb.WriteString("@entry\n")

	for _, name := range varNames {
		vtype := ti.varType(name)
		sz := typeSize(e, p, vtype)
		if sz <= 0 {
			sz = 1
		}
		al := typeAlign(e, p, vtype)
		alloc := "alloc4"
		if al > 4 {
			alloc = "alloc8"
		}
		sb.WriteString("  %")
		sb.WriteString(varSlot(name))
		sb.WriteString(" =l ")
		sb.WriteString(alloc)
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%d", sz))
		sb.WriteString("\n")
	}
	for i, param := range fn.Params {
		if _, ok := vars[param.Name]; !ok {
			continue
		}
		pt := param.Type
		if pt == "" && i < len(ti.tempTypes) {
			pt = ti.tempTypes[i]
		}
		sb.WriteString("  store")
		sb.WriteString(qbeType(pt))
		sb.WriteString(" %t")
		sb.WriteString(fmt.Sprintf("%d", i))
		sb.WriteString(", %")
		sb.WriteString(varSlot(param.Name))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	for _, blk := range fn.Blocks {
		sb.WriteString("@")
		sb.WriteString(mangleLabel(blk.Label))
		sb.WriteString("\n")
		for _, inst := range blk.Instr {
			lines := e.emitInstr(p, fn, ti, inst)
			for _, line := range lines {
				sb.WriteString("  ")
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
		if blk.Term != nil {
			for _, line := range e.emitTerm(ti, fn.ReturnType, blk.Term) {
				sb.WriteString("  ")
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("}\n")
	return sb.String()
}

func (e *emitter) emitInstr(p *ir.Program, fn *ir.Function, ti *typeInfo, inst ir.Instr) []string {
	switch i := inst.(type) {
	case *ir.LoadVar:
		if i.Addr {
			return []string{fmt.Sprintf("%%t%d =l copy %%"+varSlot(i.Name), i.Dst)}
		}
		if i.Ref {
			base := baseType(ti.tempType(i.RefTemp))
			if base == "" {
				base = "i64"
			}
			destType := ti.tempType(i.Dst)
			if destType == "" {
				destType = base
			}
			if destType == "" {
				destType = "i64"
			}
			return []string{fmt.Sprintf("%%t%d =%s load %%t%d", i.Dst, qbeType(destType), i.RefTemp)}
		}
		vt := ti.varType(i.Name)
		if vt == "" {
			vt = "i64"
		}
		return []string{fmt.Sprintf("%%t%d =%s load %%%s", i.Dst, qbeType(vt), varSlot(i.Name))}
	case *ir.StoreVar:
		srcType := ti.operandType(i.Src)
		if srcType == "" {
			srcType = "i64"
		}
		src := e.operandExpr(i.Src)
		if i.Ref {
			return []string{fmt.Sprintf("store%s %s, %%t%d", qbeType(srcType), src, i.RefTemp)}
		}
		vt := ti.varType(i.Name)
		if vt == "" {
			vt = srcType
		}
		expr, castLines := e.castOperand(ti, i.Src, vt)
		lines := append([]string{}, castLines...)
		lines = append(lines, fmt.Sprintf("store%s %s, %%%s", qbeType(vt), expr, varSlot(i.Name)))
		return lines
	case *ir.BinOp:
		lt := ti.operandType(i.Lhs)
		rt := ti.operandType(i.Rhs)
		if isDefaultIntConst(i.Lhs) && rt != "" {
			lt = rt
		}
		if isDefaultIntConst(i.Rhs) && lt != "" {
			rt = lt
		}
		if isStringType(lt) || isStringType(rt) {
			if i.Op == "+" {
				return []string{fmt.Sprintf("%%t%d =l call $dast_string_concat(l %s, l %s)", i.Dst, e.operandExpr(i.Lhs), e.operandExpr(i.Rhs))}
			}
			if i.Op == "==" || i.Op == "!=" {
				lhsExpr, lhsLines := e.castOperand(ti, i.Lhs, "String")
				rhsExpr, rhsLines := e.castOperand(ti, i.Rhs, "String")
				lines := append(lhsLines, rhsLines...)
				tmp := i.Dst
				if i.Op == "!=" {
					tmp = e.newTemp()
				}
				lines = append(lines, fmt.Sprintf("%%t%d =w call $dast_string_eq(l %s, l %s)", tmp, lhsExpr, rhsExpr))
				if i.Op == "!=" {
					lines = append(lines, fmt.Sprintf("%%t%d =w xor %%t%d, 1", i.Dst, tmp))
				}
				return lines
			}
		}
		if isCompareOp(i.Op) {
			typ := unifyType(lt, rt)
			if typ == "" {
				typ = "i64"
			}
			lq := qbeType(lt)
			rq := qbeType(rt)
			if !i.Lhs.IsConst && i.Lhs.Temp >= 0 {
				if dt := ti.defType(i.Lhs.Temp); dt != "" {
					lq = qbeType(dt)
				}
			}
			if !i.Rhs.IsConst && i.Rhs.Temp >= 0 {
				if dt := ti.defType(i.Rhs.Temp); dt != "" {
					rq = qbeType(dt)
				}
			}
			if isDefaultIntConst(i.Lhs) {
				lq = rq
			}
			if isDefaultIntConst(i.Rhs) {
				rq = lq
			}
			if lq != rq {
				if lq == "l" || rq == "l" {
					typ = "i64"
				} else {
					typ = "i32"
				}
			} else if lq == "w" {
				typ = "i32"
			} else if lq == "l" {
				typ = "i64"
			}
			lhsExpr, lhsLines := e.castOperand(ti, i.Lhs, typ)
			rhsExpr, rhsLines := e.castOperand(ti, i.Rhs, typ)
			op := qbeCmpOp(i.Op, qbeType(typ))
			lines := append(lhsLines, rhsLines...)
			lines = append(lines, fmt.Sprintf("%%t%d =w %s %s, %s", i.Dst, op, lhsExpr, rhsExpr))
			return lines
		}
		if i.Op == "&&" || i.Op == "||" {
			qop := "and"
			if i.Op == "||" {
				qop = "or"
			}
			lhsExpr, lhsLines := e.castOperand(ti, i.Lhs, "bool")
			rhsExpr, rhsLines := e.castOperand(ti, i.Rhs, "bool")
			lines := append(lhsLines, rhsLines...)
			lines = append(lines, fmt.Sprintf("%%t%d =w %s %s, %s", i.Dst, qop, lhsExpr, rhsExpr))
			return lines
		}
		typ := unifyType(lt, rt)
		if dstKnown := ti.tempType(i.Dst); dstKnown != "" {
			typ = dstKnown
		}
		if typ == "" {
			typ = "i64"
		}
		lhsExpr, lhsLines := e.castOperand(ti, i.Lhs, typ)
		rhsExpr, rhsLines := e.castOperand(ti, i.Rhs, typ)
		lines := append(lhsLines, rhsLines...)
		lines = append(lines, fmt.Sprintf("%%t%d =%s %s %s, %s", i.Dst, qbeType(typ), qbeArithOp(i.Op), lhsExpr, rhsExpr))
		return lines
	case *ir.Call:
		return e.emitCall(p, ti, i.Dst, i.Callee, i.Args)
	case *ir.CallClosure:
		funcTemp := e.newTemp()
		envTemp := e.newTemp()
		funcOff := fieldOffset(e, p, "Closure", "func")
		envOff := fieldOffset(e, p, "Closure", "env")
		if funcOff < 0 || envOff < 0 {
			panic("invalid Closure layout")
		}
		funcType := fieldType(p, ti, "Closure", "func")
		if funcType == "" {
			funcType = "i64"
		}
		envType := fieldType(p, ti, "Closure", "env")
		if envType == "" {
			envType = "i64"
		}
		funcSize := typeSize(e, p, funcType)
		envSize := typeSize(e, p, envType)
		lines := []string{
			fmt.Sprintf("%%t%d =l call $dast_struct_load_u(l %s, l %d, l %d)", funcTemp, e.operandExpr(i.Closure), funcOff, funcSize),
			fmt.Sprintf("%%t%d =l call $dast_struct_load_u(l %s, l %d, l %d)", envTemp, e.operandExpr(i.Closure), envOff, envSize),
		}
		args := []ir.Operand{{IsConst: false, Temp: envTemp}}
		args = append(args, i.Args...)
		retType := ""
		if i.Dst >= 0 {
			retType = ti.tempType(i.Dst)
		}
		callLines := e.emitCallIndirect(ti, i.Dst, funcTemp, retType, args)
		lines = append(lines, callLines...)
		return lines
	case *ir.MakeArray:
		dstType := ti.tempType(i.Dst)
		elemType := arrayElemType(dstType)
		elemSize := typeSize(e, p, elemType)
		elemAlign := typeAlign(e, p, elemType)
		lines := []string{fmt.Sprintf("%%t%d =l call $dast_array_new(l %d, l %d, l %d)", i.Dst, elemSize, elemAlign, len(i.Elems))}
		for _, elem := range i.Elems {
			expr, castLines := e.castOperand(ti, elem, "i64")
			lines = append(lines, castLines...)
			lines = append(lines, fmt.Sprintf("call $dast_array_push(l %%t%d, l %s)", i.Dst, expr))
		}
		return lines
	case *ir.Index:
		arrayExpr, arrayLines, _ := e.derefOperand(ti, i.Array)
		dstType := ti.tempType(i.Dst)
		if dstType == "" {
			dstType = "i64"
		}
		fnName := arrayGetFn(dstType, i.Unchecked)
		dstQ := qbeType(dstType)
		rawTemp := i.Dst
		if dstQ != "l" {
			rawTemp = e.newTemp()
		}
		idxExpr, idxLines := e.castOperand(ti, i.Index, "i64")
		lines := append(arrayLines, idxLines...)
		lines = append(lines, fmt.Sprintf("%%t%d =l call $%s(l %s, l %s)", rawTemp, fnName, arrayExpr, idxExpr))
		if rawTemp != i.Dst {
			lines = append(lines, fmt.Sprintf("%%t%d =%s copy %%t%d", i.Dst, dstQ, rawTemp))
		}
		return lines
	case *ir.SetIndex:
		fnName := "dast_array_set"
		if i.Unchecked {
			fnName = "dast_array_set_unchecked"
		}
		arrayExpr, arrayLines, _ := e.derefOperand(ti, i.Array)
		idxExpr, idxLines := e.castOperand(ti, i.Index, "i64")
		valExpr, valLines := e.castOperand(ti, i.Src, "i64")
		lines := append(arrayLines, idxLines...)
		lines = append(lines, valLines...)
		lines = append(lines, fmt.Sprintf("call $%s(l %s, l %s, l %s)", fnName, arrayExpr, idxExpr, valExpr))
		return lines
	case *ir.MakeStruct:
		size := structLayoutSize(e, p, i.Name)
		align := structLayoutAlign(e, p, i.Name)
		if isEnumTypeName(p, i.Name) {
			size = enumLayoutSize(e, p, i.Name)
			align = enumLayoutAlign(e, p, i.Name)
		}
		lines := []string{fmt.Sprintf("%%t%d =l call $dast_struct_new(l %s, l %d, l %d)", i.Dst, e.stringDataRef(i.Name), size, align)}
		for _, f := range i.Fields {
			if i.Name == "Closure" && f.Name == "func" && f.Src.IsConst && f.Src.Const.Kind == ir.KindString {
				off := fieldOffset(e, p, i.Name, f.Name)
				if off < 0 {
					panic("invalid Closure func offset")
				}
				size := typeSize(e, p, "i64")
				fnName := f.Src.Const.Str
				lines = append(lines, fmt.Sprintf("call $dast_struct_store(l %%t%d, l %d, l %d, l $%s)", i.Dst, off, size, mangleFunc(fnName)))
				continue
			}
			ft := ti.operandType(f.Src)
			if ft == "" {
				ft = fieldType(p, ti, i.Name, f.Name)
			}
			if ft == "" {
				ft = "i64"
			}
			valExpr, valLines := e.castOperand(ti, f.Src, "i64")
			lines = append(lines, valLines...)
			off := fieldOffset(e, p, i.Name, f.Name)
			if off < 0 {
				panic(fmt.Sprintf("unknown field %s in %s", f.Name, i.Name))
			}
			size := typeSize(e, p, ft)
			lines = append(lines, fmt.Sprintf("call $dast_struct_store(l %%t%d, l %d, l %d, l %s)", i.Dst, off, size, valExpr))
		}
		return lines
	case *ir.GetField:
		srcExpr, srcLines, srcType := e.derefOperand(ti, ir.TempOperand(i.Src))
		ft := ti.tempType(i.Dst)
		if ft == "" {
			ft = fieldType(p, ti, baseType(srcType), i.Field)
		}
		if ft == "" {
			ft = "i64"
		}
		getFn := structLoadFn(ft)
		retType := ft
		rawTemp := i.Dst
		if qbeType(retType) != "l" {
			rawTemp = e.newTemp()
		}
		off := fieldOffset(e, p, baseType(srcType), i.Field)
		if off < 0 {
			panic(fmt.Sprintf("unknown field %s in %s", i.Field, baseType(srcType)))
		}
		size := typeSize(e, p, ft)
		lines := append([]string{}, srcLines...)
		lines = append(lines, fmt.Sprintf("%%t%d =l call $%s(l %s, l %d, l %d)", rawTemp, getFn, srcExpr, off, size))
		if rawTemp != i.Dst {
			lines = append(lines, fmt.Sprintf("%%t%d =%s copy %%t%d", i.Dst, qbeType(retType), rawTemp))
		}
		return lines
	case *ir.SetField:
		srcExpr, srcLines, srcType := e.derefOperand(ti, ir.TempOperand(i.Src))
		ft := ti.operandType(i.Value)
		if ft == "" {
			ft = fieldType(p, ti, baseType(srcType), i.Field)
		}
		if ft == "" {
			ft = "i64"
		}
		valExpr, valLines := e.castOperand(ti, i.Value, "i64")
		off := fieldOffset(e, p, baseType(srcType), i.Field)
		if off < 0 {
			panic(fmt.Sprintf("unknown field %s in %s", i.Field, baseType(srcType)))
		}
		size := typeSize(e, p, ft)
		lines := append([]string{}, srcLines...)
		lines = append(lines, valLines...)
		lines = append(lines, fmt.Sprintf("call $dast_struct_store(l %s, l %d, l %d, l %s)", srcExpr, off, size, valExpr))
		return lines
	case *ir.FieldAddr:
		srcExpr, srcLines, _ := e.derefOperand(ti, ir.TempOperand(i.Src))
		lines := append([]string{}, srcLines...)
		srcType := ti.tempType(i.Src)
		off := fieldOffset(e, p, srcType, i.Field)
		if off < 0 {
			panic(fmt.Sprintf("unknown field %s in %s", i.Field, srcType))
		}
		lines = append(lines, fmt.Sprintf("%%t%d =l call $dast_struct_field_addr(l %s, l %d)", i.Dst, srcExpr, off))
		return lines
	case *ir.IndexAddr:
		baseExpr, baseLines, _ := e.derefOperand(ti, ir.TempOperand(i.Base))
		idxExpr, idxLines := e.castOperand(ti, i.Index, "i64")
		lines := append(baseLines, idxLines...)
		lines = append(lines, fmt.Sprintf("%%t%d =l call $dast_array_index_addr(l %s, l %s)", i.Dst, baseExpr, idxExpr))
		return lines
	}
	return []string{}
}

func (e *emitter) emitCall(p *ir.Program, ti *typeInfo, dst int, callee string, args []ir.Operand) []string {
	if !isUserFunction(p, callee) {
		return e.emitBuiltinCall(p, ti, dst, callee, args)
	}
	calleeName := "$" + mangleFunc(callee)
	retType := ""
	if fn, ok := p.Functions[callee]; ok {
		retType = ti.fnReturnType(fn)
	}
	if retType == "" {
		retType = builtinReturnType(callee)
	}
	var parts []string
	lines := []string{}
	for i, arg := range args {
		pt := ""
		if fn, ok := p.Functions[callee]; ok && i < len(fn.Params) {
			pt = fn.Params[i].Type
		}
		if pt == "" {
			pt = ti.operandType(arg)
		}
		if pt == "" {
			pt = "i64"
		}
		expr, castLines := e.castOperand(ti, arg, pt)
		lines = append(lines, castLines...)
		parts = append(parts, fmt.Sprintf("%s %s", qbeType(pt), expr))
	}
	if dst >= 0 && retType != "" && retType != "unit" {
		lines = append(lines, fmt.Sprintf("%%t%d =%s call %s(%s)", dst, qbeType(retType), calleeName, strings.Join(parts, ", ")))
		return lines
	}
	if dst >= 0 && retType == "unit" {
		lines = append(lines, fmt.Sprintf("call %s(%s)", calleeName, strings.Join(parts, ", ")))
		lines = append(lines, fmt.Sprintf("%%t%d =%s copy 0", dst, qbeType("unit")))
		return lines
	}
	lines = append(lines, fmt.Sprintf("call %s(%s)", calleeName, strings.Join(parts, ", ")))
	return lines
}

func (e *emitter) emitCallIndirect(ti *typeInfo, dst int, funcTemp int, retType string, args []ir.Operand) []string {
	var parts []string
	for _, arg := range args {
		pt := ""
		if pt == "" {
			pt = ti.operandType(arg)
		}
		if pt == "" {
			pt = "i64"
		}
		parts = append(parts, fmt.Sprintf("%s %s", qbeType(pt), e.operandExpr(arg)))
	}
	if dst >= 0 && retType == "" {
		retType = "i64"
	}
	if dst >= 0 && retType != "" && retType != "unit" {
		return []string{fmt.Sprintf("%%t%d =%s call %%t%d(%s)", dst, qbeType(retType), funcTemp, strings.Join(parts, ", "))}
	}
	if dst >= 0 && retType == "unit" {
		return []string{
			fmt.Sprintf("call %%t%d(%s)", funcTemp, strings.Join(parts, ", ")),
			fmt.Sprintf("%%t%d =%s copy 0", dst, qbeType("unit")),
		}
	}
	return []string{fmt.Sprintf("call %%t%d(%s)", funcTemp, strings.Join(parts, ", "))}
}

func (e *emitter) emitBuiltinCall(p *ir.Program, ti *typeInfo, dst int, callee string, args []ir.Operand) []string {
	switch callee {
	case "push":
		if len(args) != 2 {
			return []string{fmt.Sprintf("call $dast_push(l 0, l 0)")}
		}
		arrExpr, arrLines := e.castOperand(ti, args[0], "i64")
		valExpr, valLines := e.castOperand(ti, args[1], "i64")
		lines := append([]string{}, arrLines...)
		lines = append(lines, valLines...)
		lines = append(lines, fmt.Sprintf("call $dast_push(l %s, l %s)", arrExpr, valExpr))
		if dst >= 0 {
			lines = append(lines, fmt.Sprintf("%%t%d =%s copy 0", dst, qbeType("unit")))
		}
		return lines
	case "pop":
		if len(args) != 1 {
			return []string{fmt.Sprintf("call $dast_pop(l 0)")}
		}
		arrExpr, arrLines := e.castOperand(ti, args[0], "i64")
		lines := append([]string{}, arrLines...)
		if dst < 0 {
			lines = append(lines, fmt.Sprintf("call $dast_pop(l %s)", arrExpr))
			return lines
		}
		dstType := ti.tempType(dst)
		if dstType == "" {
			dstType = "i64"
		}
		dstQ := qbeType(dstType)
		rawTemp := dst
		if dstQ != "l" {
			rawTemp = e.newTemp()
		}
		lines = append(lines, fmt.Sprintf("%%t%d =l call $dast_pop(l %s)", rawTemp, arrExpr))
		if rawTemp != dst {
			lines = append(lines, fmt.Sprintf("%%t%d =%s copy %%t%d", dst, dstQ, rawTemp))
		}
		return lines
	case "print", "println", "eprint", "eprintln":
		return e.emitPrintCall(ti, dst, callee, args)
	case "len":
		return e.emitLenCall(ti, dst, args)
	case "string_clone":
		if len(args) != 1 {
			if dst >= 0 {
				return []string{fmt.Sprintf("%%t%d =l call $dast_string_clone(l 0)", dst)}
			}
			return []string{fmt.Sprintf("call $dast_string_clone(l 0)")}
		}
		argExpr, argLines, _ := e.derefOperand(ti, args[0])
		lines := append([]string{}, argLines...)
		if dst >= 0 {
			lines = append(lines, fmt.Sprintf("%%t%d =l call $dast_string_clone(l %s)", dst, argExpr))
			return lines
		}
		lines = append(lines, fmt.Sprintf("call $dast_string_clone(l %s)", argExpr))
		return lines
	case "ast_expr", "ast_stmt", "ast_item", "ast_block":
		return e.emitAstCall(ti, dst, callee, args)
	}
	rtName := builtinRuntimeName(callee)
	retType := builtinReturnType(callee)
	if retType == "" {
		retType = "unit"
	}
	argTypes := builtinArgTypes(callee)
	var parts []string
	lines := []string{}
	for _, arg := range args {
		pt := ""
		if len(argTypes) > 0 {
			pt = argTypes[0]
			argTypes = argTypes[1:]
		}
		if pt == "" {
			pt = ti.operandType(arg)
		}
		if pt == "" {
			pt = "i64"
		}
		expr, castLines := e.castOperand(ti, arg, pt)
		lines = append(lines, castLines...)
		parts = append(parts, fmt.Sprintf("%s %s", qbeType(pt), expr))
	}
	if dst >= 0 && retType != "unit" {
		lines = append(lines, fmt.Sprintf("%%t%d =%s call $%s(%s)", dst, qbeType(retType), rtName, strings.Join(parts, ", ")))
		return lines
	}
	if dst >= 0 && retType == "unit" {
		lines = append(lines, fmt.Sprintf("call $%s(%s)", rtName, strings.Join(parts, ", ")))
		lines = append(lines, fmt.Sprintf("%%t%d =%s copy 0", dst, qbeType("unit")))
		return lines
	}
	lines = append(lines, fmt.Sprintf("call $%s(%s)", rtName, strings.Join(parts, ", ")))
	return lines
}

func (e *emitter) emitLenCall(ti *typeInfo, dst int, args []ir.Operand) []string {
	if len(args) != 1 {
		return []string{fmt.Sprintf("call $dast_string_len(l 0)")}
	}
	argType := ti.operandType(args[0])
	fn := "dast_string_len"
	if isArrayType(argType) {
		fn = "dast_array_len"
	}
	if dst >= 0 {
		return []string{fmt.Sprintf("%%t%d =%s call $%s(l %s)", dst, qbeType("int"), fn, e.operandExpr(args[0]))}
	}
	return []string{fmt.Sprintf("call $%s(l %s)", fn, e.operandExpr(args[0]))}
}

func (e *emitter) emitPrintCall(ti *typeInfo, dst int, callee string, args []ir.Operand) []string {
	isErr := callee == "eprint" || callee == "eprintln"
	newline := callee == "println" || callee == "eprintln"
	lines := []string{}
	for i, arg := range args {
		if i > 0 {
			if isErr {
				lines = append(lines, "call $dast_eprint_space()")
			} else {
				lines = append(lines, "call $dast_print_space()")
			}
		}
		at := ti.operandType(arg)
		if at == "" {
			at = "i64"
		}
		fn := printFnForType(at, isErr)
		targetType := at
		switch {
		case isIntType(at):
			targetType = "i64"
		case normalizeType(at) == "bool":
			targetType = "bool"
		case isStringType(at):
			targetType = "String"
		case isArrayType(at) || isStructType(at) || isEnumType(at):
			targetType = "i64"
		default:
			targetType = "i64"
		}
		expr, castLines := e.castOperand(ti, arg, targetType)
		lines = append(lines, castLines...)
		lines = append(lines, fmt.Sprintf("call $%s(%s %s)", fn, qbeType(targetType), expr))
	}
	if newline {
		if isErr {
			lines = append(lines, "call $dast_eprint_newline()")
		} else {
			lines = append(lines, "call $dast_print_newline()")
		}
	}
	if dst >= 0 {
		lines = append(lines, fmt.Sprintf("%%t%d =%s copy 0", dst, qbeType("unit")))
	}
	return lines
}

func (e *emitter) emitAstCall(ti *typeInfo, dst int, callee string, args []ir.Operand) []string {
	name := "dast_" + callee
	if len(args) == 1 {
		name += "1"
	} else {
		name += "2"
	}
	lines := []string{}
	parts := []string{}
	if len(args) > 0 {
		expr, castLines := e.castOperand(ti, args[0], "String")
		lines = append(lines, castLines...)
		parts = append(parts, fmt.Sprintf("l %s", expr))
	}
	if len(args) > 1 {
		expr, castLines := e.castOperand(ti, args[1], "i64")
		lines = append(lines, castLines...)
		parts = append(parts, fmt.Sprintf("l %s", expr))
	}
	retType := builtinReturnType(callee)
	if retType == "" {
		retType = "i64"
	}
	if dst >= 0 {
		lines = append(lines, fmt.Sprintf("%%t%d =%s call $%s(%s)", dst, qbeType(retType), name, strings.Join(parts, ", ")))
		return lines
	}
	lines = append(lines, fmt.Sprintf("call $%s(%s)", name, strings.Join(parts, ", ")))
	return lines
}

func printFnForType(t string, isErr bool) string {
	prefix := "dast_print_"
	if isErr {
		prefix = "dast_eprint_"
	}
	switch normalizeType(t) {
	case "bool":
		return prefix + "bool"
	case "String", "str":
		return prefix + "string"
	default:
		if isArrayType(t) {
			return prefix + "array"
		}
		if isStructType(t) || isEnumType(t) {
			return prefix + "struct"
		}
		if isIntType(t) {
			return prefix + "i64"
		}
	}
	return prefix + "ptr"
}

func builtinRuntimeName(name string) string {
	switch name {
	case "char_at":
		return "dast_char_at"
	case "substr":
		return "dast_substr"
	case "read_file":
		return "dast_read_file"
	case "read_dir":
		return "dast_read_dir"
	case "write_file":
		return "dast_write_file"
	case "mkdir":
		return "dast_mkdir"
	case "args":
		return "dast_args"
	case "read_line":
		return "dast_read_line"
	case "read_bytes":
		return "dast_read_bytes"
	case "exec":
		return "dast_exec"
	case "int_to_string":
		return "dast_int_to_string"
	case "parse_int":
		return "dast_parse_int"
	case "string_to_int":
		return "dast_string_to_int"
	case "has_prefix":
		return "dast_has_prefix"
	case "string_clone":
		return "dast_string_clone"
	case "push":
		return "dast_push"
	case "pop":
		return "dast_pop"
	case "exit":
		return "dast_exit"
	case "ast_expr":
		return "dast_ast_expr"
	case "ast_stmt":
		return "dast_ast_stmt"
	case "ast_item":
		return "dast_ast_item"
	case "ast_block":
		return "dast_ast_block"
	case "ast_to_string":
		return "dast_ast_to_string"
	case "gensym":
		return "dast_gensym"
	case "bind":
		return "dast_bind"
	}
	return "dast_" + mangleName(name)
}

func (e *emitter) emitTerm(ti *typeInfo, retType string, term ir.Term) []string {
	switch t := term.(type) {
	case *ir.Jump:
		return []string{fmt.Sprintf("jmp @%s", mangleLabel(t.Target))}
	case *ir.Branch:
		return []string{fmt.Sprintf("jnz %s, @%s, @%s", e.operandExpr(t.Cond), mangleLabel(t.Then), mangleLabel(t.Else))}
	case *ir.Return:
		if t.Value == nil {
			return []string{"ret"}
		}
		expr, lines := e.castOperand(ti, *t.Value, retType)
		lines = append(lines, fmt.Sprintf("ret %s", expr))
		return lines
	}
	return []string{"ret"}
}

func (e *emitter) collectStrings(p *ir.Program) {
	fnNames := make([]string, 0, len(p.Functions))
	for name := range p.Functions {
		fnNames = append(fnNames, name)
	}
	sort.Strings(fnNames)
	for _, name := range fnNames {
		fn := p.Functions[name]
		for _, blk := range fn.Blocks {
			for _, inst := range blk.Instr {
				e.collectStringsFromInstr(inst)
			}
			if blk.Term != nil {
				e.collectStringsFromTerm(blk.Term)
			}
		}
	}
}

func (e *emitter) collectStringsFromInstr(inst ir.Instr) {
	switch i := inst.(type) {
	case *ir.StoreVar:
		e.collectStringsFromOperand(i.Src)
	case *ir.BinOp:
		e.collectStringsFromOperand(i.Lhs)
		e.collectStringsFromOperand(i.Rhs)
	case *ir.Call:
		for _, a := range i.Args {
			e.collectStringsFromOperand(a)
		}
	case *ir.CallClosure:
		e.stringDataRef("func")
		e.stringDataRef("env")
		e.collectStringsFromOperand(i.Closure)
		for _, a := range i.Args {
			e.collectStringsFromOperand(a)
		}
	case *ir.MakeArray:
		for _, a := range i.Elems {
			e.collectStringsFromOperand(a)
		}
	case *ir.Index:
		e.collectStringsFromOperand(i.Array)
		e.collectStringsFromOperand(i.Index)
	case *ir.SetIndex:
		e.collectStringsFromOperand(i.Array)
		e.collectStringsFromOperand(i.Index)
		e.collectStringsFromOperand(i.Src)
	case *ir.MakeStruct:
		e.stringDataRef(i.Name)
		for _, f := range i.Fields {
			e.stringDataRef(f.Name)
			e.collectStringsFromOperand(f.Src)
		}
	case *ir.GetField:
		e.stringDataRef(i.Field)
	case *ir.SetField:
		e.stringDataRef(i.Field)
		e.collectStringsFromOperand(i.Value)
	case *ir.FieldAddr:
		e.stringDataRef(i.Field)
	case *ir.IndexAddr:
		e.collectStringsFromOperand(i.Index)
	}
}

func (e *emitter) collectStringsFromTerm(term ir.Term) {
	switch t := term.(type) {
	case *ir.Branch:
		e.collectStringsFromOperand(t.Cond)
	case *ir.Return:
		if t.Value != nil {
			e.collectStringsFromOperand(*t.Value)
		}
	}
}

func (e *emitter) collectStringsFromOperand(op ir.Operand) {
	if !op.IsConst {
		return
	}
	if op.Const.Kind == ir.KindString {
		_ = e.stringStructRef(op.Const.Str)
	}
}

func (e *emitter) stringID(s string) int {
	if id, ok := e.strIDs[s]; ok {
		return id
	}
	id := len(e.strOrder)
	e.strIDs[s] = id
	e.strOrder = append(e.strOrder, s)
	return id
}

func (e *emitter) stringDataLabel(s string) string {
	return fmt.Sprintf("str_data_%d", e.stringID(s))
}

func (e *emitter) stringStructLabel(s string) string {
	return fmt.Sprintf("str_%d", e.stringID(s))
}

func (e *emitter) stringDataRef(s string) string {
	return "$" + e.stringDataLabel(s)
}

func (e *emitter) stringStructRef(s string) string {
	return "$" + e.stringStructLabel(s)
}

func (e *emitter) newTemp() int {
	t := e.tempID
	e.tempID++
	return t
}

func isUserFunction(p *ir.Program, name string) bool {
	if p == nil {
		return false
	}
	_, ok := p.Functions[name]
	return ok
}

func isDefaultIntConst(op ir.Operand) bool {
	return op.IsConst && op.Const.Kind == ir.KindInt && op.Const.IntType == ""
}

func (e *emitter) operandExpr(op ir.Operand) string {
	if op.IsConst {
		return e.constExpr(op.Const)
	}
	return fmt.Sprintf("%%t%d", op.Temp)
}

func (e *emitter) castOperand(ti *typeInfo, op ir.Operand, targetType string) (string, []string) {
	expr := e.operandExpr(op)
	if op.IsConst {
		return expr, nil
	}
	if targetType == "" {
		return expr, nil
	}
	srcType := ti.operandDefType(op)
	if srcType == "" {
		srcType = targetType
	}
	srcQ := qbeType(srcType)
	dstQ := qbeType(targetType)
	if srcQ == dstQ {
		return expr, nil
	}
	lines := []string{}
	if dstQ == "l" && srcQ == "w" {
		tmp := e.newTemp()
		lines = append(lines, fmt.Sprintf("%%t%d =l extsw %s", tmp, expr))
		return fmt.Sprintf("%%t%d", tmp), lines
	}
	if dstQ == "w" && srcQ == "l" {
		tmp := e.newTemp()
		lines = append(lines, fmt.Sprintf("%%t%d =w copy %s", tmp, expr))
		return fmt.Sprintf("%%t%d", tmp), lines
	}
	tmp := e.newTemp()
	lines = append(lines, fmt.Sprintf("%%t%d =%s copy %s", tmp, dstQ, expr))
	return fmt.Sprintf("%%t%d", tmp), lines
}

func (e *emitter) derefOperand(ti *typeInfo, op ir.Operand) (string, []string, string) {
	typ := ti.operandType(op)
	if !isRefType(typ) {
		return e.operandExpr(op), nil, typ
	}
	base := baseType(typ)
	tmp := e.newTemp()
	line := fmt.Sprintf("%%t%d =%s load %s", tmp, qbeType(base), e.operandExpr(op))
	return fmt.Sprintf("%%t%d", tmp), []string{line}, base
}

func (e *emitter) constExpr(v ir.Value) string {
	switch v.Kind {
	case ir.KindInt:
		return fmt.Sprintf("%d", v.Int)
	case ir.KindBool:
		if v.Bool {
			return "1"
		}
		return "0"
	case ir.KindString:
		return e.stringStructRef(v.Str)
	default:
		return "0"
	}
}

type typeInfo struct {
	tempTypes []string
	defTypes  []string
	varTypes  map[string]string
}

func inferTypes(p *ir.Program, fn *ir.Function) *typeInfo {
	ti := &typeInfo{
		tempTypes: make([]string, fn.TempCount),
		defTypes:  make([]string, fn.TempCount),
		varTypes:  map[string]string{},
	}
	for i := 0; i < len(fn.TempTypes) && i < len(ti.tempTypes); i++ {
		ti.tempTypes[i] = fn.TempTypes[i]
	}
	for i, param := range fn.Params {
		if param.Type != "" {
			ti.varTypes[param.Name] = param.Type
			if i < len(ti.tempTypes) && ti.tempTypes[i] == "" {
				ti.tempTypes[i] = param.Type
			}
		}
	}

	changed := true
	for changed {
		changed = false
		for _, blk := range fn.Blocks {
			for _, inst := range blk.Instr {
				switch i := inst.(type) {
				case *ir.LoadVar:
					if i.Addr {
						vt := ti.varType(i.Name)
						if vt == "" {
							vt = "i64"
						}
						if ti.setTempType(i.Dst, "*"+vt) {
							changed = true
						}
					} else if i.Ref {
						base := baseType(ti.tempType(i.RefTemp))
						if base == "" {
							base = "i64"
						}
						if ti.setTempType(i.Dst, base) {
							changed = true
						}
					} else {
						tt := ti.tempType(i.Dst)
						if tt != "" {
							if ti.setVarType(i.Name, tt) {
								changed = true
							}
						}
						vt := ti.varType(i.Name)
						if vt != "" && ti.setTempType(i.Dst, vt) {
							changed = true
						}
					}
				case *ir.StoreVar:
					if i.Ref {
						base := baseType(ti.tempType(i.RefTemp))
						src := ti.operandDefType(i.Src)
						if base == "" {
							base = src
						}
						if base != "" {
							if ti.setTempType(i.RefTemp, "*"+base) {
								changed = true
							}
						}
					} else {
						src := ti.operandDefType(i.Src)
						if src != "" {
							if ti.setVarType(i.Name, src) {
								changed = true
							}
						}
					}
				case *ir.BinOp:
					if isCompareOp(i.Op) || i.Op == "&&" || i.Op == "||" {
						_ = ti.setDefType(i.Dst, "bool")
						if ti.setTempType(i.Dst, "bool") {
							changed = true
						}
					} else {
						lt := ti.operandType(i.Lhs)
						rt := ti.operandType(i.Rhs)
						if isStringType(lt) || isStringType(rt) {
							_ = ti.setDefType(i.Dst, "String")
							if ti.setTempType(i.Dst, "String") {
								changed = true
							}
						} else {
							dstKnown := ti.tempType(i.Dst)
							if dstKnown != "" {
								_ = ti.setDefType(i.Dst, dstKnown)
							} else {
								_ = ti.setDefType(i.Dst, unifyType(lt, rt))
								if ti.setTempType(i.Dst, unifyType(lt, rt)) {
									changed = true
								}
							}
						}
					}
				case *ir.Call:
					if i.Dst >= 0 {
						ret := builtinReturnType(i.Callee)
						if fn, ok := p.Functions[i.Callee]; ok {
							ret = ti.fnReturnType(fn)
						}
						if ret != "" {
							_ = ti.setDefType(i.Dst, ret)
						}
						if ret != "" && ti.setTempType(i.Dst, ret) {
							changed = true
						}
					}
				case *ir.CallClosure:
					if i.Dst >= 0 {
						_ = ti.setDefType(i.Dst, "unit")
					}
					if i.Dst >= 0 && ti.setTempType(i.Dst, "unit") {
						changed = true
					}
				case *ir.MakeArray:
					elem := ""
					for _, e := range i.Elems {
						elem = unifyType(elem, ti.operandType(e))
					}
					dstKnown := ti.tempType(i.Dst)
					if elem == "" && dstKnown != "" && isArrayType(dstKnown) {
						_ = ti.setDefType(i.Dst, dstKnown)
					} else {
						if elem == "" {
							elem = "unit"
						}
						_ = ti.setDefType(i.Dst, "["+elem+"]")
						if ti.setTempType(i.Dst, "["+elem+"]") {
							changed = true
						}
					}
				case *ir.Index:
					at := baseType(ti.operandType(i.Array))
					elem := elemType(at)
					if elem == "" {
						elem = ti.tempType(i.Dst)
					}
					if elem != "" {
						_ = ti.setDefType(i.Dst, elem)
						if ti.setTempType(i.Dst, elem) {
							changed = true
						}
					}
				case *ir.SetIndex:
					at := ti.operandType(i.Array)
					if at == "" {
						src := ti.operandType(i.Src)
						if src != "" && i.Array.IsConst == false && i.Array.Temp >= 0 {
							if ti.setTempType(i.Array.Temp, "["+src+"]") {
								changed = true
							}
						}
					}
				case *ir.MakeStruct:
					_ = ti.setDefType(i.Dst, i.Name)
					if ti.setTempType(i.Dst, i.Name) {
						changed = true
					}
				case *ir.GetField:
					st := ti.tempType(i.Src)
					ft := fieldType(p, ti, st, i.Field)
					if ft != "" {
						_ = ti.setDefType(i.Dst, ft)
						if ti.setTempType(i.Dst, ft) {
							changed = true
						}
					}
				case *ir.FieldAddr:
					st := ti.tempType(i.Src)
					ft := fieldType(p, ti, st, i.Field)
					if ft != "" {
						_ = ti.setDefType(i.Dst, "*"+ft)
						if ti.setTempType(i.Dst, "*"+ft) {
							changed = true
						}
					}
				case *ir.IndexAddr:
					base := ti.tempType(i.Base)
					base = baseType(base)
					elem := elemType(base)
					if elem != "" {
						_ = ti.setDefType(i.Dst, "*"+elem)
						if ti.setTempType(i.Dst, "*"+elem) {
							changed = true
						}
					}
				}
			}
			if blk.Term != nil {
				if br, ok := blk.Term.(*ir.Branch); ok {
					if br.Cond.IsConst == false && br.Cond.Temp >= 0 {
						if ti.setTempType(br.Cond.Temp, "bool") {
							changed = true
						}
					}
				}
			}
		}
	}
	return ti
}

func (t *typeInfo) tempType(idx int) string {
	if idx < 0 || idx >= len(t.tempTypes) {
		return ""
	}
	return t.tempTypes[idx]
}

func (t *typeInfo) defType(idx int) string {
	if idx < 0 || idx >= len(t.defTypes) {
		return ""
	}
	return t.defTypes[idx]
}

func (t *typeInfo) varType(name string) string {
	return t.varTypes[name]
}

func (t *typeInfo) setTempType(idx int, typ string) bool {
	if idx < 0 || idx >= len(t.tempTypes) {
		return false
	}
	cur := t.tempTypes[idx]
	next := unifyType(cur, typ)
	if next == "" || next == cur {
		return false
	}
	t.tempTypes[idx] = next
	return true
}

func (t *typeInfo) setDefType(idx int, typ string) bool {
	if idx < 0 || idx >= len(t.defTypes) {
		return false
	}
	if t.defTypes[idx] != "" {
		return false
	}
	if typ == "" {
		return false
	}
	t.defTypes[idx] = typ
	return true
}

func (t *typeInfo) setVarType(name string, typ string) bool {
	cur := t.varTypes[name]
	next := unifyType(cur, typ)
	if next == "" || next == cur {
		return false
	}
	t.varTypes[name] = next
	return true
}

func (t *typeInfo) operandType(op ir.Operand) string {
	if op.IsConst {
		switch op.Const.Kind {
		case ir.KindInt:
			if op.Const.IntType != "" {
				return op.Const.IntType
			}
			return "int"
		case ir.KindBool:
			return "bool"
		case ir.KindString:
			return "String"
		case ir.KindStruct:
			if op.Const.Struct != nil {
				return op.Const.Struct.Name
			}
		case ir.KindEnum:
			if op.Const.Enum != nil {
				return op.Const.Enum.Name
			}
		case ir.KindArray:
			return "[]"
		case ir.KindRef:
			return "*i64"
		}
		return ""
	}
	if op.Temp >= 0 && op.Temp < len(t.tempTypes) {
		return t.tempTypes[op.Temp]
	}
	return ""
}

func (t *typeInfo) operandDefType(op ir.Operand) string {
	if op.IsConst {
		return t.operandType(op)
	}
	if op.Temp >= 0 && op.Temp < len(t.defTypes) {
		if dt := t.defTypes[op.Temp]; dt != "" {
			return dt
		}
	}
	return t.operandType(op)
}

func (t *typeInfo) fnReturnType(fn *ir.Function) string {
	if fn.ReturnType != "" {
		return fn.ReturnType
	}
	ret := ""
	for _, blk := range fn.Blocks {
		if blk.Term == nil {
			continue
		}
		if r, ok := blk.Term.(*ir.Return); ok && r.Value != nil {
			ret = unifyType(ret, t.operandType(*r.Value))
		}
	}
	if ret != "" && ret != "unit" {
		return ret
	}
	if fn.ReturnType != "" {
		return fn.ReturnType
	}
	return "unit"
}

func collectVars(fn *ir.Function) map[string]struct{} {
	out := map[string]struct{}{}
	for _, blk := range fn.Blocks {
		for _, inst := range blk.Instr {
			switch i := inst.(type) {
			case *ir.LoadVar:
				if i.Name != "" {
					out[i.Name] = struct{}{}
				}
			case *ir.StoreVar:
				if i.Name != "" {
					out[i.Name] = struct{}{}
				}
			}
		}
	}
	return out
}

func qbeType(typ string) string {
	typ = normalizeType(typ)
	if typ == "" || typ == "unit" {
		return "w"
	}
	if isRefType(typ) || isArrayType(typ) || isStructType(typ) || isStringType(typ) || isEnumType(typ) {
		return "l"
	}
	if typ == "bool" {
		return "w"
	}
	switch typ {
	case "i8", "u8":
		return "b"
	case "i16", "u16":
		return "h"
	case "i32", "u32", "char":
		return "w"
	case "i64", "int", "isize", "usize":
		return "l"
	}
	return "l"
}

func typeIntWidth(t string, ptrSize int) int64 {
	switch t {
	case "i8", "u8":
		return 8
	case "i16", "u16":
		return 16
	case "i32", "u32", "char":
		return 32
	case "i64", "u64", "int":
		return 64
	case "isize", "usize":
		return int64(ptrSize * 8)
	}
	return 0
}

func isSignedIntType(t string) bool {
	switch t {
	case "i8", "i16", "i32", "i64", "int", "isize":
		return true
	}
	return false
}

func typeSize(e *emitter, p *ir.Program, t string) int64 {
	t = normalizeType(t)
	if t == "" || t == "unit" {
		return 0
	}
	if isRefType(t) || isArrayType(t) || isStringType(t) || isStructType(t) || isEnumTypeName(p, t) {
		return int64(e.ptrSize)
	}
	if t == "bool" {
		return 1
	}
	if t == "char" {
		return 4
	}
	if w := typeIntWidth(t, e.ptrSize); w > 0 {
		return w / 8
	}
	return int64(e.ptrSize)
}

func typeAlign(e *emitter, p *ir.Program, t string) int64 {
	t = normalizeType(t)
	if t == "" || t == "unit" {
		return 1
	}
	if isRefType(t) || isArrayType(t) || isStringType(t) || isStructType(t) || isEnumTypeName(p, t) {
		return int64(e.ptrSize)
	}
	if t == "bool" {
		return 1
	}
	if t == "char" {
		return 4
	}
	if w := typeIntWidth(t, e.ptrSize); w > 0 {
		return w / 8
	}
	return int64(e.ptrSize)
}

func alignTo(n, align int64) int64 {
	if align <= 1 {
		return n
	}
	rem := n % align
	if rem == 0 {
		return n
	}
	return n + (align - rem)
}

func structFieldOffset(e *emitter, p *ir.Program, structType, field string) int64 {
	st := normalizeType(structType)
	if st == "" {
		return -1
	}
	st = baseType(st)
	td := p.TypeDecls[st]
	if td == nil {
		return -1
	}
	var off int64
	for _, f := range td.Fields {
		ft := f.Type
		al := typeAlign(e, p, ft)
		off = alignTo(off, al)
		if f.Name == field {
			return off
		}
		off += typeSize(e, p, ft)
	}
	return -1
}

func structLayoutAlign(e *emitter, p *ir.Program, structType string) int64 {
	st := normalizeType(structType)
	if st == "" {
		return 1
	}
	st = baseType(st)
	td := p.TypeDecls[st]
	if td == nil {
		return 1
	}
	var max int64 = 1
	for _, f := range td.Fields {
		al := typeAlign(e, p, f.Type)
		if al > max {
			max = al
		}
	}
	return max
}

func structLayoutSize(e *emitter, p *ir.Program, structType string) int64 {
	st := normalizeType(structType)
	if st == "" {
		return 0
	}
	st = baseType(st)
	td := p.TypeDecls[st]
	if td == nil {
		return 0
	}
	var off int64
	var max int64 = 1
	for _, f := range td.Fields {
		al := typeAlign(e, p, f.Type)
		if al > max {
			max = al
		}
		off = alignTo(off, al)
		off += typeSize(e, p, f.Type)
	}
	return alignTo(off, max)
}

func isEnumTypeName(p *ir.Program, t string) bool {
	t = baseType(normalizeType(t))
	if t == "" {
		return false
	}
	for _, e := range p.Enums {
		if e.Name == t {
			return true
		}
	}
	return false
}

func enumTagType(p *ir.Program, enumName string) string {
	for _, e := range p.Enums {
		if e.Name == enumName {
			if e.TagType != "" {
				return e.TagType
			}
			return "i32"
		}
	}
	return "i32"
}

func enumPayloadSize(e *emitter, p *ir.Program, enumName string) int64 {
	var max int64
	for _, en := range p.Enums {
		if en.Name != enumName {
			continue
		}
		for _, v := range en.Variants {
			if v.PayloadType == "" || v.PayloadType == "unit" {
				continue
			}
			sz := typeSize(e, p, v.PayloadType)
			if sz > max {
				max = sz
			}
		}
	}
	return max
}

func enumPayloadAlign(e *emitter, p *ir.Program, enumName string) int64 {
	var max int64 = 1
	for _, en := range p.Enums {
		if en.Name != enumName {
			continue
		}
		for _, v := range en.Variants {
			if v.PayloadType == "" || v.PayloadType == "unit" {
				continue
			}
			al := typeAlign(e, p, v.PayloadType)
			if al > max {
				max = al
			}
		}
	}
	return max
}

func enumPayloadOffset(e *emitter, p *ir.Program, enumName string) int64 {
	tagT := enumTagType(p, enumName)
	tagSize := typeSize(e, p, tagT)
	return alignTo(tagSize, enumPayloadAlign(e, p, enumName))
}

func enumLayoutAlign(e *emitter, p *ir.Program, enumName string) int64 {
	tagT := enumTagType(p, enumName)
	tagAlign := typeAlign(e, p, tagT)
	payloadAlign := enumPayloadAlign(e, p, enumName)
	if payloadAlign > tagAlign {
		return payloadAlign
	}
	return tagAlign
}

func enumLayoutSize(e *emitter, p *ir.Program, enumName string) int64 {
	tagT := enumTagType(p, enumName)
	tagSize := typeSize(e, p, tagT)
	payloadSize := enumPayloadSize(e, p, enumName)
	if payloadSize == 0 {
		return alignTo(tagSize, enumLayoutAlign(e, p, enumName))
	}
	total := enumPayloadOffset(e, p, enumName) + payloadSize
	return alignTo(total, enumLayoutAlign(e, p, enumName))
}

func qbeArithOp(op string) string {
	switch op {
	case "+":
		return "add"
	case "-":
		return "sub"
	case "*":
		return "mul"
	case "/":
		return "div"
	case "%":
		return "rem"
	default:
		return "add"
	}
}

func qbeCmpOp(op string, typ string) string {
	suffix := typ
	if suffix == "" {
		suffix = "l"
	}
	switch op {
	case "==":
		return "ceq" + suffix
	case "!=":
		return "cne" + suffix
	case "<":
		return "cslt" + suffix
	case "<=":
		return "csle" + suffix
	case ">":
		return "csgt" + suffix
	case ">=":
		return "csge" + suffix
	default:
		return "ceq" + suffix
	}
}

func isCompareOp(op string) bool {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

func normalizeType(t string) string {
	return strings.TrimSpace(t)
}

func unifyType(a, b string) string {
	a = normalizeType(a)
	b = normalizeType(b)
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if a == "bool" || b == "bool" {
		return "bool"
	}
	if a == "unit" {
		return b
	}
	if b == "unit" {
		return a
	}
	if a == b {
		return a
	}
	if isIntType(a) && !isIntType(b) && (a == "i64" || a == "int") {
		return b
	}
	if isIntType(b) && !isIntType(a) && (b == "i64" || b == "int") {
		return a
	}
	if isIntType(a) && isIntType(b) {
		return widerInt(a, b)
	}
	if isStringType(a) || isStringType(b) {
		return "String"
	}
	return a
}

func isIntType(t string) bool {
	switch t {
	case "int", "i8", "i16", "i32", "i64", "isize", "usize", "u8", "u16", "u32", "u64", "char":
		return true
	default:
		return false
	}
}

func widerInt(a, b string) string {
	rank := func(t string) int {
		switch t {
		case "i8", "u8":
			return 1
		case "i16", "u16":
			return 2
		case "i32", "u32", "char":
			return 3
		case "i64", "int", "isize", "usize", "u64":
			return 4
		default:
			return 0
		}
	}
	if rank(a) >= rank(b) {
		return a
	}
	return b
}

func isRefType(t string) bool {
	return strings.HasPrefix(t, "*")
}

func baseType(t string) string {
	if strings.HasPrefix(t, "*") {
		return strings.TrimPrefix(t, "*")
	}
	return t
}

func isArrayType(t string) bool {
	return strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]")
}

func elemType(t string) string {
	if !isArrayType(t) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(t, "["), "]")
}

func isStringType(t string) bool {
	return t == "String" || t == "str"
}

func isStructType(t string) bool {
	if t == "" {
		return false
	}
	if isRefType(t) || isArrayType(t) || isStringType(t) || isIntType(t) || t == "bool" || t == "unit" {
		return false
	}
	return true
}

func isEnumType(t string) bool {
	return false
}

func fieldType(p *ir.Program, ti *typeInfo, structType string, field string) string {
	st := normalizeType(structType)
	if st == "" {
		return ""
	}
	st = baseType(st)
	if p.TypeDecls != nil {
		if td, ok := p.TypeDecls[st]; ok {
			for _, f := range td.Fields {
				if f.Name == field {
					return f.Type
				}
			}
		}
	}
	for _, e := range p.Enums {
		if e.Name == st {
			if field == "_tag" {
				if e.TagType != "" {
					return e.TagType
				}
				return "i32"
			}
			if field == "_payload" {
				payload := ""
				for _, v := range e.Variants {
					if v.PayloadType == "" || v.PayloadType == "unit" {
						continue
					}
					if payload == "" {
						payload = v.PayloadType
						continue
					}
					if payload != v.PayloadType {
						return "i64"
					}
				}
				if payload != "" {
					return payload
				}
				return "unit"
			}
		}
	}
	return ""
}

func structSetFn(t string) string {
	switch normalizeType(t) {
	case "bool":
		return "dast_struct_set_bool"
	case "String", "str":
		return "dast_struct_set_string"
	default:
		if isIntType(t) {
			if qbeType(t) == "w" {
				return "dast_struct_set_i32"
			}
			return "dast_struct_set_i64"
		}
	}
	return "dast_struct_set_ptr"
}

func structGetFn(t string) string {
	switch normalizeType(t) {
	case "bool":
		return "dast_struct_get_bool"
	case "String", "str":
		return "dast_struct_get_string"
	default:
		if isIntType(t) {
			if qbeType(t) == "w" {
				return "dast_struct_get_i32"
			}
			return "dast_struct_get_i64"
		}
	}
	return "dast_struct_get_ptr"
}

func structLoadFn(t string) string {
	if isSignedIntType(normalizeType(t)) {
		return "dast_struct_load_s"
	}
	return "dast_struct_load_u"
}

func arrayGetFn(t string, unchecked bool) string {
	name := "dast_array_get_u"
	if isSignedIntType(normalizeType(t)) {
		name = "dast_array_get_s"
	}
	if unchecked {
		return name + "_unchecked"
	}
	return name
}

func arrayElemType(t string) string {
	nt := normalizeType(t)
	if strings.HasPrefix(nt, "[") && strings.HasSuffix(nt, "]") {
		return strings.TrimSuffix(strings.TrimPrefix(nt, "["), "]")
	}
	return "i64"
}

func fieldOffset(e *emitter, p *ir.Program, structType, field string) int64 {
	st := baseType(normalizeType(structType))
	if st == "" {
		return -1
	}
	if isEnumTypeName(p, st) {
		if field == "_tag" {
			return 0
		}
		if field == "_payload" {
			return enumPayloadOffset(e, p, st)
		}
	}
	return structFieldOffset(e, p, st, field)
}

func builtinReturnType(name string) string {
	switch name {
	case "len", "char_at":
		return "int"
	case "substr", "read_file", "read_line", "read_bytes", "ast_to_string", "gensym", "bind", "int_to_string":
		return "String"
	case "getenv":
		return "String"
	case "string_clone":
		return "String"
	case "args", "read_dir":
		return "[String]"
	case "print", "println", "eprint", "eprintln", "push", "exit", "write_file", "mkdir":
		return "unit"
	case "pop":
		return "int"
	case "parse_int":
		return "i32"
	case "string_to_int":
		return "i64"
	case "has_prefix":
		return "bool"
	case "exec":
		return "i64"
	case "ast_expr":
		return "AstExpr"
	case "ast_stmt":
		return "AstStmt"
	case "ast_item":
		return "AstItem"
	case "ast_block":
		return "AstBlock"
	}
	return ""
}

func builtinArgTypes(name string) []string {
	switch name {
	case "char_at":
		return []string{"String", "i64"}
	case "substr":
		return []string{"String", "i64", "i64"}
	case "string_clone":
		return []string{"String"}
	case "read_file":
		return []string{"String"}
	case "read_dir":
		return []string{"String"}
	case "getenv":
		return []string{"String"}
	case "write_file":
		return []string{"String", "String"}
	case "mkdir":
		return []string{"String"}
	case "read_bytes":
		return []string{"i64"}
	case "exec":
		return []string{"String", "[String]"}
	case "int_to_string":
		return []string{"i64"}
	case "parse_int":
		return []string{"String"}
	case "string_to_int":
		return []string{"String"}
	case "has_prefix":
		return []string{"String", "String"}
	case "exit":
		return []string{"i64"}
	case "ast_to_string":
		return []string{"String"}
	case "gensym":
		return []string{"String"}
	case "bind":
		return []string{"String"}
	}
	return nil
}

func varSlot(name string) string {
	return "v_" + mangleName(name)
}

func mangleFunc(name string) string {
	return "dast_user_" + mangleName(name)
}

func mangleName(name string) string {
	if name == "" {
		return "_"
	}
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}

func mangleLabel(label string) string {
	return mangleName(label)
}

func escapeQBEString(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '\n':
			sb.WriteString("\\n")
		case '\r':
			sb.WriteString("\\r")
		case '\t':
			sb.WriteString("\\t")
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
