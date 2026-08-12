package parser

import (
	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// parseIfExpr parses `if cond { ... }`, `else if` chains and the if-let form
// `if user = find(42) { ... }`.
func (p *parser) parseIfExpr() *ast.IfExpr {
	kw := p.advance()
	out := &ast.IfExpr{BaseNode: ast.SpanTok(kw)}

	p.withNoStructLit(true, func() {
		if bind, cond, ok := p.tryParseIfLet(); ok {
			out.Bind, out.Cond = bind, cond
			return
		}
		out.Cond = p.parseExpr(precAssign)
	})

	out.Then = p.parseBlock("if body", true)
	out.SetSpan(kw.Pos, out.Then.End())

	if p.at(lexer.TokenElse) {
		p.advance()
		p.skipNewlines()
		if p.at(lexer.TokenIf) {
			out.ElseIf = p.parseIfExpr()
			out.SetSpan(kw.Pos, out.ElseIf.End())
		} else {
			out.Else = p.parseBlock("else body", true)
			out.SetSpan(kw.Pos, out.Else.End())
		}
	}
	return out
}

// tryParseIfLet recognises the `if pattern = expr {` binding form. It reports
// false without consuming anything when the head is a plain condition.
func (p *parser) tryParseIfLet() (ast.Pattern, ast.Expr, bool) {
	if !p.at(lexer.TokenIdent) {
		return nil, nil, false
	}
	// Only a simple `name = expr` head is a binding; anything else (`a == b`,
	// `a.b = c`) is an ordinary condition.
	if p.peek(1).Type != lexer.TokenAssign {
		return nil, nil, false
	}
	name := p.advance()
	p.advance() // '='
	bind := &ast.IdentPattern{BaseNode: ast.SpanTok(name), Name: name.Lexeme}
	return bind, p.parseExpr(precAssign), true
}

// parseMatchExpr parses `match subject { pattern => body ... }`.
func (p *parser) parseMatchExpr() *ast.MatchExpr {
	kw := p.advance()
	out := &ast.MatchExpr{BaseNode: ast.SpanTok(kw), KwPos: kw.Pos}

	p.withNoStructLit(true, func() { out.Subject = p.parseExpr(precAssign) })

	lb, ok := p.expect(lexer.TokenLBrace, "match subject")
	if !ok {
		return out
	}
	out.LBrace = lb.Pos

	p.skipNewlines()
	for !p.at(lexer.TokenRBrace) && !p.atEnd() {
		before := p.pos
		arm := p.parseMatchArm()
		if arm != nil {
			out.Arms = append(out.Arms, arm)
		}
		p.skipNewlines()
		p.accept(lexer.TokenComma)
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	rb, ok := p.expect(lexer.TokenRBrace, "match arms")
	if ok {
		out.RBrace = rb.Pos
		out.SetSpan(kw.Pos, rb.End)
	}
	return out
}

// parseMatchArm parses one `pattern => body` arm.
func (p *parser) parseMatchArm() *ast.MatchArm {
	start := p.cur()
	var pat ast.Pattern
	p.withNoStructLit(false, func() { pat = p.parseMatchArmPattern() })

	arrow, ok := p.expect(lexer.TokenFatArrow, "match pattern")
	if !ok {
		p.syncStmt()
		return &ast.MatchArm{
			BaseNode: ast.Span(start.Pos, p.cur().Pos),
			Pattern:  pat,
			Body:     &ast.BadExpr{BaseNode: ast.SpanTok(start)},
		}
	}
	p.skipNewlines()

	var body ast.Expr
	if p.at(lexer.TokenLBrace) {
		block := p.parseBlock("match arm", true)
		body = &ast.BlockExpr{BaseNode: ast.SpanOf(block, block), Block: block}
	} else {
		body = p.parseExpr(precAssign)
	}
	return &ast.MatchArm{
		BaseNode: ast.Span(start.Pos, body.End()),
		Pattern:  pat,
		Body:     body,
		ArrowPos: arrow.Pos,
	}
}

// parseSpawnExpr parses `spawn { ... }` and `spawn thread { ... }`.
func (p *parser) parseSpawnExpr() ast.Expr {
	kw := p.advance()
	
	isThread := false
	if p.at(lexer.TokenIdent) && p.cur().Lexeme == "thread" {
		p.advance()
		isThread = true
	}

	p.enterFn()
	defer p.leaveFn()

	if !p.at(lexer.TokenLBrace) {
		p.errorExpected("'{' after spawn")
		return p.badExpr(kw)
	}
	block := p.parseBlock("spawn body", true)
	return &ast.SpawnExpr{
		BaseNode: ast.Span(kw.Pos, block.End()),
		Block:    block,
		KwPos:    kw.Pos,
		IsThread: isThread,
	}
}

// parseAwaitExpr parses `await expr`.
func (p *parser) parseAwaitExpr() ast.Expr {
	kw := p.advance()
	operand := p.parseExpr(precUnary)
	return &ast.AwaitExpr{
		BaseNode: ast.Span(kw.Pos, operand.End()),
		X:        operand,
		KwPos:    kw.Pos,
	}
}
