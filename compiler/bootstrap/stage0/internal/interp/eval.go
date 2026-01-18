package interp

import (
	"errors"
	"fmt"
	"os"

	"dastlang/internal/ir"
)

func evalUnary(op string, v ir.Value) (ir.Value, error) {
	switch op {
	case "-":
		if v.Kind != ir.KindInt {
			return ir.Value{Kind: ir.KindUnit}, errors.New("unary '-' requires int")
		}
		return ir.Value{Kind: ir.KindInt, Int: -v.Int}, nil
	case "!":
		if v.Kind != ir.KindBool {
			return ir.Value{Kind: ir.KindUnit}, errors.New("unary '!' requires bool")
		}
		return ir.Value{Kind: ir.KindBool, Bool: !v.Bool}, nil
	default:
		return ir.Value{Kind: ir.KindUnit}, fmt.Errorf("unknown unary op '%s'", op)
	}
}

func evalBinary(op string, lhs, rhs ir.Value) (ir.Value, error) {
	switch op {
	case "+", "-", "*", "/", "%":
		if lhs.Kind == ir.KindString && rhs.Kind == ir.KindString && op == "+" {
			return ir.Value{Kind: ir.KindString, Str: lhs.Str + rhs.Str}, nil
		}
		if lhs.Kind != ir.KindInt || rhs.Kind != ir.KindInt {
			return ir.Value{Kind: ir.KindUnit}, errors.New("binary arithmetic requires int")
		}
		switch op {
		case "+":
			return ir.Value{Kind: ir.KindInt, Int: lhs.Int + rhs.Int}, nil
		case "-":
			return ir.Value{Kind: ir.KindInt, Int: lhs.Int - rhs.Int}, nil
		case "*":
			return ir.Value{Kind: ir.KindInt, Int: lhs.Int * rhs.Int}, nil
		case "/":
			if rhs.Int == 0 {
				return ir.Value{Kind: ir.KindUnit}, errors.New("division by zero")
			}
			return ir.Value{Kind: ir.KindInt, Int: lhs.Int / rhs.Int}, nil
		case "%":
			if rhs.Int == 0 {
				return ir.Value{Kind: ir.KindUnit}, errors.New("modulo by zero")
			}
			return ir.Value{Kind: ir.KindInt, Int: lhs.Int % rhs.Int}, nil
		}
	case "==", "!=":
		eq := valuesEqual(lhs, rhs)
		if op == "!=" {
			eq = !eq
		}
		return ir.Value{Kind: ir.KindBool, Bool: eq}, nil
	case "<", "<=", ">", ">=":
		if lhs.Kind != ir.KindInt || rhs.Kind != ir.KindInt {
			return ir.Value{Kind: ir.KindUnit}, errors.New("comparison requires int")
		}
		switch op {
		case "<":
			return ir.Value{Kind: ir.KindBool, Bool: lhs.Int < rhs.Int}, nil
		case "<=":
			return ir.Value{Kind: ir.KindBool, Bool: lhs.Int <= rhs.Int}, nil
		case ">":
			return ir.Value{Kind: ir.KindBool, Bool: lhs.Int > rhs.Int}, nil
		case ">=":
			return ir.Value{Kind: ir.KindBool, Bool: lhs.Int >= rhs.Int}, nil
		}
	case "&&", "||":
		if lhs.Kind != ir.KindBool || rhs.Kind != ir.KindBool {
			return ir.Value{Kind: ir.KindUnit}, errors.New("logical op requires bool")
		}
		if op == "&&" {
			return ir.Value{Kind: ir.KindBool, Bool: lhs.Bool && rhs.Bool}, nil
		}
		return ir.Value{Kind: ir.KindBool, Bool: lhs.Bool || rhs.Bool}, nil
	}
	return ir.Value{Kind: ir.KindUnit}, fmt.Errorf("unknown binary op '%s'", op)
}

func valuesEqual(a, b ir.Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case ir.KindInt:
		return a.Int == b.Int
	case ir.KindBool:
		return a.Bool == b.Bool
	case ir.KindString:
		return a.Str == b.Str
	case ir.KindUnit:
		return true
	default:
		return false
	}
}

func (rt *Runtime) builtinPrint(newline bool) Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		for i, arg := range args {
			if i > 0 {
				if _, err := fmt.Fprint(rt.Stdout, " "); err != nil {
					return ir.Value{Kind: ir.KindUnit}, err
				}
			}
			if _, err := fmt.Fprint(rt.Stdout, formatValue(arg)); err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
		}
		if newline {
			if _, err := fmt.Fprint(rt.Stdout, "\n"); err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
		}
		return ir.Value{Kind: ir.KindUnit}, nil
	}
}

func formatValue(v ir.Value) string {
	switch v.Kind {
	case ir.KindString:
		return v.Str
	case ir.KindStruct:
		return v.Struct.Name + "{...}"
	case ir.KindEnum:
		return v.Enum.Name + "." + v.Enum.Variant
	case ir.KindArray:
		if v.Array == nil {
			return "[]"
		}
		return fmt.Sprintf("[len=%d]", len(v.Array.Elems))
	default:
		return v.String()
	}
}

func (rt *Runtime) builtinLen() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("len expects 1 argument")
		}
		arg := args[0]
		if arg.Kind == ir.KindRef {
			val, err := rt.deref(arg)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			arg = val
		}
		switch arg.Kind {
		case ir.KindString:
			return ir.Value{Kind: ir.KindInt, Int: int64(len(arg.Str))}, nil
		case ir.KindArray:
			if arg.Array == nil {
				return ir.Value{Kind: ir.KindInt, Int: 0}, nil
			}
			return ir.Value{Kind: ir.KindInt, Int: int64(len(arg.Array.Elems))}, nil
		default:
			return ir.Value{Kind: ir.KindUnit}, errors.New("len expects string or array")
		}
	}
}

func (rt *Runtime) builtinPush() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 2 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("push expects 2 arguments")
		}
		target := args[0]
		if target.Kind != ir.KindRef {
			return ir.Value{Kind: ir.KindUnit}, errors.New("push expects &mut array")
		}
		val := args[1]
		arr, err := rt.arrayValue(target)
		if err != nil {
			return ir.Value{Kind: ir.KindUnit}, err
		}
		arr.Elems = append(arr.Elems, val)
		return ir.Value{Kind: ir.KindUnit}, nil
	}
}

func (rt *Runtime) builtinPop() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("pop expects 1 argument")
		}
		target := args[0]
		if target.Kind != ir.KindRef {
			return ir.Value{Kind: ir.KindUnit}, errors.New("pop expects &mut array")
		}
		arr, err := rt.arrayValue(target)
		if err != nil {
			return ir.Value{Kind: ir.KindUnit}, err
		}
		if len(arr.Elems) == 0 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("pop from empty array")
		}
		last := arr.Elems[len(arr.Elems)-1]
		arr.Elems = arr.Elems[:len(arr.Elems)-1]
		return last, nil
	}
}

func (rt *Runtime) builtinReadFile() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("read_file expects 1 argument")
		}
		pathVal := args[0]
		if pathVal.Kind != ir.KindString {
			return ir.Value{Kind: ir.KindUnit}, errors.New("read_file expects string path")
		}
		data, err := os.ReadFile(pathVal.Str)
		if err != nil {
			return ir.Value{Kind: ir.KindUnit}, err
		}
		return ir.Value{Kind: ir.KindString, Str: string(data)}, nil
	}
}

func (rt *Runtime) builtinCharAt() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 2 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("char_at expects 2 arguments")
		}
		strVal := args[0]
		idxVal := args[1]
		if strVal.Kind != ir.KindString || idxVal.Kind != ir.KindInt {
			return ir.Value{Kind: ir.KindUnit}, errors.New("char_at expects string and int")
		}
		idx := int(idxVal.Int)
		if idx < 0 || idx >= len(strVal.Str) {
			return ir.Value{Kind: ir.KindUnit}, errors.New("char_at index out of bounds")
		}
		b := strVal.Str[idx]
		return ir.Value{Kind: ir.KindInt, Int: int64(b)}, nil
	}
}

func (rt *Runtime) builtinSubstr() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 3 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("substr expects 3 arguments")
		}
		strVal := args[0]
		startVal := args[1]
		lenVal := args[2]
		if strVal.Kind != ir.KindString || startVal.Kind != ir.KindInt || lenVal.Kind != ir.KindInt {
			return ir.Value{Kind: ir.KindUnit}, errors.New("substr expects string, int, int")
		}
		start := int(startVal.Int)
		length := int(lenVal.Int)
		if start < 0 || length < 0 || start > len(strVal.Str) || start+length > len(strVal.Str) {
			return ir.Value{Kind: ir.KindUnit}, errors.New("substr out of bounds")
		}
		return ir.Value{Kind: ir.KindString, Str: strVal.Str[start : start+length]}, nil
	}
}

func (rt *Runtime) builtinWriteFile() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 2 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("write_file expects 2 arguments")
		}
		pathVal := args[0]
		dataVal := args[1]
		if pathVal.Kind != ir.KindString || dataVal.Kind != ir.KindString {
			return ir.Value{Kind: ir.KindUnit}, errors.New("write_file expects string path and data")
		}
		if err := os.WriteFile(pathVal.Str, []byte(dataVal.Str), 0o644); err != nil {
			return ir.Value{Kind: ir.KindUnit}, err
		}
		return ir.Value{Kind: ir.KindUnit}, nil
	}
}

func (rt *Runtime) builtinArgs() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 0 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("args expects no arguments")
		}
		elems := make([]ir.Value, 0, len(rt.Args))
		for _, arg := range rt.Args {
			elems = append(elems, ir.Value{Kind: ir.KindString, Str: arg})
		}
		return ir.Value{Kind: ir.KindArray, Array: &ir.ArrayValue{Elems: elems}}, nil
	}
}
