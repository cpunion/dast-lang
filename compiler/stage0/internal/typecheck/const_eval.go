package typecheck

import (
	"strconv"

	"dastlang/internal/ast"
	"dastlang/internal/source"
)

func evalConstExpr(expr ast.Expr) (ast.ConstValue, bool) {
	switch e := expr.(type) {
	case *ast.IntLit:
		return ast.ConstValue{Kind: ast.ConstInt, Int: e.Value, Span: e.Span()}, true
	case *ast.CharLit:
		return ast.ConstValue{Kind: ast.ConstChar, Int: e.Value, Span: e.Span()}, true
	case *ast.FloatLit:
		return ast.ConstValue{Kind: ast.ConstFloat, FloatText: e.Text, Span: e.Span()}, true
	case *ast.BoolLit:
		return ast.ConstValue{Kind: ast.ConstBool, Bool: e.Value, Span: e.Span()}, true
	case *ast.StringLit:
		return ast.ConstValue{Kind: ast.ConstString, Str: e.Value, Span: e.Span()}, true
	case *ast.UnaryExpr:
		val, ok := evalConstExpr(e.Expr)
		if !ok {
			return ast.ConstValue{}, false
		}
		switch e.Op {
		case "-":
			if val.Kind != ast.ConstInt {
				if val.Kind == ast.ConstFloat {
					fv, ok := constFloatVal(val)
					if !ok {
						return ast.ConstValue{}, false
					}
					return ast.ConstValue{Kind: ast.ConstFloat, FloatText: formatFloat(-fv), Span: e.Span()}, true
				}
				return ast.ConstValue{}, false
			}
			return ast.ConstValue{Kind: ast.ConstInt, Int: -val.Int, Span: e.Span()}, true
		case "!":
			if val.Kind != ast.ConstBool {
				return ast.ConstValue{}, false
			}
			return ast.ConstValue{Kind: ast.ConstBool, Bool: !val.Bool, Span: e.Span()}, true
		default:
			return ast.ConstValue{}, false
		}
	case *ast.BinaryExpr:
		lhs, ok := evalConstExpr(e.Left)
		if !ok {
			return ast.ConstValue{}, false
		}
		rhs, ok := evalConstExpr(e.Right)
		if !ok {
			return ast.ConstValue{}, false
		}
		switch e.Op {
		case "+":
			if lhs.Kind == ast.ConstString && rhs.Kind == ast.ConstString {
				return ast.ConstValue{Kind: ast.ConstString, Str: lhs.Str + rhs.Str, Span: e.Span()}, true
			}
			return constNumericBinary(e.Op, lhs, rhs, e.Span())
		case "-", "*", "/":
			return constNumericBinary(e.Op, lhs, rhs, e.Span())
		case "%":
			return constIntBinary(e.Op, lhs, rhs, e.Span())
		case "&", "|", "^":
			return constIntBinary(e.Op, lhs, rhs, e.Span())
		case "<<", ">>":
			return constShiftBinary(e.Op, lhs, rhs, e.Span())
		case "==", "!=":
			return constEqualityBinary(e.Op, lhs, rhs, e.Span())
		case "<", "<=", ">", ">=":
			return constCompareBinary(e.Op, lhs, rhs, e.Span())
		case "&&", "||":
			return constBoolBinary(e.Op, lhs, rhs, e.Span())
		default:
			return ast.ConstValue{}, false
		}
	default:
		return ast.ConstValue{}, false
	}
}

func constIntVal(v ast.ConstValue) (int64, bool) {
	if v.Kind == ast.ConstInt {
		return v.Int, true
	}
	return 0, false
}

func constCharVal(v ast.ConstValue) (int64, bool) {
	if v.Kind == ast.ConstChar {
		return v.Int, true
	}
	return 0, false
}

func constFloatVal(v ast.ConstValue) (float64, bool) {
	if v.Kind != ast.ConstFloat {
		return 0, false
	}
	f, err := strconv.ParseFloat(v.FloatText, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func constBoolVal(v ast.ConstValue) (bool, bool) {
	if v.Kind == ast.ConstBool {
		return v.Bool, true
	}
	return false, false
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func intFitsF64(v int64) bool {
	const maxExact = 9007199254740992
	if v == -9223372036854775808 {
		return false
	}
	if v < 0 {
		v = -v
	}
	return v <= maxExact
}

func promoteFloat(lhs ast.ConstValue, rhs ast.ConstValue) (float64, float64, bool) {
	if lhs.Kind == ast.ConstFloat && rhs.Kind == ast.ConstFloat {
		lf, ok := constFloatVal(lhs)
		if !ok {
			return 0, 0, false
		}
		rf, ok := constFloatVal(rhs)
		if !ok {
			return 0, 0, false
		}
		return lf, rf, true
	}
	if lhs.Kind == ast.ConstFloat && rhs.Kind == ast.ConstInt {
		lf, ok := constFloatVal(lhs)
		if !ok || !intFitsF64(rhs.Int) {
			return 0, 0, false
		}
		return lf, float64(rhs.Int), true
	}
	if lhs.Kind == ast.ConstInt && rhs.Kind == ast.ConstFloat {
		rf, ok := constFloatVal(rhs)
		if !ok || !intFitsF64(lhs.Int) {
			return 0, 0, false
		}
		return float64(lhs.Int), rf, true
	}
	return 0, 0, false
}

func constNumericBinary(op string, lhs ast.ConstValue, rhs ast.ConstValue, span source.Span) (ast.ConstValue, bool) {
	if lhs.Kind == ast.ConstInt && rhs.Kind == ast.ConstInt {
		switch op {
		case "+":
			return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int + rhs.Int, Span: span}, true
		case "-":
			return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int - rhs.Int, Span: span}, true
		case "*":
			return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int * rhs.Int, Span: span}, true
		case "/":
			if rhs.Int == 0 {
				return ast.ConstValue{}, false
			}
			return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int / rhs.Int, Span: span}, true
		}
	}
	lf, rf, ok := promoteFloat(lhs, rhs)
	if ok {
		switch op {
		case "+":
			return ast.ConstValue{Kind: ast.ConstFloat, FloatText: formatFloat(lf + rf), Span: span}, true
		case "-":
			return ast.ConstValue{Kind: ast.ConstFloat, FloatText: formatFloat(lf - rf), Span: span}, true
		case "*":
			return ast.ConstValue{Kind: ast.ConstFloat, FloatText: formatFloat(lf * rf), Span: span}, true
		case "/":
			if rf == 0 {
				return ast.ConstValue{}, false
			}
			return ast.ConstValue{Kind: ast.ConstFloat, FloatText: formatFloat(lf / rf), Span: span}, true
		}
	}
	return ast.ConstValue{}, false
}

func constIntBinary(op string, lhs ast.ConstValue, rhs ast.ConstValue, span source.Span) (ast.ConstValue, bool) {
	if lhs.Kind != ast.ConstInt || rhs.Kind != ast.ConstInt {
		return ast.ConstValue{}, false
	}
	switch op {
	case "%":
		if rhs.Int == 0 {
			return ast.ConstValue{}, false
		}
		return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int % rhs.Int, Span: span}, true
	case "&":
		return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int & rhs.Int, Span: span}, true
	case "|":
		return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int | rhs.Int, Span: span}, true
	case "^":
		return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int ^ rhs.Int, Span: span}, true
	}
	return ast.ConstValue{}, false
}

func constShiftBinary(op string, lhs ast.ConstValue, rhs ast.ConstValue, span source.Span) (ast.ConstValue, bool) {
	if lhs.Kind != ast.ConstInt || rhs.Kind != ast.ConstInt {
		return ast.ConstValue{}, false
	}
	if rhs.Int < 0 {
		return ast.ConstValue{}, false
	}
	switch op {
	case "<<":
		return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int << rhs.Int, Span: span}, true
	case ">>":
		return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int >> rhs.Int, Span: span}, true
	}
	return ast.ConstValue{}, false
}

func constEqualityBinary(op string, lhs ast.ConstValue, rhs ast.ConstValue, span source.Span) (ast.ConstValue, bool) {
	if lhs.Kind == ast.ConstString && rhs.Kind == ast.ConstString {
		if op == "==" {
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Str == rhs.Str, Span: span}, true
		}
		return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Str != rhs.Str, Span: span}, true
	}
	if lhs.Kind == ast.ConstBool && rhs.Kind == ast.ConstBool {
		if op == "==" {
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Bool == rhs.Bool, Span: span}, true
		}
		return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Bool != rhs.Bool, Span: span}, true
	}
	if lhs.Kind == ast.ConstChar && rhs.Kind == ast.ConstChar {
		if op == "==" {
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Int == rhs.Int, Span: span}, true
		}
		return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Int != rhs.Int, Span: span}, true
	}
	if lhs.Kind == ast.ConstInt && rhs.Kind == ast.ConstInt {
		if op == "==" {
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Int == rhs.Int, Span: span}, true
		}
		return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Int != rhs.Int, Span: span}, true
	}
	if lf, rf, ok := promoteFloat(lhs, rhs); ok {
		if op == "==" {
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lf == rf, Span: span}, true
		}
		return ast.ConstValue{Kind: ast.ConstBool, Bool: lf != rf, Span: span}, true
	}
	return ast.ConstValue{}, false
}

func constCompareBinary(op string, lhs ast.ConstValue, rhs ast.ConstValue, span source.Span) (ast.ConstValue, bool) {
	if lhs.Kind == ast.ConstChar && rhs.Kind == ast.ConstChar {
		switch op {
		case "<":
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Int < rhs.Int, Span: span}, true
		case "<=":
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Int <= rhs.Int, Span: span}, true
		case ">":
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Int > rhs.Int, Span: span}, true
		case ">=":
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Int >= rhs.Int, Span: span}, true
		}
	}
	if lhs.Kind == ast.ConstInt && rhs.Kind == ast.ConstInt {
		switch op {
		case "<":
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Int < rhs.Int, Span: span}, true
		case "<=":
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Int <= rhs.Int, Span: span}, true
		case ">":
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Int > rhs.Int, Span: span}, true
		case ">=":
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Int >= rhs.Int, Span: span}, true
		}
	}
	if lf, rf, ok := promoteFloat(lhs, rhs); ok {
		switch op {
		case "<":
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lf < rf, Span: span}, true
		case "<=":
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lf <= rf, Span: span}, true
		case ">":
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lf > rf, Span: span}, true
		case ">=":
			return ast.ConstValue{Kind: ast.ConstBool, Bool: lf >= rf, Span: span}, true
		}
	}
	return ast.ConstValue{}, false
}

func constBoolBinary(op string, lhs ast.ConstValue, rhs ast.ConstValue, span source.Span) (ast.ConstValue, bool) {
	if lhs.Kind != ast.ConstBool || rhs.Kind != ast.ConstBool {
		return ast.ConstValue{}, false
	}
	switch op {
	case "&&":
		return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Bool && rhs.Bool, Span: span}, true
	case "||":
		return ast.ConstValue{Kind: ast.ConstBool, Bool: lhs.Bool || rhs.Bool, Span: span}, true
	}
	return ast.ConstValue{}, false
}
