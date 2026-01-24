package parser

import (
	"strings"

	"dastlang/internal/ast"
	"dastlang/internal/lexer"
)

func (p *Parser) parsePattern() ast.Pattern {
	pat := p.parsePatternSingle()
	if p.match(lexer.TokenPipe) {
		alts := []ast.Pattern{pat}
		for {
			alts = append(alts, p.parsePatternSingle())
			if !p.match(lexer.TokenPipe) {
				break
			}
		}
		return &ast.OrPattern{Alts: alts, SpanInfo: mergeSpan(alts[0].Span(), alts[len(alts)-1].Span())}
	}
	return pat
}

func (p *Parser) parsePatternSingle() ast.Pattern {
	pat := p.parsePatternAtom()
	if p.match(lexer.TokenDotDot) || p.match(lexer.TokenDotDotEq) {
		inclusive := p.prev().Kind == lexer.TokenDotDotEq
		startLit, ok := pat.(*ast.LiteralPattern)
		if !ok || startLit.Value.Kind != ast.ConstInt {
			p.diag.Add(p.prev().Span, "range pattern requires int literal start")
			return pat
		}
		endTok := p.expect(lexer.TokenInt, "expected int literal end for range pattern")
		end := parseInt(endTok.Lexeme)
		return &ast.RangePattern{Start: startLit.Value.Int, End: end, Inclusive: inclusive, SpanInfo: mergeSpan(startLit.Span(), endTok.Span)}
	}
	return pat
}

func (p *Parser) parsePatternAtom() ast.Pattern {
	if p.match(lexer.TokenDot) {
		variantTok := p.expect(lexer.TokenIdent, "expected variant name")
		binding := ""
		if p.match(lexer.TokenLParen) {
			bindTok := p.expect(lexer.TokenIdent, "expected binding name")
			binding = bindTok.Lexeme
			p.expect(lexer.TokenRParen, "expected ')' after binding")
		}
		return &ast.VariantPattern{EnumName: "", Variant: variantTok.Lexeme, Binding: binding, SpanInfo: mergeSpan(variantTok.Span, variantTok.Span)}
	}
	if p.at(lexer.TokenIdent) && p.peek().Lexeme == "_" {
		tok := p.advance()
		return &ast.WildcardPattern{SpanInfo: tok.Span}
	}
	switch p.peek().Kind {
	case lexer.TokenInt, lexer.TokenMinus:
		val, span := p.parseConstInt()
		cv := ast.ConstValue{Kind: ast.ConstInt, Int: val, Span: span}
		return &ast.LiteralPattern{Value: cv, SpanInfo: span}
	case lexer.TokenTrue:
		tok := p.advance()
		cv := ast.ConstValue{Kind: ast.ConstBool, Bool: true, Span: tok.Span}
		return &ast.LiteralPattern{Value: cv, SpanInfo: tok.Span}
	case lexer.TokenFalse:
		tok := p.advance()
		cv := ast.ConstValue{Kind: ast.ConstBool, Bool: false, Span: tok.Span}
		return &ast.LiteralPattern{Value: cv, SpanInfo: tok.Span}
	case lexer.TokenString:
		tok := p.advance()
		cv := ast.ConstValue{Kind: ast.ConstString, Str: tok.Lexeme, Span: tok.Span}
		return &ast.LiteralPattern{Value: cv, SpanInfo: tok.Span}
	case lexer.TokenChar:
		tok := p.advance()
		cv := ast.ConstValue{Kind: ast.ConstInt, Int: parseCharLiteral(tok.Lexeme), Span: tok.Span}
		return &ast.LiteralPattern{Value: cv, SpanInfo: tok.Span}
	}
	if p.peek().Kind == lexer.TokenIdent {
		if end, name, _, ok := p.peekQualifiedName(); ok {
			if end < len(p.tokens) && p.tokens[end].Kind == lexer.TokenLBrace {
				fullName, span := p.parseQualifiedName()
				p.expect(lexer.TokenLBrace, "expected '{' in struct pattern")
				pat := &ast.StructPattern{StructName: fullName, SpanInfo: span}
				for !p.at(lexer.TokenRBrace) && !p.at(lexer.TokenEOF) {
					fieldTok := p.expect(lexer.TokenIdent, "expected field name")
					field := p.parseStructFieldPattern(fieldTok)
					pat.Fields = append(pat.Fields, field)
					if p.match(lexer.TokenComma) {
						continue
					}
				}
				endTok := p.expect(lexer.TokenRBrace, "expected '}' in struct pattern")
				pat.SpanInfo = mergeSpan(pat.SpanInfo, endTok.Span)
				return pat
			}
			if strings.Contains(name, ".") {
				fullName, span := p.parseQualifiedName()
				parts := strings.Split(fullName, ".")
				if len(parts) < 2 {
					p.diag.Add(span, "expected enum variant pattern")
					return &ast.WildcardPattern{SpanInfo: span}
				}
				enumName := strings.Join(parts[:len(parts)-1], ".")
				variant := parts[len(parts)-1]
				binding := ""
				if p.match(lexer.TokenLParen) {
					bindTok := p.expect(lexer.TokenIdent, "expected binding name")
					binding = bindTok.Lexeme
					p.expect(lexer.TokenRParen, "expected ')' after binding")
				}
				return &ast.VariantPattern{EnumName: enumName, Variant: variant, Binding: binding, SpanInfo: span}
			}
		}
		nameTok := p.advance()
		if p.at(lexer.TokenLBrace) {
			p.expect(lexer.TokenLBrace, "expected '{' in struct pattern")
			pat := &ast.StructPattern{StructName: nameTok.Lexeme, SpanInfo: nameTok.Span}
			for !p.at(lexer.TokenRBrace) && !p.at(lexer.TokenEOF) {
				fieldTok := p.expect(lexer.TokenIdent, "expected field name")
				field := p.parseStructFieldPattern(fieldTok)
				pat.Fields = append(pat.Fields, field)
				if p.match(lexer.TokenComma) {
					continue
				}
			}
			endTok := p.expect(lexer.TokenRBrace, "expected '}' in struct pattern")
			pat.SpanInfo = mergeSpan(pat.SpanInfo, endTok.Span)
			return pat
		}
		p.diag.Add(nameTok.Span, "expected enum or struct pattern")
		return &ast.WildcardPattern{SpanInfo: nameTok.Span}
	}
	p.errorCurrent("expected pattern")
	return &ast.WildcardPattern{SpanInfo: p.peek().Span}
}

func (p *Parser) parseStructFieldPattern(fieldTok lexer.Token) ast.StructFieldPattern {
	field := ast.StructFieldPattern{Name: fieldTok.Lexeme, Span: fieldTok.Span}
	if !p.match(lexer.TokenColon) {
		field.Binding = fieldTok.Lexeme
		return field
	}
	if p.at(lexer.TokenIdent) && p.peek().Lexeme != "_" {
		if end, name, _, ok := p.peekQualifiedName(); ok {
			if strings.Contains(name, ".") {
				field.Pattern = p.parsePattern()
				return field
			}
			if end < len(p.tokens) && p.tokens[end].Kind == lexer.TokenLBrace {
				field.Pattern = p.parsePattern()
				return field
			}
			bindTok := p.advance()
			field.Binding = bindTok.Lexeme
			return field
		}
	}
	field.Pattern = p.parsePattern()
	return field
}
