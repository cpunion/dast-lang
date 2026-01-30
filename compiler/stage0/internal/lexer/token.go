package lexer

import "dastlang/internal/source"

type TokenKind int

const (
	TokenEOF TokenKind = iota
	TokenIdent
	TokenInt
	TokenFloat
	TokenString
	TokenChar

	TokenFn
	TokenStruct
	TokenEnum
	TokenImpl
	TokenTrait
	TokenSelf
	TokenSelfType
	TokenConst
	TokenType
	TokenPub
	TokenMacro
	TokenQuote
	TokenImport
	TokenAs
	TokenIn
	TokenWhere
	TokenLet
	TokenMut
	TokenIf
	TokenElse
	TokenWhile
	TokenLoop
	TokenFor
	TokenBreak
	TokenContinue
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
	TokenColonColon
	TokenDotDot
	TokenDotDotEq
	TokenAt
	TokenDollar

	TokenAssign
	TokenPlus
	TokenMinus
	TokenStar
	TokenSlash
	TokenPercent
	TokenBang
	TokenAmp
	TokenPipe
	TokenCaret
	TokenShl
	TokenShr

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
	case TokenFloat:
		return "FLOAT"
	case TokenString:
		return "STRING"
	case TokenChar:
		return "CHAR"
	case TokenFn:
		return "fn"
	case TokenStruct:
		return "struct"
	case TokenEnum:
		return "enum"
	case TokenImpl:
		return "impl"
	case TokenTrait:
		return "trait"
	case TokenSelf:
		return "self"
	case TokenSelfType:
		return "Self"
	case TokenConst:
		return "const"
	case TokenType:
		return "type"
	case TokenPub:
		return "pub"
	case TokenMacro:
		return "macro"
	case TokenQuote:
		return "quote"
	case TokenImport:
		return "import"
	case TokenAs:
		return "as"
	case TokenIn:
		return "in"
	case TokenWhere:
		return "where"
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
	case TokenLoop:
		return "loop"
	case TokenFor:
		return "for"
	case TokenBreak:
		return "break"
	case TokenContinue:
		return "continue"
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
	case TokenColonColon:
		return "::"
	case TokenDotDot:
		return ".."
	case TokenDotDotEq:
		return "..="
	case TokenAt:
		return "@"
	case TokenDollar:
		return "$"
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
	case TokenPipe:
		return "|"
	case TokenCaret:
		return "^"
	case TokenShl:
		return "<<"
	case TokenShr:
		return ">>"
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
