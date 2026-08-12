package parser

import (
	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// parseDecl parses one top-level declaration, including any annotations,
// `pub` marker and `async` marker that precede it.
func (p *parser) parseDecl() ast.Decl {
	doc := p.takeDocComments()
	start := p.cur()
	anns := p.parseAnnotations()

	pub := false
	if p.at(lexer.TokenPub) {
		p.advance()
		pub = true
	}
	async := false
	if p.at(lexer.TokenAsync) {
		p.advance()
		async = true
	}

	var decl ast.Decl
	switch p.cur().Type {
	case lexer.TokenFn:
		decl = p.parseFnDecl(anns, pub, async)
	case lexer.TokenStruct:
		decl = p.parseStructDecl(anns, pub)
	case lexer.TokenEnum:
		decl = p.parseEnumDecl(anns, pub)
	case lexer.TokenTrait:
		decl = p.parseTraitDecl(anns, pub)
	case lexer.TokenImpl:
		decl = p.parseImplDecl(anns)
	case lexer.TokenImport, lexer.TokenFrom:
		decl = p.parseImportDecl()
	case lexer.TokenConst:
		p.advance()
		d := p.parseVarDecl(anns, true, pub)
		p.expectStatementEnd("const declaration")
		decl = d
	}

	if decl == nil {
		if async {
			p.errorExpected("'fn' after async")
			p.syncDecl()
			decl = p.badDecl(start)
		} else if len(anns) == 1 && anns[0].Name == "perf" && p.at(lexer.TokenLBrace) {
			d := p.errorHere("@perf blocks are only allowed inside a function body")
			p.hint(d, "move the @perf block into a function")
			p.parseBlock("@perf block")
			decl = p.badDecl(start)
		} else if p.looksLikeVarDecl() {
			d := p.parseVarDecl(anns, false, pub)
			p.expectStatementEnd("declaration")
			decl = d
		} else {
			p.errorExpected("a declaration")
			p.syncDecl()
			decl = p.badDecl(start)
		}
	}

	if decl != nil {
		if len(doc) > 0 {
			decl.SetDoc(doc)
		}
		if c := p.takeLineComments(decl.End().Line); len(c) > 0 {
			decl.SetComment(c)
		}
	}
	return decl
}

// badDecl wraps a bad statement so declaration positions can recover.
func (p *parser) badDecl(from lexer.Token) ast.Decl {
	if p.cur().Pos.Offset == from.Pos.Offset && !p.atEnd() {
		p.advance()
	}
	return &ast.VarDecl{BaseNode: ast.Span(from.Pos, p.cur().Pos)}
}

// parseAnnotations collects the `@name` / `@name(args)` markers that precede a
// declaration.
func (p *parser) parseAnnotations() []*ast.Annotation {
	var out []*ast.Annotation
	for p.at(lexer.TokenAt) && p.peek(1).Type == lexer.TokenIdent {
		at := p.advance()
		name := p.advance()
		ann := &ast.Annotation{
			BaseNode: ast.Span(at.Pos, name.End),
			Name:     name.Lexeme,
			NamePos:  name.Pos,
			AtPos:    at.Pos,
		}
		if p.at(lexer.TokenLParen) {
			args, rp := p.parseArgs()
			for _, a := range args {
				ann.Args = append(ann.Args, a.Value)
			}
			ann.SetSpan(at.Pos, rp.End)
		}
		out = append(out, ann)
		p.skipNewlines()
	}
	return out
}

// parseFnDecl parses a function, method or trait method declaration.
func (p *parser) parseFnDecl(anns []*ast.Annotation, pub, async bool) *ast.FnDecl {
	kw := p.advance()
	out := &ast.FnDecl{
		BaseNode:    ast.SpanTok(kw),
		Annotations: anns,
		Pub:         pub,
		Async:       async,
		KwPos:       kw.Pos,
	}
	name, ok := p.expect(lexer.TokenIdent, "fn")
	if !ok {
		p.syncDecl()
		return out
	}
	out.Name, out.NamePos = name.Lexeme, name.Pos
	out.SetSpan(kw.Pos, name.End)

	if p.at(lexer.TokenLBracket) {
		out.TypeParams = p.parseTypeParams()
	}

	p.enterFn()
	out.Sig = p.parseFnSig(true, false)
	if out.Sig != nil {
		out.SetSpan(kw.Pos, out.Sig.End())
	}

	switch {
	case p.accept(lexer.TokenAssign):
		p.skipNewlines()
		out.ExprBody = p.parseExpr(precAssign)
		out.SetSpan(kw.Pos, out.ExprBody.End())
	case p.at(lexer.TokenLBrace):
		out.Body = p.parseBlock("fn body")
		out.SetSpan(kw.Pos, out.Body.End())
	}
	p.leaveFn()
	return out
}

// parseFnSig parses `(params) -> Result`. allowSelf permits a `self` receiver;
// allowUntyped permits parameters without a type, as in lambdas.
func (p *parser) parseFnSig(allowSelf, allowUntyped bool) *ast.FnSig {
	if p.at(lexer.TokenFn) {
		p.advance()
	}
	lp, ok := p.expect(lexer.TokenLParen, "fn name")
	if !ok {
		return &ast.FnSig{BaseNode: ast.SpanTok(lp), LParen: lp.Pos, RParen: lp.Pos}
	}
	out := &ast.FnSig{BaseNode: ast.SpanTok(lp), LParen: lp.Pos}

	recv, params := p.parseParams(allowSelf, allowUntyped)
	out.Recv, out.Params = recv, params

	rp, ok := p.expect(lexer.TokenRParen, "parameters")
	if !ok {
		out.SetSpan(lp.Pos, p.cur().Pos)
		return out
	}
	out.RParen = rp.Pos
	out.SetSpan(lp.Pos, rp.End)

	if p.at(lexer.TokenArrow) {
		arrow := p.advance()
		out.ArrowPos = arrow.Pos
		out.Result = p.parseType()
		out.SetSpan(lp.Pos, out.Result.End())
	}
	return out
}

// parseParams parses the parameter list body, stopping before ')'.
// The `self` receiver, when present, is returned separately (D22).
func (p *parser) parseParams(allowSelf, allowUntyped bool) (*ast.Param, []*ast.Param) {
	var recv *ast.Param
	var params []*ast.Param

	p.skipNewlines()
	for !p.at(lexer.TokenRParen) && !p.atEnd() {
		before := p.pos
		param := p.parseParam(allowSelf && len(params) == 0 && recv == nil, allowUntyped)
		switch {
		case param == nil:
			// error already reported
		case param.IsSelf:
			if recv != nil || len(params) > 0 {
				p.errorAt(p.cur(), "self must be the first parameter")
			}
			recv = param
		default:
			params = append(params, param)
		}
		p.skipNewlines()
		if !p.accept(lexer.TokenComma) {
			break
		}
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	return recv, params
}

// parseParam parses one parameter: `name Type`, `name Type = default`,
// `nums ...int`, `mut self`, `self` and the untyped lambda form `x`.
func (p *parser) parseParam(allowSelf, allowUntyped bool) *ast.Param {
	start := p.cur()
	out := &ast.Param{BaseNode: ast.SpanTok(start)}

	if p.at(lexer.TokenMut) {
		p.advance()
		out.Mut = true
	}
	if p.at(lexer.TokenSelf) {
		self := p.advance()
		if !allowSelf {
			p.errorAt(self, "self is only allowed as the first parameter of a method")
		}
		out.IsSelf = true
		out.Name = "self"
		out.NamePos = self.Pos
		out.SetSpan(start.Pos, self.End)
		return out
	}

	name, ok := p.expect(lexer.TokenIdent, "parameter list")
	if !ok {
		return nil
	}
	out.Name, out.NamePos = name.Lexeme, name.Pos
	out.SetSpan(start.Pos, name.End)

	if p.accept(lexer.TokenEllipsis) {
		out.Variadic = true
		out.Type = p.parseType()
		out.SetSpan(start.Pos, out.Type.End())
	} else if startsType(p.cur().Type) {
		out.Type = p.parseType()
		out.SetSpan(start.Pos, out.Type.End())
	} else if !allowUntyped {
		p.errorExpected("a parameter type")
	}

	if p.accept(lexer.TokenAssign) {
		out.Default = p.parseExpr(precAssign)
		out.SetSpan(start.Pos, out.Default.End())
	}
	return out
}

// parseTypeParams parses `[T]`, `[T: Comparable]`, `[T: A + B, U]`.
func (p *parser) parseTypeParams() []*ast.TypeParam {
	lb := p.advance() // '['
	var out []*ast.TypeParam
	p.skipNewlines()
	for !p.at(lexer.TokenRBracket) && !p.atEnd() {
		before := p.pos
		name, ok := p.expect(lexer.TokenIdent, "type parameter list")
		if !ok {
			p.advance()
			continue
		}
		tp := &ast.TypeParam{
			BaseNode: ast.SpanTok(name),
			Name:     name.Lexeme,
			NamePos:  name.Pos,
		}
		if p.accept(lexer.TokenColon) {
			for {
				bound := p.parseType()
				tp.Bounds = append(tp.Bounds, bound)
				tp.SetSpan(name.Pos, bound.End())
				if !p.accept(lexer.TokenPlus) {
					break
				}
			}
		}
		out = append(out, tp)
		p.skipNewlines()
		if !p.accept(lexer.TokenComma) {
			break
		}
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	if _, ok := p.expect(lexer.TokenRBracket, "type parameters"); !ok {
		_ = lb
	}
	return out
}
