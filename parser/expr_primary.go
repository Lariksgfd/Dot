package parser

import (
	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// parseUnary parses a prefix expression: the unary operators, `await`, or a
// primary expression.
func (p *parser) parseUnary() ast.Expr {
	tok := p.cur()
	switch tok.Type {
	case lexer.TokenMinus, lexer.TokenPlus, lexer.TokenNot, lexer.TokenTilde:
		p.advance()
		// Unary operators bind tighter than `**` (SYNTAX.md levels 2 vs 3),
		// so the operand is parsed at postfix strength: -2 ** 2 is (-2) ** 2.
		operand := p.parseExpr(precPostfix)
		return &ast.UnaryExpr{
			BaseNode: ast.Span(tok.Pos, operand.End()),
			Op:       tok.Type,
			OpPos:    tok.Pos,
			X:        operand,
		}
	case lexer.TokenAmp:
		p.advance()
		operand := p.parseExpr(precPostfix)
		return &ast.UnaryExpr{
			BaseNode: ast.Span(tok.Pos, operand.End()),
			Op:       tok.Type,
			OpPos:    tok.Pos,
			X:        operand,
		}
	case lexer.TokenStar:
		p.advance()
		operand := p.parseExpr(precPostfix)
		return &ast.UnaryExpr{
			BaseNode: ast.Span(tok.Pos, operand.End()),
			Op:       tok.Type,
			OpPos:    tok.Pos,
			X:        operand,
		}
	case lexer.TokenAwait:
		return p.parseAwaitExpr()
	}
	return p.parsePrimary()
}

// parsePrimary parses a primary expression: a literal, identifier, grouping,
// collection literal, lambda, or one of the block-shaped expressions.
func (p *parser) parsePrimary() ast.Expr {
	tok := p.cur()
	switch tok.Type {
	case lexer.TokenInt:
		p.advance()
		return &ast.IntLit{
			BaseNode: ast.SpanTok(tok),
			Value:    parseUintLiteral(tok),
			Base:     tok.Base,
			Suffix:   tok.Suffix,
			Raw:      tok.Lexeme,
		}
	case lexer.TokenFloat:
		p.advance()
		return &ast.FloatLit{
			BaseNode: ast.SpanTok(tok),
			Value:    parseFloatLiteral(tok),
			Suffix:   tok.Suffix,
			Raw:      tok.Lexeme,
		}
	case lexer.TokenString, lexer.TokenRawString:
		p.advance()
		return p.parseStringLit(tok)
	case lexer.TokenTrue, lexer.TokenFalse:
		p.advance()
		return &ast.BoolLit{BaseNode: ast.SpanTok(tok), Value: tok.Type == lexer.TokenTrue}
	case lexer.TokenNil:
		p.advance()
		return &ast.NilLit{BaseNode: ast.SpanTok(tok)}
	case lexer.TokenUnderscore:
		p.advance()
		return &ast.UnderscoreExpr{BaseNode: ast.SpanTok(tok)}
	case lexer.TokenSelf:
		p.advance()
		return &ast.SelfExpr{BaseNode: ast.SpanTok(tok)}
	case lexer.TokenSelfType:
		p.advance()
		return &ast.Ident{BaseNode: ast.SpanTok(tok), Name: "Self"}
	case lexer.TokenIdent:
		return p.parseIdentExpr()
	case lexer.TokenLParen:
		return p.parseParenOrTuple()
	case lexer.TokenLBracket:
		return p.parseBracketLit()
	case lexer.TokenFn:
		return p.parseFnLit()
	case lexer.TokenIf:
		return p.parseIfExpr()
	case lexer.TokenMatch:
		return p.parseMatchExpr()
	case lexer.TokenSpawn:
		return p.parseSpawnExpr()
	}

	p.errorExpected("an expression")
	return p.badExpr(tok)
}

// parseIdentExpr parses an identifier, and the `map[K]V{...}` and
// `[]T{...}`-style typed literals that begin with one.
func (p *parser) parseIdentExpr() ast.Expr {
	tok := p.cur()
	if tok.Lexeme == "map" && p.peek(1).Type == lexer.TokenLBracket {
		return p.parseMapLit()
	}
	p.advance()
	return &ast.Ident{BaseNode: ast.SpanTok(tok), Name: tok.Lexeme}
}

// parseParenOrTuple parses `(x)`, `(a, b)` and `(x: 1, y: 2)`.
func (p *parser) parseParenOrTuple() ast.Expr {
	lp := p.advance()
	p.skipNewlines()

	if p.at(lexer.TokenRParen) {
		rp := p.advance()
		return &ast.TupleLit{
			BaseNode: ast.Span(lp.Pos, rp.End),
			LParen:   lp.Pos,
			RParen:   rp.Pos,
		}
	}

	var elems []ast.TupleElem
	named := false
	for !p.at(lexer.TokenRParen) && !p.atEnd() {
		before := p.pos
		var elem ast.TupleElem
		if p.at(lexer.TokenIdent) && p.peek(1).Type == lexer.TokenColon {
			name := p.advance()
			p.advance()
			elem.Name, elem.NamePos = name.Lexeme, name.Pos
			named = true
		}
		p.withNoStructLit(false, func() {
			elem.Value = p.parseExpr(precAssign)
		})
		elems = append(elems, elem)
		p.skipNewlines()
		if !p.accept(lexer.TokenComma) {
			break
		}
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	rp, ok := p.expect(lexer.TokenRParen, "parenthesised expression")
	if !ok {
		return p.badExpr(lp)
	}

	if len(elems) == 1 && !named {
		return &ast.ParenExpr{
			BaseNode: ast.Span(lp.Pos, rp.End),
			X:        elems[0].Value,
			LParen:   lp.Pos,
			RParen:   rp.Pos,
		}
	}
	return &ast.TupleLit{
		BaseNode: ast.Span(lp.Pos, rp.End),
		Elems:    elems,
		LParen:   lp.Pos,
		RParen:   rp.Pos,
	}
}

// parseBracketLit parses `[1, 2, 3]` and the typed forms `[]int{...}` and
// `[5]int{...}`.
func (p *parser) parseBracketLit() ast.Expr {
	lb := p.cur()

	// Typed forms start with `[]` or `[N]` immediately followed by a type.
	if p.peek(1).Type == lexer.TokenRBracket || looksLikeArrayTypePrefix(p) {
		typ := p.parseType()
		out := &ast.ArrayLit{BaseNode: ast.Span(lb.Pos, typ.End()), Type: typ}
		if p.at(lexer.TokenLBrace) {
			elems, rb := p.parseBracedExprList()
			out.Elems = elems
			out.SetSpan(lb.Pos, rb.End)
		}
		return out
	}

	p.advance()
	out := &ast.ArrayLit{BaseNode: ast.SpanTok(lb)}
	p.skipNewlines()
	for !p.at(lexer.TokenRBracket) && !p.atEnd() {
		before := p.pos
		var e ast.Expr
		p.withNoStructLit(false, func() { e = p.parseExpr(precAssign) })
		out.Elems = append(out.Elems, e)
		p.skipNewlines()
		if !p.accept(lexer.TokenComma) {
			break
		}
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	rb, ok := p.expect(lexer.TokenRBracket, "array literal")
	if !ok {
		return p.badExpr(lb)
	}
	out.SetSpan(lb.Pos, rb.End)
	return out
}

// looksLikeArrayTypePrefix reports whether the `[` at the cursor introduces a
// fixed-array type `[N]T` rather than an array literal `[a, b]`. It scans to
// the matching `]` and requires a type to follow.
func looksLikeArrayTypePrefix(p *parser) bool {
	depth := 0
	for i := 0; ; i++ {
		t := p.peek(i).Type
		switch t {
		case lexer.TokenLBracket:
			depth++
		case lexer.TokenRBracket:
			depth--
			if depth == 0 {
				return startsType(p.peek(i + 1).Type)
			}
		case lexer.TokenEOF, lexer.TokenNewline, lexer.TokenLBrace,
			lexer.TokenComma:
			return false
		}
		if i > 64 {
			return false
		}
	}
}

// parseBracedExprList parses `{ e1, e2 }` used by typed collection literals.
func (p *parser) parseBracedExprList() ([]ast.Expr, lexer.Token) {
	lb := p.advance()
	var elems []ast.Expr
	p.skipNewlines()
	for !p.at(lexer.TokenRBrace) && !p.atEnd() {
		before := p.pos
		var e ast.Expr
		p.withNoStructLit(false, func() { e = p.parseExpr(precAssign) })
		elems = append(elems, e)
		p.skipNewlines()
		if !p.accept(lexer.TokenComma) {
			break
		}
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	rb, ok := p.expect(lexer.TokenRBrace, "literal")
	if !ok {
		rb = lb
	}
	return elems, rb
}

// parseMapLit parses `map[string]int{"a": 1}`.
func (p *parser) parseMapLit() ast.Expr {
	start := p.cur()
	typ := p.parseType()
	out := &ast.MapLit{BaseNode: ast.Span(start.Pos, typ.End()), Type: typ}
	if !p.at(lexer.TokenLBrace) {
		return out
	}
	p.advance()
	p.skipNewlines()
	for !p.at(lexer.TokenRBrace) && !p.atEnd() {
		before := p.pos
		var entry ast.MapEntry
		p.withNoStructLit(false, func() {
			entry.Key = p.parseExpr(precAssign)
			if _, ok := p.expect(lexer.TokenColon, "map key"); ok {
				entry.Value = p.parseExpr(precAssign)
			} else {
				entry.Value = &ast.BadExpr{BaseNode: ast.SpanTok(p.cur())}
			}
		})
		out.Entries = append(out.Entries, entry)
		p.skipNewlines()
		if !p.accept(lexer.TokenComma) {
			break
		}
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	rb, ok := p.expect(lexer.TokenRBrace, "map literal")
	if !ok {
		return p.badExpr(start)
	}
	out.SetSpan(start.Pos, rb.End)
	return out
}

// parseFnLit parses a lambda: `fn(x int) -> int { x * 2 }`, `fn(x) { x }` and
// the short form `fn(x) = x * 2`.
func (p *parser) parseFnLit() ast.Expr {
	start := p.cur()
	p.enterFn()
	defer p.leaveFn()

	sig := p.parseFnSig(false, true)
	out := &ast.FnLit{BaseNode: ast.Span(start.Pos, p.cur().Pos), Sig: sig}

	switch {
	case p.accept(lexer.TokenAssign):
		p.skipNewlines()
		out.ExprBody = p.parseExpr(precAssign)
		out.SetSpan(start.Pos, out.ExprBody.End())
	case p.at(lexer.TokenLBrace):
		out.Body = p.parseBlock("lambda body")
		out.SetSpan(start.Pos, out.Body.End())
	default:
		p.errorExpected("a lambda body")
		return p.badExpr(start)
	}
	return out
}
