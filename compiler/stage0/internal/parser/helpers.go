package parser

import (
	"fmt"
	"strings"

	"dastlang/internal/ast"
	"dastlang/internal/lexer"
	"dastlang/internal/source"
)

func (p *Parser) maybeConsumeSemicolon() {
	if p.match(lexer.TokenSemicolon) {
		return
	}
}

// newlineBefore reports whether tok starts on a later line than the last
// consumed token. We use this to implement a minimal semicolon-insertion rule:
// a new line can terminate the previous expression unless it is clearly
// continued.
func (p *Parser) newlineBefore(tok lexer.Token) bool {
	prev := p.prev()
	if prev.Kind == lexer.TokenEOF {
		return false
	}
	if prev.Span.End.Line == 0 || tok.Span.Start.Line == 0 {
		return false
	}
	return tok.Span.Start.Line > prev.Span.End.Line
}

// canEndExpr is a conservative check for tokens that may legally end an
// expression. If the next operator appears on a new line after such a token,
// we treat it as a statement break.
func canEndExpr(kind lexer.TokenKind) bool {
	switch kind {
	case lexer.TokenIdent,
		lexer.TokenInt,
		lexer.TokenString,
		lexer.TokenChar,
		lexer.TokenTrue,
		lexer.TokenFalse,
		lexer.TokenSelf,
		lexer.TokenSelfType,
		lexer.TokenRParen,
		lexer.TokenRBracket,
		lexer.TokenRBrace:
		return true
	default:
		return false
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
	tok := p.peek()
	p.errorCurrent(msg)
	if !p.at(lexer.TokenEOF) {
		p.advance()
	}
	return lexer.Token{Kind: kind, Span: tok.Span}
}

func (p *Parser) parseQualifiedName() (string, source.Span) {
	nameTok := p.expect(lexer.TokenIdent, "expected identifier")
	name := nameTok.Lexeme
	span := nameTok.Span
	for p.match(lexer.TokenDot) {
		partTok := p.expect(lexer.TokenIdent, "expected identifier")
		name += "." + partTok.Lexeme
		span = mergeSpan(span, partTok.Span)
	}
	return name, span
}

func (p *Parser) peekQualifiedName() (end int, name string, last string, ok bool) {
	if p.peek().Kind != lexer.TokenIdent {
		return 0, "", "", false
	}
	end = p.pos + 1
	name = p.peek().Lexeme
	last = name
	for end+1 < len(p.tokens) && p.tokens[end].Kind == lexer.TokenDot && p.tokens[end+1].Kind == lexer.TokenIdent {
		name += "." + p.tokens[end+1].Lexeme
		last = p.tokens[end+1].Lexeme
		end += 2
	}
	return end, name, last, true
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
	case lexer.TokenPipe:
		return 5, true
	case lexer.TokenCaret:
		return 6, true
	case lexer.TokenAmp:
		return 7, true
	case lexer.TokenShl, lexer.TokenShr:
		return 8, true
	case lexer.TokenPlus, lexer.TokenMinus:
		return 9, true
	case lexer.TokenStar, lexer.TokenSlash, lexer.TokenPercent:
		return 10, true
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
		if tokens[i].Kind == lexer.TokenType && tokens[i+1].Kind == lexer.TokenIdent {
			names[tokens[i+1].Lexeme] = struct{}{}
		}
	}
	return names
}

func (p *Parser) isStructName(name string) bool {
	_, ok := p.structNames[name]
	return ok
}

func (p *Parser) structLitStart() bool {
	end, name, _, ok := p.peekQualifiedName()
	if !ok {
		return false
	}
	if !strings.Contains(name, ".") && !p.isStructName(name) {
		return false
	}
	idx := end
	if idx < len(p.tokens) && p.tokens[idx].Kind == lexer.TokenLBracket {
		idx = p.skipBracketList(idx)
	}
	if idx >= len(p.tokens) || p.tokens[idx].Kind != lexer.TokenLBrace {
		return false
	}
	if idx+1 >= len(p.tokens) {
		return false
	}
	next := p.tokens[idx+1].Kind
	if next == lexer.TokenRBrace {
		return true
	}
	if next == lexer.TokenIdent && idx+2 < len(p.tokens) && p.tokens[idx+2].Kind == lexer.TokenColon {
		return true
	}
	return false
}

func (p *Parser) skipBracketList(start int) int {
	if start >= len(p.tokens) || p.tokens[start].Kind != lexer.TokenLBracket {
		return start
	}
	depth := 0
	i := start
	for i < len(p.tokens) {
		switch p.tokens[i].Kind {
		case lexer.TokenLBracket:
			depth++
		case lexer.TokenRBracket:
			depth--
			if depth == 0 {
				return i + 1
			}
		}
		i++
	}
	return start
}
