package interp

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"dastlang/internal/ir"
)

func evalUnary(op string, v ir.Value) (ir.Value, error) {
	switch op {
	case "-":
		if v.Kind != ir.KindInt {
			return ir.Value{Kind: ir.KindUnit}, errors.New("unary '-' requires int")
		}
		return ir.Value{Kind: ir.KindInt, Int: -v.Int, IntType: v.IntType}, nil
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
		intType := pickIntType(lhs, rhs)
		switch op {
		case "+":
			return ir.Value{Kind: ir.KindInt, Int: lhs.Int + rhs.Int, IntType: intType}, nil
		case "-":
			return ir.Value{Kind: ir.KindInt, Int: lhs.Int - rhs.Int, IntType: intType}, nil
		case "*":
			return ir.Value{Kind: ir.KindInt, Int: lhs.Int * rhs.Int, IntType: intType}, nil
		case "/":
			if rhs.Int == 0 {
				return ir.Value{Kind: ir.KindUnit}, errors.New("division by zero")
			}
			return ir.Value{Kind: ir.KindInt, Int: lhs.Int / rhs.Int, IntType: intType}, nil
		case "%":
			if rhs.Int == 0 {
				return ir.Value{Kind: ir.KindUnit}, errors.New("modulo by zero")
			}
			return ir.Value{Kind: ir.KindInt, Int: lhs.Int % rhs.Int, IntType: intType}, nil
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
	case ir.KindStruct:
		if a.Struct == nil || b.Struct == nil {
			return a.Struct == b.Struct
		}
		if a.Struct.Name != b.Struct.Name {
			return false
		}
		if len(a.Struct.Fields) != len(b.Struct.Fields) {
			return false
		}
		for name, aVal := range a.Struct.Fields {
			bVal, ok := b.Struct.Fields[name]
			if !ok || !valuesEqual(aVal, bVal) {
				return false
			}
		}
		return true
	case ir.KindEnum:
		if a.Enum == nil || b.Enum == nil {
			return a.Enum == b.Enum
		}
		if a.Enum.Name != b.Enum.Name || a.Enum.Variant != b.Enum.Variant {
			return false
		}
		if a.Enum.Payload == nil || b.Enum.Payload == nil {
			return a.Enum.Payload == b.Enum.Payload
		}
		return valuesEqual(*a.Enum.Payload, *b.Enum.Payload)
	case ir.KindAst:
		return a.AstKind == b.AstKind && a.AstSrc == b.AstSrc
	default:
		return false
	}
}

func (rt *Runtime) builtinPrintTo(w io.Writer, newline bool) Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		for i, arg := range args {
			if i > 0 {
				if _, err := fmt.Fprint(w, " "); err != nil {
					return ir.Value{Kind: ir.KindUnit}, err
				}
			}
			if _, err := fmt.Fprint(w, formatValue(arg)); err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
		}
		if newline {
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
		}
		return ir.Value{Kind: ir.KindUnit}, nil
	}
}

func (rt *Runtime) builtinPrint(newline bool) Builtin {
	return rt.builtinPrintTo(rt.Stdout, newline)
}

func (rt *Runtime) builtinEprint(newline bool) Builtin {
	return rt.builtinPrintTo(os.Stderr, newline)
}

func formatValue(v ir.Value) string {
	switch v.Kind {
	case ir.KindInt:
		return fmt.Sprintf("%d", v.Int)
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
	case ir.KindAst:
		return v.AstSrc
	default:
		return v.String()
	}
}

func pickIntType(a, b ir.Value) string {
	if a.IntType != "" {
		return a.IntType
	}
	if b.IntType != "" {
		return b.IntType
	}
	return ""
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

func (rt *Runtime) builtinExit() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		code := 0
		if len(args) > 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("exit expects 0 or 1 argument")
		}
		if len(args) == 1 {
			if args[0].Kind != ir.KindInt {
				return ir.Value{Kind: ir.KindUnit}, errors.New("exit expects int")
			}
			code = int(args[0].Int)
		}
		return ir.Value{Kind: ir.KindUnit}, ExitError{Code: code}
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
		if pathVal.Kind == ir.KindRef {
			val, err := rt.deref(pathVal)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			pathVal = val
		}
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

func (rt *Runtime) builtinReadDir() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("read_dir expects 1 argument")
		}
		pathVal := args[0]
		if pathVal.Kind == ir.KindRef {
			val, err := rt.deref(pathVal)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			pathVal = val
		}
		if pathVal.Kind != ir.KindString {
			return ir.Value{Kind: ir.KindUnit}, errors.New("read_dir expects string path")
		}
		entries, err := os.ReadDir(pathVal.Str)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
				return ir.Value{Kind: ir.KindArray, Array: &ir.ArrayValue{Elems: nil}}, nil
			}
			return ir.Value{Kind: ir.KindUnit}, err
		}
		elems := make([]ir.Value, 0, len(entries))
		for _, entry := range entries {
			elems = append(elems, ir.Value{Kind: ir.KindString, Str: entry.Name()})
		}
		return ir.Value{Kind: ir.KindArray, Array: &ir.ArrayValue{Elems: elems}}, nil
	}
}

func (rt *Runtime) builtinCharAt() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 2 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("char_at expects 2 arguments")
		}
		strVal := args[0]
		if strVal.Kind == ir.KindRef {
			val, err := rt.deref(strVal)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			strVal = val
		}
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
		if strVal.Kind == ir.KindRef {
			val, err := rt.deref(strVal)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			strVal = val
		}
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
		if pathVal.Kind == ir.KindRef {
			val, err := rt.deref(pathVal)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			pathVal = val
		}
		if dataVal.Kind == ir.KindRef {
			val, err := rt.deref(dataVal)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			dataVal = val
		}
		if pathVal.Kind != ir.KindString || dataVal.Kind != ir.KindString {
			return ir.Value{Kind: ir.KindUnit}, errors.New("write_file expects string path and data")
		}
		if err := os.WriteFile(pathVal.Str, []byte(dataVal.Str), 0o644); err != nil {
			return ir.Value{Kind: ir.KindUnit}, err
		}
		return ir.Value{Kind: ir.KindUnit}, nil
	}
}

func (rt *Runtime) builtinMkdir() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("mkdir expects 1 argument")
		}
		pathVal := args[0]
		if pathVal.Kind == ir.KindRef {
			val, err := rt.deref(pathVal)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			pathVal = val
		}
		if pathVal.Kind != ir.KindString {
			return ir.Value{Kind: ir.KindUnit}, errors.New("mkdir expects string path")
		}
		if err := os.MkdirAll(pathVal.Str, 0o755); err != nil {
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

func (rt *Runtime) builtinReadLine() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 0 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("read_line expects no arguments")
		}
		// Read line from stdin (until \n or \r\n)
		var line []byte
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				return ir.Value{Kind: ir.KindUnit}, err
			}
			if n == 0 {
				break
			}
			if buf[0] == '\n' {
				break
			}
			if buf[0] != '\r' {
				line = append(line, buf[0])
			}
		}
		return ir.Value{Kind: ir.KindString, Str: string(line)}, nil
	}
}

func (rt *Runtime) builtinReadBytes() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("read_bytes expects 1 argument")
		}
		countVal := args[0]
		if countVal.Kind == ir.KindRef {
			val, err := rt.deref(countVal)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			countVal = val
		}
		if countVal.Kind != ir.KindInt {
			return ir.Value{Kind: ir.KindUnit}, errors.New("read_bytes expects int count")
		}
		count := int(countVal.Int)
		if count < 0 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("read_bytes count must be non-negative")
		}
		buf := make([]byte, count)
		totalRead := 0
		for totalRead < count {
			n, err := os.Stdin.Read(buf[totalRead:])
			if err != nil {
				if err == io.EOF {
					break
				}
				return ir.Value{Kind: ir.KindUnit}, err
			}
			totalRead += n
		}
		return ir.Value{Kind: ir.KindString, Str: string(buf[:totalRead])}, nil
	}
}

func (rt *Runtime) builtinExec() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 2 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("exec expects 2 arguments")
		}
		cmdVal := args[0]
		if cmdVal.Kind == ir.KindRef {
			val, err := rt.deref(cmdVal)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			cmdVal = val
		}
		if cmdVal.Kind != ir.KindString {
			return ir.Value{Kind: ir.KindUnit}, errors.New("exec expects string command")
		}
		if args[1].Kind != ir.KindArray || args[1].Array == nil {
			return ir.Value{Kind: ir.KindUnit}, errors.New("exec expects [string] args")
		}
		argv := make([]string, 0, len(args[1].Array.Elems))
		for _, v := range args[1].Array.Elems {
			if v.Kind != ir.KindString {
				return ir.Value{Kind: ir.KindUnit}, errors.New("exec expects [string] args")
			}
			argv = append(argv, v.Str)
		}
		cmd := exec.Command(cmdVal.Str, argv...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = rt.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return ir.Value{Kind: ir.KindInt, Int: int64(exitErr.ExitCode())}, nil
			}
			return ir.Value{Kind: ir.KindUnit}, err
		}
		return ir.Value{Kind: ir.KindInt, Int: 0}, nil
	}
}

func (rt *Runtime) builtinAst(kind ir.AstKind) Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 && len(args) != 2 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("ast_* expects 1 or 2 arguments")
		}
		if args[0].Kind != ir.KindString {
			return ir.Value{Kind: ir.KindUnit}, errors.New("ast_* expects string")
		}
		if len(args) == 1 {
			return ir.Value{Kind: ir.KindAst, AstKind: kind, AstSrc: args[0].Str}, nil
		}
		if args[1].Kind != ir.KindArray || args[1].Array == nil {
			return ir.Value{Kind: ir.KindUnit}, errors.New("ast_* expects [Ast] splices")
		}
		src, err := rt.buildQuotedAst(kind, args[0].Str, args[1].Array.Elems)
		if err != nil {
			return ir.Value{Kind: ir.KindUnit}, err
		}
		return ir.Value{Kind: ir.KindAst, AstKind: kind, AstSrc: src}, nil
	}
}

func (rt *Runtime) builtinAstToString() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("ast_to_string expects 1 argument")
		}
		if args[0].Kind != ir.KindAst {
			return ir.Value{Kind: ir.KindUnit}, errors.New("ast_to_string expects ast")
		}
		return ir.Value{Kind: ir.KindString, Str: args[0].AstSrc}, nil
	}
}

func (rt *Runtime) builtinGensym() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("gensym expects 1 argument")
		}
		if args[0].Kind != ir.KindString {
			return ir.Value{Kind: ir.KindUnit}, errors.New("gensym expects string")
		}
		prefix := sanitizeIdent(args[0].Str)
		if prefix == "" {
			prefix = "tmp"
		}
		name := fmt.Sprintf("__dast_%s_%d", prefix, rt.gensymCount)
		rt.gensymCount++
		return ir.Value{Kind: ir.KindAst, AstKind: ir.AstExpr, AstSrc: name}, nil
	}
}

func (rt *Runtime) builtinBind() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("bind expects 1 argument")
		}
		if args[0].Kind != ir.KindString {
			return ir.Value{Kind: ir.KindUnit}, errors.New("bind expects string")
		}
		name := sanitizeIdent(args[0].Str)
		if name == "" {
			return ir.Value{Kind: ir.KindUnit}, errors.New("bind expects non-empty name")
		}
		return ir.Value{Kind: ir.KindAst, AstKind: ir.AstExpr, AstSrc: name}, nil
	}
}

func (rt *Runtime) builtinStringClone() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("string_clone expects 1 argument")
		}
		arg := args[0]
		if arg.Kind == ir.KindRef {
			val, err := rt.deref(arg)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			arg = val
		}
		if arg.Kind != ir.KindString {
			return ir.Value{Kind: ir.KindUnit}, errors.New("string_clone expects string")
		}
		return ir.Value{Kind: ir.KindString, Str: arg.Str}, nil
	}
}

func (rt *Runtime) builtinStringFree() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("string_free expects 1 argument")
		}
		return ir.Value{Kind: ir.KindUnit}, nil
	}
}

func (rt *Runtime) builtinArrayFree() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("array_free expects 1 argument")
		}
		return ir.Value{Kind: ir.KindUnit}, nil
	}
}

func (rt *Runtime) builtinStructFree() Builtin {
	return func(args []ir.Value) (ir.Value, error) {
		if len(args) != 1 {
			return ir.Value{Kind: ir.KindUnit}, errors.New("struct_free expects 1 argument")
		}
		return ir.Value{Kind: ir.KindUnit}, nil
	}
}

func sanitizeIdent(s string) string {
	var out []rune
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}
