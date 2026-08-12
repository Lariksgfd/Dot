package lexer

import (
	"fmt"
	"strings"
)

// StringPartKind distinguishes the two kinds of chunks in a string literal.
type StringPartKind int

const (
	// StringPartLiteral is a plain text chunk with escapes already decoded.
	StringPartLiteral StringPartKind = iota
	// StringPartExpr is the raw source text of a "{ ... }" interpolation,
	// with the braces stripped and surrounding whitespace trimmed.
	StringPartExpr
)

// String returns the name of the kind ("StringPartLiteral" or
// "StringPartExpr"), used in diagnostics and tests.
func (k StringPartKind) String() string {
	switch k {
	case StringPartLiteral:
		return "StringPartLiteral"
	case StringPartExpr:
		return "StringPartExpr"
	default:
		return fmt.Sprintf("StringPartKind(%d)", int(k))
	}
}

// StringPart is one chunk of an interpolated string literal.
type StringPart struct {
	// Kind tells whether Value is decoded literal text or expression source.
	Kind StringPartKind
	// Value is the decoded text (literal) or the raw expression source (expr).
	Value string
	// Pos is the position of the first character of Value in the source file.
	Pos Position
}

// Token is a single lexical unit produced by the lexer.
type Token struct {
	// Type is the lexical class of the token.
	Type TokenType
	// Lexeme is the exact source text of the token (raw, undecoded).
	Lexeme string
	// Value is the normalized value: digits without '_' separators for
	// numbers, decoded text for strings, Lexeme otherwise.
	Value string
	// Base is 10, 16, 2 or 8 for TokenInt; 10 for TokenFloat; 0 otherwise.
	Base int
	// Suffix is the numeric unit suffix, e.g. "mb" in 1_mb; "" otherwise.
	Suffix string
	// Parts is non-nil only for TokenString and TokenRawString.
	Parts []StringPart
	// Pos is the position of the first character of the token.
	Pos Position
	// End is the position of the character just past the token.
	End Position
}

// String returns a compact debug representation, for example
// "TokenIdent(foo) at main.dot:1:1".
func (t Token) String() string {
	var b strings.Builder
	b.WriteString(t.Type.String())
	if text := t.debugText(); text != "" {
		b.WriteString("(")
		b.WriteString(text)
		b.WriteString(")")
	}
	b.WriteString(" at ")
	b.WriteString(t.Pos.String())
	return b.String()
}

// debugText picks the most informative text to show for the token: the
// normalized value when there is one, otherwise the raw lexeme. Tokens whose
// spelling is implied by their type (operators, punctuation, keywords, EOF,
// newline) render without any text.
func (t Token) debugText() string {
	switch t.Type {
	case TokenEOF, TokenNewline:
		return ""
	}
	if t.Type.Literal() != "" {
		return ""
	}
	if t.Value != "" {
		return escapeDebug(t.Value)
	}
	return escapeDebug(t.Lexeme)
}

// escapeDebug makes a token value safe to print on a single line.
func escapeDebug(s string) string {
	if !strings.ContainsAny(s, "\n\r\t") {
		return s
	}
	r := strings.NewReplacer("\n", `\n`, "\r", `\r`, "\t", `\t`)
	return r.Replace(s)
}
