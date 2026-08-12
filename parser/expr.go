package parser

import (
	"strconv"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// parseExpr is the Pratt driver. It parses a prefix expression and then keeps
// folding in infix and postfix operators whose binding power is at least
// minPrec.
//
// It never returns nil: on error it records a diagnostic and returns an
// *ast.BadExpr so the caller can keep going.
func (p *parser) parseExpr(minPrec int) ast.Expr {
	left := p.parseUnary()

	for !p.atEnd() {
		tok := p.cur()

		// Postfix operators bind tighter than any infix operator.
		if handled, next := p.parsePostfix(left); handled {
			left = next
			continue
		}

		prec := precOf(tok.Type)
		if prec == precNone || prec < minPrec {
			break
		}

		switch {
		case isAssignOp(tok.Type):
			left = p.parseAssign(left)
		case tok.Type == lexer.TokenPipeArrow:
			left = p.parsePipe(left)
		case tok.Type == lexer.TokenAs:
			left = p.parseCast(left)
		case tok.Type == lexer.TokenIs:
			left = p.parseIsExpr(left)
		case isNonAssoc(tok.Type):
			left = p.parseRangeInfix(left)
		default:
			left = p.parseBinary(left)
		}
	}
	return left
}

// parsePostfix folds one postfix operator (`.`, `(`, `[`, `?`) into left.
// It reports whether anything was folded.
func (p *parser) parsePostfix(left ast.Expr) (bool, ast.Expr) {
	switch p.cur().Type {
	case lexer.TokenDot:
		return true, p.parseField(left)
	case lexer.TokenLParen:
		return true, p.parseCall(left)
	case lexer.TokenLBracket:
		return true, p.parseIndexOrSlice(left)
	case lexer.TokenQuestion:
		return true, p.parseTry(left)
	case lexer.TokenLBrace:
		if e, ok := p.maybeStructLit(left); ok {
			return true, e
		}
	}
	return false, left
}

// parseBinary folds a left-associative or right-associative binary operator.
func (p *parser) parseBinary(left ast.Expr) ast.Expr {
	op := p.advance()
	prec := precOf(op.Type)
	next := prec + 1
	if isRightAssoc(op.Type) {
		next = prec
	}
	p.skipNewlines()
	right := p.parseExpr(next)
	return &ast.BinaryExpr{
		BaseNode: ast.Span(left.Pos(), right.End()),
		Op:       op.Type,
		OpPos:    op.Pos,
		X:        left,
		Y:        right,
	}
}

// parseAssign folds `=` or a compound assignment operator. Multi-target
// assignment (`a, b = 1, 2`) is produced by the statement parser, which
// collects the target list before calling here.
func (p *parser) parseAssign(left ast.Expr) ast.Expr {
	op := p.advance()
	p.skipNewlines()
	value := p.parseExpr(precAssign)
	return &ast.AssignExpr{
		BaseNode: ast.Span(left.Pos(), value.End()),
		Op:       op.Type,
		OpPos:    op.Pos,
		Targets:  []ast.Expr{left},
		Values:   []ast.Expr{value},
	}
}

// parseRangeInfix folds `..` or `..=`. Ranges are non-associative: chaining
// two of them is a diagnostic.
func (p *parser) parseRangeInfix(left ast.Expr) ast.Expr {
	op := p.advance()
	out := &ast.RangeExpr{
		BaseNode:  ast.Span(left.Pos(), op.End),
		Low:       left,
		Inclusive: op.Type == lexer.TokenDotDotEq,
		OpPos:     op.Pos,
	}
	if startsExpr(p.cur().Type) && !p.at(lexer.TokenLBrace) {
		out.High = p.parseExpr(precRange + 1)
		out.SetSpan(left.Pos(), out.High.End())
	}
	if isNonAssoc(p.cur().Type) {
		d := p.errorHere("range operators cannot be chained")
		p.hint(d, "parenthesise one of the ranges")
	}
	return out
}

// parsePipe folds `x |> f(y)`. Desugaring to f(x, y) happens in Phase 4.
func (p *parser) parsePipe(left ast.Expr) ast.Expr {
	op := p.advance()
	p.skipNewlines()
	fn := p.parseExpr(precPipe + 1)
	return &ast.PipeExpr{
		BaseNode: ast.Span(left.Pos(), fn.End()),
		X:        left,
		Fn:       fn,
		OpPos:    op.Pos,
	}
}

// parseCast folds `x as T`.
func (p *parser) parseCast(left ast.Expr) ast.Expr {
	kw := p.advance()
	typ := p.parseType()
	return &ast.CastExpr{
		BaseNode: ast.Span(left.Pos(), typ.End()),
		X:        left,
		Type:     typ,
		KwPos:    kw.Pos,
	}
}

// parseIsExpr folds `x is T`.
func (p *parser) parseIsExpr(left ast.Expr) ast.Expr {
	kw := p.advance()
	typ := p.parseType()
	return &ast.IsExpr{
		BaseNode: ast.Span(left.Pos(), typ.End()),
		X:        left,
		Type:     typ,
		KwPos:    kw.Pos,
	}
}

// parseTry folds the postfix `?` error-propagation operator.
func (p *parser) parseTry(left ast.Expr) ast.Expr {
	op := p.advance()
	return &ast.TryExpr{
		BaseNode: ast.Span(left.Pos(), op.End),
		X:        left,
		OpPos:    op.Pos,
	}
}

// parseExprList parses a comma-separated list of expressions. It stops at the
// first token that cannot continue the list; no closing token is consumed.
func (p *parser) parseExprList() []ast.Expr {
	var list []ast.Expr
	for {
		before := p.pos
		list = append(list, p.parseExpr(precAssign))
		if !p.accept(lexer.TokenComma) {
			return list
		}
		p.skipNewlines()
		if p.pos == before {
			p.advance()
			return list
		}
	}
}

// badExpr consumes one token when no progress has been made and returns a
// placeholder node for error recovery.
func (p *parser) badExpr(from lexer.Token) *ast.BadExpr {
	if p.cur().Pos.Offset == from.Pos.Offset && !p.atEnd() {
		p.advance()
	}
	return &ast.BadExpr{BaseNode: ast.Span(from.Pos, p.cur().Pos)}
}

// --- literal decoding ----------------------------------------------------

// parseUintLiteral decodes the magnitude of an integer token. The lexer has
// already stripped separators and the base prefix, so this only has to parse
// digits in the recorded base.
func parseUintLiteral(tok lexer.Token) uint64 {
	base := tok.Base
	if base == 0 {
		base = 10
	}
	text := strings.ReplaceAll(tok.Value, "_", "")
	if text == "" {
		return 0
	}
	v, err := strconv.ParseUint(text, base, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseFloatLiteral decodes the value of a float token.
func parseFloatLiteral(tok lexer.Token) float64 {
	text := strings.ReplaceAll(tok.Value, "_", "")
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	return v
}
