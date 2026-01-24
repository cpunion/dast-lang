package macroexpand

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/compile"
	"dastlang/internal/diag"
	"dastlang/internal/interp"
	"dastlang/internal/ir"
	"dastlang/internal/parser"
	"dastlang/internal/source"
)

type siteKind int

const (
	siteExpr siteKind = iota
	siteStmt
	siteItem
)

type site struct {
	id       int
	kind     siteKind
	span     source.Span
	expr     *ast.CompileExpr
	exprSlot *ast.Expr
	stmtSlot *ast.Stmt
	itemSlot *ast.Item
	value    ir.Value
}

type expander struct {
	sites []*site
	next  int
	diag  *diag.Bag
}

func Expand(prog *ast.Program) *diag.Bag {
	if prog == nil {
		return nil
	}
	e := &expander{diag: &diag.Bag{}}
	for i := range prog.Items {
		e.rewriteItem(&prog.Items[i])
	}
	if e.diag.HasErrors() {
		return e.diag
	}
	if len(e.sites) == 0 {
		return nil
	}
	structNames := collectStructNames(prog)
	macroProg := &ast.Program{Items: e.macroItems(prog)}
	for _, site := range e.sites {
		macroProg.Items = append(macroProg.Items, e.wrapper(site))
	}
	irProg, diags := compile.CompileForMacro(macroProg)
	if diags != nil && len(diags.Items) > 0 {
		return diags
	}
	rt := interp.New(irProg)
	rt.StructNames = structNames
	for _, site := range e.sites {
		val, err := rt.Call(wrapperName(site.id), nil)
		if err != nil {
			e.diag.Add(site.span, err.Error())
			continue
		}
		site.value = val
	}
	if e.diag.HasErrors() {
		return e.diag
	}
	// Parse item expansions first to collect any new struct names.
	for _, site := range e.sites {
		if site.kind != siteItem {
			continue
		}
		item, ok := e.parseItem(site, structNames)
		if !ok {
			continue
		}
		if site.itemSlot != nil {
			*site.itemSlot = item
		}
		if sd, ok := item.(*ast.StructDecl); ok {
			structNames[sd.Name] = struct{}{}
		}
	}
	for _, site := range e.sites {
		switch site.kind {
		case siteExpr:
			expr, ok := e.parseExpr(site, structNames)
			if !ok {
				continue
			}
			if site.exprSlot != nil {
				*site.exprSlot = expr
			}
		case siteStmt:
			stmt, ok := e.parseStmt(site, structNames)
			if !ok {
				continue
			}
			if site.stmtSlot != nil {
				*site.stmtSlot = stmt
			}
		}
	}
	// Remove nil items (in case of replacement errors).
	items := prog.Items[:0]
	for _, item := range prog.Items {
		if item != nil {
			items = append(items, item)
		}
	}
	prog.Items = items
	if e.diag.HasErrors() {
		return e.diag
	}
	return nil
}

func (e *expander) rewriteItem(slot *ast.Item) {
	switch v := (*slot).(type) {
	case *ast.Function:
		e.rewriteBlock(v.Body)
	case *ast.ImplDecl:
		for i := range v.Methods {
			item := ast.Item(v.Methods[i])
			e.rewriteItem(&item)
			if fn, ok := item.(*ast.Function); ok {
				v.Methods[i] = fn
			}
		}
	case *ast.CompileItem:
		expr := v.Expr
		ce := e.prepareCompileExpr(&expr, true)
		v.Expr = expr
		if ce == nil {
			e.diag.Add(v.Span(), "compile! item expects a compile! expression")
			return
		}
		e.addSite(siteItem, v.Span(), ce, nil, nil, slot)
	}
}

func (e *expander) rewriteBlock(block *ast.Block) {
	if block == nil {
		return
	}
	for i := range block.Stmts {
		e.rewriteStmt(&block.Stmts[i])
	}
}

func (e *expander) rewriteStmt(slot *ast.Stmt) {
	switch v := (*slot).(type) {
	case *ast.LetStmt:
		e.rewriteExpr(&v.Init, siteExpr, false)
	case *ast.LetPatternStmt:
		e.rewriteExpr(&v.Init, siteExpr, false)
	case *ast.AssignStmt:
		e.rewriteExpr(&v.Target, siteExpr, false)
		e.rewriteExpr(&v.Value, siteExpr, false)
	case *ast.ExprStmt:
		expr := v.Expr
		if ce := e.prepareCompileExpr(&expr, true); ce != nil {
			v.Expr = expr
			e.addSite(siteStmt, ce.Span(), ce, nil, slot, nil)
		} else {
			e.rewriteExpr(&v.Expr, siteExpr, false)
		}
	case *ast.ReturnStmt:
		if v.Value != nil {
			e.rewriteExpr(&v.Value, siteExpr, false)
		}
	case *ast.IfStmt:
		e.rewriteExpr(&v.Cond, siteExpr, false)
		e.rewriteBlock(v.Then)
		e.rewriteBlock(v.Else)
	case *ast.IfLetStmt:
		e.rewriteExpr(&v.Expr, siteExpr, false)
		e.rewriteBlock(v.Then)
		e.rewriteBlock(v.Else)
	case *ast.WhileStmt:
		e.rewriteExpr(&v.Cond, siteExpr, false)
		e.rewriteBlock(v.Body)
	case *ast.WhileLetStmt:
		e.rewriteExpr(&v.Expr, siteExpr, false)
		e.rewriteBlock(v.Body)
	case *ast.LoopStmt:
		e.rewriteBlock(v.Body)
	case *ast.MatchStmt:
		e.rewriteExpr(&v.Expr, siteExpr, false)
		for i := range v.Arms {
			if v.Arms[i].Guard != nil {
				e.rewriteExpr(&v.Arms[i].Guard, siteExpr, false)
			}
			e.rewriteBlock(v.Arms[i].Body)
		}
	case *ast.Block:
		e.rewriteBlock(v)
	}
}

func (e *expander) rewriteExpr(slot *ast.Expr, ctx siteKind, inCompile bool) {
	switch v := (*slot).(type) {
	case *ast.CompileExpr:
		if inCompile {
			inner := v.Expr
			e.rewriteExpr(&inner, siteExpr, true)
			*slot = inner
			return
		}
		e.addSite(ctx, v.Span(), v, slot, nil, nil)
		e.rewriteExpr(&v.Expr, siteExpr, true)
	case *ast.MacroCallExpr:
		if inCompile {
			call := e.macroCallToCall(v)
			*slot = call
			e.rewriteExpr(slot, siteExpr, true)
			return
		}
		call := e.macroCallToCall(v)
		ce := &ast.CompileExpr{Expr: call, SpanInfo: v.Span()}
		*slot = ce
		e.addSite(ctx, ce.Span(), ce, slot, nil, nil)
		e.rewriteExpr(&ce.Expr, siteExpr, true)
	case *ast.ArrayLit:
		for i := range v.Elems {
			e.rewriteExpr(&v.Elems[i], siteExpr, inCompile)
		}
	case *ast.StructLit:
		for i := range v.Fields {
			e.rewriteExpr(&v.Fields[i].Value, siteExpr, inCompile)
		}
	case *ast.AccessExpr:
		e.rewriteExpr(&v.Receiver, siteExpr, inCompile)
	case *ast.IndexExpr:
		e.rewriteExpr(&v.Receiver, siteExpr, inCompile)
		e.rewriteExpr(&v.Index, siteExpr, inCompile)
	case *ast.UnaryExpr:
		e.rewriteExpr(&v.Expr, siteExpr, inCompile)
	case *ast.BinaryExpr:
		e.rewriteExpr(&v.Left, siteExpr, inCompile)
		e.rewriteExpr(&v.Right, siteExpr, inCompile)
	case *ast.RefExpr:
		e.rewriteExpr(&v.Expr, siteExpr, inCompile)
	case *ast.DerefExpr:
		e.rewriteExpr(&v.Expr, siteExpr, inCompile)
	case *ast.CallExpr:
		for i := range v.Args {
			e.rewriteExpr(&v.Args[i], siteExpr, inCompile)
		}
	case *ast.MethodCallExpr:
		e.rewriteExpr(&v.Receiver, siteExpr, inCompile)
		for i := range v.Args {
			e.rewriteExpr(&v.Args[i], siteExpr, inCompile)
		}
	case *ast.BlockExpr:
		e.rewriteBlock(v.Block)
	case *ast.IfExpr:
		e.rewriteExpr(&v.Cond, siteExpr, inCompile)
		e.rewriteExpr(&v.Then, siteExpr, inCompile)
		if v.Else != nil {
			e.rewriteExpr(&v.Else, siteExpr, inCompile)
		}
	case *ast.MatchExpr:
		e.rewriteExpr(&v.Expr, siteExpr, inCompile)
		for i := range v.Arms {
			if v.Arms[i].Guard != nil {
				e.rewriteExpr(&v.Arms[i].Guard, siteExpr, inCompile)
			}
			e.rewriteBlock(v.Arms[i].Body)
		}
	case *ast.EnumVariantExpr:
		if v.Arg != nil {
			e.rewriteExpr(&v.Arg, siteExpr, inCompile)
		}
	}
}

func (e *expander) prepareCompileExpr(slot *ast.Expr, allowMacro bool) *ast.CompileExpr {
	switch v := (*slot).(type) {
	case *ast.CompileExpr:
		e.rewriteExpr(&v.Expr, siteExpr, true)
		return v
	case *ast.MacroCallExpr:
		if !allowMacro {
			return nil
		}
		call := e.macroCallToCall(v)
		ce := &ast.CompileExpr{Expr: call, SpanInfo: v.Span()}
		*slot = ce
		e.rewriteExpr(&ce.Expr, siteExpr, true)
		return ce
	default:
		return nil
	}
}

func (e *expander) macroCallToCall(call *ast.MacroCallExpr) ast.Expr {
	switch callee := call.Callee.(type) {
	case *ast.IdentExpr:
		return &ast.CallExpr{Callee: callee.Name, Args: call.Args, SpanInfo: call.Span()}
	case *ast.AccessExpr:
		return &ast.MethodCallExpr{Receiver: callee.Receiver, Method: callee.Field, Args: call.Args, SpanInfo: call.Span()}
	default:
		e.diag.Add(call.Span(), "macro call requires identifier or field access")
		return &ast.IntLit{Value: 0, SpanInfo: call.Span()}
	}
}

func (e *expander) addSite(kind siteKind, span source.Span, expr *ast.CompileExpr, exprSlot *ast.Expr, stmtSlot *ast.Stmt, itemSlot *ast.Item) {
	id := e.next
	e.next++
	e.sites = append(e.sites, &site{
		id:       id,
		kind:     kind,
		span:     span,
		expr:     expr,
		exprSlot: exprSlot,
		stmtSlot: stmtSlot,
		itemSlot: itemSlot,
	})
}

func (e *expander) wrapper(site *site) *ast.Function {
	ret := &ast.ReturnStmt{Value: site.expr.Expr, SpanInfo: site.expr.Span()}
	block := &ast.Block{Stmts: []ast.Stmt{ret}, SpanInfo: ret.Span()}
	return &ast.Function{
		Name:     wrapperName(site.id),
		Params:   nil,
		Body:     block,
		Vis:      ast.VisPrivate,
		IsMacro:  false,
		SpanInfo: site.span,
	}
}

func wrapperName(id int) string {
	return fmt.Sprintf("__dast_compile_eval_%d", id)
}

func (e *expander) macroItems(prog *ast.Program) []ast.Item {
	if prog == nil {
		return nil
	}
	items := make([]ast.Item, 0, len(prog.Items))
	for _, item := range prog.Items {
		switch v := item.(type) {
		case *ast.StructDecl, *ast.EnumDecl, *ast.ConstDecl:
			items = append(items, item)
		case *ast.Function:
			if v.IsMacro {
				items = append(items, item)
			}
		case *ast.ImplDecl:
			var methods []*ast.Function
			for _, m := range v.Methods {
				if m.IsMacro {
					methods = append(methods, m)
				}
			}
			if len(methods) > 0 {
				copyDecl := *v
				copyDecl.Methods = methods
				items = append(items, &copyDecl)
			}
		}
	}
	return items
}

func (e *expander) parseItem(site *site, structs map[string]struct{}) (ast.Item, bool) {
	if site.value.Kind != ir.KindAst {
		e.diag.Add(site.span, "compile! expects AstItem value")
		return nil, false
	}
	if site.value.AstKind != ir.AstItem {
		e.diag.Add(site.span, fmt.Sprintf("compile! item expects AstItem, got %s", astKindString(site.value.AstKind)))
		return nil, false
	}
	item, diags := parser.ParseItemString("<macro>", site.value.AstSrc, structs)
	if diags != nil && len(diags.Items) > 0 {
		e.diag.Add(site.span, "compile! item parse error: "+diags.Items[0].Message)
		return nil, false
	}
	return item, true
}

func (e *expander) parseExpr(site *site, structs map[string]struct{}) (ast.Expr, bool) {
	if site.value.Kind != ir.KindAst {
		e.diag.Add(site.span, "compile! expects AstExpr value")
		return nil, false
	}
	switch site.value.AstKind {
	case ir.AstExpr:
		expr, diags := parser.ParseExprString("<macro>", site.value.AstSrc, structs)
		if diags != nil && len(diags.Items) > 0 {
			e.diag.Add(site.span, "compile! expr parse error: "+diags.Items[0].Message)
			return nil, false
		}
		return expr, true
	case ir.AstBlock:
		block, diags := parser.ParseBlockString("<macro>", site.value.AstSrc, structs)
		if diags != nil && len(diags.Items) > 0 {
			e.diag.Add(site.span, "compile! block parse error: "+diags.Items[0].Message)
			return nil, false
		}
		return &ast.BlockExpr{Block: block, SpanInfo: block.Span()}, true
	case ir.AstStmt:
		stmt, diags := parser.ParseStmtString("<macro>", site.value.AstSrc, structs)
		if diags != nil && len(diags.Items) > 0 {
			e.diag.Add(site.span, "compile! stmt parse error: "+diags.Items[0].Message)
			return nil, false
		}
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
			return exprStmt.Expr, true
		}
		e.diag.Add(site.span, "compile! expr expects AstExpr or expression statement")
		return nil, false
	default:
		e.diag.Add(site.span, fmt.Sprintf("compile! expr expects AstExpr, got %s", astKindString(site.value.AstKind)))
		return nil, false
	}
}

func (e *expander) parseStmt(site *site, structs map[string]struct{}) (ast.Stmt, bool) {
	if site.value.Kind != ir.KindAst {
		e.diag.Add(site.span, "compile! expects AstStmt value")
		return nil, false
	}
	switch site.value.AstKind {
	case ir.AstStmt:
		stmt, diags := parser.ParseStmtString("<macro>", site.value.AstSrc, structs)
		if diags != nil && len(diags.Items) > 0 {
			e.diag.Add(site.span, "compile! stmt parse error: "+diags.Items[0].Message)
			return nil, false
		}
		return stmt, true
	case ir.AstExpr:
		expr, diags := parser.ParseExprString("<macro>", site.value.AstSrc, structs)
		if diags != nil && len(diags.Items) > 0 {
			e.diag.Add(site.span, "compile! expr parse error: "+diags.Items[0].Message)
			return nil, false
		}
		return &ast.ExprStmt{Expr: expr, SpanInfo: expr.Span()}, true
	case ir.AstBlock:
		block, diags := parser.ParseBlockString("<macro>", site.value.AstSrc, structs)
		if diags != nil && len(diags.Items) > 0 {
			e.diag.Add(site.span, "compile! block parse error: "+diags.Items[0].Message)
			return nil, false
		}
		return block, true
	default:
		e.diag.Add(site.span, fmt.Sprintf("compile! stmt expects AstStmt/AstExpr/AstBlock, got %s", astKindString(site.value.AstKind)))
		return nil, false
	}
}

func collectStructNames(prog *ast.Program) map[string]struct{} {
	names := map[string]struct{}{}
	if prog == nil {
		return names
	}
	for _, item := range prog.Items {
		if sd, ok := item.(*ast.StructDecl); ok {
			names[sd.Name] = struct{}{}
		}
	}
	return names
}

func astKindString(kind ir.AstKind) string {
	switch kind {
	case ir.AstExpr:
		return "AstExpr"
	case ir.AstStmt:
		return "AstStmt"
	case ir.AstItem:
		return "AstItem"
	case ir.AstBlock:
		return "AstBlock"
	default:
		return "Ast"
	}
}
