// Package lexer turns Dot source text into a stream of tokens.
package lexer

import "fmt"

// Position identifies a single location in a source file.
type Position struct {
	// File is the file name as passed to Tokenize.
	File string
	// Line is the 1-based line number.
	Line int
	// Column is the 1-based column, counted in runes (tabs count as 1).
	Column int
	// Offset is the 0-based byte offset into the source string.
	Offset int
}

// String returns "file:line:column". An invalid position renders as
// "file:-" (or "-" when the file name is empty too).
func (p Position) String() string {
	if !p.IsValid() {
		if p.File == "" {
			return "-"
		}
		return p.File + ":-"
	}
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Column)
}

// IsValid reports whether the position refers to a real location (Line > 0).
func (p Position) IsValid() bool {
	return p.Line > 0
}
