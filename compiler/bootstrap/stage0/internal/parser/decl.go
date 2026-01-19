package parser

import (
	"dastlang/internal/ast"
	"dastlang/internal/lexer"
	"dastlang/internal/source"
)

func (p *Parser) parseStructDecl() ast.Item {
	nameTok := p.expect(lexer.TokenIdent, "expected struct name")
	decl := &ast.StructDecl{Name: nameTok.Lexeme, SpanInfo: nameTok.Span}
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

func (p *Parser) parseEnumDecl(repr string) ast.Item {
	nameTok := p.expect(lexer.TokenIdent, "expected enum name")
	decl := &ast.EnumDecl{Name: nameTok.Lexeme, Repr: repr, SpanInfo: nameTok.Span}
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

func (p *Parser) parseConstDecl() ast.Item {
	nameTok := p.expect(lexer.TokenIdent, "expected const name")
	decl := &ast.ConstDecl{Name: nameTok.Lexeme, SpanInfo: nameTok.Span}
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

func (p *Parser) parseFunction() ast.Item {
	nameTok := p.expect(lexer.TokenIdent, "expected function name")
	fn := &ast.Function{Name: nameTok.Lexeme, SpanInfo: mergeSpan(nameTok.Span, nameTok.Span)}

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
	t := ast.Type{}
	if p.match(lexer.TokenAmp) {
		t.IsRef = true
		if p.match(lexer.TokenMut) {
			t.IsMut = true
		}
	}
	if p.match(lexer.TokenLBracket) {
		elem := p.parseType()
		p.expect(lexer.TokenRBracket, "expected ']' in array type")
		t.IsArray = true
		t.Elem = &elem
		t.Span = mergeSpan(start, elem.Span)
		return t
	}
	if p.peek().Kind == lexer.TokenSelfType {
		nameTok := p.advance()
		t.Name = nameTok.Lexeme
		if t.Span == (source.Span{}) {
			t.Span = mergeSpan(start, nameTok.Span)
		}
		return t
	}
	nameTok := p.expect(lexer.TokenIdent, "expected type name")
	t.Name = nameTok.Lexeme
	if t.Span == (source.Span{}) {
		t.Span = mergeSpan(start, nameTok.Span)
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
	tok := p.peek()
	switch tok.Kind {
	case lexer.TokenMinus, lexer.TokenInt:
		val, span := p.parseConstInt()
		return ast.ConstValue{Kind: ast.ConstInt, Int: val, Span: span}
	case lexer.TokenTrue:
		p.advance()
		return ast.ConstValue{Kind: ast.ConstBool, Bool: true, Span: tok.Span}
	case lexer.TokenFalse:
		p.advance()
		return ast.ConstValue{Kind: ast.ConstBool, Bool: false, Span: tok.Span}
	case lexer.TokenString:
		p.advance()
		return ast.ConstValue{Kind: ast.ConstString, Str: tok.Lexeme, Span: tok.Span}
	case lexer.TokenChar:
		p.advance()
		return ast.ConstValue{Kind: ast.ConstInt, Int: parseCharLiteral(tok.Lexeme), Span: tok.Span}
	default:
		p.errorCurrent("expected const literal")
		p.advance()
		return ast.ConstValue{Kind: ast.ConstInt, Int: 0, Span: tok.Span}
	}
}

func (p *Parser) parseImplDecl() ast.Item {
	start := p.prev().Span
	typeTok := p.expect(lexer.TokenIdent, "expected type name after impl")
	impl := &ast.ImplDecl{TypeName: typeTok.Lexeme, SpanInfo: mergeSpan(start, typeTok.Span)}
	p.expect(lexer.TokenLBrace, "expected '{' after impl type")
	for !p.at(lexer.TokenRBrace) && !p.at(lexer.TokenEOF) {
		if p.match(lexer.TokenFn) {
			fn := p.parseFunction().(*ast.Function)
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
