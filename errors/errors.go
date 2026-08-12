// Package errors provides the diagnostic types shared by every phase of the
// Dot compiler: a located lexical error, a severity classification, an
// accumulating error list with a hard cap, and human-friendly rendering.
//
// This package deliberately does not import the lexer package (that would
// create an import cycle), so source locations are stored as plain
// file/line/column/offset integers rather than a lexer.Position.
package errors

import (
	"fmt"
	"strings"
)

// MaxErrors is the maximum number of errors collected before lexing/parsing
// bails out. Once reached, further diagnostics are dropped and the list is
// marked as truncated.
const MaxErrors = 100

// Severity classifies a diagnostic.
type Severity int

const (
	// SeverityError marks a diagnostic that prevents compilation.
	SeverityError Severity = iota
	// SeverityWarning marks a diagnostic that does not prevent compilation.
	SeverityWarning
)

// String returns the lowercase name of the severity ("error", "warning").
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return fmt.Sprintf("Severity(%d)", int(s))
	}
}

// LexError is a single lexical diagnostic with full source location.
type LexError struct {
	// File is the source file name ("<input>" when unknown).
	File string
	// Line is the 1-based line number of the offending lexeme.
	Line int
	// Column is the 1-based column, counted in runes.
	Column int
	// Offset is the 0-based byte offset into the source.
	Offset int
	// Length is the length of the offending lexeme in bytes (>= 1).
	Length int
	// Message describes the problem, e.g. "unterminated string literal".
	Message string
	// Hint is an optional suggestion, e.g. "use 'and' instead of '&&'".
	// It is empty when there is no hint.
	Hint string
	// Severity classifies the diagnostic.
	Severity Severity
}

// NewLexError builds a LexError with SeverityError. A length below 1 is
// clamped to 1 so that rendering always underlines at least one character.
func NewLexError(file string, line, col, offset, length int, message string) *LexError {
	if length < 1 {
		length = 1
	}
	return &LexError{
		File:     file,
		Line:     line,
		Column:   col,
		Offset:   offset,
		Length:   length,
		Message:  message,
		Severity: SeverityError,
	}
}

// WithHint returns a copy of e with the hint set (fluent helper).
func (e *LexError) WithHint(hint string) *LexError {
	if e == nil {
		return nil
	}
	c := *e
	c.Hint = hint
	return &c
}

// Error implements the error interface.
// Format: "file:line:col: message" (+ " (hint: ...)" when Hint != "").
func (e *LexError) Error() string {
	if e == nil {
		return "<nil error>"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Column, e.Message))
	if e.Hint != "" {
		b.WriteString(" (hint: ")
		b.WriteString(e.Hint)
		b.WriteString(")")
	}
	return b.String()
}

// ErrorList accumulates diagnostics from a compiler phase.
type ErrorList struct {
	// Errors holds the collected diagnostics in the order they were added.
	Errors []error
	// Truncated is true when MaxErrors was reached and further errors were
	// dropped.
	Truncated bool
}

// Add appends err unless MaxErrors has been reached.
// It returns false when the list is full (caller should stop).
// A nil err is ignored and reported as successfully added.
func (l *ErrorList) Add(err error) bool {
	if err == nil {
		return true
	}
	if len(l.Errors) >= MaxErrors {
		l.Truncated = true
		return false
	}
	l.Errors = append(l.Errors, err)
	return true
}

// Len returns the number of collected errors.
func (l *ErrorList) Len() int {
	if l == nil {
		return 0
	}
	return len(l.Errors)
}

// Err returns l if it contains at least one error, otherwise nil.
// Always use this when returning an *ErrorList as an error.
func (l *ErrorList) Err() error {
	if l == nil || len(l.Errors) == 0 {
		return nil
	}
	return l
}

// Diagnostic is the shared diagnostic type for every compiler phase.
// It is an ALIAS of LexError, so *LexError and *Diagnostic are the same type
// and FormatAll/FormatLexError keep working unchanged (D26).
type Diagnostic = LexError

// NewParseError builds a Diagnostic with SeverityError. Identical to
// NewLexError; it exists so parser call sites read correctly.
func NewParseError(file string, line, col, offset, length int, message string) *Diagnostic {
	return NewLexError(file, line, col, offset, length, message)
}

// NewTypeError builds a Diagnostic with SeverityError. Identical to
// NewLexError; it exists so checker call sites read correctly.
func NewTypeError(file string, line, col, offset, length int, message string) *Diagnostic {
	return NewLexError(file, line, col, offset, length, message)
}

// NewTypeWarning builds a Diagnostic with SeverityWarning (unreachable match
// arms, unused-result style lints).
func NewTypeWarning(file string, line, col, offset, length int, message string) *Diagnostic {
	if length < 1 {
		length = 1
	}
	return &Diagnostic{
		File:     file,
		Line:     line,
		Column:   col,
		Offset:   offset,
		Length:   length,
		Message:  message,
		Severity: SeverityWarning,
	}
}

// Error implements the error interface: a newline-joined list of all messages.
func (l *ErrorList) Error() string {
	if l == nil || len(l.Errors) == 0 {
		return "no errors"
	}
	msgs := make([]string, 0, len(l.Errors)+1)
	for _, err := range l.Errors {
		msgs = append(msgs, err.Error())
	}
	if l.Truncated {
		msgs = append(msgs, fmt.Sprintf("too many errors (stopped after %d)", MaxErrors))
	}
	return strings.Join(msgs, "\n")
}
