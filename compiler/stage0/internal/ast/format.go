package ast

import (
	"strconv"
	"strings"
)

func FormatExpr(e Expr) string {
	switch v := e.(type) {
	case *IdentExpr:
		return v.Name
	case *IntLit:
		return strconv.FormatInt(v.Value, 10)
	case *CharLit:
		return "'" + escapeChar(rune(v.Value)) + "'"
	case *FloatLit:
		return v.Text
	case *BoolLit:
		if v.Value {
			return "true"
		}
		return "false"
	case *StringLit:
		return `"` + escapeString(v.Value) + `"`
	case *ArrayLit:
		return "[" + formatExprList(v.Elems) + "]"
	case *TupleLit:
		if len(v.Elems) == 0 {
			return "()"
		}
		if len(v.Elems) == 1 {
			return "(" + FormatExpr(v.Elems[0]) + ",)"
		}
		return "(" + formatExprList(v.Elems) + ")"
	case *UnaryExpr:
		return v.Op + formatExprNested(v.Expr)
	case *RefExpr:
		if v.Mutable {
			return "&mut " + formatExprNested(v.Expr)
		}
		return "&" + formatExprNested(v.Expr)
	case *DerefExpr:
		return "*" + formatExprNested(v.Expr)
	case *BinaryExpr:
		return "(" + FormatExpr(v.Left) + " " + v.Op + " " + FormatExpr(v.Right) + ")"
	case *CallExpr:
		if len(v.TypeArgs) > 0 {
			return v.Callee + "::[" + formatTypeArgs(v.TypeArgs) + "](" + formatExprList(v.Args) + ")"
		}
		return v.Callee + "(" + formatExprList(v.Args) + ")"
	case *MethodCallExpr:
		return FormatExpr(v.Receiver) + "." + v.Method + "(" + formatExprList(v.Args) + ")"
	case *ClosureExpr:
		parts := make([]string, 0, len(v.Params))
		for _, p := range v.Params {
			entry := p.Name
			if p.Type != nil {
				entry += ": " + FormatType(*p.Type)
			}
			parts = append(parts, entry)
		}
		out := "|" + strings.Join(parts, ", ") + "|"
		if v.ReturnType != nil {
			out += " -> " + FormatType(*v.ReturnType)
		}
		out += " " + FormatExpr(v.Body)
		return out
	case *StructLit:
		var parts []string
		for _, f := range v.Fields {
			parts = append(parts, f.Name+": "+FormatExpr(f.Value))
		}
		name := v.Name
		if len(v.TypeArgs) > 0 {
			name += "[" + formatTypeArgs(v.TypeArgs) + "]"
		}
		return name + " { " + strings.Join(parts, ", ") + " }"
	case *AccessExpr:
		return FormatExpr(v.Receiver) + "." + v.Field
	case *IndexExpr:
		return FormatExpr(v.Receiver) + "[" + FormatExpr(v.Index) + "]"
	case *EnumVariantExpr:
		name := v.EnumName
		if len(v.TypeArgs) > 0 {
			name += "[" + formatTypeArgs(v.TypeArgs) + "]"
		}
		if v.Arg == nil {
			return name + "." + v.Variant
		}
		return name + "." + v.Variant + "(" + FormatExpr(v.Arg) + ")"
	case *BlockExpr:
		return FormatBlock(v.Block)
	case *LoopExpr:
		return "loop " + FormatBlock(v.Body)
	case *IfExpr:
		return "if " + FormatExpr(v.Cond) + " " + formatBlockExpr(v.Then) + " else " + formatBlockExpr(v.Else)
	case *MatchExpr:
		var parts []string
		for _, arm := range v.Arms {
			parts = append(parts, formatMatchArm(arm))
		}
		return "match " + FormatExpr(v.Expr) + " { " + strings.Join(parts, ", ") + " }"
	case *CompileExpr:
		return "compile!(" + FormatExpr(v.Expr) + ")"
	case *MacroCallExpr:
		return FormatExpr(v.Callee) + "!(" + formatExprList(v.Args) + ")"
	case *QuoteExpr:
		return formatQuoteExpr(v)
	default:
		return "<expr>"
	}
}

func FormatStmt(s Stmt) string {
	switch v := s.(type) {
	case *Block:
		return FormatBlock(v)
	case *LetStmt:
		out := "let "
		if v.Mutable {
			out += "mut "
		}
		out += v.Name
		if v.Type != nil {
			out += ": " + FormatType(*v.Type)
		}
		out += " = " + FormatExpr(v.Init) + ";"
		return out
	case *LetPatternStmt:
		out := "let "
		if v.Mutable {
			out += "mut "
		}
		out += FormatPattern(v.Pattern)
		if v.Type != nil {
			out += ": " + FormatType(*v.Type)
		}
		out += " = " + FormatExpr(v.Init) + ";"
		return out
	case *AssignStmt:
		return FormatExpr(v.Target) + " = " + FormatExpr(v.Value) + ";"
	case *ExprStmt:
		return FormatExpr(v.Expr) + ";"
	case *ReturnStmt:
		if v.Value == nil {
			return "return;"
		}
		return "return " + FormatExpr(v.Value) + ";"
	case *IfStmt:
		out := "if " + FormatExpr(v.Cond) + " " + FormatBlock(v.Then)
		if v.Else != nil {
			out += " else " + FormatBlock(v.Else)
		}
		return out
	case *IfLetStmt:
		out := "if let " + FormatPattern(v.Pattern) + " = " + FormatExpr(v.Expr) + " " + FormatBlock(v.Then)
		if v.Else != nil {
			out += " else " + FormatBlock(v.Else)
		}
		return out
	case *WhileStmt:
		prefix := ""
		if v.Label != "" {
			prefix = v.Label + ": "
		}
		return prefix + "while " + FormatExpr(v.Cond) + " " + FormatBlock(v.Body)
	case *WhileLetStmt:
		prefix := ""
		if v.Label != "" {
			prefix = v.Label + ": "
		}
		return prefix + "while let " + FormatPattern(v.Pattern) + " = " + FormatExpr(v.Expr) + " " + FormatBlock(v.Body)
	case *LoopStmt:
		prefix := ""
		if v.Label != "" {
			prefix = v.Label + ": "
		}
		return prefix + "loop " + FormatBlock(v.Body)
	case *BreakStmt:
		out := "break"
		if v.Label != "" {
			out += " " + v.Label
		}
		if v.Value != nil {
			if v.Label != "" {
				out += ": " + FormatExpr(v.Value)
			} else {
				out += " " + FormatExpr(v.Value)
			}
		}
		return out + ";"
	case *ContinueStmt:
		if v.Label != "" {
			return "continue " + v.Label + ";"
		}
		return "continue;"
	case *ForStmt:
		prefix := ""
		if v.Label != "" {
			prefix = v.Label + ": "
		}
		return prefix + "for " + FormatPattern(v.Pattern) + " in " + FormatExpr(v.Expr) + " " + FormatBlock(v.Body)
	case *MatchStmt:
		var parts []string
		for _, arm := range v.Arms {
			parts = append(parts, formatMatchArm(arm))
		}
		return "match " + FormatExpr(v.Expr) + " { " + strings.Join(parts, ", ") + " }"
	default:
		return "<stmt>"
	}
}

func FormatBlock(b *Block) string {
	if b == nil {
		return "{ }"
	}
	var parts []string
	for _, stmt := range b.Stmts {
		parts = append(parts, FormatStmt(stmt))
	}
	return "{ " + strings.Join(parts, " ") + " }"
}

func FormatItem(i Item) string {
	switch v := i.(type) {
	case *Function:
		out := formatVisibility(v.Vis)
		if v.IsMacro {
			out += "macro "
		}
		out += "fn " + v.Name
		if len(v.TypeParams) > 0 {
			out += "[" + formatTypeParams(v.TypeParams) + "]"
		}
		out += "(" + formatParams(v.Params) + ")"
		if v.ReturnType != nil {
			out += " -> " + FormatType(*v.ReturnType)
		}
		out += " " + FormatBlock(v.Body)
		return out
	case *StructDecl:
		out := ""
		if v.Repr != "" {
			out += "@repr(" + v.Repr + ") "
		}
		out += formatVisibility(v.Vis) + "struct " + v.Name
		if len(v.TypeParams) > 0 {
			out += "[" + formatTypeParams(v.TypeParams) + "]"
		}
		out += " { "
		var fields []string
		for _, f := range v.Fields {
			fields = append(fields, f.Name+": "+FormatType(f.Type))
		}
		out += strings.Join(fields, ", ") + " }"
		return out
	case *EnumDecl:
		out := ""
		if v.Repr != "" {
			out += "@repr(" + v.Repr + ") "
		}
		out += formatVisibility(v.Vis) + "enum " + v.Name
		if len(v.TypeParams) > 0 {
			out += "[" + formatTypeParams(v.TypeParams) + "]"
		}
		out += " { "
		var variants []string
		for _, variant := range v.Variants {
			entry := variant.Name
			if variant.Payload != nil {
				entry += "(" + FormatType(*variant.Payload) + ")"
			}
			if variant.HasValue {
				entry += " = " + strconv.FormatInt(variant.Value, 10)
			}
			variants = append(variants, entry)
		}
		out += strings.Join(variants, ", ") + " }"
		return out
	case *ConstDecl:
		out := formatVisibility(v.Vis) + "const " + v.Name
		if v.Type != nil {
			out += ": " + FormatType(*v.Type)
		}
		if v.Expr != nil {
			out += " = " + FormatExpr(v.Expr) + ";"
		} else {
			out += " = " + formatConstValue(v.Value) + ";"
		}
		return out
	case *ImplDecl:
		var methods []string
		for _, m := range v.Methods {
			methods = append(methods, FormatItem(m))
		}
		out := formatVisibility(v.Vis) + "impl "
		if len(v.TypeParams) > 0 {
			out += "[" + formatTypeParams(v.TypeParams) + "] "
		}
		out += v.TypeName
		if len(v.TypeArgs) > 0 {
			out += "[" + formatTypeArgs(v.TypeArgs) + "]"
		}
		out += " { " + strings.Join(methods, " ") + " }"
		return out
	case *TraitDecl:
		out := formatVisibility(v.Vis) + "trait " + v.Name
		if len(v.TypeParams) > 0 {
			out += "[" + formatTypeParams(v.TypeParams) + "]"
		}
		var methods []string
		for _, m := range v.Methods {
			methods = append(methods, formatTraitMethod(m))
		}
		return out + " { " + strings.Join(methods, " ") + " }"
	case *ImplTraitDecl:
		out := formatVisibility(v.Vis) + "impl "
		if len(v.TypeParams) > 0 {
			out += "[" + formatTypeParams(v.TypeParams) + "] "
		}
		out += v.TraitName
		if len(v.TraitArgs) > 0 {
			out += "[" + formatTypeArgs(v.TraitArgs) + "]"
		}
		out += " for " + v.ForTypeName
		if len(v.ForTypeArgs) > 0 {
			out += "[" + formatTypeArgs(v.ForTypeArgs) + "]"
		}
		var methods []string
		for _, m := range v.Methods {
			methods = append(methods, FormatItem(m))
		}
		return out + " { " + strings.Join(methods, " ") + " }"
	case *TypeAlias:
		out := formatVisibility(v.Vis) + "type " + v.Name
		if len(v.TypeParams) > 0 {
			out += "[" + formatTypeParams(v.TypeParams) + "]"
		}
		out += " = " + FormatType(v.Value) + ";"
		return out
	case *ImportDecl:
		out := "import \"" + escapeString(v.Path) + "\""
		if v.Alias != "" {
			out += " as " + v.Alias
		}
		return out + ";"
	default:
		return "<item>"
	}
}

func FormatType(t Type) string {
	base := t.Name
	if t.IsTuple {
		if len(t.TupleElems) == 0 {
			base = "()"
		} else if len(t.TupleElems) == 1 {
			base = "(" + FormatType(t.TupleElems[0]) + ",)"
		} else {
			base = "(" + formatTypeArgs(t.TupleElems) + ")"
		}
	}
	if t.IsArray {
		if t.Elem != nil {
			base = "[" + FormatType(*t.Elem) + "]"
		} else {
			base = "[]"
		}
	}
	if len(t.Args) > 0 && !t.IsArray {
		base = base + "[" + formatTypeArgs(t.Args) + "]"
	}
	if t.IsRef {
		if t.IsMut {
			return "&mut " + base
		}
		return "&" + base
	}
	return base
}

func FormatPattern(p Pattern) string {
	switch v := p.(type) {
	case *WildcardPattern:
		return "_"
	case *BindingPattern:
		return v.Name
	case *VariantPattern:
		name := v.Variant
		if v.EnumName != "" {
			name = v.EnumName + "." + v.Variant
		} else {
			name = "." + v.Variant
		}
		if v.Payload != nil {
			return name + "(" + FormatPattern(v.Payload) + ")"
		}
		if v.Binding != "" {
			return name + "(" + v.Binding + ")"
		}
		return name
	case *LiteralPattern:
		return formatConstValue(v.Value)
	case *RangePattern:
		dots := ".."
		if v.Inclusive {
			dots = "..="
		}
		start := ""
		end := ""
		if v.Start != nil {
			start = FormatExpr(v.Start)
		}
		if v.End != nil {
			end = FormatExpr(v.End)
		}
		return start + dots + end
	case *OrPattern:
		var parts []string
		for _, alt := range v.Alts {
			parts = append(parts, FormatPattern(alt))
		}
		return strings.Join(parts, " | ")
	case *StructPattern:
		var fields []string
		for _, f := range v.Fields {
			if f.Pattern == nil {
				if f.Binding != "" && f.Binding != f.Name {
					fields = append(fields, f.Name+": "+f.Binding)
				} else {
					fields = append(fields, f.Name)
				}
				continue
			}
			fields = append(fields, f.Name+": "+FormatPattern(f.Pattern))
		}
		return v.StructName + " { " + strings.Join(fields, ", ") + " }"
	case *TuplePattern:
		if len(v.Elems) == 0 {
			return "()"
		}
		if len(v.Elems) == 1 {
			return "(" + FormatPattern(v.Elems[0]) + ",)"
		}
		var parts []string
		for _, e := range v.Elems {
			parts = append(parts, FormatPattern(e))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case *ArrayPattern:
		var parts []string
		for _, e := range v.Elems {
			parts = append(parts, FormatPattern(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return "_"
	}
}

func formatVisibility(vis Visibility) string {
	if vis == VisPublic {
		return "pub "
	}
	return ""
}

func formatExprList(exprs []Expr) string {
	if len(exprs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(exprs))
	for _, e := range exprs {
		parts = append(parts, FormatExpr(e))
	}
	return strings.Join(parts, ", ")
}

func formatParams(params []Param) string {
	if len(params) == 0 {
		return ""
	}
	parts := make([]string, 0, len(params))
	for _, p := range params {
		parts = append(parts, p.Name+": "+FormatType(p.Type))
	}
	return strings.Join(parts, ", ")
}

func formatTypeParams(params []TypeParam) string {
	if len(params) == 0 {
		return ""
	}
	parts := make([]string, 0, len(params))
	for _, p := range params {
		entry := p.Name
		if len(p.Bounds) > 0 {
			bounds := make([]string, 0, len(p.Bounds))
			for _, b := range p.Bounds {
				bounds = append(bounds, FormatType(b))
			}
			entry += ": " + strings.Join(bounds, " + ")
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, ", ")
}

func formatTypeArgs(args []Type) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, FormatType(a))
	}
	return strings.Join(parts, ", ")
}

func formatTraitMethod(m TraitMethod) string {
	out := "fn " + m.Name + "(" + formatParams(m.Params) + ")"
	if m.ReturnType != nil {
		out += " -> " + FormatType(*m.ReturnType)
	}
	return out + ";"
}

func formatConstValue(v ConstValue) string {
	switch v.Kind {
	case ConstBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case ConstString:
		return `"` + escapeString(v.Str) + `"`
	case ConstChar:
		return "'" + escapeChar(rune(v.Int)) + "'"
	case ConstFloat:
		return v.FloatText
	default:
		return strconv.FormatInt(v.Int, 10)
	}
}

func formatExprNested(e Expr) string {
	switch e.(type) {
	case *IdentExpr, *IntLit, *CharLit, *FloatLit, *BoolLit, *StringLit:
		return FormatExpr(e)
	default:
		return "(" + FormatExpr(e) + ")"
	}
}

func escapeChar(r rune) string {
	switch r {
	case '\\':
		return "\\\\"
	case '\'':
		return "\\'"
	case '\n':
		return "\\n"
	case '\t':
		return "\\t"
	case '\r':
		return "\\r"
	default:
		return string(r)
	}
}

func formatBlockExpr(e Expr) string {
	if block, ok := e.(*BlockExpr); ok {
		return FormatBlock(block.Block)
	}
	return "{ " + FormatExpr(e) + " }"
}

func formatMatchArm(arm MatchArm) string {
	out := FormatPattern(arm.Pattern)
	if arm.Guard != nil {
		out += " if " + FormatExpr(arm.Guard)
	}
	out += " => " + FormatBlock(arm.Body)
	return out
}

func formatQuoteExpr(q *QuoteExpr) string {
	var parts []string
	for _, part := range q.Parts {
		if part.Expr == nil {
			parts = append(parts, part.Text)
			continue
		}
		parts = append(parts, "$("+FormatExpr(part.Expr)+")")
	}
	return "quote { " + strings.Join(parts, "") + " }"
}

func escapeString(s string) string {
	var out []rune
	for _, r := range s {
		switch r {
		case '\\':
			out = append(out, '\\', '\\')
		case '"':
			out = append(out, '\\', '"')
		case '\n':
			out = append(out, '\\', 'n')
		case '\t':
			out = append(out, '\\', 't')
		case '\r':
			out = append(out, '\\', 'r')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}
