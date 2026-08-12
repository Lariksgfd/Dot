package parser

import (
	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// parseBlock parses a brace-delimited statement list. context names the
// construct being parsed and is used in diagnostics. When ownScope is true the
// block opens its own lexical scope; function and loop bodies pass false so
// they share the scope of their parameters and loop variables (D53).
func (p *parser) parseBlock(context string, ownScope bool) *ast.BlockStmt {
	lb, ok := p.expect(lexer.TokenLBrace, context)
	if !ok {
		return &ast.BlockStmt{BaseNode: ast.SpanTok(lb), LBrace: lb.Pos, RBrace: lb.Pos}
	}
	out := &ast.BlockStmt{BaseNode: ast.SpanTok(lb), LBrace: lb.Pos}

	if ownScope {
		p.pushScope()
		defer p.popScope()
	}

	// A block body never inherits the enclosing no-struct-literal restriction.
	p.withNoStructLit(false, func() {
		p.skipNewlines()
		for !p.at(lexer.TokenRBrace) && !p.atEnd() {
			before := p.pos
			out.Stmts = append(out.Stmts, p.parseStmt())
			p.skipNewlines()
			if p.pos == before {
				p.advance()
			}
		}
	})

	rb, ok := p.expect(lexer.TokenRBrace, context)
	if ok {
		out.RBrace = rb.Pos
		out.SetSpan(lb.Pos, rb.End)
	} else {
		out.SetSpan(lb.Pos, p.cur().Pos)
	}
	return out
}

// parseStmt parses a single statement, including nested declarations.
func (p *parser) parseStmt() ast.Stmt {
	doc := p.takeDocComments()
	var stmt ast.Stmt

	switch p.cur().Type {
	case lexer.TokenReturn:
		stmt = p.finishStmt(p.parseReturnStmt(), "return")
	case lexer.TokenBreak:
		stmt = p.finishStmt(p.parseBreakStmt(), "break")
	case lexer.TokenContinue:
		stmt = p.finishStmt(p.parseContinueStmt(), "continue")
	case lexer.TokenDefer:
		stmt = p.finishStmt(p.parseDeferStmt(), "defer")
	case lexer.TokenFor:
		stmt = p.parseForStmt("", ast.Position{})
	case lexer.TokenLBrace:
		stmt = p.parseBlock("block", true)
	case lexer.TokenConst, lexer.TokenPub, lexer.TokenFn, lexer.TokenStruct,
		lexer.TokenTrait, lexer.TokenImpl, lexer.TokenEnum, lexer.TokenImport,
		lexer.TokenFrom, lexer.TokenAsync:
		stmt = p.parseDecl()
	case lexer.TokenAt:
		stmt = p.parseAtStmt()
	default:
		stmt = p.parseSimpleStmt()
	}

	if stmt != nil {
		if len(doc) > 0 {
			stmt.SetDoc(doc)
		}
		if c := p.takeLineComments(stmt.End().Line); len(c) > 0 {
			stmt.SetComment(c)
		}
	}
	return stmt
}

// parseAtStmt handles the three `@`-led statement forms: an arena block
// (`@perf { }`), a labelled loop (`@outer for ... { }`) and an annotated
// declaration.
func (p *parser) parseAtStmt() ast.Stmt {
	at := p.cur()
	name := p.peek(1)
	if name.Type != lexer.TokenIdent {
		p.errorExpected("a name after '@'")
		return p.badStmt(at)
	}

	if name.Lexeme == "perf" && p.peek(2).Type == lexer.TokenLBrace {
		p.advance()
		p.advance()
		return p.parsePerfBlock(at)
	}
	if p.peek(2).Type == lexer.TokenFor {
		p.advance()
		p.advance()
		return p.parseForStmt(name.Lexeme, name.Pos)
	}
	return p.parseDecl()
}

// finishStmt consumes the statement terminator after a simple statement.
func (p *parser) finishStmt(s ast.Stmt, context string) ast.Stmt {
	p.expectStatementEnd(context)
	return s
}

// parseSimpleStmt parses a variable declaration, a reassignment or an
// expression statement.
func (p *parser) parseSimpleStmt() ast.Stmt {
	if p.isReassign() {
		x := p.parseExpr(precAssign)
		out := &ast.ExprStmt{BaseNode: ast.SpanOf(x, x), X: x}
		p.expectStatementEnd("assignment")
		return out
	}
	if p.looksLikeVarDecl() {
		decl := p.parseVarDecl(nil, false, false)
		p.declareVarNames(decl)
		return p.finishStmt(decl, "declaration")
	}
	start := p.cur()
	if !startsExpr(start.Type) {
		p.errorExpected("a statement")
		s := p.badStmt(start)
		p.syncStmt()
		return s
	}
	x := p.parseExpr(precAssign)
	out := &ast.ExprStmt{BaseNode: ast.SpanOf(x, x), X: x}
	p.expectStatementEnd("expression")
	return out
}

// isReassign reports whether the upcoming statement is a plain
// `name = ...` whose name is already declared in the current scope: that is a
// reassignment, not a declaration (D53). Multi-name forms keep the VarDecl
// shape so the checker can expand tuple values per name.
func (p *parser) isReassign() bool {
	if !p.at(lexer.TokenIdent) {
		return false
	}
	if p.peek(1).Type != lexer.TokenAssign {
		return false
	}
	return p.nameDeclared(p.cur().Lexeme)
}

// declareVarNames records the names of a parsed variable declaration in the
// current scope so later statements can tell reassignment from shadowing.
func (p *parser) declareVarNames(decl *ast.VarDecl) {
	for _, n := range decl.Names {
		if id, ok := n.(*ast.Ident); ok {
			p.declareName(id.Name)
		}
	}
}

// looksLikeVarDecl decides between a declaration and an assignment expression
// using bounded lookahead (D31):
//
//	IDENT (, IDENT)* '='          -> declaration
//	IDENT <type> ['=' ...]        -> declaration
//
// Anything else (`items[0] = 5`, `p.x = 1`, `f()`) is an expression statement.
func (p *parser) looksLikeVarDecl() bool {
	if !p.atAny(lexer.TokenIdent, lexer.TokenUnderscore) {
		return false
	}
	// Name list followed by '='.
	i := 0
	for {
		t := p.peek(i).Type
		if t != lexer.TokenIdent && t != lexer.TokenUnderscore {
			break
		}
		switch p.peek(i + 1).Type {
		case lexer.TokenComma:
			i += 2
			continue
		case lexer.TokenAssign:
			return true
		}
		break
	}
	// Single name followed by a type.
	if p.cur().Type == lexer.TokenIdent && startsType(p.peek(1).Type) {
		// `f (x)` is a call, not a declaration with a tuple type.
		if p.peek(1).Type == lexer.TokenLParen {
			return false
		}
		if p.peek(1).Type == lexer.TokenLBracket {
			return p.typeAfterBracketRun(1)
		}
		if p.peek(1).Type == lexer.TokenStar {
			return false
		}
		return true
	}
	return false
}

// typeAfterBracketRun reports whether the bracket run starting at lookahead
// offset i closes and is followed by something that can start a type, which
// distinguishes `items []int = ...` from `items[0] = ...`.
func (p *parser) typeAfterBracketRun(i int) bool {
	depth := 0
	for n := i; n < i+64; n++ {
		switch p.peek(n).Type {
		case lexer.TokenLBracket:
			depth++
		case lexer.TokenRBracket:
			depth--
			if depth == 0 {
				return startsType(p.peek(n + 1).Type)
			}
		case lexer.TokenEOF, lexer.TokenNewline:
			return false
		}
	}
	return false
}

// parseVarDecl parses `x = 42`, `count int = 0`, `a, b = 1, 2` and
// `const PI float = 3.14`.
func (p *parser) parseVarDecl(anns []*ast.Annotation, isConst, isPub bool) *ast.VarDecl {
	start := p.cur()
	out := &ast.VarDecl{
		BaseNode:    ast.SpanTok(start),
		Annotations: anns,
		Const:       isConst,
		Pub:         isPub,
	}

	for {
		tok := p.cur()
		switch tok.Type {
		case lexer.TokenIdent:
			p.advance()
			out.Names = append(out.Names, &ast.Ident{BaseNode: ast.SpanTok(tok), Name: tok.Lexeme})
		case lexer.TokenUnderscore:
			p.advance()
			out.Names = append(out.Names, &ast.UnderscoreExpr{BaseNode: ast.SpanTok(tok)})
		default:
			p.errorExpected("a variable name")
			return out
		}
		if !p.accept(lexer.TokenComma) {
			break
		}
	}

	stop := p.cur().Pos
	if !p.at(lexer.TokenAssign) && startsType(p.cur().Type) {
		out.Type = p.parseType()
		stop = out.Type.End()
	}
	if p.at(lexer.TokenAssign) {
		eq := p.advance()
		out.AssignPos = eq.Pos
		p.skipNewlines()
		out.Values = p.parseExprList()
		if n := len(out.Values); n > 0 {
			stop = out.Values[n-1].End()
		}
	}
	out.SetSpan(start.Pos, stop)
	return out
}

// parseForStmt parses all five loop forms into a single ast.ForStmt (D20).
// The `for` keyword must be the current token; label is "" when unlabelled.
func (p *parser) parseForStmt(label string, labelPos ast.Position) *ast.ForStmt {
	kw := p.advance()
	out := &ast.ForStmt{
		BaseNode: ast.SpanTok(kw),
		Label:    label,
		LabelPos: labelPos,
		KwPos:    kw.Pos,
	}
	start := kw.Pos
	if label != "" {
		start = labelPos
	}

	p.pushScope()
	defer p.popScope()

	switch {
	case p.at(lexer.TokenLBrace):
		out.Kind = ast.ForInfinite
	case p.looksLikeForIn():
		out.Kind = ast.ForIn
		p.parseForInHead(out)
	default:
		out.Kind = ast.ForCond
		p.withNoStructLit(true, func() { out.Cond = p.parseExpr(precAssign) })
	}

	p.enterLoop(label)
	out.Body = p.parseBlock("for body", false)
	p.leaveLoop()
	out.SetSpan(start, out.Body.End())
	return out
}

// looksLikeForIn reports whether the loop head is the `x in xs` or
// `k, v in m` form.
func (p *parser) looksLikeForIn() bool {
	if !p.atAny(lexer.TokenIdent, lexer.TokenUnderscore) {
		return false
	}
	switch p.peek(1).Type {
	case lexer.TokenIn:
		return true
	case lexer.TokenComma:
		return p.peek(2).Type == lexer.TokenIdent ||
			p.peek(2).Type == lexer.TokenUnderscore
	}
	return false
}

// parseForInHead parses `x in xs` and `k, v in m` into out.
func (p *parser) parseForInHead(out *ast.ForStmt) {
	first := p.parseLoopVar()
	if p.accept(lexer.TokenComma) {
		out.Key = first
		out.Value = p.parseLoopVar()
	} else {
		out.Value = first
	}
	if _, ok := p.expect(lexer.TokenIn, "loop variable"); !ok {
		return
	}
	p.withNoStructLit(true, func() { out.Iterable = p.parseExpr(precAssign) })
}

// parseLoopVar parses one loop variable name or `_`.
func (p *parser) parseLoopVar() ast.Expr {
	tok := p.cur()
	switch tok.Type {
	case lexer.TokenIdent:
		p.advance()
		p.declareName(tok.Lexeme)
		return &ast.Ident{BaseNode: ast.SpanTok(tok), Name: tok.Lexeme}
	case lexer.TokenUnderscore:
		p.advance()
		return &ast.UnderscoreExpr{BaseNode: ast.SpanTok(tok)}
	}
	p.errorExpected("a loop variable")
	return p.badExpr(tok)
}

// parseReturnStmt parses `return`, `return x` and `return a, b`.
func (p *parser) parseReturnStmt() *ast.ReturnStmt {
	kw := p.advance()
	out := &ast.ReturnStmt{BaseNode: ast.SpanTok(kw), KwPos: kw.Pos}
	if p.fnDepth == 0 {
		d := p.errorAt(kw, "return outside of a function")
		p.hint(d, "return may only appear inside a function body")
	}
	if p.atAny(lexer.TokenNewline, lexer.TokenRBrace, lexer.TokenEOF) {
		return out
	}
	out.Values = p.parseExprList()
	if n := len(out.Values); n > 0 {
		out.SetSpan(kw.Pos, out.Values[n-1].End())
	}
	return out
}

// parseBreakStmt parses `break` and `break @label`.
func (p *parser) parseBreakStmt() *ast.BreakStmt {
	kw := p.advance()
	out := &ast.BreakStmt{BaseNode: ast.SpanTok(kw), KwPos: kw.Pos}
	if p.loopDepth == 0 {
		d := p.errorAt(kw, "break outside of a loop")
		p.hint(d, "break may only appear inside a for loop")
	}
	if label, pos, end, ok := p.parseLabelRef(); ok {
		out.Label, out.LabelPos = label, pos
		out.SetSpan(kw.Pos, end)
	}
	return out
}

// parseContinueStmt parses `continue` and `continue @label`.
func (p *parser) parseContinueStmt() *ast.ContinueStmt {
	kw := p.advance()
	out := &ast.ContinueStmt{BaseNode: ast.SpanTok(kw), KwPos: kw.Pos}
	if p.loopDepth == 0 {
		d := p.errorAt(kw, "continue outside of a loop")
		p.hint(d, "continue may only appear inside a for loop")
	}
	if label, pos, end, ok := p.parseLabelRef(); ok {
		out.Label, out.LabelPos = label, pos
		out.SetSpan(kw.Pos, end)
	}
	return out
}

// parseLabelRef parses the optional `@label` suffix of break and continue.
func (p *parser) parseLabelRef() (string, ast.Position, ast.Position, bool) {
	if !p.at(lexer.TokenAt) {
		return "", ast.Position{}, ast.Position{}, false
	}
	p.advance()
	name, ok := p.expect(lexer.TokenIdent, "'@'")
	if !ok {
		return "", ast.Position{}, ast.Position{}, false
	}
	if !p.knownLabel(name.Lexeme) {
		d := p.errorAt(name, "unknown loop label @%s", name.Lexeme)
		p.hint(d, "labels are introduced as '@name for ... { }'")
	}
	return name.Lexeme, name.Pos, name.End, true
}

// knownLabel reports whether name is an enclosing loop label.
func (p *parser) knownLabel(name string) bool {
	for _, l := range p.labels {
		if l == name {
			return true
		}
	}
	return false
}

// parseDeferStmt parses `defer f()`.
func (p *parser) parseDeferStmt() *ast.DeferStmt {
	kw := p.advance()
	call := p.parseExpr(precAssign)
	if _, ok := call.(*ast.CallExpr); !ok {
		if _, bad := call.(*ast.BadExpr); !bad {
			d := p.errorAt(kw, "defer requires a function call")
			p.hint(d, "write 'defer f()' rather than 'defer f'")
		}
	}
	return &ast.DeferStmt{
		BaseNode: ast.Span(kw.Pos, call.End()),
		Call:     call,
		KwPos:    kw.Pos,
	}
}

// parsePerfBlock parses `@perf { ... }`. The '@' and `perf` tokens have
// already been consumed; at is the '@' token.
func (p *parser) parsePerfBlock(at lexer.Token) *ast.PerfBlock {
	block := p.parseBlock("@perf block", true)
	return &ast.PerfBlock{
		BaseNode: ast.Span(at.Pos, block.End()),
		Block:    block,
		AtPos:    at.Pos,
	}
}

// badStmt consumes one token when no progress has been made and returns a
// placeholder node for error recovery.
func (p *parser) badStmt(from lexer.Token) *ast.BadStmt {
	if p.cur().Pos.Offset == from.Pos.Offset && !p.atEnd() {
		p.advance()
	}
	return &ast.BadStmt{BaseNode: ast.Span(from.Pos, p.cur().Pos)}
}
