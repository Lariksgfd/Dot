package parser

import (
	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// parseStructDecl parses a `struct` declaration.
func (p *parser) parseStructDecl(anns []*ast.Annotation, pub bool) *ast.StructDecl {
	kw := p.advance()
	out := &ast.StructDecl{
		BaseNode:    ast.SpanTok(kw),
		Annotations: anns,
		Pub:         pub,
		KwPos:       kw.Pos,
	}
	name, ok := p.expect(lexer.TokenIdent, "struct")
	if !ok {
		p.syncDecl()
		return out
	}
	out.Name, out.NamePos = name.Lexeme, name.Pos
	if p.at(lexer.TokenLBracket) {
		out.TypeParams = p.parseTypeParams()
	}
	if _, ok := p.expect(lexer.TokenLBrace, "struct name"); !ok {
		p.syncDecl()
		return out
	}
	p.skipNewlines()
	for !p.at(lexer.TokenRBrace) && !p.atEnd() {
		before := p.pos
		if f := p.parseFieldDecl(); f != nil {
			out.Fields = append(out.Fields, f)
		}
		p.skipNewlines()
		p.accept(lexer.TokenComma)
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	rb, ok := p.expect(lexer.TokenRBrace, "struct body")
	if ok {
		out.SetSpan(kw.Pos, rb.End)
	}
	return out
}

// parseFieldDecl parses one struct field, including `embed T` and defaults.
func (p *parser) parseFieldDecl() *ast.FieldDecl {
	anns := p.parseAnnotations()
	start := p.cur()
	out := &ast.FieldDecl{BaseNode: ast.SpanTok(start), Annotations: anns}

	if p.at(lexer.TokenEmbed) {
		p.advance()
		out.Embed = true
		out.Type = p.parseType()
		out.SetSpan(start.Pos, out.Type.End())
		return out
	}

	name, ok := p.expect(lexer.TokenIdent, "struct body")
	if !ok {
		return nil
	}
	out.Name, out.NamePos = name.Lexeme, name.Pos
	out.Type = p.parseType()
	out.SetSpan(start.Pos, out.Type.End())

	if p.accept(lexer.TokenAssign) {
		out.Default = p.parseExpr(precAssign)
		out.SetSpan(start.Pos, out.Default.End())
	}
	return out
}

// parseEnumDecl parses an `enum` declaration with plain or payload variants.
func (p *parser) parseEnumDecl(anns []*ast.Annotation, pub bool) *ast.EnumDecl {
	kw := p.advance()
	out := &ast.EnumDecl{
		BaseNode:    ast.SpanTok(kw),
		Annotations: anns,
		Pub:         pub,
		KwPos:       kw.Pos,
	}
	name, ok := p.expect(lexer.TokenIdent, "enum")
	if !ok {
		p.syncDecl()
		return out
	}
	out.Name, out.NamePos = name.Lexeme, name.Pos
	if p.at(lexer.TokenLBracket) {
		out.TypeParams = p.parseTypeParams()
	}
	if _, ok := p.expect(lexer.TokenLBrace, "enum name"); !ok {
		p.syncDecl()
		return out
	}
	p.skipNewlines()
	for !p.at(lexer.TokenRBrace) && !p.atEnd() {
		before := p.pos
		vname, ok := p.expect(lexer.TokenIdent, "enum body")
		if !ok {
			p.advance()
			continue
		}
		variant := &ast.EnumVariant{
			BaseNode: ast.SpanTok(vname),
			Name:     vname.Lexeme,
			NamePos:  vname.Pos,
		}
		if p.at(lexer.TokenLParen) {
			p.advance()
			variant.HasParens = true
			_, fields := p.parseParams(false, false)
			variant.Fields = fields
			rp, ok := p.expect(lexer.TokenRParen, "variant fields")
			if ok {
				variant.SetSpan(vname.Pos, rp.End)
			}
		}
		out.Variants = append(out.Variants, variant)
		p.skipNewlines()
		p.accept(lexer.TokenComma)
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	rb, ok := p.expect(lexer.TokenRBrace, "enum body")
	if ok {
		out.SetSpan(kw.Pos, rb.End)
	}
	return out
}

// parseTraitDecl parses a `trait` declaration.
func (p *parser) parseTraitDecl(anns []*ast.Annotation, pub bool) *ast.TraitDecl {
	kw := p.advance()
	out := &ast.TraitDecl{
		BaseNode:    ast.SpanTok(kw),
		Annotations: anns,
		Pub:         pub,
		KwPos:       kw.Pos,
	}
	name, ok := p.expect(lexer.TokenIdent, "trait")
	if !ok {
		p.syncDecl()
		return out
	}
	out.Name, out.NamePos = name.Lexeme, name.Pos
	if p.at(lexer.TokenLBracket) {
		out.TypeParams = p.parseTypeParams()
	}
	out.Methods = p.parseMethodBlock("trait body")
	out.SetSpan(kw.Pos, p.prevEnd())
	return out
}

// parseImplDecl parses `impl T { }`, `impl Trait for T { }` and
// `impl[T] Stack[T] { }`.
func (p *parser) parseImplDecl(anns []*ast.Annotation) *ast.ImplDecl {
	kw := p.advance()
	out := &ast.ImplDecl{
		BaseNode:    ast.SpanTok(kw),
		Annotations: anns,
		KwPos:       kw.Pos,
	}
	if p.at(lexer.TokenLBracket) {
		out.TypeParams = p.parseTypeParams()
	}
	first := p.parseType()
	if p.at(lexer.TokenFor) {
		forKw := p.advance()
		out.Trait = first
		out.ForPos = forKw.Pos
		out.Type = p.parseType()
	} else {
		out.Type = first
	}
	out.Methods = p.parseMethodBlock("impl body")
	out.SetSpan(kw.Pos, p.prevEnd())
	return out
}

// parseMethodBlock parses a brace-delimited list of method declarations,
// shared by traits and impls.
func (p *parser) parseMethodBlock(context string) []*ast.FnDecl {
	if _, ok := p.expect(lexer.TokenLBrace, context); !ok {
		p.syncDecl()
		return nil
	}
	var methods []*ast.FnDecl
	p.skipNewlines()
	for !p.at(lexer.TokenRBrace) && !p.atEnd() {
		before := p.pos
		anns := p.parseAnnotations()
		pub := p.accept(lexer.TokenPub)
		async := p.accept(lexer.TokenAsync)
		if p.at(lexer.TokenFn) {
			methods = append(methods, p.parseFnDecl(anns, pub, async))
		} else {
			p.errorExpected("a method declaration")
			p.advance()
		}
		p.skipNewlines()
		if p.pos == before {
			p.advance()
		}
	}
	p.expect(lexer.TokenRBrace, context)
	return methods
}

func (p *parser) expectIdentOrKeyword(msg string) (lexer.Token, bool) {
	tok := p.cur()
	if tok.Type == lexer.TokenIdent || (tok.Type >= lexer.TokenFn && tok.Type <= lexer.TokenOverride) {
		p.advance()
		return tok, true
	}
	p.errorExpected(msg)
	return lexer.Token{}, false
}

// parseImportDecl parses `import a.b`, `import a.b as c` and
// `from a.b import x, y`.
func (p *parser) parseImportDecl() *ast.ImportDecl {
	kw := p.advance()
	out := &ast.ImportDecl{
		BaseNode: ast.SpanTok(kw),
		From:     kw.Type == lexer.TokenFrom,
		KwPos:    kw.Pos,
	}

	if p.at(lexer.TokenString) {
		seg := p.advance()
		// Remove quotes
		val := seg.Lexeme[1 : len(seg.Lexeme)-1]
		out.Path = append(out.Path, val)
		out.PathPos = append(out.PathPos, seg.Pos)
		out.SetSpan(kw.Pos, seg.End)
	} else {
		for {
			seg, ok := p.expectIdentOrKeyword("import path")
			if !ok {
				p.syncDecl()
				return out
			}
			out.Path = append(out.Path, seg.Lexeme)
			out.PathPos = append(out.PathPos, seg.Pos)
			out.SetSpan(kw.Pos, seg.End)
			if !p.accept(lexer.TokenDot) {
				break
			}
		}
	}

	if out.From {
		if _, ok := p.expect(lexer.TokenImport, "import path"); !ok {
			p.syncDecl()
			return out
		}
		for {
			name, ok := p.expect(lexer.TokenIdent, "import list")
			if !ok {
				break
			}
			item := ast.ImportName{Name: name.Lexeme, NamePos: name.Pos}
			if p.at(lexer.TokenAs) {
				p.advance()
				alias, ok := p.expect(lexer.TokenIdent, "as")
				if ok {
					item.Alias, item.AliasPos = alias.Lexeme, alias.Pos
					out.SetSpan(kw.Pos, alias.End)
				}
			} else {
				out.SetSpan(kw.Pos, name.End)
			}
			out.Names = append(out.Names, item)
			if !p.accept(lexer.TokenComma) {
				break
			}
			p.skipNewlines()
		}
	} else if p.at(lexer.TokenAs) {
		p.advance()
		alias, ok := p.expect(lexer.TokenIdent, "as")
		if ok {
			out.Alias, out.AliasPos = alias.Lexeme, alias.Pos
			out.SetSpan(kw.Pos, alias.End)
		}
	}

	p.expectStatementEnd("import")
	return out
}

// prevEnd returns the end position of the most recently consumed token.
func (p *parser) prevEnd() ast.Position {
	if p.pos == 0 {
		return p.cur().End
	}
	return p.toks[p.pos-1].End
}
