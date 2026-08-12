package parser

import (
	"strconv"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// parseCall folds a call `f(a, b)` onto left.
func (p *parser) parseCall(left ast.Expr) ast.Expr {
	lp := p.cur()
	args, rp := p.parseArgs()
	return &ast.CallExpr{
		BaseNode: ast.Span(left.Pos(), rp.End),
		Fn:       left,
		Args:     args,
		LParen:   lp.Pos,
		RParen:   rp.Pos,
	}
}

// parseArgs parses a parenthesised argument list, supporting named arguments
// (`port: 3000`) and spread (`xs...`). The cursor must be on '('.
func (p *parser) parseArgs() ([]ast.Arg, lexer.Token) {
	lp := p.advance()
	var args []ast.Arg
	p.skipNewlines()
	for !p.at(lexer.TokenRParen) && !p.atEnd() {
		before := p.pos
		var arg ast.Arg
		if p.at(lexer.TokenIdent) && p.peek(1).Type == lexer.TokenColon {
			name := p.advance()
			p.advance()
			arg.Name, arg.NamePos = name.Lexeme, name.Pos
		}
		p.withNoStructLit(false, func() { arg.Value = p.parseExpr(precAssign) })
		if p.accept(lexer.TokenEllipsis) {
			arg.Spread = true
		}
		args = append(args, arg)
		p.skipNewlines()
		if !p.accept(lexer.TokenComma) {
			break
		}
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	rp, ok := p.expect(lexer.TokenRParen, "arguments")
	if !ok {
		rp = lp
	}
	return args, rp
}

// parseIndexOrSlice folds `a[i]`, the generic instantiation `Stack[int]` and
// the slicing forms `a[1..3]`, `a[..3]`, `a[1..]`, `a[..]`.
func (p *parser) parseIndexOrSlice(left ast.Expr) ast.Expr {
	lb := p.advance()
	p.skipNewlines()

	// Open low bound: `a[..high]`.
	if p.atAny(lexer.TokenDotDot, lexer.TokenDotDotEq) {
		op := p.advance()
		out := &ast.SliceExpr{
			X:         left,
			Inclusive: op.Type == lexer.TokenDotDotEq,
			LBracket:  lb.Pos,
		}
		if !p.at(lexer.TokenRBracket) {
			p.withNoStructLit(false, func() { out.High = p.parseExpr(precRange + 1) })
		}
		rb, ok := p.expect(lexer.TokenRBracket, "slice expression")
		if !ok {
			return p.badExpr(lb)
		}
		out.RBracket = rb.Pos
		out.SetSpan(left.Pos(), rb.End)
		return out
	}

	var first ast.Expr
	p.withNoStructLit(false, func() { first = p.parseExpr(precRange + 1) })

	if p.atAny(lexer.TokenDotDot, lexer.TokenDotDotEq) {
		op := p.advance()
		out := &ast.SliceExpr{
			X:         left,
			Low:       first,
			Inclusive: op.Type == lexer.TokenDotDotEq,
			LBracket:  lb.Pos,
		}
		if !p.at(lexer.TokenRBracket) {
			p.withNoStructLit(false, func() { out.High = p.parseExpr(precRange + 1) })
		}
		rb, ok := p.expect(lexer.TokenRBracket, "slice expression")
		if !ok {
			return p.badExpr(lb)
		}
		out.RBracket = rb.Pos
		out.SetSpan(left.Pos(), rb.End)
		return out
	}

	indices := []ast.Expr{first}
	for p.accept(lexer.TokenComma) {
		p.skipNewlines()
		if p.at(lexer.TokenRBracket) {
			break
		}
		var e ast.Expr
		p.withNoStructLit(false, func() { e = p.parseExpr(precAssign) })
		indices = append(indices, e)
	}
	rb, ok := p.expect(lexer.TokenRBracket, "index expression")
	if !ok {
		return p.badExpr(lb)
	}
	return &ast.IndexExpr{
		BaseNode: ast.Span(left.Pos(), rb.End),
		X:        left,
		Indices:  indices,
		LBracket: lb.Pos,
		RBracket: rb.Pos,
	}
}

// parseField folds `a.b` and the tuple index `t.0`.
func (p *parser) parseField(left ast.Expr) ast.Expr {
	dot := p.advance()
	tok := p.cur()

	switch tok.Type {
	case lexer.TokenIdent:
		p.advance()
		return &ast.FieldExpr{
			BaseNode: ast.Span(left.Pos(), tok.End),
			X:        left,
			Name:     tok.Lexeme,
			NamePos:  tok.Pos,
		}
	case lexer.TokenInt:
		p.advance()
		idx, err := strconv.Atoi(tok.Value)
		if err != nil {
			idx = 0
		}
		return &ast.FieldExpr{
			BaseNode:     ast.Span(left.Pos(), tok.End),
			X:            left,
			Name:         tok.Lexeme,
			NamePos:      tok.Pos,
			Index:        idx,
			IsTupleIndex: true,
		}
	}

	p.errorExpected("a field name after '.'")
	return p.badExpr(dot)
}

// maybeStructLit folds `Name { ... }` into a struct literal when the current
// context allows it. It reports false when the brace must be read as a block.
func (p *parser) maybeStructLit(left ast.Expr) (ast.Expr, bool) {
	if p.noStructLit {
		return left, false
	}
	typ, ok := p.exprToType(left)
	if !ok {
		return left, false
	}
	start := lexer.Token{Pos: left.Pos(), End: left.End()}
	return p.parseStructLitBody(typ, start), true
}

// parseStructLitBody parses the `{ field: value, shorthand }` part of a struct
// literal whose type has already been parsed.
func (p *parser) parseStructLitBody(typ ast.Type, start lexer.Token) *ast.StructLit {
	lb := p.advance() // '{'
	out := &ast.StructLit{BaseNode: ast.Span(start.Pos, lb.End), Type: typ}

	p.skipNewlines()
	for !p.at(lexer.TokenRBrace) && !p.atEnd() {
		before := p.pos
		name, ok := p.expect(lexer.TokenIdent, "struct literal field")
		if !ok {
			p.advance()
			continue
		}
		field := ast.StructLitField{Name: name.Lexeme, NamePos: name.Pos}
		if p.accept(lexer.TokenColon) {
			p.withNoStructLit(false, func() { field.Value = p.parseExpr(precAssign) })
		} else {
			field.Shorthand = true
			field.Value = &ast.Ident{BaseNode: ast.SpanTok(name), Name: name.Lexeme}
		}
		out.Fields = append(out.Fields, field)
		p.skipNewlines()
		if !p.accept(lexer.TokenComma) {
			break
		}
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	rb, ok := p.expect(lexer.TokenRBrace, "struct literal")
	if ok {
		out.SetSpan(start.Pos, rb.End)
	}
	return out
}

// exprToType reinterprets an already-parsed expression as a type, which is how
// `Point { ... }` and `Stack[int] { ... }` are recognised after the fact.
// It reports false when the expression cannot denote a type.
func (p *parser) exprToType(e ast.Expr) (ast.Type, bool) {
	switch x := e.(type) {
	case *ast.Ident:
		return &ast.NamedType{
			BaseNode: ast.Span(x.Pos(), x.End()),
			Name:     x.Name,
			NamePos:  x.Pos(),
		}, true
	case *ast.FieldExpr:
		base, ok := x.X.(*ast.Ident)
		if !ok || x.IsTupleIndex {
			return nil, false
		}
		return &ast.NamedType{
			BaseNode: ast.Span(x.Pos(), x.End()),
			Pkg:      base.Name,
			PkgPos:   base.Pos(),
			Name:     x.Name,
			NamePos:  x.NamePos,
		}, true
	case *ast.IndexExpr:
		base, ok := p.exprToType(x.X)
		if !ok {
			return nil, false
		}
		args := make([]ast.Type, 0, len(x.Indices))
		for _, idx := range x.Indices {
			at, ok := p.exprToType(idx)
			if !ok {
				return nil, false
			}
			args = append(args, at)
		}
		return &ast.GenericType{
			BaseNode: ast.Span(x.Pos(), x.End()),
			Base:     base,
			Args:     args,
			LBracket: x.LBracket,
			RBracket: x.RBracket,
		}, true
	}
	return nil, false
}
