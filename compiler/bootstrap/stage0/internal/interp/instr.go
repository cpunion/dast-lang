package interp

import (
	"errors"
	"fmt"

	"dastlang/internal/ir"
)

func (rt *Runtime) execInstr(fr *frame, inst ir.Instr) error {
	switch i := inst.(type) {
	case *ir.Const:
		return rt.setTemp(fr, i.Dst, i.Value)
	case *ir.LoadVar:
		if i.Ref {
			ref, err := rt.getTemp(fr, i.RefTemp)
			if err != nil {
				return err
			}
			val, err := rt.deref(ref)
			if err != nil {
				return err
			}
			return rt.setTemp(fr, i.Dst, val)
		}
		ptr, ok := fr.vars[i.Name]
		if !ok {
			return fmt.Errorf("undefined variable '%s'", i.Name)
		}
		if i.Addr {
			addr, ok := fr.varAddrs[i.Name]
			if !ok {
				addr = rt.allocAddr(ptr)
				fr.varAddrs[i.Name] = addr
			}
			return rt.setTemp(fr, i.Dst, ir.Value{Kind: ir.KindRef, Ref: addr})
		}
		return rt.setTemp(fr, i.Dst, *ptr)
	case *ir.StoreVar:
		if i.Ref {
			ref, err := rt.getTemp(fr, i.RefTemp)
			if err != nil {
				return err
			}
			val, err := rt.getTemp(fr, i.Src)
			if err != nil {
				return err
			}
			return rt.storeRef(ref, val)
		}
		val, err := rt.getTemp(fr, i.Src)
		if err != nil {
			return err
		}
		ptr, ok := fr.vars[i.Name]
		if !ok {
			v := val
			fr.vars[i.Name] = &v
			fr.varAddrs[i.Name] = rt.allocAddr(&v)
			return nil
		}
		*ptr = val
		return nil
	case *ir.BinOp:
		lhs, err := rt.evalOperand(fr, i.Lhs)
		if err != nil {
			return err
		}
		rhs, err := rt.evalOperand(fr, i.Rhs)
		if err != nil {
			return err
		}
		res, err := evalBinary(i.Op, lhs, rhs)
		if err != nil {
			return err
		}
		return rt.setTemp(fr, i.Dst, res)
	case *ir.Call:
		args := make([]ir.Value, 0, len(i.Args))
		for _, arg := range i.Args {
			val, err := rt.getTemp(fr, arg)
			if err != nil {
				return err
			}
			args = append(args, val)
		}
		res, err := rt.callFunction(i.Callee, args)
		if err != nil {
			return err
		}
		if i.Dst >= 0 {
			return rt.setTemp(fr, i.Dst, res)
		}
		return nil
	case *ir.MakeStruct:
		fields := map[string]ir.Value{}
		for _, f := range i.Fields {
			val, err := rt.getTemp(fr, f.Src)
			if err != nil {
				return err
			}
			fields[f.Name] = val
		}
		val := ir.Value{Kind: ir.KindStruct, Struct: &ir.StructValue{Name: i.Name, Fields: fields}}
		return rt.setTemp(fr, i.Dst, val)
	case *ir.MakeArray:
		elems := make([]ir.Value, 0, len(i.Elems))
		for _, e := range i.Elems {
			val, err := rt.getTemp(fr, e)
			if err != nil {
				return err
			}
			elems = append(elems, val)
		}
		val := ir.Value{Kind: ir.KindArray, Array: &ir.ArrayValue{Elems: elems}}
		return rt.setTemp(fr, i.Dst, val)
	case *ir.Index:
		arrayVal, err := rt.getTemp(fr, i.Array)
		if err != nil {
			return err
		}
		indexVal, err := rt.getTemp(fr, i.Index)
		if err != nil {
			return err
		}
		var res ir.Value
		if i.Unchecked {
			res, err = rt.indexUnchecked(arrayVal, indexVal)
		} else {
			res, err = rt.index(arrayVal, indexVal)
		}
		if err != nil {
			return err
		}
		return rt.setTemp(fr, i.Dst, res)
	case *ir.SetIndex:
		arrayVal, err := rt.getTemp(fr, i.Array)
		if err != nil {
			return err
		}
		indexVal, err := rt.getTemp(fr, i.Index)
		if err != nil {
			return err
		}
		val, err := rt.getTemp(fr, i.Src)
		if err != nil {
			return err
		}
		if i.Unchecked {
			return rt.setIndexUnchecked(arrayVal, indexVal, val)
		}
		return rt.setIndex(arrayVal, indexVal, val)
	case *ir.GetField:
		src, err := rt.getTemp(fr, i.Src)
		if err != nil {
			return err
		}
		val, err := rt.getStructField(src, i.Field)
		if err != nil {
			return err
		}
		return rt.setTemp(fr, i.Dst, val)
	case *ir.SetField:
		src, err := rt.getTemp(fr, i.Src)
		if err != nil {
			return err
		}
		val, err := rt.getTemp(fr, i.Value)
		if err != nil {
			return err
		}
		return rt.setStructField(src, i.Field, val)
	default:
		return errors.New("unknown instruction")
	}
}

func (rt *Runtime) evalOperand(fr *frame, op ir.Operand) (ir.Value, error) {
	if op.IsConst {
		return op.Const, nil
	}
	return rt.getTemp(fr, op.Temp)
}

func (rt *Runtime) getTemp(fr *frame, idx int) (ir.Value, error) {
	if idx < 0 || idx >= len(fr.temps) {
		return ir.Value{Kind: ir.KindUnit}, fmt.Errorf("invalid temp t%d", idx)
	}
	return fr.temps[idx], nil
}

func (rt *Runtime) setTemp(fr *frame, idx int, val ir.Value) error {
	if idx < 0 {
		return fmt.Errorf("invalid temp t%d", idx)
	}
	if idx >= len(fr.temps) {
		newTemps := make([]ir.Value, idx+1)
		copy(newTemps, fr.temps)
		fr.temps = newTemps
	}
	fr.temps[idx] = val
	return nil
}

func (rt *Runtime) deref(v ir.Value) (ir.Value, error) {
	if v.Kind != ir.KindRef {
		return ir.Value{Kind: ir.KindUnit}, errors.New("deref requires ref")
	}
	ptr, ok := rt.heap[v.Ref]
	if !ok {
		return ir.Value{Kind: ir.KindUnit}, errors.New("invalid reference")
	}
	return *ptr, nil
}

func (rt *Runtime) storeRef(ref ir.Value, val ir.Value) error {
	if ref.Kind != ir.KindRef {
		return errors.New("store_ref requires ref")
	}
	ptr, ok := rt.heap[ref.Ref]
	if !ok {
		return errors.New("invalid reference")
	}
	*ptr = val
	return nil
}

func (rt *Runtime) getStructField(v ir.Value, field string) (ir.Value, error) {
	if v.Kind == ir.KindRef {
		val, err := rt.deref(v)
		if err != nil {
			return ir.Value{Kind: ir.KindUnit}, err
		}
		v = val
	}
	if v.Kind != ir.KindStruct || v.Struct == nil {
		return ir.Value{Kind: ir.KindUnit}, errors.New("field access requires struct")
	}
	val, ok := v.Struct.Fields[field]
	if !ok {
		return ir.Value{Kind: ir.KindUnit}, fmt.Errorf("unknown field '%s'", field)
	}
	return val, nil
}

func (rt *Runtime) setStructField(v ir.Value, field string, value ir.Value) error {
	if v.Kind == ir.KindRef {
		val, err := rt.deref(v)
		if err != nil {
			return err
		}
		v = val
	}
	if v.Kind != ir.KindStruct || v.Struct == nil {
		return errors.New("field assignment requires struct")
	}
	if _, ok := v.Struct.Fields[field]; !ok {
		return fmt.Errorf("unknown field '%s'", field)
	}
	v.Struct.Fields[field] = value
	return nil
}

func (rt *Runtime) arrayValue(v ir.Value) (*ir.ArrayValue, error) {
	if v.Kind == ir.KindRef {
		val, err := rt.deref(v)
		if err != nil {
			return nil, err
		}
		v = val
	}
	if v.Kind != ir.KindArray || v.Array == nil {
		return nil, errors.New("array required")
	}
	return v.Array, nil
}

func (rt *Runtime) index(arrayVal ir.Value, indexVal ir.Value) (ir.Value, error) {
	if indexVal.Kind != ir.KindInt {
		return ir.Value{Kind: ir.KindUnit}, errors.New("index requires int")
	}
	arr, err := rt.arrayValue(arrayVal)
	if err != nil {
		return ir.Value{Kind: ir.KindUnit}, err
	}
	idx := int(indexVal.Int)
	if idx < 0 || idx >= len(arr.Elems) {
		return ir.Value{Kind: ir.KindUnit}, fmt.Errorf("index out of bounds: %d", idx)
	}
	return arr.Elems[idx], nil
}

func (rt *Runtime) setIndex(arrayVal ir.Value, indexVal ir.Value, val ir.Value) error {
	if indexVal.Kind != ir.KindInt {
		return errors.New("index requires int")
	}
	arr, err := rt.arrayValue(arrayVal)
	if err != nil {
		return err
	}
	idx := int(indexVal.Int)
	if idx < 0 || idx >= len(arr.Elems) {
		return fmt.Errorf("index out of bounds: %d", idx)
	}
	arr.Elems[idx] = val
	return nil
}

func (rt *Runtime) indexUnchecked(arrayVal ir.Value, indexVal ir.Value) (ir.Value, error) {
	if indexVal.Kind != ir.KindInt {
		return ir.Value{Kind: ir.KindUnit}, errors.New("index requires int")
	}
	arr, err := rt.arrayValue(arrayVal)
	if err != nil {
		return ir.Value{Kind: ir.KindUnit}, err
	}
	idx := int(indexVal.Int)
	return arr.Elems[idx], nil
}

func (rt *Runtime) setIndexUnchecked(arrayVal ir.Value, indexVal ir.Value, val ir.Value) error {
	if indexVal.Kind != ir.KindInt {
		return errors.New("index requires int")
	}
	arr, err := rt.arrayValue(arrayVal)
	if err != nil {
		return err
	}
	idx := int(indexVal.Int)
	arr.Elems[idx] = val
	return nil
}
