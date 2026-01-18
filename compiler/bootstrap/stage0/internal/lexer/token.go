package lexer

import "dastlang/internal/source"

type TokenKind int

const (
	TokenEOF TokenKind = iota
	TokenIdent
	TokenInt
	TokenString

	TokenFn
	TokenStruct
	TokenEnum
	TokenImpl
	TokenSelf
	TokenSelfType
	TokenConst
	TokenLet
	TokenMut
	TokenIf
	TokenElse
	TokenWhile
	TokenMatch
	TokenReturn
	TokenTrue
	TokenFalse

	TokenLParen
	TokenRParen
	TokenLBrace
	TokenRBrace
	TokenLBracket
	TokenRBracket
	TokenComma
	TokenColon
	TokenSemicolon
	TokenArrow
	TokenFatArrow
	TokenDot
	TokenAt

	TokenAssign
	TokenPlus
	TokenMinus
	TokenStar
	TokenSlash
	TokenPercent
	TokenBang
	TokenAmp

	TokenEqEq
	TokenNotEq
	TokenLt
	TokenLtEq
	TokenGt
	TokenGtEq
	TokenAndAnd
	TokenOrOr
)

type Token struct {
	Kind   TokenKind
	Lexeme string
	Span   source.Span
}

func (k TokenKind) String() string {
	switch k {
	case TokenEOF:
		return "EOF"
	case TokenIdent:
		return "IDENT"
	case TokenInt:
		return "INT"
	case TokenString:
		return "STRING"
	case TokenFn:
		return "fn"
	case TokenStruct:
		return "struct"
	case TokenEnum:
		return "enum"
	case TokenImpl:
		return "impl"
	case TokenSelf:
		return "self"
	case TokenSelfType:
		return "Self"
	case TokenConst:
		return "const"
	case TokenLet:
		return "let"
	case TokenMut:
		return "mut"
	case TokenIf:
		return "if"
	case TokenElse:
		return "else"
	case TokenWhile:
		return "while"
	case TokenMatch:
		return "match"
	case TokenReturn:
		return "return"
	case TokenTrue:
		return "true"
	case TokenFalse:
		return "false"
	case TokenLParen:
		return "("
	case TokenRParen:
		return ")"
	case TokenLBrace:
		return "{"
	case TokenRBrace:
		return "}"
	case TokenLBracket:
		return "["
	case TokenRBracket:
		return "]"
	case TokenComma:
		return ","
	case TokenColon:
		return ":"
	case TokenSemicolon:
		return ";"
	case TokenArrow:
		return "->"
	case TokenFatArrow:
		return "=>"
	case TokenDot:
		return "."
	case TokenAt:
		return "@"
	case TokenAssign:
		return "="
	case TokenPlus:
		return "+"
	case TokenMinus:
		return "-"
	case TokenStar:
		return "*"
	case TokenSlash:
		return "/"
	case TokenPercent:
		return "%"
	case TokenBang:
		return "!"
	case TokenAmp:
		return "&"
	case TokenEqEq:
		return "=="
	case TokenNotEq:
		return "!="
	case TokenLt:
		return "<"
	case TokenLtEq:
		return "<="
	case TokenGt:
		return ">"
	case TokenGtEq:
		return ">="
	case TokenAndAnd:
		return "&&"
	case TokenOrOr:
		return "||"
	default:
		return "?"
	}
}
