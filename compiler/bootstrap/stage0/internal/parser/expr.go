package parser

import (
	"dastlang/internal/ast"
	"dastlang/internal/lexer"
)

func (p *Parser) parseExpr(minPrec int) ast.Expr {
	expr := p.parseUnary()
	for {
		opTok := p.peek()
		prec, ok := infixPrec(opTok.Kind)
		if !ok || prec < minPrec {
			break
		}
		p.advance()
		nextMin := prec + 1
		right := p.parseExpr(nextMin)
		expr = &ast.BinaryExpr{Op: opTok.Lexeme, Left: expr, Right: right, SpanInfo: mergeSpan(expr.Span(), right.Span())}
	}
	return expr
}

func (p *Parser) parseUnary() ast.Expr {
	tok := p.peek()
	switch tok.Kind {
	case lexer.TokenMinus, lexer.TokenBang:
		p.advance()
		rhs := p.parseUnary()
		return &ast.UnaryExpr{Op: tok.Lexeme, Expr: rhs, SpanInfo: mergeSpan(tok.Span, rhs.Span())}
	case lexer.TokenAmp:
		p.advance()
		mutable := false
		if p.match(lexer.TokenMut) {
			mutable = true
		}
		rhs := p.parseUnary()
		return &ast.RefExpr{Mutable: mutable, Expr: rhs, SpanInfo: mergeSpan(tok.Span, rhs.Span())}
	case lexer.TokenStar:
		p.advance()
		rhs := p.parseUnary()
		return &ast.DerefExpr{Expr: rhs, SpanInfo: mergeSpan(tok.Span, rhs.Span())}
	default:
		return p.parsePostfix()
	}
}

func (p *Parser) parsePostfix() ast.Expr {
	expr := p.parsePrimary()
	for {
		if p.match(lexer.TokenDot) {
			nameTok := p.expect(lexer.TokenIdent, "expected field or variant name")
			expr = &ast.AccessExpr{Receiver: expr, Field: nameTok.Lexeme, SpanInfo: mergeSpan(expr.Span(), nameTok.Span)}
			continue
		}
		if p.match(lexer.TokenLBracket) {
			index := p.parseExpr(0)
			endTok := p.expect(lexer.TokenRBracket, "expected ']' after index")
			expr = &ast.IndexExpr{Receiver: expr, Index: index, SpanInfo: mergeSpan(expr.Span(), endTok.Span)}
			continue
		}
		if p.match(lexer.TokenLParen) {
			var args []ast.Expr
			if !p.at(lexer.TokenRParen) {
				for {
					args = append(args, p.parseExpr(0))
					if p.match(lexer.TokenComma) {
						continue
					}
					break
				}
			}
			p.expect(lexer.TokenRParen, "expected ')' after arguments")
			if ident, ok := expr.(*ast.IdentExpr); ok {
				expr = &ast.CallExpr{Callee: ident.Name, Args: args, SpanInfo: mergeSpan(expr.Span(), spanOfExprSlice(args))}
				continue
			}
			if access, ok := expr.(*ast.AccessExpr); ok {
				expr = &ast.MethodCallExpr{
					Receiver: access.Receiver,
					Method:   access.Field,
					Args:     args,
					SpanInfo: mergeSpan(expr.Span(), spanOfExprSlice(args)),
				}
				continue
			}
			p.diag.Add(expr.Span(), "can only call identifiers or field accesses in stage 0")
			return expr
		}
		break
	}
	return expr
}

func (p *Parser) parsePrimary() ast.Expr {
	tok := p.peek()
	switch tok.Kind {
	case lexer.TokenIdent:
		if end, _, last, ok := p.peekQualifiedName(); ok {
			if end < len(p.tokens) && p.tokens[end].Kind == lexer.TokenLBrace && p.isStructName(last) {
				return p.parseStructLit()
			}
		}
		p.advance()
		return &ast.IdentExpr{Name: tok.Lexeme, SpanInfo: tok.Span}
	case lexer.TokenSelf:
		p.advance()
		return &ast.IdentExpr{Name: tok.Lexeme, SpanInfo: tok.Span}
	case lexer.TokenInt:
		p.advance()
		return &ast.IntLit{Value: parseInt(tok.Lexeme), SpanInfo: tok.Span}
	case lexer.TokenString:
		p.advance()
		return &ast.StringLit{Value: tok.Lexeme, SpanInfo: tok.Span}
	case lexer.TokenChar:
		p.advance()
		return &ast.IntLit{Value: parseCharLiteral(tok.Lexeme), SpanInfo: tok.Span}
	case lexer.TokenTrue:
		p.advance()
		return &ast.BoolLit{Value: true, SpanInfo: tok.Span}
	case lexer.TokenFalse:
		p.advance()
		return &ast.BoolLit{Value: false, SpanInfo: tok.Span}
	case lexer.TokenLParen:
		p.advance()
		expr := p.parseExpr(0)
		p.expect(lexer.TokenRParen, "expected ')' after expression")
		return expr
	case lexer.TokenLBracket:
		return p.parseArrayLit()
	default:
		p.errorCurrent("unexpected token in expression")
		p.advance()
		return &ast.IntLit{Value: 0, SpanInfo: tok.Span}
	}
}

func (p *Parser) parseStructLit() ast.Expr {
	name, span := p.parseQualifiedName()
	start := span
	p.expect(lexer.TokenLBrace, "expected '{' in struct literal")
	lit := &ast.StructLit{Name: name, SpanInfo: start}
	for !p.at(lexer.TokenRBrace) && !p.at(lexer.TokenEOF) {
		fieldTok := p.expect(lexer.TokenIdent, "expected field name")
		p.expect(lexer.TokenColon, "expected ':' in field initializer")
		val := p.parseExpr(0)
		lit.Fields = append(lit.Fields, ast.FieldInit{Name: fieldTok.Lexeme, Value: val, Span: mergeSpan(fieldTok.Span, val.Span())})
		if p.match(lexer.TokenComma) {
			continue
		}
	}
	rbrace := p.expect(lexer.TokenRBrace, "expected '}'")
	lit.SpanInfo = mergeSpan(start, rbrace.Span)
	return lit
}

func parseInt(s string) int64 {
	var v int64
	for _, r := range s {
		v = v*10 + int64(r-'0')
	}
	return v
}

func parseCharLiteral(s string) int64 {
	if s == "" {
		return 0
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return 0
	}
	return int64(runes[0])
}

func (p *Parser) parseArrayLit() ast.Expr {
	startTok := p.expect(lexer.TokenLBracket, "expected '['")
	lit := &ast.ArrayLit{SpanInfo: startTok.Span}
	if !p.at(lexer.TokenRBracket) {
		for {
			elem := p.parseExpr(0)
			lit.Elems = append(lit.Elems, elem)
			if p.match(lexer.TokenComma) {
				continue
			}
			break
		}
	}
	endTok := p.expect(lexer.TokenRBracket, "expected ']'")
	lit.SpanInfo = mergeSpan(startTok.Span, endTok.Span)
	return lit
}
