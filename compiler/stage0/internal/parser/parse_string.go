package parser

import (
	"dastlang/internal/ast"
	"dastlang/internal/diag"
	"dastlang/internal/lexer"
)

func ParseExprString(filename string, input string, structs map[string]struct{}) (ast.Expr, *diag.Bag) {
	toks := lexTokens(filename, input)
	structNames := mergeStructNames(structs, toks)
	p := &Parser{tokens: toks, pos: 0, diag: &diag.Bag{}, structNames: structNames}
	expr := p.parseExpr(0)
	if !p.at(lexer.TokenEOF) {
		p.diag.Add(p.peek().Span, "unexpected token after expression")
	}
	return expr, p.diag
}

func ParseStmtString(filename string, input string, structs map[string]struct{}) (ast.Stmt, *diag.Bag) {
	toks := lexTokens(filename, input)
	structNames := mergeStructNames(structs, toks)
	p := &Parser{tokens: toks, pos: 0, diag: &diag.Bag{}, structNames: structNames}
	stmt := p.parseStmt()
	if !p.at(lexer.TokenEOF) {
		p.diag.Add(p.peek().Span, "unexpected token after statement")
	}
	return stmt, p.diag
}

func ParseBlockString(filename string, input string, structs map[string]struct{}) (*ast.Block, *diag.Bag) {
	toks := lexTokens(filename, input)
	structNames := mergeStructNames(structs, toks)
	p := &Parser{tokens: toks, pos: 0, diag: &diag.Bag{}, structNames: structNames}
	block := p.parseBlock()
	if !p.at(lexer.TokenEOF) {
		p.diag.Add(p.peek().Span, "unexpected token after block")
	}
	return block, p.diag
}

func ParseItemString(filename string, input string, structs map[string]struct{}) (ast.Item, *diag.Bag) {
	toks := lexTokens(filename, input)
	structNames := mergeStructNames(structs, toks)
	p := &Parser{tokens: toks, pos: 0, diag: &diag.Bag{}, structNames: structNames}
	item := p.parseItem()
	p.maybeConsumeSemicolon()
	if item == nil {
		p.diag.Add(p.peek().Span, "expected item")
	}
	if !p.at(lexer.TokenEOF) {
		p.diag.Add(p.peek().Span, "unexpected token after item")
	}
	return item, p.diag
}

func mergeStructNames(extra map[string]struct{}, toks []lexer.Token) map[string]struct{} {
	structNames := map[string]struct{}{}
	for name := range extra {
		structNames[name] = struct{}{}
	}
	for name := range collectStructNames(toks) {
		structNames[name] = struct{}{}
	}
	return structNames
}
