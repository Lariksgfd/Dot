package parser

import (
	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/errors"
	"github.com/dotlang/dot/lexer"
)

// maxInterpDepth caps how deeply string interpolations may nest before the
// parser refuses to descend further (D35).
const maxInterpDepth = 8

// parseStringLit converts a TokenString or TokenRawString into its AST node.
//
// Literal chunks are copied across as ast.PartText; interpolation chunks are
// re-lexed and re-parsed as expressions, with every position remapped onto the
// absolute location of the chunk in the enclosing file.
func (p *parser) parseStringLit(tok lexer.Token) ast.Expr {
	if tok.Type == lexer.TokenRawString {
		return &ast.RawStringLit{BaseNode: ast.SpanTok(tok), Value: tok.Value}
	}

	lit := &ast.StringLit{BaseNode: ast.SpanTok(tok), Raw: tok.Lexeme}
	for _, part := range tok.Parts {
		switch part.Kind {
		case lexer.StringPartLiteral:
			lit.Parts = append(lit.Parts, ast.StringPart{
				Kind: ast.PartText,
				Text: part.Value,
				Pos:  part.Pos,
			})
		case lexer.StringPartExpr:
			lit.Parts = append(lit.Parts, ast.StringPart{
				Kind: ast.PartExpr,
				Expr: p.parseInterpolation(part),
				Pos:  part.Pos,
			})
		}
	}
	if len(lit.Parts) == 0 && tok.Value != "" {
		lit.Parts = append(lit.Parts, ast.StringPart{
			Kind: ast.PartText,
			Text: tok.Value,
			Pos:  tok.Pos,
		})
	}
	return lit
}

// parseInterpolation lexes and parses the source of one "{ ... }" chunk as an
// expression. Diagnostics produced by the nested pass are merged into the
// enclosing error list with absolute positions.
func (p *parser) parseInterpolation(part lexer.StringPart) ast.Expr {
	if p.interpDepth >= maxInterpDepth {
		d := p.errorAt(lexer.Token{Pos: part.Pos, End: part.Pos},
			"string interpolation nested too deeply (limit %d)", maxInterpDepth)
		p.hint(d, "extract the inner expression into a variable")
		return &ast.BadExpr{BaseNode: ast.Span(part.Pos, part.Pos)}
	}

	toks, lexErr := lexer.Tokenize(part.Value, part.Pos.File)
	for i := range toks {
		toks[i].Pos = remapPosition(part.Pos, toks[i].Pos)
		toks[i].End = remapPosition(part.Pos, toks[i].End)
	}
	if lexErr != nil {
		p.mergeRemapped(lexErr, part.Pos)
	}

	sub := &parser{
		toks:        toks,
		file:        p.file,
		errs:        p.errs,
		interpDepth: p.interpDepth + 1,
		posBase:     &part.Pos,
		scopes:      []map[string]bool{{}},
	}
	sub.skipNewlines()
	if sub.atEnd() {
		return &ast.BadExpr{BaseNode: ast.Span(part.Pos, part.Pos)}
	}
	expr := sub.parseExpr(precAssign)
	if p.errs.Len() >= errors.MaxErrors {
		p.bail = true
	}
	if expr == nil {
		return &ast.BadExpr{BaseNode: ast.Span(part.Pos, part.Pos)}
	}
	return expr
}

// mergeRemapped adds the diagnostics of a nested lex/parse pass to the
// enclosing error list, translating their positions into absolute ones.
func (p *parser) mergeRemapped(err error, base lexer.Position) {
	list, ok := err.(*errors.ErrorList)
	if !ok {
		return
	}
	for _, e := range list.Errors {
		d, ok := e.(*errors.Diagnostic)
		if !ok {
			p.errs.Add(e)
			continue
		}
		abs := remapPosition(base, lexer.Position{
			File: d.File, Line: d.Line, Column: d.Column, Offset: d.Offset,
		})
		clone := *d
		clone.File, clone.Line, clone.Column, clone.Offset = abs.File, abs.Line, abs.Column, abs.Offset
		p.errs.Add(&clone)
	}
}

// remapPosition translates rel, a position inside an interpolation chunk that
// was lexed standalone, into an absolute position in the enclosing file.
//
// The chunk starts at base, so offsets simply shift; line 1 of the chunk
// continues base's line and its columns shift, while later lines keep their
// own columns and only their line numbers shift.
func remapPosition(base, rel lexer.Position) lexer.Position {
	out := lexer.Position{
		File:   base.File,
		Line:   base.Line + rel.Line - 1,
		Column: rel.Column,
		Offset: base.Offset + rel.Offset,
	}
	if rel.Line == 1 {
		out.Column = base.Column + rel.Column - 1
	}
	return out
}
