// Package parser turns a stream of tokens from the lexer into an abstract
// syntax tree.
//
// It is a Pratt (precedence-climbing) parser with panic-mode recovery: it
// never stops at the first error, but recovers and keeps going so that as many
// diagnostics as possible are reported at once, inserting ast.Bad* nodes where
// it could not build a real node. See Design Decisions D26–D42.
package parser

import (
	"fmt"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/errors"
	"github.com/dotlang/dot/lexer"
)

// Parse turns a token slice from lexer.Tokenize into a *ast.Program.
//
// The returned Program is always non-nil, even on failure: the parser
// recovers and keeps going so that as many diagnostics as possible are
// reported at once, inserting ast.BadExpr / BadStmt / BadType / BadPattern
// where it could not build a real node.
//
// The returned error is nil on success, otherwise a *errors.ErrorList whose
// elements are all *errors.Diagnostic.
func Parse(tokens []lexer.Token, filename string) (*ast.Program, error) {
	p := &parser{
		toks:   tokens,
		file:   filename,
		errs:   &errors.ErrorList{},
		scopes: []map[string]bool{{}},
	}
	prog := p.parseProgram()
	return prog, p.errs.Err()
}

// ParseFile lexes and parses source in one step. Lexer and parser
// diagnostics are merged into a single *errors.ErrorList, in source order
// (all lexer diagnostics first). Parsing proceeds even when lexing failed.
func ParseFile(source, filename string) (*ast.Program, error) {
	tokens, lexErr := lexer.Tokenize(source, filename)
	p := &parser{
		toks:   tokens,
		file:   filename,
		errs:   &errors.ErrorList{},
		scopes: []map[string]bool{{}},
	}
	if lexErr != nil {
		if list, ok := lexErr.(*errors.ErrorList); ok {
			for _, e := range list.Errors {
				p.errs.Add(e)
			}
		}
	}
	prog := p.parseProgram()
	return prog, p.errs.Err()
}

// parser holds the mutable state of the recursive-descent pass.
type parser struct {
	toks []lexer.Token // never empty; always ends TokenNewline, TokenEOF
	pos  int           // index of the current token
	file string
	errs *errors.ErrorList

	panicking bool // suppress cascading diagnostics until the next sync point
	bail      bool // errors.MaxErrors reached: unwind, emit Bad* nodes

	noStructLit bool     // D30: a bare `{` after a name is a block, not a literal
	loopDepth   int      // > 0 inside a for body (reset inside lambdas/spawn)
	fnDepth     int      // > 0 inside any function/lambda body
	labels      []string // active loop labels, innermost last

	interpDepth int             // D35: nesting level of string-interpolation re-parses
	posBase     *lexer.Position // non-nil in an interpolation sub-parser

	// scopes is the stack of names declared per lexical scope. The top map is
	// the current scope; the bottom map is the file scope. parseSimpleStmt
	// consults it to tell declarations apart from reassignments (D53).
	scopes []map[string]bool

	// fnLoopStack saves (loopDepth, labels) for each fn nesting level so that
	// enterFn/leaveFn can save and restore loop state around lambdas/spawn.
	fnLoopStack []fnLoopState

	comments []lexer.Token
}

// fnLoopState is one saved frame of loop bookkeeping.
type fnLoopState struct {
	loopDepth int
	labels    []string
}

// parseProgram is the top-level declaration loop with the progress watchdog.
func (p *parser) parseProgram() *ast.Program {
	prog := &ast.Program{
		File:     p.file,
		BaseNode: ast.SpanTok(p.cur()),
	}
	p.skipNewlines()
	for !p.atEnd() {
		before := p.pos
		prog.Decls = append(prog.Decls, p.parseDecl())
		if p.pos == before {
			p.advance()
		}
		p.skipNewlines()
	}
	return prog
}

// --- cursor -------------------------------------------------------------

func (p *parser) cur() lexer.Token {
	// Skip comments
	for p.pos < len(p.toks) && p.toks[p.pos].Type == lexer.TokenComment {
		p.comments = append(p.comments, p.toks[p.pos])
		p.pos++
	}
	if p.pos >= len(p.toks) {
		return lexer.Token{Type: lexer.TokenEOF}
	}
	return p.toks[p.pos]
}

func (p *parser) peek(n int) lexer.Token {
	idx := p.pos
	for i := 0; i < n; {
		idx++
		if idx >= len(p.toks) {
			return lexer.Token{Type: lexer.TokenEOF}
		}
		if p.toks[idx].Type != lexer.TokenComment {
			i++
		}
	}
	return p.toks[idx]
}

func (p *parser) at(t lexer.TokenType) bool { return p.cur().Type == t }

func (p *parser) atAny(ts ...lexer.TokenType) bool {
	cur := p.cur().Type
	for _, t := range ts {
		if cur == t {
			return true
		}
	}
	return false
}

// atEnd reports whether parsing has reached the end: either the stream is
// exhausted or the error cap has been hit (p.bail).
func (p *parser) atEnd() bool { return p.cur().Type == lexer.TokenEOF || p.bail }

func (p *parser) takeDocComments() []lexer.Token {
	if len(p.comments) == 0 {
		return nil
	}
	doc := p.comments
	p.comments = nil
	return doc
}

func (p *parser) takeLineComments(line int) []lexer.Token {
	var lineComments []lexer.Token
	var rest []lexer.Token
	for _, c := range p.comments {
		if c.Pos.Line == line {
			lineComments = append(lineComments, c)
		} else {
			rest = append(rest, c)
		}
	}
	p.comments = rest
	return lineComments
}

// advance consumes and returns the current token. Never steps past the final
// TokenEOF.
func (p *parser) advance() lexer.Token {
	tok := p.cur()
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	// skip comments for next cur()
	p.cur() 
	return tok
}

func (p *parser) accept(t lexer.TokenType) bool {
	if p.at(t) {
		p.advance()
		return true
	}
	return false
}

// expect consumes a token of type t in the given context. If the token does
// not match, it records an "expected ... after ..." diagnostic and returns
// false.
func (p *parser) expect(t lexer.TokenType, context string) (lexer.Token, bool) {
	if p.at(t) {
		return p.advance(), true
	}
	what := spellTok(t)
	if context != "" {
		what += " after " + context
	}
	p.errorExpected(what)
	return p.cur(), false
}

func (p *parser) skipNewlines() {
	for p.at(lexer.TokenNewline) && !p.atEnd() {
		p.advance()
	}
}

// expectStatementEnd enforces the required statement terminator (D29): a
// TokenNewline (consumed), or TokenRBrace / TokenEOF (left for the caller).
// Anything else is a diagnostic plus syncStmt recovery.
func (p *parser) expectStatementEnd(context string) bool {
	if p.at(lexer.TokenNewline) {
		p.advance()
		return true
	}
	if p.at(lexer.TokenRBrace) || p.at(lexer.TokenEOF) {
		return true
	}
	d := p.errorHere("expected end of statement after %s", context)
	p.hint(d, "Dot statements end at the newline")
	p.syncStmt()
	return false
}

// --- diagnostics --------------------------------------------------------

// errorAt records a diagnostic at the position of tok. While panic mode is
// active, the diagnostic is constructed (so callers can attach hints) but not
// added to the error list — this suppresses cascading diagnostics. The first
// error entering panic mode flips p.panicking; reaching MaxErrors flips p.bail.
func (p *parser) errorAt(tok lexer.Token, format string, args ...any) *errors.Diagnostic {
	msg := fmt.Sprintf(format, args...)
	length := tok.End.Offset - tok.Pos.Offset
	if length < 1 {
		length = 1
	}
	d := errors.NewParseError(tok.Pos.File, tok.Pos.Line, tok.Pos.Column, tok.Pos.Offset, length, msg)
	if !p.panicking {
		p.errs.Add(d)
		p.panicking = true
		if p.errs.Len() >= errors.MaxErrors {
			p.bail = true
		}
	}
	return d
}

func (p *parser) errorHere(format string, args ...any) *errors.Diagnostic {
	return p.errorAt(p.cur(), format, args...)
}

// errorExpected records a diagnostic of the form "expected <what>, found
// <current token>".
func (p *parser) errorExpected(what string) {
	p.errorAt(p.cur(), "expected %s, found %s", what, spellTok(p.cur().Type))
}

func (p *parser) hint(d *errors.Diagnostic, hint string) { d.Hint = hint }

// --- recovery -----------------------------------------------------------

// syncStmt advances to the next statement boundary so parsing can resume:
// TokenNewline (consumed), TokenRBrace / TokenEOF (not consumed), or a
// statement-starting keyword (not consumed). Bracket depth is tracked so a
// newline inside ()/[] is not a false sync point.
func (p *parser) syncStmt() {
	depth := 0
	for !p.atEnd() {
		switch p.cur().Type {
		case lexer.TokenNewline:
			if depth <= 0 {
				p.advance()
				p.recovered()
				return
			}
		case lexer.TokenRBrace, lexer.TokenEOF:
			if depth <= 0 {
				p.recovered()
				return
			}
		}
		if depth <= 0 && isStmtSyncKeyword(p.cur().Type) {
			p.recovered()
			return
		}
		switch p.cur().Type {
		case lexer.TokenLParen, lexer.TokenLBracket:
			depth++
		case lexer.TokenRParen, lexer.TokenRBracket:
			depth--
		}
		p.advance()
	}
	p.recovered()
}

// syncDecl advances to the next top-level declaration keyword at bracket
// depth 0, or to TokenEOF.
func (p *parser) syncDecl() {
	depth := 0
	for !p.atEnd() {
		switch p.cur().Type {
		case lexer.TokenLBrace, lexer.TokenLBracket, lexer.TokenLParen:
			depth++
		case lexer.TokenRBrace, lexer.TokenRBracket, lexer.TokenRParen:
			if depth > 0 {
				depth--
			}
		}
		if depth <= 0 && isDeclSyncKeyword(p.cur().Type) {
			p.recovered()
			return
		}
		p.advance()
	}
	p.recovered()
}

// syncBlock advances, counting { / }, until the } that closes the current
// block; the } is left unconsumed so the caller's expect(TokenRBrace)
// succeeds.
func (p *parser) syncBlock() {
	depth := 1
	for !p.atEnd() {
		switch p.cur().Type {
		case lexer.TokenLBrace:
			depth++
		case lexer.TokenRBrace:
			depth--
			if depth == 0 {
				p.recovered()
				return
			}
		}
		p.advance()
	}
	p.recovered()
}

func (p *parser) recovered() { p.panicking = false }

// --- flag scoping -------------------------------------------------------

func (p *parser) withNoStructLit(v bool, f func()) {
	old := p.noStructLit
	p.noStructLit = v
	f()
	p.noStructLit = old
}

// pushScope opens a fresh lexical scope for declared names.
func (p *parser) pushScope() {
	p.scopes = append(p.scopes, map[string]bool{})
}

// popScope closes the innermost lexical scope. The file scope is never popped.
func (p *parser) popScope() {
	if len(p.scopes) > 1 {
		p.scopes = p.scopes[:len(p.scopes)-1]
	}
}

// declareName records name as declared in the current scope. `_` and empty
// names are never recorded.
func (p *parser) declareName(name string) {
	if name == "" || name == "_" {
		return
	}
	p.scopes[len(p.scopes)-1][name] = true
}

// nameDeclared reports whether name is declared in the current scope.
func (p *parser) nameDeclared(name string) bool {
	return p.scopes[len(p.scopes)-1][name]
}

func (p *parser) enterLoop(label string) {
	p.loopDepth++
	if label != "" {
		p.labels = append(p.labels, label)
	}
}

func (p *parser) leaveLoop() {
	if p.loopDepth > 0 {
		p.loopDepth--
	}
	if len(p.labels) > 0 {
		p.labels = p.labels[:len(p.labels)-1]
	}
}

// enterFn increments fn depth and saves then zeroes the loop bookkeeping, so
// that break/continue inside a lambda/spawn cannot refer to an enclosing loop.
func (p *parser) enterFn() {
	p.fnLoopStack = append(p.fnLoopStack, fnLoopState{p.loopDepth, p.labels})
	p.fnDepth++
	p.loopDepth = 0
	p.labels = nil
}

// leaveFn restores the loop bookkeeping saved by enterFn.
func (p *parser) leaveFn() {
	if p.fnDepth > 0 {
		p.fnDepth--
	}
	if n := len(p.fnLoopStack); n > 0 {
		frame := p.fnLoopStack[n-1]
		p.fnLoopStack = p.fnLoopStack[:n-1]
		p.loopDepth = frame.loopDepth
		p.labels = frame.labels
	}
}

// --- local helpers ------------------------------------------------------

// spellTok returns the source spelling of operators, punctuation and
// keywords, and the canonical name for everything else.
func spellTok(t lexer.TokenType) string {
	if lit := t.Literal(); lit != "" {
		return lit
	}
	return t.String()
}

func isStmtSyncKeyword(t lexer.TokenType) bool {
	switch t {
	case lexer.TokenIf, lexer.TokenFor, lexer.TokenMatch, lexer.TokenReturn,
		lexer.TokenBreak, lexer.TokenContinue, lexer.TokenDefer, lexer.TokenSpawn,
		lexer.TokenFn, lexer.TokenStruct, lexer.TokenTrait, lexer.TokenImpl,
		lexer.TokenEnum, lexer.TokenImport, lexer.TokenFrom, lexer.TokenConst,
		lexer.TokenAt:
		return true
	}
	return false
}

func isDeclSyncKeyword(t lexer.TokenType) bool {
	switch t {
	case lexer.TokenFn, lexer.TokenStruct, lexer.TokenTrait, lexer.TokenImpl,
		lexer.TokenEnum, lexer.TokenImport, lexer.TokenFrom, lexer.TokenConst,
		lexer.TokenPub, lexer.TokenAt, lexer.TokenAsync,
		// A top-level variable declaration starts with a plain name, so an
		// identifier at depth 0 is also a valid resynchronisation point.
		lexer.TokenIdent, lexer.TokenUnderscore:
		return true
	}
	return false
}
