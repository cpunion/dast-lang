package parser

import (
	"dastlang/internal/ast"
	"dastlang/internal/diag"
	"dastlang/internal/lexer"
)

type Parser struct {
	tokens      []lexer.Token
	pos         int
	diag        *diag.Bag
	structNames map[string]struct{}
}

func Parse(filename string, input string) (*ast.Program, *diag.Bag) {
	return ParseWithStructs(filename, input, nil)
}

type Source struct {
	Filename string
	Input    string
}

func ParseFiles(sources []Source) (*ast.Program, *diag.Bag) {
	structs := map[string]struct{}{}
	for _, src := range sources {
		toks := lexTokens(src.Filename, src.Input)
		for name := range collectStructNames(toks) {
			structs[name] = struct{}{}
		}
	}
	merged := &ast.Program{}
	diagBag := &diag.Bag{}
	for _, src := range sources {
		prog, diags := ParseWithStructs(src.Filename, src.Input, structs)
		merged.Items = append(merged.Items, prog.Items...)
		if diags != nil && len(diags.Items) > 0 {
			diagBag.Items = append(diagBag.Items, diags.Items...)
		}
	}
	return merged, diagBag
}

func ParseWithStructs(filename string, input string, structs map[string]struct{}) (*ast.Program, *diag.Bag) {
	toks := lexTokens(filename, input)
	structNames := map[string]struct{}{}
	for name := range structs {
		structNames[name] = struct{}{}
	}
	for name := range collectStructNames(toks) {
		structNames[name] = struct{}{}
	}
	p := &Parser{tokens: toks, pos: 0, diag: &diag.Bag{}, structNames: structNames}
	return p.parseProgram(), p.diag
}

func (p *Parser) parseProgram() *ast.Program {
	prog := &ast.Program{}
	for !p.at(lexer.TokenEOF) {
		item := p.parseItem()
		if item != nil {
			prog.Items = append(prog.Items, item)
		} else {
			p.advance()
		}
	}
	return prog
}

func (p *Parser) parseItem() ast.Item {
	repr := ""
	for p.match(lexer.TokenAt) {
		attr := p.parseReprAttr()
		if attr != "" {
			if repr != "" {
				p.diag.Add(p.peek().Span, "duplicate repr attribute")
			} else {
				repr = attr
			}
		}
	}
	if p.match(lexer.TokenFn) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseFunction()
	}
	if p.match(lexer.TokenImport) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseImport()
	}
	if p.match(lexer.TokenMod) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseMod()
	}
	if p.match(lexer.TokenStruct) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseStructDecl()
	}
	if p.match(lexer.TokenEnum) {
		return p.parseEnumDecl(repr)
	}
	if p.match(lexer.TokenConst) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseConstDecl()
	}
	if p.match(lexer.TokenImpl) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseImplDecl()
	}
	if repr != "" {
		p.diag.Add(p.peek().Span, "@repr only valid on enum")
	}
	p.errorCurrent("expected item")
	return nil
}
