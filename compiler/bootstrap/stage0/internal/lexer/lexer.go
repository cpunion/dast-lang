package lexer

import (
	"fmt"
	"unicode"

	"dastlang/internal/source"
)

type Lexer struct {
	src      []rune
	filename string
	idx      int
	line     int
	col      int
}

func New(filename string, input string) *Lexer {
	return &Lexer{
		src:      []rune(input),
		filename: filename,
		idx:      0,
		line:     1,
		col:      1,
	}
}

func (l *Lexer) Next() Token {
	l.skipWhitespaceAndComments()
	start := l.position()
	if l.eof() {
		return Token{Kind: TokenEOF, Span: source.Span{Start: start, End: start}}
	}

	r := l.peek()
	if isIdentStart(r) {
		lex := l.readIdent()
		kind := lookupKeyword(lex)
		return Token{Kind: kind, Lexeme: lex, Span: source.Span{Start: start, End: l.position()}}
	}
	if unicode.IsDigit(r) {
		lex := l.readNumber()
		return Token{Kind: TokenInt, Lexeme: lex, Span: source.Span{Start: start, End: l.position()}}
	}

	switch r {
	case '"':
		lex, ok := l.readString()
		if !ok {
			return Token{Kind: TokenEOF, Span: source.Span{Start: start, End: l.position()}}
		}
		return Token{Kind: TokenString, Lexeme: lex, Span: source.Span{Start: start, End: l.position()}}
	case '\'':
		lex, ok := l.readChar()
		if !ok {
			return Token{Kind: TokenEOF, Span: source.Span{Start: start, End: l.position()}}
		}
		return Token{Kind: TokenChar, Lexeme: lex, Span: source.Span{Start: start, End: l.position()}}
	case '(':
		l.advance()
		return Token{Kind: TokenLParen, Lexeme: "(", Span: source.Span{Start: start, End: l.position()}}
	case ')':
		l.advance()
		return Token{Kind: TokenRParen, Lexeme: ")", Span: source.Span{Start: start, End: l.position()}}
	case '{':
		l.advance()
		return Token{Kind: TokenLBrace, Lexeme: "{", Span: source.Span{Start: start, End: l.position()}}
	case '}':
		l.advance()
		return Token{Kind: TokenRBrace, Lexeme: "}", Span: source.Span{Start: start, End: l.position()}}
	case '$':
		l.advance()
		return Token{Kind: TokenDollar, Lexeme: "$", Span: source.Span{Start: start, End: l.position()}}
	case '[':
		l.advance()
		return Token{Kind: TokenLBracket, Lexeme: "[", Span: source.Span{Start: start, End: l.position()}}
	case ']':
		l.advance()
		return Token{Kind: TokenRBracket, Lexeme: "]", Span: source.Span{Start: start, End: l.position()}}
	case ',':
		l.advance()
		return Token{Kind: TokenComma, Lexeme: ",", Span: source.Span{Start: start, End: l.position()}}
	case ':':
		l.advance()
		if l.match(':') {
			return Token{Kind: TokenColonColon, Lexeme: "::", Span: source.Span{Start: start, End: l.position()}}
		}
		return Token{Kind: TokenColon, Lexeme: ":", Span: source.Span{Start: start, End: l.position()}}
	case ';':
		l.advance()
		return Token{Kind: TokenSemicolon, Lexeme: ";", Span: source.Span{Start: start, End: l.position()}}
	case '.':
		l.advance()
		if l.match('.') {
			if l.match('=') {
				return Token{Kind: TokenDotDotEq, Lexeme: "..=", Span: source.Span{Start: start, End: l.position()}}
			}
			return Token{Kind: TokenDotDot, Lexeme: "..", Span: source.Span{Start: start, End: l.position()}}
		}
		return Token{Kind: TokenDot, Lexeme: ".", Span: source.Span{Start: start, End: l.position()}}
	case '@':
		l.advance()
		return Token{Kind: TokenAt, Lexeme: "@", Span: source.Span{Start: start, End: l.position()}}
	case '+':
		l.advance()
		return Token{Kind: TokenPlus, Lexeme: "+", Span: source.Span{Start: start, End: l.position()}}
	case '-':
		l.advance()
		if l.match('>') {
			return Token{Kind: TokenArrow, Lexeme: "->", Span: source.Span{Start: start, End: l.position()}}
		}
		return Token{Kind: TokenMinus, Lexeme: "-", Span: source.Span{Start: start, End: l.position()}}
	case '*':
		l.advance()
		return Token{Kind: TokenStar, Lexeme: "*", Span: source.Span{Start: start, End: l.position()}}
	case '/':
		l.advance()
		return Token{Kind: TokenSlash, Lexeme: "/", Span: source.Span{Start: start, End: l.position()}}
	case '%':
		l.advance()
		return Token{Kind: TokenPercent, Lexeme: "%", Span: source.Span{Start: start, End: l.position()}}
	case '!':
		l.advance()
		if l.match('=') {
			return Token{Kind: TokenNotEq, Lexeme: "!=", Span: source.Span{Start: start, End: l.position()}}
		}
		return Token{Kind: TokenBang, Lexeme: "!", Span: source.Span{Start: start, End: l.position()}}
	case '=':
		l.advance()
		if l.match('=') {
			return Token{Kind: TokenEqEq, Lexeme: "==", Span: source.Span{Start: start, End: l.position()}}
		}
		if l.match('>') {
			return Token{Kind: TokenFatArrow, Lexeme: "=>", Span: source.Span{Start: start, End: l.position()}}
		}
		return Token{Kind: TokenAssign, Lexeme: "=", Span: source.Span{Start: start, End: l.position()}}
	case '<':
		l.advance()
		if l.match('=') {
			return Token{Kind: TokenLtEq, Lexeme: "<=", Span: source.Span{Start: start, End: l.position()}}
		}
		return Token{Kind: TokenLt, Lexeme: "<", Span: source.Span{Start: start, End: l.position()}}
	case '>':
		l.advance()
		if l.match('=') {
			return Token{Kind: TokenGtEq, Lexeme: ">=", Span: source.Span{Start: start, End: l.position()}}
		}
		return Token{Kind: TokenGt, Lexeme: ">", Span: source.Span{Start: start, End: l.position()}}
	case '&':
		l.advance()
		if l.match('&') {
			return Token{Kind: TokenAndAnd, Lexeme: "&&", Span: source.Span{Start: start, End: l.position()}}
		}
		return Token{Kind: TokenAmp, Lexeme: "&", Span: source.Span{Start: start, End: l.position()}}
	case '|':
		l.advance()
		if l.match('|') {
			return Token{Kind: TokenOrOr, Lexeme: "||", Span: source.Span{Start: start, End: l.position()}}
		}
		return Token{Kind: TokenPipe, Lexeme: "|", Span: source.Span{Start: start, End: l.position()}}
	}

	l.advance()
	return Token{Kind: TokenEOF, Span: source.Span{Start: start, End: l.position()}, Lexeme: string(r)}
}

func (l *Lexer) position() source.Position {
	return source.Position{Filename: l.filename, Line: l.line, Column: l.col}
}

func (l *Lexer) eof() bool {
	return l.idx >= len(l.src)
}

func (l *Lexer) peek() rune {
	if l.eof() {
		return 0
	}
	return l.src[l.idx]
}

func (l *Lexer) advance() rune {
	if l.eof() {
		return 0
	}
	r := l.src[l.idx]
	l.idx++
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

func (l *Lexer) match(expected rune) bool {
	if l.eof() {
		return false
	}
	if l.src[l.idx] != expected {
		return false
	}
	l.advance()
	return true
}

func (l *Lexer) skipWhitespaceAndComments() {
	for {
		if l.eof() {
			return
		}
		r := l.peek()
		if unicode.IsSpace(r) {
			l.advance()
			continue
		}
		if r == '/' {
			if l.peekAhead("//") {
				l.advance()
				l.advance()
				for !l.eof() && l.peek() != '\n' {
					l.advance()
				}
				continue
			}
			if l.peekAhead("/*") {
				l.advance()
				l.advance()
				for !l.eof() {
					if l.peekAhead("*/") {
						l.advance()
						l.advance()
						break
					}
					l.advance()
				}
				continue
			}
		}
		return
	}
}

func (l *Lexer) peekAhead(s string) bool {
	if l.idx+len([]rune(s)) > len(l.src) {
		return false
	}
	for i, r := range s {
		if l.src[l.idx+i] != r {
			return false
		}
	}
	return true
}

func (l *Lexer) readIdent() string {
	start := l.idx
	l.advance()
	for !l.eof() && isIdentPart(l.peek()) {
		l.advance()
	}
	return string(l.src[start:l.idx])
}

func (l *Lexer) readNumber() string {
	start := l.idx
	for !l.eof() && unicode.IsDigit(l.peek()) {
		l.advance()
	}
	return string(l.src[start:l.idx])
}

func (l *Lexer) readString() (string, bool) {
	l.advance() // opening quote
	var out []rune
	for !l.eof() {
		r := l.advance()
		if r == '"' {
			return string(out), true
		}
		if r == '\\' {
			if l.eof() {
				return "", false
			}
			n := l.advance()
			switch n {
			case 'n':
				out = append(out, '\n')
			case 't':
				out = append(out, '\t')
			case '\\':
				out = append(out, '\\')
			case '"':
				out = append(out, '"')
			default:
				out = append(out, n)
			}
			continue
		}
		out = append(out, r)
	}
	return "", false
}

func (l *Lexer) readChar() (string, bool) {
	l.advance() // opening quote
	if l.eof() {
		return "", false
	}
	r := l.advance()
	var out rune
	if r == '\\' {
		if l.eof() {
			return "", false
		}
		n := l.advance()
		switch n {
		case 'n':
			out = '\n'
		case 't':
			out = '\t'
		case 'r':
			out = '\r'
		case '\\':
			out = '\\'
		case '\'':
			out = '\''
		default:
			out = n
		}
	} else {
		out = r
	}
	if l.eof() {
		return "", false
	}
	if l.peek() != '\'' {
		return "", false
	}
	l.advance()
	return string(out), true
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func lookupKeyword(lex string) TokenKind {
	switch lex {
	case "fn":
		return TokenFn
	case "struct":
		return TokenStruct
	case "enum":
		return TokenEnum
	case "impl":
		return TokenImpl
	case "trait":
		return TokenTrait
	case "self":
		return TokenSelf
	case "Self":
		return TokenSelfType
	case "const":
		return TokenConst
	case "type":
		return TokenType
	case "pub":
		return TokenPub
	case "macro":
		return TokenMacro
	case "quote":
		return TokenQuote
	case "import":
		return TokenImport
	case "as":
		return TokenAs
	case "let":
		return TokenLet
	case "mut":
		return TokenMut
	case "if":
		return TokenIf
	case "else":
		return TokenElse
	case "while":
		return TokenWhile
	case "loop":
		return TokenLoop
	case "for":
		return TokenFor
	case "break":
		return TokenBreak
	case "continue":
		return TokenContinue
	case "match":
		return TokenMatch
	case "return":
		return TokenReturn
	case "true":
		return TokenTrue
	case "false":
		return TokenFalse
	default:
		return TokenIdent
	}
}

func DebugTokens(filename, input string) []Token {
	lx := New(filename, input)
	var out []Token
	for {
		tok := lx.Next()
		out = append(out, tok)
		if tok.Kind == TokenEOF {
			break
		}
	}
	return out
}

func (t Token) Format() string {
	if t.Lexeme == "" {
		return t.Kind.String()
	}
	return fmt.Sprintf("%s(%s)", t.Kind.String(), t.Lexeme)
}
