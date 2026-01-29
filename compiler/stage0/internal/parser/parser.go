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
	vis := ast.VisPrivate
	if p.match(lexer.TokenPub) {
		vis = ast.VisPublic
	}
	if p.match(lexer.TokenMacro) {
		if p.match(lexer.TokenFn) {
			// allow "macro fn" for readability
		}
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseFunction(vis, true)
	}
	if p.match(lexer.TokenFn) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseFunction(vis, false)
	}
	if p.match(lexer.TokenImport) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseImport()
	}
	if p.match(lexer.TokenStruct) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseStructDecl(vis)
	}
	if p.match(lexer.TokenTrait) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseTraitDecl(vis)
	}
	if p.match(lexer.TokenEnum) {
		return p.parseEnumDecl(repr, vis)
	}
	if p.match(lexer.TokenConst) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseConstDecl(vis)
	}
	if p.match(lexer.TokenType) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseTypeAlias(vis)
	}
	if p.match(lexer.TokenImpl) {
		if repr != "" {
			p.diag.Add(p.peek().Span, "@repr only valid on enum")
		}
		return p.parseImplDecl(vis)
	}
	if repr != "" {
		p.diag.Add(p.peek().Span, "@repr only valid on enum")
	}
	if p.peek().Kind == lexer.TokenIdent {
		expr := p.parseExpr(0)
		p.maybeConsumeSemicolon()
		switch expr.(type) {
		case *ast.CompileExpr, *ast.MacroCallExpr:
			return &ast.CompileItem{Expr: expr, SpanInfo: expr.Span()}
		default:
			p.diag.Add(expr.Span(), "expected item")
			return nil
		}
	}
	p.errorCurrent("expected item")
	return nil
}
