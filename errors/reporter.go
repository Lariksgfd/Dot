package errors

import (
	"fmt"
	"strings"
)

// FormatLexError renders a single diagnostic with the offending source line
// and a caret underline:
//
//	error: unterminated string literal at main.dot:5:12
//	  |
//	5 |     x = "hello" + 42
//	  |                   ^^
//	  | strings must be closed on the same line
//
// The gutter width adapts to the width of the line number, the caret run is
// e.Length characters long (at least one), and the hint line is omitted when
// e.Hint is empty. When the source line cannot be found, only the header line
// is returned.
func FormatLexError(e *LexError, source string) string {
	if e == nil {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s at %s:%d:%d", e.Severity, e.Message, e.File, e.Line, e.Column)

	line, ok := sourceLine(source, e.Line)
	if !ok {
		if e.Hint != "" {
			b.WriteString("\n  = hint: ")
			b.WriteString(e.Hint)
		}
		return b.String()
	}

	num := fmt.Sprintf("%d", e.Line)
	gutter := strings.Repeat(" ", len(num))

	b.WriteString("\n")
	b.WriteString(gutter)
	b.WriteString(" |")
	b.WriteString("\n")
	b.WriteString(num)
	b.WriteString(" | ")
	b.WriteString(line)
	b.WriteString("\n")
	b.WriteString(gutter)
	b.WriteString(" | ")
	b.WriteString(caretPad(line, e.Column))
	b.WriteString(strings.Repeat("^", caretWidth(e.Length)))
	if e.Hint != "" {
		b.WriteString("\n")
		b.WriteString(gutter)
		b.WriteString(" | ")
		b.WriteString(e.Hint)
	}
	return b.String()
}

// FormatDiagnostic is the phase-neutral name for FormatLexError.
func FormatDiagnostic(d *Diagnostic, source string) string {
	return FormatLexError(d, source)
}

// FormatAll renders every diagnostic in list, separated by blank lines,
// followed by a summary line ("3 errors"). Diagnostics that are not
// *LexError values are rendered with their Error method.
func FormatAll(list *ErrorList, source string) string {
	if list == nil || len(list.Errors) == 0 {
		return ""
	}

	blocks := make([]string, 0, len(list.Errors)+2)
	for _, err := range list.Errors {
		if le, ok := err.(*LexError); ok {
			blocks = append(blocks, FormatLexError(le, source))
			continue
		}
		blocks = append(blocks, fmt.Sprintf("%s: %s", SeverityError, err.Error()))
	}
	if list.Truncated {
		blocks = append(blocks, fmt.Sprintf("%s: too many errors (stopped after %d)",
			SeverityError, MaxErrors))
	}
	blocks = append(blocks, summary(len(list.Errors)))
	return strings.Join(blocks, "\n\n")
}

// summary returns "1 error" or "N errors".
func summary(n int) string {
	if n == 1 {
		return "1 error"
	}
	return fmt.Sprintf("%d errors", n)
}

// caretWidth clamps a lexeme length to a printable caret run length.
func caretWidth(length int) int {
	if length < 1 {
		return 1
	}
	return length
}

// sourceLine returns the text of the 1-based line n of source, without its
// trailing newline. The second result is false when the line does not exist.
func sourceLine(source string, n int) (string, bool) {
	if n < 1 || source == "" {
		return "", false
	}
	start := 0
	cur := 1
	for i := 0; i < len(source); i++ {
		if source[i] != '\n' {
			continue
		}
		if cur == n {
			return strings.TrimSuffix(source[start:i], "\r"), true
		}
		cur++
		start = i + 1
	}
	if cur == n {
		return strings.TrimSuffix(source[start:], "\r"), true
	}
	return "", false
}

// caretPad builds the whitespace prefix that aligns a caret under the 1-based
// rune column col of line. Tabs in the source are preserved so that the caret
// stays aligned regardless of the reader's tab width.
func caretPad(line string, col int) string {
	if col < 2 {
		return ""
	}
	var b strings.Builder
	i := 1
	for _, r := range line {
		if i >= col {
			break
		}
		if r == '\t' {
			b.WriteRune('\t')
		} else {
			b.WriteRune(' ')
		}
		i++
	}
	for ; i < col; i++ {
		b.WriteRune(' ')
	}
	return b.String()
}
