package typecheck

import (
	"fmt"

	"dastlang/internal/ast"
)

func (c *Checker) expandAlias(alias *ast.TypeAlias, use ast.Type) (ast.Type, bool) {
	if alias == nil {
		return ast.Type{}, false
	}
	if len(alias.TypeParams) == 0 {
		if len(use.Args) > 0 {
			c.diag.Add(use.Span, fmt.Sprintf("type alias '%s' does not accept type args", alias.Name))
			return ast.Type{}, false
		}
		out := alias.Value
		if use.IsRef {
			out.IsRef = true
			out.IsMut = use.IsMut
		}
		return out, true
	}
	if len(use.Args) != len(alias.TypeParams) {
		c.diag.Add(use.Span, fmt.Sprintf("type alias '%s' expects %d type args, got %d", alias.Name, len(alias.TypeParams), len(use.Args)))
		return ast.Type{}, false
	}
	subst := map[string]ast.Type{}
	for i, p := range alias.TypeParams {
		subst[p.Name] = use.Args[i]
	}
	out := substAstType(alias.Value, subst)
	if use.IsRef {
		out.IsRef = true
		out.IsMut = use.IsMut
	}
	return out, true
}

func substAstType(t ast.Type, subst map[string]ast.Type) ast.Type {
	out := t
	if t.IsArray && t.Elem != nil {
		elem := substAstType(*t.Elem, subst)
		out.Elem = &elem
	}
	if len(t.Args) > 0 {
		out.Args = make([]ast.Type, 0, len(t.Args))
		for _, arg := range t.Args {
			out.Args = append(out.Args, substAstType(arg, subst))
		}
	}
	if rep, ok := subst[t.Name]; ok && !t.IsArray {
		base := rep
		if t.IsRef {
			base.IsRef = true
			base.IsMut = t.IsMut
		}
		return base
	}
	return out
}

func (c *Checker) expandAliasType(t ast.Type) ast.Type {
	if alias, ok := c.aliases[t.Name]; ok {
		expanded, ok := c.expandAlias(alias, t)
		if ok {
			return c.expandAliasType(expanded)
		}
	}
	if t.IsArray && t.Elem != nil {
		elem := c.expandAliasType(*t.Elem)
		t.Elem = &elem
	}
	if t.IsTuple && len(t.TupleElems) > 0 {
		for i := range t.TupleElems {
			t.TupleElems[i] = c.expandAliasType(t.TupleElems[i])
		}
	}
	if len(t.Args) > 0 {
		for i := range t.Args {
			t.Args[i] = c.expandAliasType(t.Args[i])
		}
	}
	return t
}

func (c *Checker) expandAliasesInProgram(prog *ast.Program) {
	for _, item := range prog.Items {
		switch t := item.(type) {
		case *ast.Function:
			for i := range t.Params {
				t.Params[i].Type = c.expandAliasType(t.Params[i].Type)
			}
			if t.ReturnType != nil {
				rt := c.expandAliasType(*t.ReturnType)
				t.ReturnType = &rt
			}
			c.expandAliasesInBlock(t.Body)
		case *ast.StructDecl:
			for i := range t.Fields {
				t.Fields[i].Type = c.expandAliasType(t.Fields[i].Type)
			}
		case *ast.EnumDecl:
			for i := range t.Variants {
				if t.Variants[i].Payload != nil {
					pt := c.expandAliasType(*t.Variants[i].Payload)
					t.Variants[i].Payload = &pt
				}
			}
		case *ast.ConstDecl:
			if t.Type != nil {
				tt := c.expandAliasType(*t.Type)
				t.Type = &tt
			}
		case *ast.TypeAlias:
			t.Value = c.expandAliasType(t.Value)
		case *ast.ImplDecl:
			for i := range t.TypeArgs {
				t.TypeArgs[i] = c.expandAliasType(t.TypeArgs[i])
			}
			for _, m := range t.Methods {
				for i := range m.Params {
					m.Params[i].Type = c.expandAliasType(m.Params[i].Type)
				}
				if m.ReturnType != nil {
					rt := c.expandAliasType(*m.ReturnType)
					m.ReturnType = &rt
				}
				c.expandAliasesInBlock(m.Body)
			}
		case *ast.ImplTraitDecl:
			for i := range t.TraitArgs {
				t.TraitArgs[i] = c.expandAliasType(t.TraitArgs[i])
			}
			for i := range t.ForTypeArgs {
				t.ForTypeArgs[i] = c.expandAliasType(t.ForTypeArgs[i])
			}
			for _, m := range t.Methods {
				for i := range m.Params {
					m.Params[i].Type = c.expandAliasType(m.Params[i].Type)
				}
				if m.ReturnType != nil {
					rt := c.expandAliasType(*m.ReturnType)
					m.ReturnType = &rt
				}
				c.expandAliasesInBlock(m.Body)
			}
		}
	}
}

func (c *Checker) expandAliasesInBlock(b *ast.Block) {
	if b == nil {
		return
	}
	for _, stmt := range b.Stmts {
		c.expandAliasesInStmt(stmt)
	}
}

func (c *Checker) expandAliasesInStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		if s.Type != nil {
			t := c.expandAliasType(*s.Type)
			s.Type = &t
		}
		c.expandAliasesInExpr(s.Init)
	case *ast.LetPatternStmt:
		if s.Type != nil {
			t := c.expandAliasType(*s.Type)
			s.Type = &t
		}
		c.expandAliasesInExpr(s.Init)
	case *ast.AssignStmt:
		c.expandAliasesInExpr(s.Target)
		c.expandAliasesInExpr(s.Value)
	case *ast.ExprStmt:
		c.expandAliasesInExpr(s.Expr)
	case *ast.ReturnStmt:
		if s.Value != nil {
			c.expandAliasesInExpr(s.Value)
		}
	case *ast.IfStmt:
		c.expandAliasesInExpr(s.Cond)
		c.expandAliasesInBlock(s.Then)
		c.expandAliasesInBlock(s.Else)
	case *ast.IfLetStmt:
		c.expandAliasesInExpr(s.Expr)
		c.expandAliasesInBlock(s.Then)
		c.expandAliasesInBlock(s.Else)
	case *ast.WhileStmt:
		c.expandAliasesInExpr(s.Cond)
		c.expandAliasesInBlock(s.Body)
	case *ast.WhileLetStmt:
		c.expandAliasesInExpr(s.Expr)
		c.expandAliasesInBlock(s.Body)
	case *ast.LoopStmt:
		c.expandAliasesInBlock(s.Body)
	case *ast.ForStmt:
		c.expandAliasesInExpr(s.Expr)
		c.expandAliasesInBlock(s.Body)
	case *ast.BreakStmt:
		if s.Value != nil {
			c.expandAliasesInExpr(s.Value)
		}
	case *ast.MatchStmt:
		c.expandAliasesInExpr(s.Expr)
		for i := range s.Arms {
			if s.Arms[i].Guard != nil {
				c.expandAliasesInExpr(s.Arms[i].Guard)
			}
			c.expandAliasesInBlock(s.Arms[i].Body)
		}
	case *ast.Block:
		c.expandAliasesInBlock(s)
	}
}

func (c *Checker) expandAliasesInExpr(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.ArrayLit:
		for _, el := range e.Elems {
			c.expandAliasesInExpr(el)
		}
	case *ast.TupleLit:
		for _, el := range e.Elems {
			c.expandAliasesInExpr(el)
		}
	case *ast.UnaryExpr:
		c.expandAliasesInExpr(e.Expr)
	case *ast.RefExpr:
		c.expandAliasesInExpr(e.Expr)
	case *ast.DerefExpr:
		c.expandAliasesInExpr(e.Expr)
	case *ast.BinaryExpr:
		c.expandAliasesInExpr(e.Left)
		c.expandAliasesInExpr(e.Right)
	case *ast.CallExpr:
		for i := range e.TypeArgs {
			e.TypeArgs[i] = c.expandAliasType(e.TypeArgs[i])
		}
		for _, arg := range e.Args {
			c.expandAliasesInExpr(arg)
		}
	case *ast.MethodCallExpr:
		c.expandAliasesInExpr(e.Receiver)
		for _, arg := range e.Args {
			c.expandAliasesInExpr(arg)
		}
	case *ast.StructLit:
		for i := range e.TypeArgs {
			e.TypeArgs[i] = c.expandAliasType(e.TypeArgs[i])
		}
		for _, f := range e.Fields {
			c.expandAliasesInExpr(f.Value)
		}
	case *ast.AccessExpr:
		c.expandAliasesInExpr(e.Receiver)
	case *ast.IndexExpr:
		c.expandAliasesInExpr(e.Receiver)
		c.expandAliasesInExpr(e.Index)
	case *ast.EnumVariantExpr:
		for i := range e.TypeArgs {
			e.TypeArgs[i] = c.expandAliasType(e.TypeArgs[i])
		}
		if e.Arg != nil {
			c.expandAliasesInExpr(e.Arg)
		}
	case *ast.BlockExpr:
		c.expandAliasesInBlock(e.Block)
	case *ast.LoopExpr:
		c.expandAliasesInBlock(e.Body)
	case *ast.IfExpr:
		c.expandAliasesInExpr(e.Cond)
		c.expandAliasesInExpr(e.Then)
		if e.Else != nil {
			c.expandAliasesInExpr(e.Else)
		}
	case *ast.MatchExpr:
		c.expandAliasesInExpr(e.Expr)
		for i := range e.Arms {
			if e.Arms[i].Guard != nil {
				c.expandAliasesInExpr(e.Arms[i].Guard)
			}
			c.expandAliasesInBlock(e.Arms[i].Body)
		}
	case *ast.CompileExpr:
		c.expandAliasesInExpr(e.Expr)
	case *ast.QuoteExpr:
		for _, part := range e.Parts {
			if part.Expr != nil {
				c.expandAliasesInExpr(part.Expr)
			}
		}
	case *ast.MacroCallExpr:
		c.expandAliasesInExpr(e.Callee)
		for _, arg := range e.Args {
			c.expandAliasesInExpr(arg)
		}
	}
}
