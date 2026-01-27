package parser

import (
	"dastlang/internal/ast"
	"dastlang/internal/lexer"
	"dastlang/internal/source"
)

func (p *Parser) parseStructDecl(vis ast.Visibility) ast.Item {
	nameTok := p.expect(lexer.TokenIdent, "expected struct name")
	decl := &ast.StructDecl{Name: nameTok.Lexeme, Vis: vis, SpanInfo: nameTok.Span}
	if p.at(lexer.TokenLBracket) {
		decl.TypeParams = p.parseTypeParams()
	}
	p.expect(lexer.TokenLBrace, "expected '{'")
	for !p.at(lexer.TokenRBrace) && !p.at(lexer.TokenEOF) {
		fieldName := p.expect(lexer.TokenIdent, "expected field name")
		p.expect(lexer.TokenColon, "expected ':' in field")
		t := p.parseType()
		decl.Fields = append(decl.Fields, ast.FieldDef{Name: fieldName.Lexeme, Type: t, Span: mergeSpan(fieldName.Span, t.Span)})
		if p.match(lexer.TokenComma) {
			continue
		}
	}
	rbrace := p.expect(lexer.TokenRBrace, "expected '}'")
	decl.SpanInfo = mergeSpan(decl.SpanInfo, rbrace.Span)
	return decl
}

func (p *Parser) parseImport() ast.Item {
	startTok := p.prev()
	pathTok := p.expect(lexer.TokenString, "expected import path")
	alias := ""
	endSpan := pathTok.Span
	if p.match(lexer.TokenAs) {
		aliasTok := p.expect(lexer.TokenIdent, "expected import alias")
		alias = aliasTok.Lexeme
		endSpan = aliasTok.Span
	}
	p.maybeConsumeSemicolon()
	return &ast.ImportDecl{Path: pathTok.Lexeme, Alias: alias, SpanInfo: mergeSpan(startTok.Span, endSpan)}
}

func (p *Parser) parseEnumDecl(repr string, vis ast.Visibility) ast.Item {
	nameTok := p.expect(lexer.TokenIdent, "expected enum name")
	decl := &ast.EnumDecl{Name: nameTok.Lexeme, Repr: repr, Vis: vis, SpanInfo: nameTok.Span}
	if p.at(lexer.TokenLBracket) {
		decl.TypeParams = p.parseTypeParams()
	}
	p.expect(lexer.TokenLBrace, "expected '{'")
	for !p.at(lexer.TokenRBrace) && !p.at(lexer.TokenEOF) {
		variantTok := p.expect(lexer.TokenIdent, "expected variant name")
		variant := ast.VariantDef{Name: variantTok.Lexeme, SpanInfo: variantTok.Span}
		if p.match(lexer.TokenLParen) {
			t := p.parseType()
			variant.Payload = &t
			p.expect(lexer.TokenRParen, "expected ')' after variant payload")
		}
		if p.match(lexer.TokenAssign) {
			value, span := p.parseConstInt()
			variant.HasValue = true
			variant.Value = value
			variant.ValueSpan = span
		}
		decl.Variants = append(decl.Variants, variant)
		if p.match(lexer.TokenComma) {
			continue
		}
	}
	rbrace := p.expect(lexer.TokenRBrace, "expected '}'")
	decl.SpanInfo = mergeSpan(decl.SpanInfo, rbrace.Span)
	return decl
}

func (p *Parser) parseConstDecl(vis ast.Visibility) ast.Item {
	nameTok := p.expect(lexer.TokenIdent, "expected const name")
	decl := &ast.ConstDecl{Name: nameTok.Lexeme, Vis: vis, SpanInfo: nameTok.Span}
	if p.match(lexer.TokenColon) {
		t := p.parseType()
		decl.Type = &t
	}
	p.expect(lexer.TokenAssign, "expected '=' in const declaration")
	value := p.parseConstValue()
	decl.Value = value
	if p.match(lexer.TokenSemicolon) {
	}
	decl.SpanInfo = mergeSpan(decl.SpanInfo, value.Span)
	return decl
}

func (p *Parser) parseTypeAlias(vis ast.Visibility) ast.Item {
	nameTok := p.expect(lexer.TokenIdent, "expected type name")
	alias := &ast.TypeAlias{Name: nameTok.Lexeme, Vis: vis, SpanInfo: nameTok.Span}
	if p.at(lexer.TokenLBracket) {
		alias.TypeParams = p.parseTypeParams()
	}
	p.expect(lexer.TokenAssign, "expected '=' in type alias")
	val := p.parseType()
	alias.Value = val
	p.maybeConsumeSemicolon()
	alias.SpanInfo = mergeSpan(alias.SpanInfo, val.Span)
	return alias
}

func (p *Parser) parseTraitDecl(vis ast.Visibility) ast.Item {
	nameTok := p.expect(lexer.TokenIdent, "expected trait name")
	decl := &ast.TraitDecl{Name: nameTok.Lexeme, Vis: vis, SpanInfo: nameTok.Span}
	if p.at(lexer.TokenLBracket) {
		decl.TypeParams = p.parseTypeParams()
	}
	p.expect(lexer.TokenLBrace, "expected '{' after trait name")
	for !p.at(lexer.TokenRBrace) && !p.at(lexer.TokenEOF) {
		vis := ast.VisPrivate
		if p.match(lexer.TokenPub) {
			vis = ast.VisPublic
		}
		if !p.match(lexer.TokenFn) {
			p.errorCurrent("expected trait method")
			p.advance()
			continue
		}
		method := p.parseTraitMethod(vis)
		decl.Methods = append(decl.Methods, method)
	}
	rbrace := p.expect(lexer.TokenRBrace, "expected '}'")
	decl.SpanInfo = mergeSpan(decl.SpanInfo, rbrace.Span)
	return decl
}

func (p *Parser) parseTraitMethod(vis ast.Visibility) ast.TraitMethod {
	nameTok := p.expect(lexer.TokenIdent, "expected method name")
	method := ast.TraitMethod{Name: nameTok.Lexeme, SpanInfo: nameTok.Span}
	_ = vis
	p.expect(lexer.TokenLParen, "expected '('")
	if !p.at(lexer.TokenRParen) {
		for {
			param := p.parseParam()
			method.Params = append(method.Params, param)
			if p.match(lexer.TokenComma) {
				continue
			}
			break
		}
	}
	p.expect(lexer.TokenRParen, "expected ')'")
	if p.match(lexer.TokenArrow) {
		ret := p.parseType()
		method.ReturnType = &ret
		method.SpanInfo = mergeSpan(method.SpanInfo, ret.Span)
	}
	p.expect(lexer.TokenSemicolon, "expected ';' after trait method")
	return method
}

func (p *Parser) parseFunction(vis ast.Visibility, isMacro bool) *ast.Function {
	nameTok := p.expect(lexer.TokenIdent, "expected function name")
	fn := &ast.Function{Name: nameTok.Lexeme, Vis: vis, IsMacro: isMacro, SpanInfo: mergeSpan(nameTok.Span, nameTok.Span)}
	if p.at(lexer.TokenLBracket) {
		fn.TypeParams = p.parseTypeParams()
	}

	p.expect(lexer.TokenLParen, "expected '('")
	if !p.at(lexer.TokenRParen) {
		for {
			param := p.parseParam()
			fn.Params = append(fn.Params, param)
			if p.match(lexer.TokenComma) {
				continue
			}
			break
		}
	}
	p.expect(lexer.TokenRParen, "expected ')'")

	if p.match(lexer.TokenArrow) {
		ret := p.parseType()
		fn.ReturnType = &ret
	}

	body := p.parseBlock()
	fn.Body = body
	fn.SpanInfo = mergeSpan(fn.SpanInfo, body.Span())
	return fn
}

func (p *Parser) parseTypeParams() []ast.TypeParam {
	if !p.match(lexer.TokenLBracket) {
		return nil
	}
	var params []ast.TypeParam
	for !p.at(lexer.TokenRBracket) && !p.at(lexer.TokenEOF) {
		nameTok := p.expect(lexer.TokenIdent, "expected type parameter")
		param := ast.TypeParam{Name: nameTok.Lexeme, Span: nameTok.Span}
		if p.match(lexer.TokenColon) {
			for {
				boundName, boundSpan := p.parseQualifiedName()
				if boundName == "" {
					p.diag.Add(boundSpan, "expected trait bound")
				} else {
					param.Bounds = append(param.Bounds, boundName)
				}
				if !p.match(lexer.TokenPlus) {
					break
				}
			}
		}
		params = append(params, param)
		if p.match(lexer.TokenComma) {
			continue
		}
		break
	}
	p.expect(lexer.TokenRBracket, "expected ']' after type parameters")
	return params
}

func (p *Parser) parseTypeArgs() []ast.Type {
	if !p.match(lexer.TokenLBracket) {
		return nil
	}
	var args []ast.Type
	for !p.at(lexer.TokenRBracket) && !p.at(lexer.TokenEOF) {
		arg := p.parseType()
		args = append(args, arg)
		if p.match(lexer.TokenComma) {
			continue
		}
		break
	}
	p.expect(lexer.TokenRBracket, "expected ']' after type arguments")
	return args
}

func (p *Parser) parseParam() ast.Param {
	nameTok := p.peek()
	if nameTok.Kind != lexer.TokenIdent && nameTok.Kind != lexer.TokenSelf {
		p.errorCurrent("expected parameter name")
		nameTok = p.advance()
	} else {
		p.advance()
	}
	p.expect(lexer.TokenColon, "expected ':' in parameter")
	t := p.parseType()
	return ast.Param{Name: nameTok.Lexeme, Type: t, Span: mergeSpan(nameTok.Span, t.Span)}
}

func (p *Parser) parseReprAttr() string {
	nameTok := p.expect(lexer.TokenIdent, "expected attribute name")
	if nameTok.Lexeme != "repr" {
		p.diag.Add(nameTok.Span, "unknown attribute '"+nameTok.Lexeme+"'")
		if p.match(lexer.TokenLParen) {
			for !p.at(lexer.TokenRParen) && !p.at(lexer.TokenEOF) {
				p.advance()
			}
			p.match(lexer.TokenRParen)
		}
		return ""
	}
	p.expect(lexer.TokenLParen, "expected '(' after repr")
	reprTok := p.expect(lexer.TokenIdent, "expected repr type")
	p.expect(lexer.TokenRParen, "expected ')'")
	return reprTok.Lexeme
}

func (p *Parser) parseType() ast.Type {
	start := p.peek().Span
	ref := false
	mut := false
	if p.match(lexer.TokenAmp) {
		ref = true
		if p.match(lexer.TokenMut) {
			mut = true
		}
	}
	t := ast.Type{}
	if p.match(lexer.TokenLBracket) {
		elem := p.parseType()
		endTok := p.expect(lexer.TokenRBracket, "expected ']' in array type")
		t.IsArray = true
		t.Elem = &elem
		t.Span = mergeSpan(start, endTok.Span)
		if ref {
			t.IsRef = true
			t.IsMut = mut
		}
		return t
	}
	if p.match(lexer.TokenLParen) {
		if p.at(lexer.TokenRParen) {
			endTok := p.advance()
			t.IsTuple = true
			t.TupleElems = nil
			t.Span = mergeSpan(start, endTok.Span)
			if ref {
				t.IsRef = true
				t.IsMut = mut
			}
			return t
		}
		first := p.parseType()
		if p.match(lexer.TokenComma) {
			elems := []ast.Type{first}
			for !p.at(lexer.TokenRParen) && !p.at(lexer.TokenEOF) {
				elems = append(elems, p.parseType())
				if p.match(lexer.TokenComma) {
					continue
				}
				break
			}
			endTok := p.expect(lexer.TokenRParen, "expected ')' in tuple type")
			t.IsTuple = true
			t.TupleElems = elems
			t.Span = mergeSpan(start, endTok.Span)
			if ref {
				t.IsRef = true
				t.IsMut = mut
			}
			return t
		}
		endTok := p.expect(lexer.TokenRParen, "expected ')' in type")
		t = first
		t.Span = mergeSpan(start, endTok.Span)
		if ref {
			t.IsRef = true
			t.IsMut = mut
		}
		return t
	}
	if p.peek().Kind == lexer.TokenSelfType {
		nameTok := p.advance()
		t.Name = nameTok.Lexeme
		if t.Span == (source.Span{}) {
			t.Span = mergeSpan(start, nameTok.Span)
		}
		if p.at(lexer.TokenLBracket) {
			p.diag.Add(p.peek().Span, "type arguments not allowed on Self")
		}
		if ref {
			t.IsRef = true
			t.IsMut = mut
		}
		return t
	}
	name, span := p.parseQualifiedName()
	t.Name = name
	if t.Span == (source.Span{}) {
		t.Span = mergeSpan(start, span)
	}
	if p.match(lexer.TokenLBracket) {
		for !p.at(lexer.TokenRBracket) && !p.at(lexer.TokenEOF) {
			arg := p.parseType()
			t.Args = append(t.Args, arg)
			if p.match(lexer.TokenComma) {
				continue
			}
			break
		}
		end := p.expect(lexer.TokenRBracket, "expected ']' after type arguments")
		t.Span = mergeSpan(t.Span, end.Span)
	}
	if ref {
		t.IsRef = true
		t.IsMut = mut
	}
	return t
}

func (p *Parser) parseConstInt() (int64, source.Span) {
	negTok := p.peek()
	if p.match(lexer.TokenMinus) {
		intTok := p.expect(lexer.TokenInt, "expected int literal after '-'")
		val := -parseInt(intTok.Lexeme)
		return val, mergeSpan(negTok.Span, intTok.Span)
	}
	intTok := p.expect(lexer.TokenInt, "expected int literal")
	return parseInt(intTok.Lexeme), intTok.Span
}

func (p *Parser) parseConstValue() ast.ConstValue {
	expr := p.parseExpr(0)
	val, ok := evalConstExpr(expr)
	if !ok {
		p.diag.Add(expr.Span(), "unsupported const expression in stage 0")
		return ast.ConstValue{Kind: ast.ConstInt, Int: 0, Span: expr.Span()}
	}
	return val
}

func evalConstExpr(expr ast.Expr) (ast.ConstValue, bool) {
	switch e := expr.(type) {
	case *ast.IntLit:
		return ast.ConstValue{Kind: ast.ConstInt, Int: e.Value, Span: e.Span()}, true
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
			if lhs.Kind == ast.ConstInt && rhs.Kind == ast.ConstInt {
				return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int + rhs.Int, Span: e.Span()}, true
			}
			if lhs.Kind == ast.ConstString && rhs.Kind == ast.ConstString {
				return ast.ConstValue{Kind: ast.ConstString, Str: lhs.Str + rhs.Str, Span: e.Span()}, true
			}
			return ast.ConstValue{}, false
		case "-":
			if lhs.Kind != ast.ConstInt || rhs.Kind != ast.ConstInt {
				return ast.ConstValue{}, false
			}
			return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int - rhs.Int, Span: e.Span()}, true
		case "*":
			if lhs.Kind != ast.ConstInt || rhs.Kind != ast.ConstInt {
				return ast.ConstValue{}, false
			}
			return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int * rhs.Int, Span: e.Span()}, true
		case "/":
			if lhs.Kind != ast.ConstInt || rhs.Kind != ast.ConstInt || rhs.Int == 0 {
				return ast.ConstValue{}, false
			}
			return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int / rhs.Int, Span: e.Span()}, true
		case "%":
			if lhs.Kind != ast.ConstInt || rhs.Kind != ast.ConstInt || rhs.Int == 0 {
				return ast.ConstValue{}, false
			}
			return ast.ConstValue{Kind: ast.ConstInt, Int: lhs.Int % rhs.Int, Span: e.Span()}, true
		default:
			return ast.ConstValue{}, false
		}
	default:
		return ast.ConstValue{}, false
	}
}

func (p *Parser) parseImplDecl(vis ast.Visibility) ast.Item {
	start := p.prev().Span
	var typeParams []ast.TypeParam
	if p.at(lexer.TokenLBracket) {
		typeParams = p.parseTypeParams()
	}
	baseType := p.parseType()
	if baseType.IsRef || baseType.IsArray {
		p.diag.Add(baseType.Span, "impl target must be nominal type")
	}
	if p.match(lexer.TokenFor) {
		traitType := baseType
		forType := p.parseType()
		if forType.IsRef || forType.IsArray {
			p.diag.Add(forType.Span, "impl target must be nominal type")
		}
		impl := &ast.ImplTraitDecl{
			TraitName: traitType.Name,
			TraitArgs: traitType.Args,
			ForTypeName: forType.Name,
			ForTypeArgs: forType.Args,
			TypeParams: typeParams,
			Vis:       vis,
			SpanInfo:  mergeSpan(start, forType.Span),
		}
		p.expect(lexer.TokenLBrace, "expected '{' after impl")
		for !p.at(lexer.TokenRBrace) && !p.at(lexer.TokenEOF) {
			vis := ast.VisPrivate
			if p.match(lexer.TokenPub) {
				vis = ast.VisPublic
			}
			if p.match(lexer.TokenFn) {
				fn := p.parseFunction(vis, false)
				impl.Methods = append(impl.Methods, fn)
				continue
			}
			p.errorCurrent("expected method")
			p.advance()
		}
		rbrace := p.expect(lexer.TokenRBrace, "expected '}'")
		impl.SpanInfo = mergeSpan(impl.SpanInfo, rbrace.Span)
		return impl
	}
	impl := &ast.ImplDecl{
		TypeName:  baseType.Name,
		TypeArgs:  baseType.Args,
		TypeParams: typeParams,
		Vis:       vis,
		SpanInfo:  mergeSpan(start, baseType.Span),
	}
	p.expect(lexer.TokenLBrace, "expected '{' after impl type")
	for !p.at(lexer.TokenRBrace) && !p.at(lexer.TokenEOF) {
		vis := ast.VisPrivate
		if p.match(lexer.TokenPub) {
			vis = ast.VisPublic
		}
		if p.match(lexer.TokenFn) {
			fn := p.parseFunction(vis, false)
			impl.Methods = append(impl.Methods, fn)
			continue
		}
		p.errorCurrent("expected method")
		p.advance()
	}
	rbrace := p.expect(lexer.TokenRBrace, "expected '}'")
	impl.SpanInfo = mergeSpan(impl.SpanInfo, rbrace.Span)
	return impl
}
