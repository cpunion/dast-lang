package typecheck

import "dastlang/internal/ast"

func typeToAst(t Type) ast.Type {
	out := ast.Type{}
	if t.Kind == TypeTuple {
		out.IsTuple = true
		if len(t.Elems) > 0 {
			out.TupleElems = make([]ast.Type, 0, len(t.Elems))
			for _, e := range t.Elems {
				out.TupleElems = append(out.TupleElems, typeToAst(e))
			}
		}
	} else if t.Kind == TypeArray && t.Elem != nil {
		elem := typeToAst(*t.Elem)
		out.IsArray = true
		out.Elem = &elem
	} else {
		if t.Name != "" {
			out.Name = t.Name
		} else {
			out.Name = t.baseName()
		}
		if len(t.Args) > 0 {
			out.Args = make([]ast.Type, 0, len(t.Args))
			for _, a := range t.Args {
				out.Args = append(out.Args, typeToAst(a))
			}
		}
	}
	if t.Ref {
		out.IsRef = true
		out.IsMut = t.Mut
	}
	return out
}

func (c *Checker) cloneType(t ast.Type, subst map[string]Type) ast.Type {
	if t.Name == "Self" && !t.IsArray {
		if rep, ok := subst["Self"]; ok {
			out := typeToAst(rep)
			if t.IsRef {
				out.IsRef = true
				out.IsMut = t.IsMut
			}
			return out
		}
		if c.selfType != nil {
			out := typeToAst(*c.selfType)
			if t.IsRef {
				out.IsRef = true
				out.IsMut = t.IsMut
			}
			return out
		}
	}
	if rep, ok := subst[t.Name]; ok && !t.IsArray {
		out := typeToAst(rep)
		if t.IsRef {
			out.IsRef = true
			out.IsMut = t.IsMut
		}
		return out
	}
	out := t
	if t.IsArray && t.Elem != nil {
		elem := c.cloneType(*t.Elem, subst)
		out.Elem = &elem
	}
	if t.IsTuple && len(t.TupleElems) > 0 {
		out.TupleElems = make([]ast.Type, 0, len(t.TupleElems))
		for _, e := range t.TupleElems {
			out.TupleElems = append(out.TupleElems, c.cloneType(e, subst))
		}
	}
	if len(t.Args) > 0 {
		out.Args = make([]ast.Type, 0, len(t.Args))
		for _, a := range t.Args {
			out.Args = append(out.Args, c.cloneType(a, subst))
		}
	}
	return out
}

func (c *Checker) cloneParam(p ast.Param, subst map[string]Type) ast.Param {
	out := p
	out.Type = c.cloneType(p.Type, subst)
	return out
}

func (c *Checker) cloneFunction(fn *ast.Function, subst map[string]Type) *ast.Function {
	out := *fn
	out.TypeParams = nil
	out.Params = make([]ast.Param, 0, len(fn.Params))
	for _, p := range fn.Params {
		out.Params = append(out.Params, c.cloneParam(p, subst))
	}
	if fn.ReturnType != nil {
		rt := c.cloneType(*fn.ReturnType, subst)
		out.ReturnType = &rt
	}
	out.Body = c.cloneBlock(fn.Body, subst)
	return &out
}

func (c *Checker) cloneBlock(b *ast.Block, subst map[string]Type) *ast.Block {
	if b == nil {
		return nil
	}
	out := &ast.Block{SpanInfo: b.SpanInfo}
	if len(b.Stmts) == 0 {
		return out
	}
	out.Stmts = make([]ast.Stmt, 0, len(b.Stmts))
	for _, s := range b.Stmts {
		out.Stmts = append(out.Stmts, c.cloneStmt(s, subst))
	}
	return out
}

func (c *Checker) cloneStmt(stmt ast.Stmt, subst map[string]Type) ast.Stmt {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		out := *s
		if s.Type != nil {
			t := c.cloneType(*s.Type, subst)
			out.Type = &t
		}
		out.Init = c.cloneExpr(s.Init, subst)
		return &out
	case *ast.LetPatternStmt:
		out := *s
		out.Pattern = c.clonePattern(s.Pattern, subst)
		if s.Type != nil {
			t := c.cloneType(*s.Type, subst)
			out.Type = &t
		}
		out.Init = c.cloneExpr(s.Init, subst)
		return &out
	case *ast.AssignStmt:
		out := *s
		out.Target = c.cloneExpr(s.Target, subst)
		out.Value = c.cloneExpr(s.Value, subst)
		return &out
	case *ast.ExprStmt:
		out := *s
		out.Expr = c.cloneExpr(s.Expr, subst)
		return &out
	case *ast.ReturnStmt:
		out := *s
		if s.Value != nil {
			out.Value = c.cloneExpr(s.Value, subst)
		}
		return &out
	case *ast.IfStmt:
		out := *s
		out.Cond = c.cloneExpr(s.Cond, subst)
		out.Then = c.cloneBlock(s.Then, subst)
		out.Else = c.cloneBlock(s.Else, subst)
		return &out
	case *ast.IfLetStmt:
		out := *s
		out.Pattern = c.clonePattern(s.Pattern, subst)
		out.Expr = c.cloneExpr(s.Expr, subst)
		out.Then = c.cloneBlock(s.Then, subst)
		out.Else = c.cloneBlock(s.Else, subst)
		return &out
	case *ast.WhileStmt:
		out := *s
		out.Cond = c.cloneExpr(s.Cond, subst)
		out.Body = c.cloneBlock(s.Body, subst)
		return &out
	case *ast.WhileLetStmt:
		out := *s
		out.Pattern = c.clonePattern(s.Pattern, subst)
		out.Expr = c.cloneExpr(s.Expr, subst)
		out.Body = c.cloneBlock(s.Body, subst)
		return &out
	case *ast.LoopStmt:
		out := *s
		out.Body = c.cloneBlock(s.Body, subst)
		return &out
	case *ast.BreakStmt:
		out := *s
		if s.Value != nil {
			out.Value = c.cloneExpr(s.Value, subst)
		}
		return &out
	case *ast.ContinueStmt:
		out := *s
		return &out
	case *ast.ForStmt:
		out := *s
		out.Pattern = c.clonePattern(s.Pattern, subst)
		out.Expr = c.cloneExpr(s.Expr, subst)
		out.Body = c.cloneBlock(s.Body, subst)
		return &out
	case *ast.MatchStmt:
		out := *s
		out.Expr = c.cloneExpr(s.Expr, subst)
		if len(s.Arms) > 0 {
			out.Arms = make([]ast.MatchArm, 0, len(s.Arms))
			for _, arm := range s.Arms {
				armCopy := arm
				armCopy.Pattern = c.clonePattern(arm.Pattern, subst)
				if arm.Guard != nil {
					armCopy.Guard = c.cloneExpr(arm.Guard, subst)
				}
				armCopy.Body = c.cloneBlock(arm.Body, subst)
				out.Arms = append(out.Arms, armCopy)
			}
		}
		return &out
	case *ast.Block:
		return c.cloneBlock(s, subst)
	default:
		return stmt
	}
}

func (c *Checker) cloneExpr(expr ast.Expr, subst map[string]Type) ast.Expr {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		out := *e
		return &out
	case *ast.IntLit:
		out := *e
		return &out
	case *ast.CharLit:
		out := *e
		return &out
	case *ast.FloatLit:
		out := *e
		return &out
	case *ast.BoolLit:
		out := *e
		return &out
	case *ast.StringLit:
		out := *e
		return &out
	case *ast.ArrayLit:
		out := *e
		out.Elems = nil
		for _, el := range e.Elems {
			out.Elems = append(out.Elems, c.cloneExpr(el, subst))
		}
		return &out
	case *ast.TupleLit:
		out := *e
		out.Elems = nil
		for _, el := range e.Elems {
			out.Elems = append(out.Elems, c.cloneExpr(el, subst))
		}
		return &out
	case *ast.UnaryExpr:
		out := *e
		out.Expr = c.cloneExpr(e.Expr, subst)
		return &out
	case *ast.RefExpr:
		out := *e
		out.Expr = c.cloneExpr(e.Expr, subst)
		return &out
	case *ast.DerefExpr:
		out := *e
		out.Expr = c.cloneExpr(e.Expr, subst)
		return &out
	case *ast.BinaryExpr:
		out := *e
		out.Left = c.cloneExpr(e.Left, subst)
		out.Right = c.cloneExpr(e.Right, subst)
		return &out
	case *ast.CallExpr:
		out := *e
		out.Args = nil
		for _, arg := range e.Args {
			out.Args = append(out.Args, c.cloneExpr(arg, subst))
		}
		out.TypeArgs = nil
		for _, ta := range e.TypeArgs {
			out.TypeArgs = append(out.TypeArgs, c.cloneType(ta, subst))
		}
		out.SpanInfo = e.SpanInfo
		return &out
	case *ast.MethodCallExpr:
		out := *e
		out.Receiver = c.cloneExpr(e.Receiver, subst)
		out.Args = nil
		for _, arg := range e.Args {
			out.Args = append(out.Args, c.cloneExpr(arg, subst))
		}
		out.ResolvedName = ""
		out.ResolvedSelf = false
		out.EnumName = ""
		return &out
	case *ast.StructLit:
		out := *e
		out.Fields = nil
		for _, f := range e.Fields {
			fc := f
			fc.Value = c.cloneExpr(f.Value, subst)
			out.Fields = append(out.Fields, fc)
		}
		out.TypeArgs = nil
		for _, ta := range e.TypeArgs {
			out.TypeArgs = append(out.TypeArgs, c.cloneType(ta, subst))
		}
		return &out
	case *ast.AccessExpr:
		out := *e
		out.Receiver = c.cloneExpr(e.Receiver, subst)
		return &out
	case *ast.IndexExpr:
		out := *e
		out.Receiver = c.cloneExpr(e.Receiver, subst)
		out.Index = c.cloneExpr(e.Index, subst)
		return &out
	case *ast.EnumVariantExpr:
		out := *e
		if e.Arg != nil {
			out.Arg = c.cloneExpr(e.Arg, subst)
		}
		out.TypeArgs = nil
		for _, ta := range e.TypeArgs {
			out.TypeArgs = append(out.TypeArgs, c.cloneType(ta, subst))
		}
		return &out
	case *ast.BlockExpr:
		out := *e
		out.Block = c.cloneBlock(e.Block, subst)
		return &out
	case *ast.LoopExpr:
		out := *e
		out.Body = c.cloneBlock(e.Body, subst)
		return &out
	case *ast.IfExpr:
		out := *e
		out.Cond = c.cloneExpr(e.Cond, subst)
		out.Then = c.cloneExpr(e.Then, subst)
		if e.Else != nil {
			out.Else = c.cloneExpr(e.Else, subst)
		}
		return &out
	case *ast.MatchExpr:
		out := *e
		out.Expr = c.cloneExpr(e.Expr, subst)
		out.Arms = nil
		for _, arm := range e.Arms {
			armCopy := arm
			armCopy.Pattern = c.clonePattern(arm.Pattern, subst)
			if arm.Guard != nil {
				armCopy.Guard = c.cloneExpr(arm.Guard, subst)
			}
			armCopy.Body = c.cloneBlock(arm.Body, subst)
			out.Arms = append(out.Arms, armCopy)
		}
		return &out
	case *ast.CompileExpr:
		out := *e
		out.Expr = c.cloneExpr(e.Expr, subst)
		return &out
	case *ast.QuoteExpr:
		out := *e
		out.Parts = nil
		for _, part := range e.Parts {
			pc := part
			if part.Expr != nil {
				pc.Expr = c.cloneExpr(part.Expr, subst)
			}
			out.Parts = append(out.Parts, pc)
		}
		return &out
	case *ast.MacroCallExpr:
		out := *e
		out.Callee = c.cloneExpr(e.Callee, subst)
		out.Args = nil
		for _, arg := range e.Args {
			out.Args = append(out.Args, c.cloneExpr(arg, subst))
		}
		return &out
	default:
		return expr
	}
}

func (c *Checker) clonePattern(pat ast.Pattern, subst map[string]Type) ast.Pattern {
	switch p := pat.(type) {
	case *ast.WildcardPattern:
		out := *p
		return &out
	case *ast.BindingPattern:
		out := *p
		return &out
	case *ast.LiteralPattern:
		out := *p
		return &out
	case *ast.RangePattern:
		out := *p
		if p.Start != nil {
			out.Start = c.cloneExpr(p.Start, subst)
		}
		if p.End != nil {
			out.End = c.cloneExpr(p.End, subst)
		}
		return &out
	case *ast.OrPattern:
		out := *p
		out.Alts = nil
		for _, alt := range p.Alts {
			out.Alts = append(out.Alts, c.clonePattern(alt, subst))
		}
		return &out
	case *ast.VariantPattern:
		out := *p
		if p.Payload != nil {
			out.Payload = c.clonePattern(p.Payload, subst)
		}
		return &out
	case *ast.StructPattern:
		out := *p
		out.Fields = nil
		for _, f := range p.Fields {
			fc := f
			if f.Pattern != nil {
				fc.Pattern = c.clonePattern(f.Pattern, subst)
			}
			out.Fields = append(out.Fields, fc)
		}
		return &out
	case *ast.TuplePattern:
		out := *p
		out.Elems = nil
		for _, el := range p.Elems {
			out.Elems = append(out.Elems, c.clonePattern(el, subst))
		}
		return &out
	case *ast.ArrayPattern:
		out := *p
		out.Elems = nil
		for _, el := range p.Elems {
			out.Elems = append(out.Elems, c.clonePattern(el, subst))
		}
		return &out
	default:
		return pat
	}
}
