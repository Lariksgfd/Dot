package parser

import (
	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// parseMatchArmPattern parses a full match-arm pattern: alternatives joined by
// `|`, optionally followed by an `if` guard. Guards may only appear here, at
// the top level of an arm.
func (p *parser) parseMatchArmPattern() ast.Pattern {
	pat := p.parsePattern()
	if p.at(lexer.TokenIf) {
		kw := p.advance()
		var guard ast.Expr
		p.withNoStructLit(true, func() {
			guard = p.parseExpr(precAssign)
		})
		return &ast.GuardedPattern{
			BaseNode: ast.Span(pat.Pos(), guard.End()),
			Pattern:  pat,
			Guard:    guard,
			KwPos:    kw.Pos,
		}
	}
	return pat
}

// parsePattern parses a pattern including `|` alternatives, but without a
// guard.
func (p *parser) parsePattern() ast.Pattern {
	first := p.parsePatternPrimary()
	if !p.at(lexer.TokenPipe) {
		return first
	}
	alts := []ast.Pattern{first}
	for p.accept(lexer.TokenPipe) {
		p.skipNewlines()
		alts = append(alts, p.parsePatternPrimary())
	}
	return &ast.OrPattern{
		BaseNode: ast.Span(first.Pos(), alts[len(alts)-1].End()),
		Alts:     alts,
	}
}

// parsePatternPrimary parses a single pattern with no `|` and no guard.
func (p *parser) parsePatternPrimary() ast.Pattern {
	tok := p.cur()
	switch tok.Type {
	case lexer.TokenUnderscore:
		p.advance()
		return &ast.WildcardPattern{BaseNode: ast.SpanTok(tok)}

	case lexer.TokenInt, lexer.TokenFloat, lexer.TokenString,
		lexer.TokenRawString, lexer.TokenTrue, lexer.TokenFalse, lexer.TokenNil:
		return p.parseLiteralOrRangePattern()

	case lexer.TokenMinus:
		return p.parseLiteralOrRangePattern()

	case lexer.TokenLParen:
		return p.parseTuplePattern()

	case lexer.TokenMut:
		p.advance()
		name, ok := p.expect(lexer.TokenIdent, "mut")
		if !ok {
			return p.badPattern(tok)
		}
		return &ast.IdentPattern{
			BaseNode: ast.Span(tok.Pos, name.End),
			Name:     name.Lexeme,
			Mut:      true,
		}

	case lexer.TokenIdent:
		return p.parseIdentPattern()

	case lexer.TokenLBracket, lexer.TokenStar, lexer.TokenSelfType,
		lexer.TokenFn, lexer.TokenDyn, lexer.TokenWeak:
		// A type pattern such as `[]int arr` or `fn(int) -> int f`.
		return p.parseTypePattern()
	}

	p.errorExpected("a pattern")
	return p.badPattern(tok)
}

// parseIdentPattern disambiguates the identifier-led pattern forms:
// enum variants (`Color.Red`, `Ok(n)`), struct patterns (`Point { x, y }`),
// type patterns (`int n`) and plain bindings (`n`).
func (p *parser) parseIdentPattern() ast.Pattern {
	// Qualified or call-like: an enum variant.
	if p.peek(1).Type == lexer.TokenDot || p.peek(1).Type == lexer.TokenLParen {
		return p.parseEnumPattern()
	}
	// `Name { ... }` is a struct pattern unless struct literals are barred.
	if p.peek(1).Type == lexer.TokenLBrace && !p.noStructLit {
		return p.parseStructPattern()
	}
	// `Type binding` — an identifier directly followed by another identifier
	// or `_`, or a generic type such as `Option[int] o`.
	if p.peek(1).Type == lexer.TokenIdent || p.peek(1).Type == lexer.TokenUnderscore ||
		p.peek(1).Type == lexer.TokenLBracket {
		return p.parseTypePattern()
	}

	name := p.advance()
	// A bare capitalised name with no payload is treated as an unqualified
	// variant such as `None`; the type checker decides what it resolves to.
	if isVariantName(name.Lexeme) {
		return &ast.EnumPattern{
			BaseNode:   ast.SpanTok(name),
			Variant:    name.Lexeme,
			VariantPos: name.Pos,
		}
	}
	return &ast.IdentPattern{BaseNode: ast.SpanTok(name), Name: name.Lexeme}
}

// parseEnumPattern parses `Color.Red`, `Shape.Circle(r)`, `Ok(n)`.
func (p *parser) parseEnumPattern() ast.Pattern {
	first := p.advance()
	out := &ast.EnumPattern{
		BaseNode:   ast.SpanTok(first),
		Variant:    first.Lexeme,
		VariantPos: first.Pos,
	}
	if p.accept(lexer.TokenDot) {
		variant, ok := p.expect(lexer.TokenIdent, "enum name")
		if !ok {
			return p.badPattern(first)
		}
		out.Enum, out.EnumPos = first.Lexeme, first.Pos
		out.Variant, out.VariantPos = variant.Lexeme, variant.Pos
		out.SetSpan(first.Pos, variant.End)
	}
	if p.at(lexer.TokenLParen) {
		p.advance()
		out.HasArgs = true
		p.skipNewlines()
		for !p.at(lexer.TokenRParen) && !p.atEnd() {
			before := p.pos
			out.Args = append(out.Args, p.parsePattern())
			p.skipNewlines()
			if !p.accept(lexer.TokenComma) {
				break
			}
			p.skipNewlines()
			if p.pos == before {
				p.advance()
			}
		}
		rp, ok := p.expect(lexer.TokenRParen, "variant patterns")
		if !ok {
			return p.badPattern(first)
		}
		out.SetSpan(first.Pos, rp.End)
	}
	return out
}

// parseStructPattern parses `Point { x: 0, y }`.
func (p *parser) parseStructPattern() ast.Pattern {
	start := p.cur()
	typ := p.parseType()
	lb, ok := p.expect(lexer.TokenLBrace, "struct pattern")
	if !ok {
		return p.badPattern(start)
	}
	_ = lb
	out := &ast.StructPattern{BaseNode: ast.Span(start.Pos, p.cur().End), Type: typ}
	p.skipNewlines()
	for !p.at(lexer.TokenRBrace) && !p.atEnd() {
		before := p.pos
		name, ok := p.expect(lexer.TokenIdent, "struct pattern field")
		if !ok {
			p.advance()
			continue
		}
		field := ast.StructPatternField{Name: name.Lexeme, NamePos: name.Pos}
		if p.accept(lexer.TokenColon) {
			field.Pattern = p.parsePattern()
		} else {
			field.Shorthand = true
			field.Pattern = &ast.IdentPattern{BaseNode: ast.SpanTok(name), Name: name.Lexeme}
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
	rb, ok := p.expect(lexer.TokenRBrace, "struct pattern")
	if !ok {
		return p.badPattern(start)
	}
	out.SetSpan(start.Pos, rb.End)
	return out
}

// parseTypePattern parses `int n`, `[]int arr`, `Option[int] o` and the same
// forms with the binding omitted.
func (p *parser) parseTypePattern() ast.Pattern {
	start := p.cur()
	typ := p.parseType()
	out := &ast.TypePattern{BaseNode: ast.Span(start.Pos, typ.End()), Type: typ}
	switch p.cur().Type {
	case lexer.TokenIdent:
		name := p.advance()
		out.Binding = &ast.IdentPattern{BaseNode: ast.SpanTok(name), Name: name.Lexeme}
		out.SetSpan(start.Pos, name.End)
	case lexer.TokenUnderscore:
		p.advance()
	}
	return out
}

// parseTuplePattern parses `(a, b)`.
func (p *parser) parseTuplePattern() ast.Pattern {
	lp := p.advance()
	out := &ast.TuplePattern{BaseNode: ast.SpanTok(lp)}
	p.skipNewlines()
	for !p.at(lexer.TokenRParen) && !p.atEnd() {
		before := p.pos
		out.Elems = append(out.Elems, p.parsePattern())
		p.skipNewlines()
		if !p.accept(lexer.TokenComma) {
			break
		}
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	rp, ok := p.expect(lexer.TokenRParen, "tuple pattern")
	if !ok {
		return p.badPattern(lp)
	}
	out.SetSpan(lp.Pos, rp.End)
	return out
}

// parseLiteralOrRangePattern parses a literal pattern and, when a range
// operator follows, folds it into a RangePattern.
func (p *parser) parseLiteralOrRangePattern() ast.Pattern {
	start := p.cur()
	low := p.parsePatternLiteral()
	if p.atAny(lexer.TokenDotDot, lexer.TokenDotDotEq) {
		op := p.advance()
		out := &ast.RangePattern{
			BaseNode:  ast.Span(start.Pos, op.End),
			Low:       low,
			Inclusive: op.Type == lexer.TokenDotDotEq,
		}
		if startsPatternLiteral(p.cur().Type) {
			out.High = p.parsePatternLiteral()
			out.SetSpan(start.Pos, out.High.End())
		}
		return out
	}
	return &ast.LiteralPattern{BaseNode: ast.Span(start.Pos, low.End()), Value: low}
}

// parsePatternLiteral parses one literal value usable in a pattern, allowing a
// leading unary minus.
func (p *parser) parsePatternLiteral() ast.Expr {
	tok := p.cur()
	if tok.Type == lexer.TokenMinus {
		p.advance()
		operand := p.parsePatternLiteral()
		return &ast.UnaryExpr{
			BaseNode: ast.Span(tok.Pos, operand.End()),
			Op:       lexer.TokenMinus,
			OpPos:    tok.Pos,
			X:        operand,
		}
	}
	p.advance()
	switch tok.Type {
	case lexer.TokenInt:
		return &ast.IntLit{
			BaseNode: ast.SpanTok(tok),
			Value:    parseUintLiteral(tok),
			Base:     tok.Base,
			Suffix:   tok.Suffix,
			Raw:      tok.Lexeme,
		}
	case lexer.TokenFloat:
		return &ast.FloatLit{
			BaseNode: ast.SpanTok(tok),
			Value:    parseFloatLiteral(tok),
			Suffix:   tok.Suffix,
			Raw:      tok.Lexeme,
		}
	case lexer.TokenString, lexer.TokenRawString:
		return p.parseStringLit(tok)
	case lexer.TokenTrue, lexer.TokenFalse:
		return &ast.BoolLit{BaseNode: ast.SpanTok(tok), Value: tok.Type == lexer.TokenTrue}
	case lexer.TokenNil:
		return &ast.NilLit{BaseNode: ast.SpanTok(tok)}
	}
	p.errorAt(tok, "expected a literal in pattern, found %s", spellTok(tok.Type))
	return &ast.BadExpr{BaseNode: ast.SpanTok(tok)}
}

// startsPatternLiteral reports whether t can begin a literal pattern value.
func startsPatternLiteral(t lexer.TokenType) bool {
	switch t {
	case lexer.TokenInt, lexer.TokenFloat, lexer.TokenString,
		lexer.TokenRawString, lexer.TokenTrue, lexer.TokenFalse,
		lexer.TokenNil, lexer.TokenMinus:
		return true
	}
	return false
}

// isVariantName reports whether name looks like an enum variant, i.e. starts
// with an upper-case letter.
func isVariantName(name string) bool {
	if name == "" {
		return false
	}
	c := name[0]
	return c >= 'A' && c <= 'Z'
}

// badPattern consumes one token when no progress has been made and returns a
// placeholder node for error recovery.
func (p *parser) badPattern(from lexer.Token) *ast.BadPattern {
	if p.cur().Pos.Offset == from.Pos.Offset && !p.atEnd() {
		p.advance()
	}
	return &ast.BadPattern{BaseNode: ast.Span(from.Pos, p.cur().Pos)}
}
