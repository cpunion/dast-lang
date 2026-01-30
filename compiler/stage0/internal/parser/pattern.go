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
	if p.match(lexer.TokenDotDot) || p.match(lexer.TokenDotDotEq) {
		startSpan := p.prev().Span
		inclusive := p.prev().Kind == lexer.TokenDotDotEq
		var endExpr ast.Expr
		if !p.rangeEndTerminator() {
			endExpr = p.parseRangeExpr()
		} else if inclusive {
			p.diag.Add(startSpan, "range pattern requires end for ..=")
		}
		endSpan := startSpan
		if endExpr != nil {
			endSpan = endExpr.Span()
		}
		return &ast.RangePattern{Start: nil, End: endExpr, Inclusive: inclusive, SpanInfo: mergeSpan(startSpan, endSpan)}
	}
	pat := p.parsePatternAtom()
	if p.match(lexer.TokenDotDot) || p.match(lexer.TokenDotDotEq) {
		inclusive := p.prev().Kind == lexer.TokenDotDotEq
		startExpr, ok := patternToRangeExpr(pat)
		if !ok {
			p.diag.Add(p.prev().Span, "range pattern requires simple start")
			return pat
		}
		var endExpr ast.Expr
		if !p.rangeEndTerminator() {
			endExpr = p.parseRangeExpr()
		} else if inclusive {
			p.diag.Add(p.prev().Span, "range pattern requires end for ..=")
		}
		endSpan := p.prev().Span
		if endExpr != nil {
			endSpan = endExpr.Span()
		}
		return &ast.RangePattern{Start: startExpr, End: endExpr, Inclusive: inclusive, SpanInfo: mergeSpan(pat.Span(), endSpan)}
	}
	return pat
}

func (p *Parser) parsePatternAtom() ast.Pattern {
	if p.match(lexer.TokenDot) {
		variantTok := p.expect(lexer.TokenIdent, "expected variant name")
		binding := ""
		var payload ast.Pattern
		if p.match(lexer.TokenLParen) {
			if p.at(lexer.TokenRParen) {
				endTok := p.advance()
				payload = &ast.WildcardPattern{SpanInfo: mergeSpan(variantTok.Span, endTok.Span)}
			} else {
				payload = p.parsePattern()
				p.expect(lexer.TokenRParen, "expected ')' after binding")
			}
			if payload != nil {
				binding = ""
			}
		}
		return &ast.VariantPattern{EnumName: "", Variant: variantTok.Lexeme, Binding: binding, Payload: payload, SpanInfo: mergeSpan(variantTok.Span, variantTok.Span)}
	}
	if p.at(lexer.TokenIdent) && p.peek().Lexeme == "_" {
		tok := p.advance()
		return &ast.WildcardPattern{SpanInfo: tok.Span}
	}
	if p.match(lexer.TokenLParen) {
		start := p.prev().Span
		if p.at(lexer.TokenRParen) {
			endTok := p.advance()
			return &ast.TuplePattern{Elems: nil, SpanInfo: mergeSpan(start, endTok.Span)}
		}
		first := p.parsePattern()
		if p.match(lexer.TokenComma) {
			elems := []ast.Pattern{first}
			for !p.at(lexer.TokenRParen) && !p.at(lexer.TokenEOF) {
				elems = append(elems, p.parsePattern())
				if p.match(lexer.TokenComma) {
					continue
				}
				break
			}
			endTok := p.expect(lexer.TokenRParen, "expected ')' in tuple pattern")
			return &ast.TuplePattern{Elems: elems, SpanInfo: mergeSpan(start, endTok.Span)}
		}
		p.expect(lexer.TokenRParen, "expected ')' in pattern")
		return first
	}
	if p.match(lexer.TokenLBracket) {
		start := p.prev().Span
		var elems []ast.Pattern
		if !p.at(lexer.TokenRBracket) {
			for {
				elems = append(elems, p.parsePattern())
				if p.match(lexer.TokenComma) {
					if p.at(lexer.TokenRBracket) {
						break
					}
					continue
				}
				break
			}
		}
		endTok := p.expect(lexer.TokenRBracket, "expected ']' in array pattern")
		return &ast.ArrayPattern{Elems: elems, SpanInfo: mergeSpan(start, endTok.Span)}
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
				var payload ast.Pattern
				if p.match(lexer.TokenLParen) {
					if p.at(lexer.TokenRParen) {
						endTok := p.advance()
						payload = &ast.WildcardPattern{SpanInfo: mergeSpan(span, endTok.Span)}
					} else {
						payload = p.parsePattern()
						p.expect(lexer.TokenRParen, "expected ')' after binding")
					}
					if payload != nil {
						binding = ""
					}
				}
				return &ast.VariantPattern{EnumName: enumName, Variant: variant, Binding: binding, Payload: payload, SpanInfo: span}
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
		return &ast.BindingPattern{Name: nameTok.Lexeme, SpanInfo: nameTok.Span}
	}
	p.errorCurrent("expected pattern")
	return &ast.WildcardPattern{SpanInfo: p.peek().Span}
}

func (p *Parser) rangeEndTerminator() bool {
	switch p.peek().Kind {
	case lexer.TokenComma, lexer.TokenPipe, lexer.TokenFatArrow, lexer.TokenRParen, lexer.TokenRBrace, lexer.TokenIf, lexer.TokenEOF:
		return true
	}
	return false
}

func (p *Parser) parseRangeExpr() ast.Expr {
	return p.parseExpr(0)
}

func patternToRangeExpr(pat ast.Pattern) (ast.Expr, bool) {
	switch p := pat.(type) {
	case *ast.BindingPattern:
		return &ast.IdentExpr{Name: p.Name, SpanInfo: p.SpanInfo}, true
	case *ast.LiteralPattern:
		switch p.Value.Kind {
		case ast.ConstInt:
			return &ast.IntLit{Value: p.Value.Int, SpanInfo: p.SpanInfo}, true
		case ast.ConstChar:
			return &ast.CharLit{Value: p.Value.Int, SpanInfo: p.SpanInfo}, true
		case ast.ConstBool:
			return &ast.BoolLit{Value: p.Value.Bool, SpanInfo: p.SpanInfo}, true
		case ast.ConstString:
			return &ast.StringLit{Value: p.Value.Str, SpanInfo: p.SpanInfo}, true
		case ast.ConstFloat:
			return &ast.FloatLit{Text: p.Value.FloatText, SpanInfo: p.SpanInfo}, true
		}
	}
	return nil, false
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
