package parser

import (
	"dastlang/internal/ast"
	"dastlang/internal/lexer"
)

func (p *Parser) parsePattern() ast.Pattern {
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
	if p.match(lexer.TokenIdent) {
		nameTok := p.prev()
		if p.match(lexer.TokenDot) {
			variantTok := p.expect(lexer.TokenIdent, "expected variant name")
			binding := ""
			if p.match(lexer.TokenLParen) {
				bindTok := p.expect(lexer.TokenIdent, "expected binding name")
				binding = bindTok.Lexeme
				p.expect(lexer.TokenRParen, "expected ')' after binding")
			}
			return &ast.VariantPattern{EnumName: nameTok.Lexeme, Variant: variantTok.Lexeme, Binding: binding, SpanInfo: mergeSpan(nameTok.Span, variantTok.Span)}
		}
		p.diag.Add(nameTok.Span, "expected enum variant pattern")
		return &ast.WildcardPattern{SpanInfo: nameTok.Span}
	}
	p.errorCurrent("expected pattern")
	return &ast.WildcardPattern{SpanInfo: p.peek().Span}
}
