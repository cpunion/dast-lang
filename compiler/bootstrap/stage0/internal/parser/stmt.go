package parser

import (
	"strings"

	"dastlang/internal/ast"
	"dastlang/internal/lexer"
)

func (p *Parser) parseBlock() *ast.Block {
	lbrace := p.expect(lexer.TokenLBrace, "expected '{'")
	block := &ast.Block{SpanInfo: lbrace.Span}
	for !p.at(lexer.TokenRBrace) && !p.at(lexer.TokenEOF) {
		stmt := p.parseStmt()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
		} else {
			p.advance()
		}
	}
	rbrace := p.expect(lexer.TokenRBrace, "expected '}'")
	block.SpanInfo = mergeSpan(lbrace.Span, rbrace.Span)
	return block
}

func (p *Parser) parseStmt() ast.Stmt {
	switch p.peek().Kind {
	case lexer.TokenLet:
		return p.parseLet()
	case lexer.TokenReturn:
		return p.parseReturn()
	case lexer.TokenIf:
		return p.parseIf()
	case lexer.TokenWhile:
		return p.parseWhile()
	case lexer.TokenLoop:
		return p.parseLoop()
	case lexer.TokenBreak:
		return p.parseBreak()
	case lexer.TokenContinue:
		return p.parseContinue()
	case lexer.TokenMatch:
		return p.parseMatch()
	case lexer.TokenLBrace:
		return p.parseBlock()
	default:
		expr := p.parseExpr(0)
		if p.match(lexer.TokenAssign) {
			value := p.parseExpr(0)
			p.maybeConsumeSemicolon()
			if !isAssignable(expr) {
				p.diag.Add(expr.Span(), "invalid assignment target")
				return &ast.ExprStmt{Expr: expr, SpanInfo: expr.Span()}
			}
			return &ast.AssignStmt{Target: expr, Value: value, SpanInfo: mergeSpan(expr.Span(), value.Span())}
		}
		p.maybeConsumeSemicolon()
		return &ast.ExprStmt{Expr: expr, SpanInfo: expr.Span()}
	}
}

func (p *Parser) parseLet() ast.Stmt {
	start := p.expect(lexer.TokenLet, "expected 'let'")
	mutable := false
	if p.match(lexer.TokenMut) {
		mutable = true
	}
	if p.at(lexer.TokenDot) || (p.at(lexer.TokenIdent) && p.peek().Lexeme == "_") {
		pat := p.parsePattern()
		if mutable {
			p.diag.Add(start.Span, "let pattern does not support 'mut' in stage 0")
		}
		if p.match(lexer.TokenColon) {
			p.diag.Add(p.prev().Span, "let pattern type annotation not supported in stage 0")
			_ = p.parseType()
		}
		p.expect(lexer.TokenAssign, "expected '=' in let pattern")
		init := p.parseExpr(0)
		p.maybeConsumeSemicolon()
		return &ast.LetPatternStmt{Pattern: pat, Init: init, SpanInfo: mergeSpan(start.Span, init.Span())}
	}
	if p.at(lexer.TokenIdent) {
		if end, name, _, ok := p.peekQualifiedName(); ok {
			if end < len(p.tokens) && p.tokens[end].Kind == lexer.TokenLBrace {
				pat := p.parsePattern()
				if mutable {
					p.diag.Add(start.Span, "let pattern does not support 'mut' in stage 0")
				}
				if p.match(lexer.TokenColon) {
					p.diag.Add(p.prev().Span, "let pattern type annotation not supported in stage 0")
					_ = p.parseType()
				}
				p.expect(lexer.TokenAssign, "expected '=' in let pattern")
				init := p.parseExpr(0)
				p.maybeConsumeSemicolon()
				return &ast.LetPatternStmt{Pattern: pat, Init: init, SpanInfo: mergeSpan(start.Span, init.Span())}
			}
			if strings.Contains(name, ".") {
				pat := p.parsePattern()
				if mutable {
					p.diag.Add(start.Span, "let pattern does not support 'mut' in stage 0")
				}
				if p.match(lexer.TokenColon) {
					p.diag.Add(p.prev().Span, "let pattern type annotation not supported in stage 0")
					_ = p.parseType()
				}
				p.expect(lexer.TokenAssign, "expected '=' in let pattern")
				init := p.parseExpr(0)
				p.maybeConsumeSemicolon()
				return &ast.LetPatternStmt{Pattern: pat, Init: init, SpanInfo: mergeSpan(start.Span, init.Span())}
			}
		}
	}
	nameTok := p.expect(lexer.TokenIdent, "expected variable name")
	var typ *ast.Type
	if p.match(lexer.TokenColon) {
		t := p.parseType()
		typ = &t
	}
	var init ast.Expr
	if p.match(lexer.TokenAssign) {
		init = p.parseExpr(0)
	}
	p.maybeConsumeSemicolon()
	end := nameTok.Span
	if init != nil {
		end = init.Span()
	}
	return &ast.LetStmt{Name: nameTok.Lexeme, Mutable: mutable, Type: typ, Init: init, SpanInfo: mergeSpan(start.Span, end)}
}

func (p *Parser) parseReturn() ast.Stmt {
	start := p.expect(lexer.TokenReturn, "expected 'return'")
	var val ast.Expr
	if !p.at(lexer.TokenSemicolon) && !p.at(lexer.TokenRBrace) {
		val = p.parseExpr(0)
	}
	p.maybeConsumeSemicolon()
	return &ast.ReturnStmt{Value: val, SpanInfo: mergeSpan(start.Span, spanOfExpr(val))}
}

func (p *Parser) parseIf() ast.Stmt {
	start := p.expect(lexer.TokenIf, "expected 'if'")
	if p.match(lexer.TokenLet) {
		pat := p.parsePattern()
		p.expect(lexer.TokenAssign, "expected '=' in if let")
		expr := p.parseExpr(0)
		thenBlock := p.parseBlock()
		var elseBlock *ast.Block
		if p.match(lexer.TokenElse) {
			if p.at(lexer.TokenIf) {
				nested := p.parseIf()
				elseBlock = &ast.Block{Stmts: []ast.Stmt{nested}, SpanInfo: nested.Span()}
			} else {
				elseBlock = p.parseBlock()
			}
		}
		end := thenBlock.Span()
		if elseBlock != nil {
			end = elseBlock.Span()
		}
		return &ast.IfLetStmt{Pattern: pat, Expr: expr, Then: thenBlock, Else: elseBlock, SpanInfo: mergeSpan(start.Span, end)}
	}
	cond := p.parseExpr(0)
	thenBlock := p.parseBlock()
	var elseBlock *ast.Block
	if p.match(lexer.TokenElse) {
		if p.at(lexer.TokenIf) {
			nested := p.parseIf()
			elseBlock = &ast.Block{Stmts: []ast.Stmt{nested}, SpanInfo: nested.Span()}
		} else {
			elseBlock = p.parseBlock()
		}
	}
	return &ast.IfStmt{Cond: cond, Then: thenBlock, Else: elseBlock, SpanInfo: mergeSpan(start.Span, thenBlock.Span())}
}

func (p *Parser) parseWhile() ast.Stmt {
	start := p.expect(lexer.TokenWhile, "expected 'while'")
	if p.match(lexer.TokenLet) {
		pat := p.parsePattern()
		p.expect(lexer.TokenAssign, "expected '=' in while let")
		expr := p.parseExpr(0)
		body := p.parseBlock()
		return &ast.WhileLetStmt{Pattern: pat, Expr: expr, Body: body, SpanInfo: mergeSpan(start.Span, body.Span())}
	}
	cond := p.parseExpr(0)
	body := p.parseBlock()
	return &ast.WhileStmt{Cond: cond, Body: body, SpanInfo: mergeSpan(start.Span, body.Span())}
}

func (p *Parser) parseLoop() ast.Stmt {
	start := p.expect(lexer.TokenLoop, "expected 'loop'")
	body := p.parseBlock()
	return &ast.LoopStmt{Body: body, SpanInfo: mergeSpan(start.Span, body.Span())}
}

func (p *Parser) parseBreak() ast.Stmt {
	start := p.expect(lexer.TokenBreak, "expected 'break'")
	if !p.at(lexer.TokenSemicolon) && !p.at(lexer.TokenRBrace) {
		p.diag.Add(start.Span, "break value not supported in stage 0")
		_ = p.parseExpr(0)
	}
	p.maybeConsumeSemicolon()
	return &ast.BreakStmt{SpanInfo: start.Span}
}

func (p *Parser) parseContinue() ast.Stmt {
	start := p.expect(lexer.TokenContinue, "expected 'continue'")
	p.maybeConsumeSemicolon()
	return &ast.ContinueStmt{SpanInfo: start.Span}
}

func (p *Parser) parseMatch() ast.Stmt {
	start := p.expect(lexer.TokenMatch, "expected 'match'")
	expr := p.parseExpr(0)
	p.expect(lexer.TokenLBrace, "expected '{' after match")
	stmt := &ast.MatchStmt{Expr: expr, SpanInfo: start.Span}
	for !p.at(lexer.TokenRBrace) && !p.at(lexer.TokenEOF) {
		pat := p.parsePattern()
		var guard ast.Expr
		if p.match(lexer.TokenIf) {
			guard = p.parseExpr(0)
		}
		p.expect(lexer.TokenFatArrow, "expected '=>' in match arm")
		var body *ast.Block
		if p.at(lexer.TokenLBrace) {
			body = p.parseBlock()
		} else {
			expr := p.parseExpr(0)
			p.maybeConsumeSemicolon()
			body = &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: expr, SpanInfo: expr.Span()}}, SpanInfo: expr.Span()}
		}
		arm := ast.MatchArm{Pattern: pat, Guard: guard, Body: body, SpanInfo: mergeSpan(pat.Span(), body.Span())}
		stmt.Arms = append(stmt.Arms, arm)
		if p.match(lexer.TokenComma) {
			continue
		}
	}
	rbrace := p.expect(lexer.TokenRBrace, "expected '}'")
	stmt.SpanInfo = mergeSpan(stmt.SpanInfo, rbrace.Span)
	return stmt
}
