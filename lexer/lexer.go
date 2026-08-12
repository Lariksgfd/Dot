// Package lexer turns Dot source text into a stream of tokens.
//
// The entry point is Tokenize. Scanning never stops at the first problem:
// every diagnostic is collected in an *errors.ErrorList and the scanner
// recovers, so a single run reports as many errors as possible.
package lexer

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dotlang/dot/errors"
)

// eof is the sentinel rune returned by the cursor helpers past the end of
// the source. It can never appear in valid UTF-8 text.
const eof = rune(-1)

// defaultFilename is used when Tokenize is called without a file name.
const defaultFilename = "<input>"

// lexer holds the scanning state for one source file.
type lexer struct {
	src  string // the whole source text
	file string // file name reported in positions and diagnostics

	offset int // current byte offset into src
	line   int // current 1-based line
	col    int // current 1-based column, counted in runes

	tokens   []Token           // tokens emitted so far
	errs     *errors.ErrorList // collected diagnostics
	brackets []TokenType       // stack of currently open (, [ and {
	halted   bool              // set once MaxErrors diagnostics were reported
}

// Tokenize splits source into a sequence of tokens.
//
// The returned slice is always non-nil and always terminated by a TokenNewline
// followed by a TokenEOF, even when errors occurred: the lexer recovers and
// keeps scanning so that as many diagnostics as possible are reported at once.
//
// The returned error is nil on success, otherwise a *errors.ErrorList whose
// elements are all *errors.LexError.
func Tokenize(source string, filename string) ([]Token, error) {
	if filename == "" {
		filename = defaultFilename
	}
	l := &lexer{
		src:    source,
		file:   filename,
		line:   1,
		col:    1,
		tokens: make([]Token, 0, len(source)/4+8),
		errs:   &errors.ErrorList{},
	}
	l.skipBOM()
	l.run()
	return filterNewlines(l.tokens), l.errs.Err()
}

// skipBOM consumes a leading UTF-8 byte order mark, if present.
func (l *lexer) skipBOM() {
	const bom = "\ufeff"
	if strings.HasPrefix(l.src, bom) {
		l.offset += len(bom)
	}
}

// run is the main scan loop. It fills l.tokens and always ends with TokenEOF.
func (l *lexer) run() {
	for !l.halted {
		l.skipWhitespace()
		if l.atEOF() {
			break
		}
		before := l.offset
		l.scanToken()
		if l.offset == before {
			// Defensive: no scanner may ever stand still.
			l.next()
		}
	}
	l.emit(TokenEOF, l.pos(), "")
}

// scanToken dispatches on the next rune and scans exactly one token.
func (l *lexer) scanToken() {
	start := l.pos()
	c := l.peek()
	switch {
	case c == '\n':
		l.next()
		l.emitNewline(start, "\n")
	case c == '/' && l.peekAt(1) == '/':
		l.scanLineComment()
	case c == '/' && l.peekAt(1) == '*':
		l.scanBlockComment()
	case isIdentStart(c):
		l.scanIdentOrKeyword()
	case isASCIIDigit(c):
		l.scanNumber(l.prevType() == TokenDot)
	case c == '"':
		l.scanString()
	case c == '`':
		l.scanRawString()
	default:
		if l.scanOperator() {
			return
		}
		l.next()
		l.errorf(start, l.offset-start.Offset, "unexpected character %q", c)
		l.emit(TokenIllegal, start, l.src[start.Offset:l.offset])
	}
}

// ---------------------------------------------------------------------------
// cursor helpers
// ---------------------------------------------------------------------------

// atEOF reports whether the cursor sits past the last byte of the source.
func (l *lexer) atEOF() bool { return l.offset >= len(l.src) }

// peek returns the rune at the cursor without consuming it.
func (l *lexer) peek() rune { return l.peekAt(0) }

// peekAt returns the rune n runes ahead of the cursor without consuming
// anything. It returns eof when the lookahead runs past the end of the source.
func (l *lexer) peekAt(n int) rune {
	off := l.offset
	for i := 0; i < n; i++ {
		if off >= len(l.src) {
			return eof
		}
		_, size := utf8.DecodeRuneInString(l.src[off:])
		off += size
	}
	if off >= len(l.src) {
		return eof
	}
	r, _ := utf8.DecodeRuneInString(l.src[off:])
	return r
}

// next consumes and returns the rune at the cursor, updating line and column.
func (l *lexer) next() rune {
	if l.offset >= len(l.src) {
		return eof
	}
	r, size := utf8.DecodeRuneInString(l.src[l.offset:])
	l.offset += size
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

// match consumes s and reports true when it is the exact text at the cursor.
// s must be plain ASCII without newlines (all fixed operator spellings are).
func (l *lexer) match(s string) bool {
	if !strings.HasPrefix(l.src[l.offset:], s) {
		return false
	}
	l.offset += len(s)
	l.col += len(s)
	return true
}

// pos returns the position of the cursor.
func (l *lexer) pos() Position {
	return Position{File: l.file, Line: l.line, Column: l.col, Offset: l.offset}
}

// advancePosition returns the position reached after reading text starting
// at p. It is used to locate substrings (such as interpolated expressions)
// that were scanned as one blob.
func advancePosition(p Position, text string) Position {
	for _, r := range text {
		p.Offset += utf8.RuneLen(r)
		if r == '\n' {
			p.Line++
			p.Column = 1
		} else {
			p.Column++
		}
	}
	return p
}

// ---------------------------------------------------------------------------
// emitting
// ---------------------------------------------------------------------------

// emit appends a token whose Value equals its Lexeme, spanning from start to
// the current cursor position.
func (l *lexer) emit(t TokenType, start Position, lexeme string) {
	l.emitToken(Token{Type: t, Lexeme: lexeme, Value: lexeme, Pos: start, End: l.pos()})
}

// emitToken appends a fully built token and maintains the bracket stack.
func (l *lexer) emitToken(tok Token) {
	switch tok.Type {
	case TokenLParen, TokenLBracket, TokenLBrace:
		l.brackets = append(l.brackets, tok.Type)
	case TokenRParen, TokenRBracket, TokenRBrace:
		if n := len(l.brackets); n > 0 {
			l.brackets = l.brackets[:n-1]
		}
	}
	l.tokens = append(l.tokens, tok)
}

// emitNewline appends a TokenNewline unless the innermost open bracket is a
// "(" or "[", where line breaks are always continuations (rule D2.1).
func (l *lexer) emitNewline(start Position, lexeme string) {
	if l.inGrouping() {
		return
	}
	l.emitToken(Token{Type: TokenNewline, Lexeme: lexeme, Value: lexeme, Pos: start, End: l.pos()})
}

// inGrouping reports whether the innermost open bracket is "(" or "[".
func (l *lexer) inGrouping() bool {
	n := len(l.brackets)
	if n == 0 {
		return false
	}
	switch l.brackets[n-1] {
	case TokenLParen, TokenLBracket:
		return true
	}
	return false
}

// prevType returns the type of the last emitted token, or TokenIllegal when
// nothing has been emitted yet.
func (l *lexer) prevType() TokenType {
	if len(l.tokens) == 0 {
		return TokenIllegal
	}
	return l.tokens[len(l.tokens)-1].Type
}

// ---------------------------------------------------------------------------
// diagnostics
// ---------------------------------------------------------------------------

// errorf records a diagnostic covering length bytes starting at start.
func (l *lexer) errorf(start Position, length int, format string, args ...any) {
	l.errorHintf(start, length, "", format, args...)
}

// errorHintf records a diagnostic with an optional hint. Scanning halts once
// errors.MaxErrors diagnostics have been collected.
func (l *lexer) errorHintf(start Position, length int, hint string, format string, args ...any) {
	err := errors.NewLexError(start.File, start.Line, start.Column, start.Offset,
		length, fmt.Sprintf(format, args...))
	if hint != "" {
		err = err.WithHint(hint)
	}
	if !l.errs.Add(err) {
		l.halted = true
	}
}

// ---------------------------------------------------------------------------
// whitespace and comments
// ---------------------------------------------------------------------------

// skipWhitespace consumes blanks, but not newlines (handled by scanToken).
func (l *lexer) skipWhitespace() {
	for !l.halted {
		switch c := l.peek(); {
		case c == ' ' || c == '\t' || c == '\r' || c == '\v' || c == '\f':
			l.next()
		default:
			return
		}
	}
}

// scanLineComment consumes "// ..." up to but not including the newline,
// which stays in the stream as a statement terminator (rule D2.7).
func (l *lexer) scanLineComment() {
	start := l.pos()
	for !l.atEOF() && l.peek() != '\n' {
		l.next()
	}
	l.emit(TokenComment, start, l.src[start.Offset:l.offset])
}

// scanBlockComment consumes a "/* ... */" comment. Block comments nest. A
// comment spanning several lines leaves one newline token behind, because it
// is a line break in the token stream (rule D2.7).
func (l *lexer) scanBlockComment() {
	start := l.pos()
	l.next()
	l.next()
	depth := 1
	for depth > 0 {
		if l.atEOF() {
			l.errorHintf(start, 2, "block comments are closed with '*/'",
				"unterminated block comment")
			return
		}
		switch {
		case l.peek() == '/' && l.peekAt(1) == '*':
			l.next()
			l.next()
			depth++
		case l.peek() == '*' && l.peekAt(1) == '/':
			l.next()
			l.next()
			depth--
		default:
			l.next()
		}
	}
	l.emit(TokenComment, start, l.src[start.Offset:l.offset])
	if l.line > start.Line {
		l.emitNewline(l.pos(), "\n")
	}
}
