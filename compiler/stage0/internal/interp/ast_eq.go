package interp

import (
	"errors"

	"dastlang/internal/ast"
	"dastlang/internal/ir"
	"dastlang/internal/parser"
)

func (rt *Runtime) parseAstValue(val ir.Value) (ir.AstKind, any, error) {
	if val.Kind != ir.KindAst {
		return 0, nil, errors.New("ast_eq expects ast")
	}
	structs := rt.structNameSet()
	switch val.AstKind {
	case ir.AstExpr:
		expr, diags := parser.ParseExprString("<ast_eq>", val.AstSrc, structs)
		if diags != nil && len(diags.Items) > 0 {
			return val.AstKind, nil, errors.New(diags.Items[0].Message)
		}
		return val.AstKind, expr, nil
	case ir.AstStmt:
		stmt, diags := parser.ParseStmtString("<ast_eq>", val.AstSrc, structs)
		if diags != nil && len(diags.Items) > 0 {
			return val.AstKind, nil, errors.New(diags.Items[0].Message)
		}
		return val.AstKind, stmt, nil
	case ir.AstBlock:
		block, diags := parser.ParseBlockString("<ast_eq>", val.AstSrc, structs)
		if diags != nil && len(diags.Items) > 0 {
			return val.AstKind, nil, errors.New(diags.Items[0].Message)
		}
		return val.AstKind, block, nil
	case ir.AstItem:
		item, diags := parser.ParseItemString("<ast_eq>", val.AstSrc, structs)
		if diags != nil && len(diags.Items) > 0 {
			return val.AstKind, nil, errors.New(diags.Items[0].Message)
		}
		return val.AstKind, item, nil
	default:
		return val.AstKind, nil, errors.New("unknown ast kind")
	}
}

func astExprEq(a, b ast.Expr) bool {
	if a == nil || b == nil {
		return a == b
	}
	switch ta := a.(type) {
	case *ast.IdentExpr:
		tb, ok := b.(*ast.IdentExpr)
		return ok && ta.Name == tb.Name
	case *ast.IntLit:
		tb, ok := b.(*ast.IntLit)
		return ok && ta.Value == tb.Value
	case *ast.CharLit:
		tb, ok := b.(*ast.CharLit)
		return ok && ta.Value == tb.Value
	case *ast.FloatLit:
		tb, ok := b.(*ast.FloatLit)
		return ok && ta.Text == tb.Text
	case *ast.BoolLit:
		tb, ok := b.(*ast.BoolLit)
		return ok && ta.Value == tb.Value
	case *ast.StringLit:
		tb, ok := b.(*ast.StringLit)
		return ok && ta.Value == tb.Value
	case *ast.ArrayLit:
		tb, ok := b.(*ast.ArrayLit)
		return ok && exprListEq(ta.Elems, tb.Elems)
	case *ast.TupleLit:
		tb, ok := b.(*ast.TupleLit)
		return ok && exprListEq(ta.Elems, tb.Elems)
	case *ast.UnaryExpr:
		tb, ok := b.(*ast.UnaryExpr)
		return ok && ta.Op == tb.Op && astExprEq(ta.Expr, tb.Expr)
	case *ast.RefExpr:
		tb, ok := b.(*ast.RefExpr)
		return ok && ta.Mutable == tb.Mutable && astExprEq(ta.Expr, tb.Expr)
	case *ast.CompileExpr:
		tb, ok := b.(*ast.CompileExpr)
		return ok && ta.ID == tb.ID && astExprEq(ta.Expr, tb.Expr)
	case *ast.QuoteExpr:
		tb, ok := b.(*ast.QuoteExpr)
		return ok && ta.Kind == tb.Kind && quotePartListEq(ta.Parts, tb.Parts)
	case *ast.MacroCallExpr:
		tb, ok := b.(*ast.MacroCallExpr)
		return ok && astExprEq(ta.Callee, tb.Callee) && exprListEq(ta.Args, tb.Args)
	case *ast.DerefExpr:
		tb, ok := b.(*ast.DerefExpr)
		return ok && astExprEq(ta.Expr, tb.Expr)
	case *ast.BinaryExpr:
		tb, ok := b.(*ast.BinaryExpr)
		return ok && ta.Op == tb.Op && astExprEq(ta.Left, tb.Left) && astExprEq(ta.Right, tb.Right)
	case *ast.CallExpr:
		tb, ok := b.(*ast.CallExpr)
		return ok && ta.Callee == tb.Callee && typeListEq(ta.TypeArgs, tb.TypeArgs) && exprListEq(ta.Args, tb.Args)
	case *ast.MethodCallExpr:
		tb, ok := b.(*ast.MethodCallExpr)
		return ok && ta.Method == tb.Method && ta.ResolvedName == tb.ResolvedName && ta.ResolvedSelf == tb.ResolvedSelf && ta.EnumName == tb.EnumName &&
			astExprEq(ta.Receiver, tb.Receiver) && exprListEq(ta.Args, tb.Args)
	case *ast.ClosureExpr:
		tb, ok := b.(*ast.ClosureExpr)
		return ok && closureParamListEq(ta.Params, tb.Params) && typePtrEq(ta.ReturnType, tb.ReturnType) && astExprEq(ta.Body, tb.Body)
	case *ast.StructLit:
		tb, ok := b.(*ast.StructLit)
		return ok && ta.Name == tb.Name && typeListEq(ta.TypeArgs, tb.TypeArgs) && fieldInitListEq(ta.Fields, tb.Fields)
	case *ast.AccessExpr:
		tb, ok := b.(*ast.AccessExpr)
		return ok && ta.Field == tb.Field && astExprEq(ta.Receiver, tb.Receiver)
	case *ast.IndexExpr:
		tb, ok := b.(*ast.IndexExpr)
		return ok && astExprEq(ta.Receiver, tb.Receiver) && astExprEq(ta.Index, tb.Index)
	case *ast.EnumVariantExpr:
		tb, ok := b.(*ast.EnumVariantExpr)
		return ok && ta.EnumName == tb.EnumName && ta.Variant == tb.Variant && typeListEq(ta.TypeArgs, tb.TypeArgs) && astExprEq(ta.Arg, tb.Arg)
	case *ast.BlockExpr:
		tb, ok := b.(*ast.BlockExpr)
		return ok && astBlockEq(ta.Block, tb.Block)
	case *ast.LoopExpr:
		tb, ok := b.(*ast.LoopExpr)
		return ok && astBlockEq(ta.Body, tb.Body)
	case *ast.IfExpr:
		tb, ok := b.(*ast.IfExpr)
		return ok && astExprEq(ta.Cond, tb.Cond) && astExprEq(ta.Then, tb.Then) && astExprEq(ta.Else, tb.Else)
	case *ast.MatchExpr:
		tb, ok := b.(*ast.MatchExpr)
		return ok && astExprEq(ta.Expr, tb.Expr) && matchArmListEq(ta.Arms, tb.Arms)
	default:
		return false
	}
}

func astStmtEq(a, b ast.Stmt) bool {
	if a == nil || b == nil {
		return a == b
	}
	switch ta := a.(type) {
	case *ast.Block:
		tb, ok := b.(*ast.Block)
		return ok && astBlockEq(ta, tb)
	case *ast.LetStmt:
		tb, ok := b.(*ast.LetStmt)
		return ok && ta.Name == tb.Name && ta.Mutable == tb.Mutable && typePtrEq(ta.Type, tb.Type) && astExprEq(ta.Init, tb.Init)
	case *ast.LetPatternStmt:
		tb, ok := b.(*ast.LetPatternStmt)
		return ok && astPatternEq(ta.Pattern, tb.Pattern) && astExprEq(ta.Init, tb.Init)
	case *ast.AssignStmt:
		tb, ok := b.(*ast.AssignStmt)
		return ok && astExprEq(ta.Target, tb.Target) && astExprEq(ta.Value, tb.Value)
	case *ast.ExprStmt:
		tb, ok := b.(*ast.ExprStmt)
		return ok && astExprEq(ta.Expr, tb.Expr)
	case *ast.ReturnStmt:
		tb, ok := b.(*ast.ReturnStmt)
		return ok && astExprEq(ta.Value, tb.Value)
	case *ast.IfStmt:
		tb, ok := b.(*ast.IfStmt)
		return ok && astExprEq(ta.Cond, tb.Cond) && astBlockEq(ta.Then, tb.Then) && astBlockEq(ta.Else, tb.Else)
	case *ast.IfLetStmt:
		tb, ok := b.(*ast.IfLetStmt)
		return ok && astPatternEq(ta.Pattern, tb.Pattern) && astExprEq(ta.Expr, tb.Expr) && astBlockEq(ta.Then, tb.Then) && astBlockEq(ta.Else, tb.Else)
	case *ast.WhileStmt:
		tb, ok := b.(*ast.WhileStmt)
		return ok && ta.Label == tb.Label && astExprEq(ta.Cond, tb.Cond) && astBlockEq(ta.Body, tb.Body)
	case *ast.WhileLetStmt:
		tb, ok := b.(*ast.WhileLetStmt)
		return ok && ta.Label == tb.Label && astPatternEq(ta.Pattern, tb.Pattern) && astExprEq(ta.Expr, tb.Expr) && astBlockEq(ta.Body, tb.Body)
	case *ast.LoopStmt:
		tb, ok := b.(*ast.LoopStmt)
		return ok && ta.Label == tb.Label && astBlockEq(ta.Body, tb.Body)
	case *ast.BreakStmt:
		tb, ok := b.(*ast.BreakStmt)
		return ok && ta.Label == tb.Label && astExprEq(ta.Value, tb.Value)
	case *ast.ContinueStmt:
		tb, ok := b.(*ast.ContinueStmt)
		return ok && ta.Label == tb.Label
	case *ast.MatchStmt:
		tb, ok := b.(*ast.MatchStmt)
		return ok && astExprEq(ta.Expr, tb.Expr) && matchArmListEq(ta.Arms, tb.Arms)
	case *ast.ForStmt:
		tb, ok := b.(*ast.ForStmt)
		return ok && ta.Label == tb.Label && astPatternEq(ta.Pattern, tb.Pattern) && astExprEq(ta.Expr, tb.Expr) && astBlockEq(ta.Body, tb.Body)
	default:
		return false
	}
}

func astBlockEq(a, b *ast.Block) bool {
	if a == nil || b == nil {
		return a == b
	}
	return stmtListEq(a.Stmts, b.Stmts)
}

func astItemEq(a, b ast.Item) bool {
	if a == nil || b == nil {
		return a == b
	}
	switch ta := a.(type) {
	case *ast.Function:
		tb, ok := b.(*ast.Function)
		return ok && ta.Name == tb.Name && typeParamListEq(ta.TypeParams, tb.TypeParams) && paramListEq(ta.Params, tb.Params) &&
			typePtrEq(ta.ReturnType, tb.ReturnType) && astBlockEq(ta.Body, tb.Body) && ta.Vis == tb.Vis && ta.IsMacro == tb.IsMacro
	case *ast.ImportDecl:
		tb, ok := b.(*ast.ImportDecl)
		return ok && ta.Path == tb.Path && ta.Alias == tb.Alias
	case *ast.StructDecl:
		tb, ok := b.(*ast.StructDecl)
		return ok && ta.Name == tb.Name && typeParamListEq(ta.TypeParams, tb.TypeParams) && fieldDefListEq(ta.Fields, tb.Fields) && ta.Vis == tb.Vis
	case *ast.ConstDecl:
		tb, ok := b.(*ast.ConstDecl)
		return ok && ta.Name == tb.Name && typePtrEq(ta.Type, tb.Type) && constValueEq(ta.Value, tb.Value) && ta.Vis == tb.Vis
	case *ast.EnumDecl:
		tb, ok := b.(*ast.EnumDecl)
		return ok && ta.Name == tb.Name && typeParamListEq(ta.TypeParams, tb.TypeParams) && ta.Repr == tb.Repr && variantDefListEq(ta.Variants, tb.Variants) && ta.Vis == tb.Vis
	case *ast.ImplDecl:
		tb, ok := b.(*ast.ImplDecl)
		return ok && ta.TypeName == tb.TypeName && typeListEq(ta.TypeArgs, tb.TypeArgs) && typeParamListEq(ta.TypeParams, tb.TypeParams) &&
			funcListEq(ta.Methods, tb.Methods) && ta.Vis == tb.Vis
	case *ast.TraitDecl:
		tb, ok := b.(*ast.TraitDecl)
		return ok && ta.Name == tb.Name && typeParamListEq(ta.TypeParams, tb.TypeParams) && traitMethodListEq(ta.Methods, tb.Methods) && ta.Vis == tb.Vis
	case *ast.ImplTraitDecl:
		tb, ok := b.(*ast.ImplTraitDecl)
		return ok && ta.TraitName == tb.TraitName && typeListEq(ta.TraitArgs, tb.TraitArgs) && ta.ForTypeName == tb.ForTypeName &&
			typeListEq(ta.ForTypeArgs, tb.ForTypeArgs) && typeParamListEq(ta.TypeParams, tb.TypeParams) && funcListEq(ta.Methods, tb.Methods) && ta.Vis == tb.Vis
	case *ast.TypeAlias:
		tb, ok := b.(*ast.TypeAlias)
		return ok && ta.Name == tb.Name && typeParamListEq(ta.TypeParams, tb.TypeParams) && typeEq(ta.Value, tb.Value) && ta.Vis == tb.Vis
	case *ast.CompileItem:
		tb, ok := b.(*ast.CompileItem)
		return ok && astExprEq(ta.Expr, tb.Expr)
	default:
		return false
	}
}

func astPatternEq(a, b ast.Pattern) bool {
	if a == nil || b == nil {
		return a == b
	}
	switch ta := a.(type) {
	case *ast.WildcardPattern:
		_, ok := b.(*ast.WildcardPattern)
		return ok
	case *ast.BindingPattern:
		tb, ok := b.(*ast.BindingPattern)
		return ok && ta.Name == tb.Name
	case *ast.VariantPattern:
		tb, ok := b.(*ast.VariantPattern)
		return ok && ta.EnumName == tb.EnumName && ta.Variant == tb.Variant && ta.Binding == tb.Binding && astPatternEq(ta.Payload, tb.Payload)
	case *ast.LiteralPattern:
		tb, ok := b.(*ast.LiteralPattern)
		return ok && constValueEq(ta.Value, tb.Value)
	case *ast.RangePattern:
		tb, ok := b.(*ast.RangePattern)
		return ok && ta.Inclusive == tb.Inclusive && astExprEq(ta.Start, tb.Start) && astExprEq(ta.End, tb.End)
	case *ast.OrPattern:
		tb, ok := b.(*ast.OrPattern)
		return ok && patternListEq(ta.Alts, tb.Alts)
	case *ast.StructPattern:
		tb, ok := b.(*ast.StructPattern)
		return ok && ta.StructName == tb.StructName && structFieldPatternListEq(ta.Fields, tb.Fields)
	case *ast.TuplePattern:
		tb, ok := b.(*ast.TuplePattern)
		return ok && patternListEq(ta.Elems, tb.Elems)
	case *ast.ArrayPattern:
		tb, ok := b.(*ast.ArrayPattern)
		return ok && patternListEq(ta.Elems, tb.Elems)
	default:
		return false
	}
}

func exprListEq(a, b []ast.Expr) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !astExprEq(a[i], b[i]) {
			return false
		}
	}
	return true
}

func stmtListEq(a, b []ast.Stmt) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !astStmtEq(a[i], b[i]) {
			return false
		}
	}
	return true
}

func patternListEq(a, b []ast.Pattern) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !astPatternEq(a[i], b[i]) {
			return false
		}
	}
	return true
}

func quotePartListEq(a, b []ast.QuotePart) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Text != b[i].Text {
			return false
		}
		if !astExprEq(a[i].Expr, b[i].Expr) {
			return false
		}
	}
	return true
}

func closureParamListEq(a, b []ast.ClosureParam) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || !typePtrEq(a[i].Type, b[i].Type) {
			return false
		}
	}
	return true
}

func fieldInitListEq(a, b []ast.FieldInit) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || !astExprEq(a[i].Value, b[i].Value) {
			return false
		}
	}
	return true
}

func structFieldPatternListEq(a, b []ast.StructFieldPattern) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Binding != b[i].Binding || !astPatternEq(a[i].Pattern, b[i].Pattern) {
			return false
		}
	}
	return true
}

func matchArmListEq(a, b []ast.MatchArm) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !astPatternEq(a[i].Pattern, b[i].Pattern) || !astExprEq(a[i].Guard, b[i].Guard) || !astBlockEq(a[i].Body, b[i].Body) {
			return false
		}
	}
	return true
}

func typeListEq(a, b []ast.Type) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !typeEq(a[i], b[i]) {
			return false
		}
	}
	return true
}

func typePtrEq(a, b *ast.Type) bool {
	if a == nil || b == nil {
		return a == b
	}
	return typeEq(*a, *b)
}

func typeEq(a, b ast.Type) bool {
	if a.Name != b.Name || a.IsRef != b.IsRef || a.IsMut != b.IsMut || a.IsArray != b.IsArray || a.IsTuple != b.IsTuple {
		return false
	}
	if !typePtrEq(a.Elem, b.Elem) {
		return false
	}
	if !typeListEq(a.TupleElems, b.TupleElems) {
		return false
	}
	if !typeListEq(a.Args, b.Args) {
		return false
	}
	return true
}

func typeParamListEq(a, b []ast.TypeParam) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || typeListEq(a[i].Bounds, b[i].Bounds) == false {
			return false
		}
	}
	return true
}

func paramListEq(a, b []ast.Param) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || !typeEq(a[i].Type, b[i].Type) {
			return false
		}
	}
	return true
}

func fieldDefListEq(a, b []ast.FieldDef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || !typeEq(a[i].Type, b[i].Type) {
			return false
		}
	}
	return true
}

func variantDefListEq(a, b []ast.VariantDef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].HasValue != b[i].HasValue || a[i].Value != b[i].Value || !typePtrEq(a[i].Payload, b[i].Payload) {
			return false
		}
	}
	return true
}

func funcListEq(a, b []*ast.Function) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !astItemEq(a[i], b[i]) {
			return false
		}
	}
	return true
}

func traitMethodListEq(a, b []ast.TraitMethod) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || !paramListEq(a[i].Params, b[i].Params) || !typePtrEq(a[i].ReturnType, b[i].ReturnType) {
			return false
		}
	}
	return true
}

func constValueEq(a, b ast.ConstValue) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case ast.ConstInt:
		return a.Int == b.Int
	case ast.ConstBool:
		return a.Bool == b.Bool
	case ast.ConstString:
		return a.Str == b.Str
	case ast.ConstFloat:
		return a.FloatText == b.FloatText
	default:
		return false
	}
}

func stringListEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
