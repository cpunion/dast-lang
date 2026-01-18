package parser

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/lexer"
	"dastlang/internal/source"
)

func (p *Parser) maybeConsumeSemicolon() {
	if p.match(lexer.TokenSemicolon) {
		return
	}
}

func lexTokens(filename string, input string) []lexer.Token {
	lx := lexer.New(filename, input)
	var toks []lexer.Token
	for {
		tok := lx.Next()
		toks = append(toks, tok)
		if tok.Kind == lexer.TokenEOF {
			break
		}
	}
	return toks
}

func (p *Parser) at(kind lexer.TokenKind) bool {
	return p.peek().Kind == kind
}

func (p *Parser) peek() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Kind: lexer.TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekN(n int) lexer.Token {
	idx := p.pos + n
	if idx >= len(p.tokens) {
		return lexer.Token{Kind: lexer.TokenEOF}
	}
	return p.tokens[idx]
}

func (p *Parser) prev() lexer.Token {
	idx := p.pos - 1
	if idx < 0 {
		return lexer.Token{Kind: lexer.TokenEOF}
	}
	return p.tokens[idx]
}

func (p *Parser) advance() lexer.Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) match(kind lexer.TokenKind) bool {
	if p.at(kind) {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) expect(kind lexer.TokenKind, msg string) lexer.Token {
	if p.at(kind) {
		return p.advance()
	}
	p.errorCurrent(msg)
	return lexer.Token{Kind: kind, Span: p.peek().Span}
}

func (p *Parser) errorCurrent(msg string) {
	tok := p.peek()
	p.diag.Add(tok.Span, fmt.Sprintf("%s: found %s", msg, tok.Kind.String()))
}

func mergeSpan(a source.Span, b source.Span) source.Span {
	if a == (source.Span{}) {
		return b
	}
	if b == (source.Span{}) {
		return a
	}
	return source.Span{Start: a.Start, End: b.End}
}

func spanOfExpr(expr ast.Expr) source.Span {
	if expr == nil {
		return source.Span{}
	}
	return expr.Span()
}

func spanOfExprSlice(exprs []ast.Expr) source.Span {
	if len(exprs) == 0 {
		return source.Span{}
	}
	return mergeSpan(exprs[0].Span(), exprs[len(exprs)-1].Span())
}

func infixPrec(kind lexer.TokenKind) (int, bool) {
	switch kind {
	case lexer.TokenOrOr:
		return 1, true
	case lexer.TokenAndAnd:
		return 2, true
	case lexer.TokenEqEq, lexer.TokenNotEq:
		return 3, true
	case lexer.TokenLt, lexer.TokenLtEq, lexer.TokenGt, lexer.TokenGtEq:
		return 4, true
	case lexer.TokenPlus, lexer.TokenMinus:
		return 5, true
	case lexer.TokenStar, lexer.TokenSlash, lexer.TokenPercent:
		return 6, true
	default:
		return 0, false
	}
}

func isAssignable(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.IdentExpr:
		return true
	case *ast.DerefExpr:
		return true
	case *ast.IndexExpr:
		return true
	case *ast.AccessExpr:
		return true
	default:
		return false
	}
}

func collectStructNames(tokens []lexer.Token) map[string]struct{} {
	names := map[string]struct{}{}
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i].Kind == lexer.TokenStruct && tokens[i+1].Kind == lexer.TokenIdent {
			names[tokens[i+1].Lexeme] = struct{}{}
		}
	}
	return names
}

func (p *Parser) isStructName(name string) bool {
	_, ok := p.structNames[name]
	return ok
}
