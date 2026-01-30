package parser

import (
	"dastlang/internal/ast"
	"dastlang/internal/lexer"
	"dastlang/internal/source"
)

func (p *Parser) parseExpr(minPrec int) ast.Expr {
	expr := p.parseUnary()
	for {
		opTok := p.peek()
		prec, ok := infixPrec(opTok.Kind)
		if !ok || prec < minPrec {
			break
		}
		// Minimal semicolon insertion: if an infix operator starts on a new
		// line after a token that can end an expression, treat it as a break.
		if p.newlineBefore(opTok) && canEndExpr(p.prev().Kind) {
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
			if p.at(lexer.TokenInt) {
				nameTok := p.advance()
				expr = &ast.AccessExpr{Receiver: expr, Field: nameTok.Lexeme, SpanInfo: mergeSpan(expr.Span(), nameTok.Span)}
				continue
			}
			nameTok := p.expect(lexer.TokenIdent, "expected field or variant name")
			if ident, ok := expr.(*ast.IdentExpr); ok && len(ident.Name) > 0 {
				c := ident.Name[0]
				if c >= 'A' && c <= 'Z' {
					var arg ast.Expr
					endSpan := nameTok.Span
					if p.match(lexer.TokenLParen) {
						if !p.at(lexer.TokenRParen) {
							arg = p.parseExpr(0)
						}
						endTok := p.expect(lexer.TokenRParen, "expected ')' after variant payload")
						endSpan = endTok.Span
					}
					expr = &ast.EnumVariantExpr{EnumName: ident.Name, Variant: nameTok.Lexeme, Arg: arg, SpanInfo: mergeSpan(expr.Span(), endSpan)}
					continue
				}
			}
			expr = &ast.AccessExpr{Receiver: expr, Field: nameTok.Lexeme, SpanInfo: mergeSpan(expr.Span(), nameTok.Span)}
			continue
		}
		if p.match(lexer.TokenBang) {
			var args []ast.Expr
			var endSpan source.Span
			switch {
			case p.match(lexer.TokenLParen):
				if !p.at(lexer.TokenRParen) {
					for {
						args = append(args, p.parseExpr(0))
						if p.match(lexer.TokenComma) {
							continue
						}
						break
					}
				}
				endTok := p.expect(lexer.TokenRParen, "expected ')' after macro args")
				endSpan = endTok.Span
			case p.match(lexer.TokenLBracket):
				if !p.at(lexer.TokenRBracket) {
					for {
						args = append(args, p.parseExpr(0))
						if p.match(lexer.TokenComma) {
							continue
						}
						break
					}
				}
				endTok := p.expect(lexer.TokenRBracket, "expected ']' after macro args")
				endSpan = endTok.Span
			case p.match(lexer.TokenLBrace):
				block := p.parseBlock()
				args = append(args, &ast.BlockExpr{Block: block, SpanInfo: block.Span()})
				endSpan = block.Span()
			default:
				p.errorCurrent("expected '(', '[', or '{' after '!'")
				return expr
			}
			if ident, ok := expr.(*ast.IdentExpr); ok && ident.Name == "compile" {
				if len(args) != 1 {
					p.diag.Add(ident.Span(), "compile! expects exactly one argument")
					expr = &ast.CompileExpr{Expr: &ast.IntLit{Value: 0, SpanInfo: ident.Span()}, SpanInfo: ident.Span()}
					continue
				}
				expr = &ast.CompileExpr{Expr: args[0], SpanInfo: mergeSpan(expr.Span(), endSpan)}
				continue
			}
			expr = &ast.MacroCallExpr{Callee: expr, Args: args, SpanInfo: mergeSpan(expr.Span(), endSpan)}
			continue
		}
		if p.match(lexer.TokenLBracket) {
			index := p.parseExpr(0)
			endTok := p.expect(lexer.TokenRBracket, "expected ']' after index")
			expr = &ast.IndexExpr{Receiver: expr, Index: index, SpanInfo: mergeSpan(expr.Span(), endTok.Span)}
			continue
		}
		if ident, ok := expr.(*ast.IdentExpr); ok && p.match(lexer.TokenColonColon) {
			if !p.at(lexer.TokenLBracket) {
				p.errorCurrent("expected '[' after '::'")
				return expr
			}
			typeArgs := p.parseTypeArgs()
			if !p.match(lexer.TokenLParen) {
				p.errorCurrent("expected '(' after type arguments")
				return expr
			}
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
			endTok := p.expect(lexer.TokenRParen, "expected ')' after arguments")
			expr = &ast.CallExpr{Callee: ident.Name, TypeArgs: typeArgs, Args: args, SpanInfo: mergeSpan(expr.Span(), endTok.Span)}
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
			endTok := p.expect(lexer.TokenRParen, "expected ')' after arguments")
			if ident, ok := expr.(*ast.IdentExpr); ok {
				expr = &ast.CallExpr{Callee: ident.Name, Args: args, SpanInfo: mergeSpan(expr.Span(), endTok.Span)}
				continue
			}
			if access, ok := expr.(*ast.AccessExpr); ok {
				expr = &ast.MethodCallExpr{
					Receiver: access.Receiver,
					Method:   access.Field,
					Args:     args,
					SpanInfo: mergeSpan(expr.Span(), endTok.Span),
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
	case lexer.TokenQuote:
		return p.parseQuoteExpr()
	case lexer.TokenIdent:
		if p.structLitStart() {
			return p.parseStructLit()
		}
		p.advance()
		return &ast.IdentExpr{Name: tok.Lexeme, SpanInfo: tok.Span}
	case lexer.TokenSelf:
		p.advance()
		return &ast.IdentExpr{Name: tok.Lexeme, SpanInfo: tok.Span}
	case lexer.TokenInt:
		p.advance()
		return &ast.IntLit{Value: parseInt(tok.Lexeme), SpanInfo: tok.Span}
	case lexer.TokenFloat:
		p.advance()
		return &ast.FloatLit{Text: tok.Lexeme, SpanInfo: tok.Span}
	case lexer.TokenString:
		p.advance()
		return &ast.StringLit{Value: tok.Lexeme, SpanInfo: tok.Span}
	case lexer.TokenChar:
		p.advance()
		return &ast.CharLit{Value: parseCharLiteral(tok.Lexeme), SpanInfo: tok.Span}
	case lexer.TokenTrue:
		p.advance()
		return &ast.BoolLit{Value: true, SpanInfo: tok.Span}
	case lexer.TokenFalse:
		p.advance()
		return &ast.BoolLit{Value: false, SpanInfo: tok.Span}
	case lexer.TokenLParen:
		start := p.advance()
		if p.at(lexer.TokenRParen) {
			endTok := p.advance()
			return &ast.TupleLit{Elems: nil, SpanInfo: mergeSpan(start.Span, endTok.Span)}
		}
		first := p.parseExpr(0)
		if p.match(lexer.TokenComma) {
			elems := []ast.Expr{first}
			for !p.at(lexer.TokenRParen) && !p.at(lexer.TokenEOF) {
				elems = append(elems, p.parseExpr(0))
				if p.match(lexer.TokenComma) {
					continue
				}
				break
			}
			endTok := p.expect(lexer.TokenRParen, "expected ')' after tuple")
			return &ast.TupleLit{Elems: elems, SpanInfo: mergeSpan(start.Span, endTok.Span)}
		}
		p.expect(lexer.TokenRParen, "expected ')' after expression")
		return first
	case lexer.TokenLBracket:
		return p.parseArrayLit()
	case lexer.TokenLBrace:
		block := p.parseBlock()
		return &ast.BlockExpr{Block: block, SpanInfo: block.Span()}
	case lexer.TokenLoop:
		startTok := p.advance()
		body := p.parseBlock()
		return &ast.LoopExpr{Body: body, SpanInfo: mergeSpan(startTok.Span, body.Span())}
	case lexer.TokenIf:
		return p.parseIfExpr()
	case lexer.TokenMatch:
		return p.parseMatchExpr()
	case lexer.TokenPipe:
		return p.parseClosure()
	case lexer.TokenOrOr:
		startTok := p.advance()
		var body ast.Expr
		if p.at(lexer.TokenLBrace) {
			block := p.parseBlock()
			body = &ast.BlockExpr{Block: block, SpanInfo: block.Span()}
		} else {
			body = p.parseExpr(0)
		}
		return &ast.ClosureExpr{Params: nil, ReturnType: nil, Body: body, SpanInfo: mergeSpan(startTok.Span, body.Span())}
	case lexer.TokenDot:
		dotTok := p.advance()
		nameTok := p.expect(lexer.TokenIdent, "expected variant name")
		var arg ast.Expr
		endSpan := nameTok.Span
		if p.match(lexer.TokenLParen) {
			if !p.at(lexer.TokenRParen) {
				arg = p.parseExpr(0)
			}
			endTok := p.expect(lexer.TokenRParen, "expected ')' after variant payload")
			endSpan = endTok.Span
		}
		return &ast.EnumVariantExpr{EnumName: "", Variant: nameTok.Lexeme, Arg: arg, SpanInfo: mergeSpan(dotTok.Span, endSpan)}
	default:
		p.errorCurrent("unexpected token in expression")
		p.advance()
		return &ast.IntLit{Value: 0, SpanInfo: tok.Span}
	}
}

func (p *Parser) parseClosure() ast.Expr {
	startTok := p.expect(lexer.TokenPipe, "expected '|' to start closure")
	var params []ast.ClosureParam
	if !p.at(lexer.TokenPipe) {
		for {
			nameTok := p.expect(lexer.TokenIdent, "expected closure parameter name")
			param := ast.ClosureParam{Name: nameTok.Lexeme, Span: nameTok.Span}
			if p.match(lexer.TokenColon) {
				t := p.parseType()
				param.Type = &t
			}
			params = append(params, param)
			if p.match(lexer.TokenComma) {
				continue
			}
			break
		}
	}
	p.expect(lexer.TokenPipe, "expected '|' to close closure params")
	var retType *ast.Type
	if p.match(lexer.TokenArrow) {
		t := p.parseType()
		retType = &t
	}
	var body ast.Expr
	if p.at(lexer.TokenLBrace) {
		block := p.parseBlock()
		body = &ast.BlockExpr{Block: block, SpanInfo: block.Span()}
	} else {
		body = p.parseExpr(0)
	}
	return &ast.ClosureExpr{Params: params, ReturnType: retType, Body: body, SpanInfo: mergeSpan(startTok.Span, body.Span())}
}

func (p *Parser) parseQuoteExpr() ast.Expr {
	startTok := p.expect(lexer.TokenQuote, "expected 'quote'")
	kind := ast.QuoteExprKind
	if p.match(lexer.TokenIdent) {
		kindTok := p.prev()
		switch kindTok.Lexeme {
		case "expr":
			kind = ast.QuoteExprKind
		case "stmt":
			kind = ast.QuoteStmtKind
		case "item":
			kind = ast.QuoteItemKind
		case "block":
			kind = ast.QuoteBlockKind
		default:
			p.diag.Add(kindTok.Span, "unknown quote kind '"+kindTok.Lexeme+"'")
		}
	}
	p.expect(lexer.TokenLBrace, "expected '{' after quote")
	parts, endSpan := p.parseQuoteParts()
	return &ast.QuoteExpr{Kind: kind, Parts: parts, SpanInfo: mergeSpan(startTok.Span, endSpan)}
}

func (p *Parser) parseQuoteParts() ([]ast.QuotePart, source.Span) {
	var parts []ast.QuotePart
	var buf []lexer.Token
	depth := 1
	for !p.at(lexer.TokenEOF) {
		tok := p.peek()
		if tok.Kind == lexer.TokenDollar {
			if len(buf) > 0 {
				parts = append(parts, ast.QuotePart{Text: tokensToSource(buf)})
				buf = nil
			}
			p.advance()
			parts = append(parts, ast.QuotePart{Expr: p.parseQuoteSplice()})
			continue
		}
		if tok.Kind == lexer.TokenLBrace {
			depth++
		} else if tok.Kind == lexer.TokenRBrace {
			depth--
			if depth == 0 {
				p.advance()
				if len(buf) > 0 {
					parts = append(parts, ast.QuotePart{Text: tokensToSource(buf)})
				}
				return parts, tok.Span
			}
		}
		buf = append(buf, tok)
		p.advance()
	}
	p.diag.Add(p.prev().Span, "unterminated quote")
	return parts, p.prev().Span
}

func (p *Parser) parseQuoteSplice() ast.Expr {
	if p.match(lexer.TokenLParen) {
		var toks []lexer.Token
		depth := 1
		for !p.at(lexer.TokenEOF) {
			tok := p.peek()
			if tok.Kind == lexer.TokenLParen {
				depth++
			} else if tok.Kind == lexer.TokenRParen {
				depth--
				if depth == 0 {
					p.advance()
					break
				}
			}
			toks = append(toks, tok)
			p.advance()
		}
		if depth != 0 {
			p.diag.Add(p.prev().Span, "unterminated splice expression")
			return &ast.IntLit{Value: 0, SpanInfo: p.prev().Span}
		}
		return p.parseExprFromTokens(toks)
	}
	if p.peek().Kind == lexer.TokenIdent || p.peek().Kind == lexer.TokenSelf {
		tok := p.advance()
		return &ast.IdentExpr{Name: tok.Lexeme, SpanInfo: tok.Span}
	}
	p.errorCurrent("expected identifier or '(...)' after '$'")
	return &ast.IntLit{Value: 0, SpanInfo: p.prev().Span}
}

func (p *Parser) parseExprFromTokens(toks []lexer.Token) ast.Expr {
	if len(toks) == 0 {
		p.diag.Add(p.prev().Span, "empty splice expression")
		return &ast.IntLit{Value: 0, SpanInfo: p.prev().Span}
	}
	last := toks[len(toks)-1]
	eof := lexer.Token{Kind: lexer.TokenEOF, Span: last.Span}
	sub := &Parser{tokens: append(toks, eof), pos: 0, diag: p.diag, structNames: p.structNames}
	expr := sub.parseExpr(0)
	if !sub.at(lexer.TokenEOF) {
		sub.diag.Add(sub.peek().Span, "unexpected token after splice expression")
	}
	return expr
}

func tokensToSource(toks []lexer.Token) string {
	if len(toks) == 0 {
		return ""
	}
	var out []rune
	for _, tok := range toks {
		if tok.Kind == lexer.TokenEOF {
			continue
		}
		text := tokenText(tok)
		for _, r := range text {
			out = append(out, r)
		}
		out = append(out, ' ')
	}
	return string(out)
}

func tokenText(tok lexer.Token) string {
	switch tok.Kind {
	case lexer.TokenString:
		return `"` + escapeString(tok.Lexeme) + `"`
	case lexer.TokenChar:
		return `'` + escapeChar(tok.Lexeme) + `'`
	default:
		return tok.Lexeme
	}
}

func escapeString(s string) string {
	var out []rune
	for _, r := range s {
		switch r {
		case '\\':
			out = append(out, '\\', '\\')
		case '"':
			out = append(out, '\\', '"')
		case '\n':
			out = append(out, '\\', 'n')
		case '\t':
			out = append(out, '\\', 't')
		case '\r':
			out = append(out, '\\', 'r')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

func escapeChar(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return ""
	}
	switch runes[0] {
	case '\\':
		return "\\\\"
	case '\'':
		return "\\'"
	case '\n':
		return "\\n"
	case '\t':
		return "\\t"
	case '\r':
		return "\\r"
	default:
		return string(runes[0])
	}
}

func (p *Parser) parseStructLit() ast.Expr {
	name, span := p.parseQualifiedName()
	var typeArgs []ast.Type
	if p.at(lexer.TokenLBracket) {
		typeArgs = p.parseTypeArgs()
	}
	start := span
	p.expect(lexer.TokenLBrace, "expected '{' in struct literal")
	lit := &ast.StructLit{Name: name, TypeArgs: typeArgs, SpanInfo: start}
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

func (p *Parser) parseIfExpr() ast.Expr {
	start := p.expect(lexer.TokenIf, "expected 'if'")
	cond := p.parseExpr(0)
	thenBlock := p.parseBlock()
	thenExpr := &ast.BlockExpr{Block: thenBlock, SpanInfo: thenBlock.Span()}
	var elseExpr ast.Expr
	if p.match(lexer.TokenElse) {
		if p.at(lexer.TokenIf) {
			elseExpr = p.parseIfExpr()
		} else if p.at(lexer.TokenLBrace) {
			elseBlock := p.parseBlock()
			elseExpr = &ast.BlockExpr{Block: elseBlock, SpanInfo: elseBlock.Span()}
		} else {
			p.errorCurrent("expected 'if' or '{' after else")
		}
	}
	end := thenExpr.Span()
	if elseExpr != nil {
		end = elseExpr.Span()
	}
	return &ast.IfExpr{Cond: cond, Then: thenExpr, Else: elseExpr, SpanInfo: mergeSpan(start.Span, end)}
}

func (p *Parser) parseMatchExpr() ast.Expr {
	start := p.expect(lexer.TokenMatch, "expected 'match'")
	expr := p.parseExpr(0)
	p.expect(lexer.TokenLBrace, "expected '{' after match")
	out := &ast.MatchExpr{Expr: expr, SpanInfo: start.Span}
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
		out.Arms = append(out.Arms, arm)
		if p.match(lexer.TokenComma) {
			continue
		}
	}
	rbrace := p.expect(lexer.TokenRBrace, "expected '}'")
	out.SpanInfo = mergeSpan(out.SpanInfo, rbrace.Span)
	return out
}
