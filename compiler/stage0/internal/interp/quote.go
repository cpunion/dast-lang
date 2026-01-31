package interp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"dastlang/internal/ast"
	"dastlang/internal/ir"
	"dastlang/internal/parser"
)

const (
	splicePrefix = "__dast_splice_"
	spliceSuffix = "__"
)

type parsedSplice struct {
	kind   ir.AstKind
	expr   ast.Expr
	stmt   ast.Stmt
	block  *ast.Block
	item   ast.Item
	parsed bool
}

type quoteCtx struct {
	structs map[string]struct{}
	splices []ir.Value
	parsed  []parsedSplice
}

func (rt *Runtime) buildQuotedAst(kind ir.AstKind, template string, splices []ir.Value) (string, error) {
	structs := rt.structNameSet()
	q := &quoteCtx{
		structs: structs,
		splices: splices,
		parsed:  make([]parsedSplice, len(splices)),
	}
	ren := newHygieneRenamer(&rt.gensymCount)
	switch kind {
	case ir.AstExpr:
		expr, err := q.parseExpr(template)
		if err != nil {
			return "", err
		}
		ren.renameExpr(expr)
		expr, err = q.replaceExpr(expr)
		if err != nil {
			return "", err
		}
		return ast.FormatExpr(expr), nil
	case ir.AstStmt:
		stmt, err := q.parseStmt(template)
		if err != nil {
			return "", err
		}
		ren.renameStmt(stmt)
		stmt, err = q.replaceStmt(stmt)
		if err != nil {
			return "", err
		}
		return ast.FormatStmt(stmt), nil
	case ir.AstBlock:
		block, err := q.parseBlock(template)
		if err != nil {
			return "", err
		}
		ren.renameBlock(block)
		if err := q.replaceBlock(block); err != nil {
			return "", err
		}
		return ast.FormatBlock(block), nil
	case ir.AstItem:
		item, err := q.parseItem(template)
		if err != nil {
			return "", err
		}
		ren.renameItem(item)
		item, err = q.replaceItem(item)
		if err != nil {
			return "", err
		}
		return ast.FormatItem(item), nil
	default:
		return "", errors.New("unknown ast kind")
	}
}

func (rt *Runtime) structNameSet() map[string]struct{} {
	names := map[string]struct{}{}
	for name := range rt.StructNames {
		names[name] = struct{}{}
	}
	if rt.Prog == nil {
		return names
	}
	for name := range rt.Prog.TypeDecls {
		names[name] = struct{}{}
	}
	return names
}

func (q *quoteCtx) parseExpr(src string) (ast.Expr, error) {
	expr, diags := parser.ParseExprString("<quote>", src, q.structs)
	if diags != nil && len(diags.Items) > 0 {
		return nil, errors.New(diags.Items[0].Message)
	}
	return expr, nil
}

func (q *quoteCtx) parseStmt(src string) (ast.Stmt, error) {
	stmt, diags := parser.ParseStmtString("<quote>", src, q.structs)
	if diags != nil && len(diags.Items) > 0 {
		return nil, errors.New(diags.Items[0].Message)
	}
	return stmt, nil
}

func (q *quoteCtx) parseBlock(src string) (*ast.Block, error) {
	block, diags := parser.ParseBlockString("<quote>", src, q.structs)
	if diags != nil && len(diags.Items) > 0 {
		return nil, errors.New(diags.Items[0].Message)
	}
	return block, nil
}

func (q *quoteCtx) parseItem(src string) (ast.Item, error) {
	item, diags := parser.ParseItemString("<quote>", src, q.structs)
	if diags != nil && len(diags.Items) > 0 {
		return nil, errors.New(diags.Items[0].Message)
	}
	return item, nil
}

func (q *quoteCtx) parseSplice(idx int) (parsedSplice, error) {
	if idx < 0 || idx >= len(q.splices) {
		return parsedSplice{}, errors.New("splice index out of range")
	}
	if q.parsed[idx].parsed {
		return q.parsed[idx], nil
	}
	val := q.splices[idx]
	if val.Kind != ir.KindAst {
		return parsedSplice{}, errors.New("splice expects ast")
	}
	out := parsedSplice{kind: val.AstKind, parsed: true}
	switch val.AstKind {
	case ir.AstExpr:
		expr, err := q.parseExpr(val.AstSrc)
		if err != nil {
			return parsedSplice{}, err
		}
		out.expr = expr
	case ir.AstStmt:
		stmt, err := q.parseStmt(val.AstSrc)
		if err != nil {
			return parsedSplice{}, err
		}
		out.stmt = stmt
	case ir.AstBlock:
		block, err := q.parseBlock(val.AstSrc)
		if err != nil {
			return parsedSplice{}, err
		}
		out.block = block
	case ir.AstItem:
		item, err := q.parseItem(val.AstSrc)
		if err != nil {
			return parsedSplice{}, err
		}
		out.item = item
	default:
		return parsedSplice{}, errors.New("unknown splice kind")
	}
	q.parsed[idx] = out
	return out, nil
}

func (q *quoteCtx) spliceExpr(idx int) (ast.Expr, error) {
	val, err := q.parseSplice(idx)
	if err != nil {
		return nil, err
	}
	switch val.kind {
	case ir.AstExpr:
		return val.expr, nil
	case ir.AstBlock:
		return &ast.BlockExpr{Block: val.block, SpanInfo: val.block.Span()}, nil
	case ir.AstStmt:
		if exprStmt, ok := val.stmt.(*ast.ExprStmt); ok {
			return exprStmt.Expr, nil
		}
		return nil, errors.New("splice stmt used in expr position")
	default:
		return nil, errors.New("splice kind not usable in expr position")
	}
}

func (q *quoteCtx) spliceStmt(idx int) (ast.Stmt, error) {
	val, err := q.parseSplice(idx)
	if err != nil {
		return nil, err
	}
	switch val.kind {
	case ir.AstStmt:
		return val.stmt, nil
	case ir.AstExpr:
		return &ast.ExprStmt{Expr: val.expr, SpanInfo: val.expr.Span()}, nil
	case ir.AstBlock:
		return val.block, nil
	default:
		return nil, errors.New("splice kind not usable in stmt position")
	}
}

func (q *quoteCtx) spliceBlock(idx int) (*ast.Block, error) {
	val, err := q.parseSplice(idx)
	if err != nil {
		return nil, err
	}
	if val.kind != ir.AstBlock || val.block == nil {
		return nil, errors.New("splice expects AstBlock")
	}
	return val.block, nil
}

func (q *quoteCtx) spliceItem(idx int) (ast.Item, error) {
	val, err := q.parseSplice(idx)
	if err != nil {
		return nil, err
	}
	if val.kind != ir.AstItem || val.item == nil {
		return nil, errors.New("splice expects AstItem")
	}
	return val.item, nil
}

func (q *quoteCtx) spliceIdent(idx int) (string, error) {
	expr, err := q.spliceExpr(idx)
	if err != nil {
		return "", err
	}
	ident, ok := expr.(*ast.IdentExpr)
	if !ok {
		return "", errors.New("splice expects identifier")
	}
	return ident.Name, nil
}

func (q *quoteCtx) replaceExpr(expr ast.Expr) (ast.Expr, error) {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		if idx, ok := placeholderIndex(e.Name); ok {
			return q.spliceExpr(idx)
		}
		return e, nil
	case *ast.UnaryExpr:
		var err error
		e.Expr, err = q.replaceExpr(e.Expr)
		return e, err
	case *ast.RefExpr:
		var err error
		e.Expr, err = q.replaceExpr(e.Expr)
		return e, err
	case *ast.DerefExpr:
		var err error
		e.Expr, err = q.replaceExpr(e.Expr)
		return e, err
	case *ast.BinaryExpr:
		var err error
		e.Left, err = q.replaceExpr(e.Left)
		if err != nil {
			return e, err
		}
		e.Right, err = q.replaceExpr(e.Right)
		return e, err
	case *ast.CallExpr:
		for i, arg := range e.Args {
			repl, err := q.replaceExpr(arg)
			if err != nil {
				return e, err
			}
			e.Args[i] = repl
		}
		return e, nil
	case *ast.MethodCallExpr:
		var err error
		e.Receiver, err = q.replaceExpr(e.Receiver)
		if err != nil {
			return e, err
		}
		for i, arg := range e.Args {
			repl, err := q.replaceExpr(arg)
			if err != nil {
				return e, err
			}
			e.Args[i] = repl
		}
		return e, nil
	case *ast.StructLit:
		if idx, ok := placeholderIndex(e.Name); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return e, err
			}
			e.Name = name
		}
		for i := range e.Fields {
			if idx, ok := placeholderIndex(e.Fields[i].Name); ok {
				name, err := q.spliceIdent(idx)
				if err != nil {
					return e, err
				}
				e.Fields[i].Name = name
			}
			repl, err := q.replaceExpr(e.Fields[i].Value)
			if err != nil {
				return e, err
			}
			e.Fields[i].Value = repl
		}
		return e, nil
	case *ast.AccessExpr:
		var err error
		e.Receiver, err = q.replaceExpr(e.Receiver)
		if err != nil {
			return e, err
		}
		if idx, ok := placeholderIndex(e.Field); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return e, err
			}
			e.Field = name
		}
		return e, nil
	case *ast.IndexExpr:
		var err error
		e.Receiver, err = q.replaceExpr(e.Receiver)
		if err != nil {
			return e, err
		}
		e.Index, err = q.replaceExpr(e.Index)
		return e, err
	case *ast.EnumVariantExpr:
		if idx, ok := placeholderIndex(e.EnumName); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return e, err
			}
			e.EnumName = name
		}
		if idx, ok := placeholderIndex(e.Variant); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return e, err
			}
			e.Variant = name
		}
		if e.Arg != nil {
			var err error
			e.Arg, err = q.replaceExpr(e.Arg)
			if err != nil {
				return e, err
			}
		}
		return e, nil
	case *ast.BlockExpr:
		if err := q.replaceBlock(e.Block); err != nil {
			return e, err
		}
		return e, nil
	case *ast.IfExpr:
		var err error
		e.Cond, err = q.replaceExpr(e.Cond)
		if err != nil {
			return e, err
		}
		e.Then, err = q.replaceExpr(e.Then)
		if err != nil {
			return e, err
		}
		e.Else, err = q.replaceExpr(e.Else)
		return e, err
	case *ast.MatchExpr:
		var err error
		e.Expr, err = q.replaceExpr(e.Expr)
		if err != nil {
			return e, err
		}
		for i := range e.Arms {
			arm := &e.Arms[i]
			if err := q.replacePattern(arm.Pattern); err != nil {
				return e, err
			}
			if arm.Guard != nil {
				arm.Guard, err = q.replaceExpr(arm.Guard)
				if err != nil {
					return e, err
				}
			}
			if err := q.replaceBlock(arm.Body); err != nil {
				return e, err
			}
		}
		return e, nil
	case *ast.ArrayLit:
		for i, elem := range e.Elems {
			repl, err := q.replaceExpr(elem)
			if err != nil {
				return e, err
			}
			e.Elems[i] = repl
		}
		return e, nil
	case *ast.CompileExpr:
		var err error
		e.Expr, err = q.replaceExpr(e.Expr)
		return e, err
	case *ast.MacroCallExpr:
		var err error
		e.Callee, err = q.replaceExpr(e.Callee)
		if err != nil {
			return e, err
		}
		for i, arg := range e.Args {
			repl, err := q.replaceExpr(arg)
			if err != nil {
				return e, err
			}
			e.Args[i] = repl
		}
		return e, nil
	default:
		return e, nil
	}
}

func (q *quoteCtx) replaceStmt(stmt ast.Stmt) (ast.Stmt, error) {
	switch s := stmt.(type) {
	case *ast.Block:
		if err := q.replaceBlock(s); err != nil {
			return s, err
		}
		return s, nil
	case *ast.LetStmt:
		if idx, ok := placeholderIndex(s.Name); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return s, err
			}
			s.Name = name
		}
		repl, err := q.replaceExpr(s.Init)
		if err != nil {
			return s, err
		}
		s.Init = repl
		if err := q.replaceType(s.Type); err != nil {
			return s, err
		}
		return s, nil
	case *ast.LetPatternStmt:
		if err := q.replacePattern(s.Pattern); err != nil {
			return s, err
		}
		if err := q.replaceType(s.Type); err != nil {
			return s, err
		}
		repl, err := q.replaceExpr(s.Init)
		if err != nil {
			return s, err
		}
		s.Init = repl
		return s, nil
	case *ast.AssignStmt:
		var err error
		s.Target, err = q.replaceExpr(s.Target)
		if err != nil {
			return s, err
		}
		s.Value, err = q.replaceExpr(s.Value)
		return s, err
	case *ast.ExprStmt:
		if ident, ok := s.Expr.(*ast.IdentExpr); ok {
			if idx, ok := placeholderIndex(ident.Name); ok {
				return q.spliceStmt(idx)
			}
		}
		repl, err := q.replaceExpr(s.Expr)
		if err != nil {
			return s, err
		}
		s.Expr = repl
		return s, nil
	case *ast.ReturnStmt:
		if s.Value == nil {
			return s, nil
		}
		repl, err := q.replaceExpr(s.Value)
		if err != nil {
			return s, err
		}
		s.Value = repl
		return s, nil
	case *ast.IfStmt:
		var err error
		s.Cond, err = q.replaceExpr(s.Cond)
		if err != nil {
			return s, err
		}
		if err := q.replaceBlock(s.Then); err != nil {
			return s, err
		}
		if s.Else != nil {
			if err := q.replaceBlock(s.Else); err != nil {
				return s, err
			}
		}
		return s, nil
	case *ast.IfLetStmt:
		var err error
		if err := q.replacePattern(s.Pattern); err != nil {
			return s, err
		}
		s.Expr, err = q.replaceExpr(s.Expr)
		if err != nil {
			return s, err
		}
		if err := q.replaceBlock(s.Then); err != nil {
			return s, err
		}
		if s.Else != nil {
			if err := q.replaceBlock(s.Else); err != nil {
				return s, err
			}
		}
		return s, nil
	case *ast.WhileStmt:
		var err error
		s.Cond, err = q.replaceExpr(s.Cond)
		if err != nil {
			return s, err
		}
		if err := q.replaceBlock(s.Body); err != nil {
			return s, err
		}
		return s, nil
	case *ast.WhileLetStmt:
		var err error
		if err := q.replacePattern(s.Pattern); err != nil {
			return s, err
		}
		s.Expr, err = q.replaceExpr(s.Expr)
		if err != nil {
			return s, err
		}
		if err := q.replaceBlock(s.Body); err != nil {
			return s, err
		}
		return s, nil
	case *ast.LoopStmt:
		if err := q.replaceBlock(s.Body); err != nil {
			return s, err
		}
		return s, nil
	case *ast.MatchStmt:
		var err error
		s.Expr, err = q.replaceExpr(s.Expr)
		if err != nil {
			return s, err
		}
		for i := range s.Arms {
			arm := &s.Arms[i]
			if err := q.replacePattern(arm.Pattern); err != nil {
				return s, err
			}
			if arm.Guard != nil {
				arm.Guard, err = q.replaceExpr(arm.Guard)
				if err != nil {
					return s, err
				}
			}
			if err := q.replaceBlock(arm.Body); err != nil {
				return s, err
			}
		}
		return s, nil
	default:
		return s, nil
	}
}

func (q *quoteCtx) replaceBlock(block *ast.Block) error {
	if block == nil {
		return nil
	}
	stmts := make([]ast.Stmt, 0, len(block.Stmts))
	for _, stmt := range block.Stmts {
		repl, err := q.replaceStmt(stmt)
		if err != nil {
			return err
		}
		if repl != nil {
			stmts = append(stmts, repl)
		}
	}
	block.Stmts = stmts
	return nil
}

func (q *quoteCtx) replaceItem(item ast.Item) (ast.Item, error) {
	switch v := item.(type) {
	case *ast.Function:
		if idx, ok := placeholderIndex(v.Name); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return v, err
			}
			v.Name = name
		}
		for i := range v.Params {
			if idx, ok := placeholderIndex(v.Params[i].Name); ok {
				name, err := q.spliceIdent(idx)
				if err != nil {
					return v, err
				}
				v.Params[i].Name = name
			}
			if err := q.replaceType(&v.Params[i].Type); err != nil {
				return v, err
			}
		}
		if err := q.replaceType(v.ReturnType); err != nil {
			return v, err
		}
		if err := q.replaceBlock(v.Body); err != nil {
			return v, err
		}
		return v, nil
	case *ast.StructDecl:
		if idx, ok := placeholderIndex(v.Name); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return v, err
			}
			v.Name = name
		}
		if idx, ok := placeholderIndex(v.Repr); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return v, err
			}
			v.Repr = name
		}
		for i := range v.Fields {
			if idx, ok := placeholderIndex(v.Fields[i].Name); ok {
				name, err := q.spliceIdent(idx)
				if err != nil {
					return v, err
				}
				v.Fields[i].Name = name
			}
			if err := q.replaceType(&v.Fields[i].Type); err != nil {
				return v, err
			}
		}
		return v, nil
	case *ast.EnumDecl:
		if idx, ok := placeholderIndex(v.Name); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return v, err
			}
			v.Name = name
		}
		if idx, ok := placeholderIndex(v.Repr); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return v, err
			}
			v.Repr = name
		}
		for i := range v.Variants {
			if idx, ok := placeholderIndex(v.Variants[i].Name); ok {
				name, err := q.spliceIdent(idx)
				if err != nil {
					return v, err
				}
				v.Variants[i].Name = name
			}
			if v.Variants[i].Payload != nil {
				if err := q.replaceType(v.Variants[i].Payload); err != nil {
					return v, err
				}
			}
		}
		return v, nil
	case *ast.ConstDecl:
		if idx, ok := placeholderIndex(v.Name); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return v, err
			}
			v.Name = name
		}
		if err := q.replaceType(v.Type); err != nil {
			return v, err
		}
		if v.Expr != nil {
			repl, err := q.replaceExpr(v.Expr)
			if err != nil {
				return v, err
			}
			v.Expr = repl
		}
		return v, nil
	case *ast.ImplDecl:
		if idx, ok := placeholderIndex(v.TypeName); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return v, err
			}
			v.TypeName = name
		}
		for i := range v.Methods {
			item, err := q.replaceItem(v.Methods[i])
			if err != nil {
				return v, err
			}
			fn, ok := item.(*ast.Function)
			if ok {
				v.Methods[i] = fn
			}
		}
		return v, nil
	default:
		return item, nil
	}
}

func (q *quoteCtx) replacePattern(pat ast.Pattern) error {
	switch p := pat.(type) {
	case *ast.BindingPattern:
		if idx, ok := placeholderIndex(p.Name); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return err
			}
			p.Name = name
		}
	case *ast.VariantPattern:
		if idx, ok := placeholderIndex(p.EnumName); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return err
			}
			p.EnumName = name
		}
		if idx, ok := placeholderIndex(p.Variant); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return err
			}
			p.Variant = name
		}
		if p.Payload != nil {
			if err := q.replacePattern(p.Payload); err != nil {
				return err
			}
		} else if p.Binding != "" {
			if idx, ok := placeholderIndex(p.Binding); ok {
				name, err := q.spliceIdent(idx)
				if err != nil {
					return err
				}
				p.Binding = name
			}
		}
	case *ast.StructPattern:
		if idx, ok := placeholderIndex(p.StructName); ok {
			name, err := q.spliceIdent(idx)
			if err != nil {
				return err
			}
			p.StructName = name
		}
		for i := range p.Fields {
			if idx, ok := placeholderIndex(p.Fields[i].Name); ok {
				name, err := q.spliceIdent(idx)
				if err != nil {
					return err
				}
				p.Fields[i].Name = name
			}
			if p.Fields[i].Binding != "" {
				if idx, ok := placeholderIndex(p.Fields[i].Binding); ok {
					name, err := q.spliceIdent(idx)
					if err != nil {
						return err
					}
					p.Fields[i].Binding = name
				}
			}
			if p.Fields[i].Pattern != nil {
				if err := q.replacePattern(p.Fields[i].Pattern); err != nil {
					return err
				}
			}
		}
	case *ast.TuplePattern:
		for _, el := range p.Elems {
			if err := q.replacePattern(el); err != nil {
				return err
			}
		}
	case *ast.ArrayPattern:
		for _, el := range p.Elems {
			if err := q.replacePattern(el); err != nil {
				return err
			}
		}
	case *ast.OrPattern:
		for _, alt := range p.Alts {
			if err := q.replacePattern(alt); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *quoteCtx) replaceType(t *ast.Type) error {
	if t == nil {
		return nil
	}
	if t.IsArray {
		if t.Elem != nil {
			if err := q.replaceType(t.Elem); err != nil {
				return err
			}
		}
		return nil
	}
	if idx, ok := placeholderIndex(t.Name); ok {
		name, err := q.spliceIdent(idx)
		if err != nil {
			return err
		}
		t.Name = name
	}
	return nil
}

func placeholderIndex(name string) (int, bool) {
	if !strings.HasPrefix(name, splicePrefix) || !strings.HasSuffix(name, spliceSuffix) {
		return 0, false
	}
	num := strings.TrimSuffix(strings.TrimPrefix(name, splicePrefix), spliceSuffix)
	idx, err := strconv.Atoi(num)
	if err != nil || idx < 0 {
		return 0, false
	}
	return idx, true
}

type hygieneRenamer struct {
	nextID *int64
	scopes []map[string]string
}

func newHygieneRenamer(counter *int64) *hygieneRenamer {
	r := &hygieneRenamer{nextID: counter}
	r.push()
	return r
}

func (r *hygieneRenamer) push() {
	r.scopes = append(r.scopes, map[string]string{})
}

func (r *hygieneRenamer) pop() {
	if len(r.scopes) == 0 {
		return
	}
	r.scopes = r.scopes[:len(r.scopes)-1]
}

func (r *hygieneRenamer) declare(name string) string {
	if name == "" || isPlaceholder(name) {
		return name
	}
	newName := r.unique(name)
	r.scopes[len(r.scopes)-1][name] = newName
	return newName
}

func (r *hygieneRenamer) declareRenamed(name, renamed string) {
	if name == "" || isPlaceholder(name) {
		return
	}
	r.scopes[len(r.scopes)-1][name] = renamed
}

func (r *hygieneRenamer) lookup(name string) (string, bool) {
	for i := len(r.scopes) - 1; i >= 0; i-- {
		if v, ok := r.scopes[i][name]; ok {
			return v, true
		}
	}
	return "", false
}

func (r *hygieneRenamer) unique(base string) string {
	prefix := sanitizeIdent(base)
	if prefix == "" {
		prefix = "tmp"
	}
	name := fmt.Sprintf("__dast_hyg_%s_%d", prefix, *r.nextID)
	*r.nextID++
	return name
}

func (r *hygieneRenamer) renameItem(item ast.Item) {
	switch v := item.(type) {
	case *ast.Function:
		r.push()
		for _, p := range v.Params {
			r.declareRenamed(p.Name, p.Name)
		}
		r.renameBlock(v.Body)
		r.pop()
	case *ast.ImplDecl:
		for _, m := range v.Methods {
			r.renameItem(m)
		}
	}
}

func (r *hygieneRenamer) renameBlock(block *ast.Block) {
	if block == nil {
		return
	}
	r.push()
	for _, stmt := range block.Stmts {
		r.renameStmt(stmt)
	}
	r.pop()
}

func (r *hygieneRenamer) renameStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.Block:
		r.renameBlock(s)
	case *ast.LetStmt:
		r.renameExpr(s.Init)
		s.Name = r.declare(s.Name)
	case *ast.LetPatternStmt:
		r.renameExpr(s.Init)
		binds := r.renamePattern(s.Pattern)
		for old, renamed := range binds {
			r.declareRenamed(old, renamed)
		}
	case *ast.AssignStmt:
		r.renameExpr(s.Target)
		r.renameExpr(s.Value)
	case *ast.ExprStmt:
		r.renameExpr(s.Expr)
	case *ast.ReturnStmt:
		if s.Value != nil {
			r.renameExpr(s.Value)
		}
	case *ast.IfStmt:
		r.renameExpr(s.Cond)
		r.renameBlock(s.Then)
		if s.Else != nil {
			r.renameBlock(s.Else)
		}
	case *ast.IfLetStmt:
		r.renameExpr(s.Expr)
		binds := r.renamePattern(s.Pattern)
		r.push()
		for old, renamed := range binds {
			r.declareRenamed(old, renamed)
		}
		r.renameBlock(s.Then)
		r.pop()
		if s.Else != nil {
			r.renameBlock(s.Else)
		}
	case *ast.WhileStmt:
		r.renameExpr(s.Cond)
		r.renameBlock(s.Body)
	case *ast.WhileLetStmt:
		r.renameExpr(s.Expr)
		binds := r.renamePattern(s.Pattern)
		r.push()
		for old, renamed := range binds {
			r.declareRenamed(old, renamed)
		}
		r.renameBlock(s.Body)
		r.pop()
	case *ast.LoopStmt:
		r.renameBlock(s.Body)
	case *ast.MatchStmt:
		r.renameExpr(s.Expr)
		for i := range s.Arms {
			arm := &s.Arms[i]
			binds := r.renamePattern(arm.Pattern)
			r.push()
			for old, renamed := range binds {
				r.declareRenamed(old, renamed)
			}
			if arm.Guard != nil {
				r.renameExpr(arm.Guard)
			}
			r.renameBlock(arm.Body)
			r.pop()
		}
	}
}

func (r *hygieneRenamer) renameExpr(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		if renamed, ok := r.lookup(e.Name); ok && !isPlaceholder(e.Name) {
			e.Name = renamed
		}
	case *ast.UnaryExpr:
		r.renameExpr(e.Expr)
	case *ast.RefExpr:
		r.renameExpr(e.Expr)
	case *ast.DerefExpr:
		r.renameExpr(e.Expr)
	case *ast.BinaryExpr:
		r.renameExpr(e.Left)
		r.renameExpr(e.Right)
	case *ast.CallExpr:
		for _, arg := range e.Args {
			r.renameExpr(arg)
		}
	case *ast.MethodCallExpr:
		r.renameExpr(e.Receiver)
		for _, arg := range e.Args {
			r.renameExpr(arg)
		}
	case *ast.StructLit:
		for _, f := range e.Fields {
			r.renameExpr(f.Value)
		}
	case *ast.AccessExpr:
		r.renameExpr(e.Receiver)
	case *ast.IndexExpr:
		r.renameExpr(e.Receiver)
		r.renameExpr(e.Index)
	case *ast.EnumVariantExpr:
		if e.Arg != nil {
			r.renameExpr(e.Arg)
		}
	case *ast.BlockExpr:
		r.renameBlock(e.Block)
	case *ast.IfExpr:
		r.renameExpr(e.Cond)
		r.renameExpr(e.Then)
		r.renameExpr(e.Else)
	case *ast.MatchExpr:
		r.renameExpr(e.Expr)
		for i := range e.Arms {
			arm := &e.Arms[i]
			binds := r.renamePattern(arm.Pattern)
			r.push()
			for old, renamed := range binds {
				r.declareRenamed(old, renamed)
			}
			if arm.Guard != nil {
				r.renameExpr(arm.Guard)
			}
			r.renameBlock(arm.Body)
			r.pop()
		}
	case *ast.ArrayLit:
		for _, elem := range e.Elems {
			r.renameExpr(elem)
		}
	case *ast.CompileExpr:
		r.renameExpr(e.Expr)
	case *ast.MacroCallExpr:
		r.renameExpr(e.Callee)
		for _, arg := range e.Args {
			r.renameExpr(arg)
		}
	}
}

func (r *hygieneRenamer) renamePattern(pat ast.Pattern) map[string]string {
	binds := map[string]string{}
	r.renamePatternWithMap(pat, binds, true)
	return binds
}

func (r *hygieneRenamer) renamePatternWithMap(pat ast.Pattern, binds map[string]string, generate bool) {
	switch p := pat.(type) {
	case *ast.BindingPattern:
		if p.Name != "" && !isPlaceholder(p.Name) {
			if renamed, ok := binds[p.Name]; ok {
				p.Name = renamed
				return
			}
			if !generate {
				return
			}
			renamed := r.unique(p.Name)
			binds[p.Name] = renamed
			p.Name = renamed
		}
	case *ast.VariantPattern:
		if p.Payload != nil {
			r.renamePatternWithMap(p.Payload, binds, generate)
			return
		}
		if p.Binding != "" && !isPlaceholder(p.Binding) {
			if renamed, ok := binds[p.Binding]; ok {
				p.Binding = renamed
				return
			}
			if !generate {
				return
			}
			renamed := r.unique(p.Binding)
			binds[p.Binding] = renamed
			p.Binding = renamed
		}
	case *ast.StructPattern:
		for i := range p.Fields {
			field := &p.Fields[i]
			if field.Pattern == nil {
				if field.Binding != "" && !isPlaceholder(field.Binding) {
					if renamed, ok := binds[field.Binding]; ok {
						field.Binding = renamed
						continue
					}
					if generate {
						renamed := r.unique(field.Binding)
						binds[field.Binding] = renamed
						field.Binding = renamed
					}
				}
				continue
			}
			r.renamePatternWithMap(field.Pattern, binds, generate)
		}
	case *ast.TuplePattern:
		for _, el := range p.Elems {
			r.renamePatternWithMap(el, binds, generate)
		}
	case *ast.ArrayPattern:
		for _, el := range p.Elems {
			r.renamePatternWithMap(el, binds, generate)
		}
	case *ast.OrPattern:
		if len(p.Alts) == 0 {
			return
		}
		r.renamePatternWithMap(p.Alts[0], binds, generate)
		for i := 1; i < len(p.Alts); i++ {
			r.renamePatternWithMap(p.Alts[i], binds, false)
		}
	}
}

func isPlaceholder(name string) bool {
	_, ok := placeholderIndex(name)
	return ok
}
