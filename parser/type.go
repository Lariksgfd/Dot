package parser

import (
	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// parseType parses a type expression, including any trailing `?` shorthands.
//
// It never returns nil: on error it records a diagnostic, consumes at least
// one token and returns an *ast.BadType so the caller can keep going.
func (p *parser) parseType() ast.Type {
	t := p.parseTypeAtom()
	for p.at(lexer.TokenQuestion) {
		q := p.advance()
		t = &ast.OptionalType{
			BaseNode: ast.Span(t.Pos(), q.End),
			Elem:     t,
		}
	}
	return t
}

// parseTypeAtom parses a type without the postfix `?` shorthand.
func (p *parser) parseTypeAtom() ast.Type {
	switch p.cur().Type {
	case lexer.TokenIdent:
		return p.parseNamedOrGeneric()
	case lexer.TokenSelfType:
		tok := p.advance()
		return &ast.SelfTypeNode{BaseNode: ast.SpanTok(tok)}
	case lexer.TokenLBracket:
		return p.parseSliceOrArrayType()
	case lexer.TokenLParen:
		return p.parseTupleType()
	case lexer.TokenStar:
		tok := p.advance()
		elem := p.parseType()
		return &ast.PointerType{BaseNode: ast.Span(tok.Pos, elem.End()), Elem: elem}
	case lexer.TokenDyn:
		tok := p.advance()
		inner := p.parseType()
		return &ast.DynType{BaseNode: ast.Span(tok.Pos, inner.End()), Trait: inner}
	case lexer.TokenWeak:
		tok := p.advance()
		elem := p.parseType()
		return &ast.WeakType{BaseNode: ast.Span(tok.Pos, elem.End()), Elem: elem}
	case lexer.TokenFn:
		return p.parseFnType()
	}

	// `map` is a contextual keyword: the lexer reports it as an identifier,
	// so it is handled in parseNamedOrGeneric, not here.
	tok := p.cur()
	p.errorExpected("a type")
	return p.badType(tok)
}

// parseNamedOrGeneric parses `int`, `User`, `http.Client`, `map[K]V` and any
// of those followed by type arguments (`Option[int]`, `Result[int, Error]`).
func (p *parser) parseNamedOrGeneric() ast.Type {
	first := p.advance()

	if first.Lexeme == "map" && p.at(lexer.TokenLBracket) {
		return p.parseMapType(first)
	}

	named := &ast.NamedType{
		BaseNode: ast.SpanTok(first),
		Name:     first.Lexeme,
		NamePos:  first.Pos,
	}
	// Qualified name: pkg.Type (a single dot; deeper paths are folded into Pkg).
	for p.at(lexer.TokenDot) && p.peek(1).Type == lexer.TokenIdent {
		p.advance()
		next := p.advance()
		if named.Pkg == "" {
			named.Pkg = named.Name
			named.PkgPos = named.NamePos
		} else {
			named.Pkg += "." + named.Name
		}
		named.Name = next.Lexeme
		named.NamePos = next.Pos
		named.SetSpan(first.Pos, next.End)
	}

	if p.at(lexer.TokenLBracket) {
		args, lb, rb := p.parseTypeArgs()
		return &ast.GenericType{
			BaseNode: ast.Span(first.Pos, rb.End),
			Base:     named,
			Args:     args,
			LBracket: lb.Pos,
			RBracket: rb.Pos,
		}
	}
	return named
}

// parseMapType parses `map[K]V`. mapTok is the already consumed `map`.
func (p *parser) parseMapType(mapTok lexer.Token) ast.Type {
	if _, ok := p.expect(lexer.TokenLBracket, "map"); !ok {
		return p.badType(mapTok)
	}
	key := p.parseType()
	if _, ok := p.expect(lexer.TokenRBracket, "map key type"); !ok {
		return p.badType(mapTok)
	}
	value := p.parseType()
	return &ast.MapType{
		BaseNode: ast.Span(mapTok.Pos, value.End()),
		Key:      key,
		Value:    value,
	}
}

// parseSliceOrArrayType parses `[]T` and `[N]T`.
func (p *parser) parseSliceOrArrayType() ast.Type {
	lb := p.advance() // '['
	if p.at(lexer.TokenRBracket) {
		p.advance()
		elem := p.parseType()
		return &ast.SliceType{BaseNode: ast.Span(lb.Pos, elem.End()), Elem: elem}
	}
	length := p.parseExpr(precAssign)
	if _, ok := p.expect(lexer.TokenRBracket, "array length"); !ok {
		return p.badType(lb)
	}
	elem := p.parseType()
	return &ast.ArrayType{
		BaseNode: ast.Span(lb.Pos, elem.End()),
		Len:      length,
		Elem:     elem,
	}
}

// parseTupleType parses `(A, B)`. A single parenthesized type `(A)` is the
// type A itself, not a one-element tuple; `()` is the empty tuple.
func (p *parser) parseTupleType() ast.Type {
	lp := p.advance() // '('
	p.skipNewlines()
	if p.at(lexer.TokenRParen) {
		rp := p.advance()
		return &ast.TupleType{BaseNode: ast.Span(lp.Pos, rp.End)}
	}
	elems := p.parseTypeList(lexer.TokenRParen)
	rp, ok := p.expect(lexer.TokenRParen, "tuple type")
	if !ok {
		return p.badType(lp)
	}
	if len(elems) == 1 {
		// Parenthesized type: keep the inner node, widen its span.
		if bn, okc := elems[0].(interface{ SetSpan(a, b ast.Position) }); okc {
			bn.SetSpan(lp.Pos, rp.End)
		}
		return elems[0]
	}
	return &ast.TupleType{BaseNode: ast.Span(lp.Pos, rp.End), Elems: elems}
}

// parseFnType parses `fn(A, B) -> C`, `fn(A)` and `fn(...A) -> C`.
func (p *parser) parseFnType() ast.Type {
	fnTok := p.advance() // 'fn'
	stop := fnTok.End
	ft := &ast.FnType{BaseNode: ast.SpanTok(fnTok)}

	if _, ok := p.expect(lexer.TokenLParen, "fn type"); !ok {
		return p.badType(fnTok)
	}
	p.skipNewlines()
	for !p.at(lexer.TokenRParen) && !p.atEnd() {
		if p.accept(lexer.TokenEllipsis) {
			ft.Variadic = true
		}
		ft.Params = append(ft.Params, p.parseType())
		p.skipNewlines()
		if !p.accept(lexer.TokenComma) {
			break
		}
		p.skipNewlines()
	}
	rp, ok := p.expect(lexer.TokenRParen, "fn type parameters")
	if !ok {
		return p.badType(fnTok)
	}
	stop = rp.End

	if p.accept(lexer.TokenArrow) {
		ft.Result = p.parseType()
		stop = ft.Result.End()
	}
	ft.SetSpan(fnTok.Pos, stop)
	return ft
}

// parseTypeList parses a comma-separated list of types, stopping before the
// close token. The close token itself is not consumed.
func (p *parser) parseTypeList(close lexer.TokenType) []ast.Type {
	var list []ast.Type
	p.skipNewlines()
	for !p.at(close) && !p.atEnd() {
		before := p.pos
		list = append(list, p.parseType())
		p.skipNewlines()
		if !p.accept(lexer.TokenComma) {
			break
		}
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	return list
}

// parseTypeArgs parses a bracketed type-argument list `[A, B]`, returning the
// arguments plus the bracket tokens. It must only be called when the current
// token is '['.
func (p *parser) parseTypeArgs() ([]ast.Type, lexer.Token, lexer.Token) {
	lb := p.advance() // '['
	args := p.parseTypeList(lexer.TokenRBracket)
	rb, ok := p.expect(lexer.TokenRBracket, "type arguments")
	if !ok {
		rb = lb
	}
	if len(args) == 0 {
		d := p.errorAt(lb, "empty type argument list")
		p.hint(d, "write at least one type inside the brackets, as in Option[int]")
	}
	return args, lb, rb
}

// badType records nothing itself; it consumes one token when no progress has
// been made and returns a placeholder node for error recovery.
func (p *parser) badType(from lexer.Token) *ast.BadType {
	if p.cur().Pos.Offset == from.Pos.Offset && !p.atEnd() {
		p.advance()
	}
	return &ast.BadType{BaseNode: ast.Span(from.Pos, p.cur().Pos)}
}
