package lexer

import (
	"strings"
	"unicode/utf8"
)

// stringBuf accumulates the decoded text of one literal chunk of a string
// literal together with the source position of its first character.
type stringBuf struct {
	b    strings.Builder
	pos  Position
	open bool
}

// add appends r to the current literal chunk, remembering at as the position
// of the chunk when it is the first character written.
func (s *stringBuf) add(r rune, at Position) {
	if !s.open {
		s.pos = at
		s.open = true
	}
	s.b.WriteRune(r)
}

// flush appends the accumulated chunk to parts and resets the buffer.
func (s *stringBuf) flush(parts *[]StringPart) {
	if !s.open {
		return
	}
	*parts = append(*parts, StringPart{
		Kind:  StringPartLiteral,
		Value: s.b.String(),
		Pos:   s.pos,
	})
	s.b.Reset()
	s.open = false
}

// scanString scans a double-quoted string literal. Every Dot string is
// interpolated, so the literal is split into alternating StringPartLiteral
// chunks (with escapes already decoded) and StringPartExpr chunks holding the
// raw source of each "{ ... }" expression (design decision D1). Exactly one
// TokenString is emitted, whose Value is the concatenation of the literal
// chunks.
//
// "{{" and "}}" are the escapes for literal braces. A string that is not
// closed before the end of the line or the end of the file is reported and
// then emitted with the parts collected so far.
func (l *lexer) scanString() {
	start := l.pos()
	l.next() // opening quote

	var parts []StringPart
	var lit stringBuf
	terminated := false

loop:
	for {
		at := l.pos()
		switch c := l.peek(); {
		case c == eof || c == '\n':
			break loop
		case c == '"':
			l.next()
			terminated = true
			break loop
		case c == '{' && l.peekAt(1) == '{':
			l.next()
			l.next()
			lit.add('{', at)
		case c == '}' && l.peekAt(1) == '}':
			l.next()
			l.next()
			lit.add('}', at)
		case c == '{':
			lit.flush(&parts)
			l.scanInterpolation(&parts)
		case c == '}':
			l.errorHintf(at, 1, "use '}}' for a literal '}'",
				"unmatched '}' in string literal")
			l.next()
			lit.add('}', at)
		case c == '\\':
			if r, ok := l.decodeEscape(); ok {
				lit.add(r, at)
			}
		default:
			l.next()
			lit.add(c, at)
		}
	}
	lit.flush(&parts)

	if !terminated {
		l.errorHintf(start, 1, "strings must be closed on the same line",
			"unterminated string literal")
	}
	l.emitString(TokenString, start, parts)
}

// scanRawString scans a backtick string literal: no escapes, no
// interpolation, and newlines are allowed inside (design decision D1). One
// TokenRawString with a single literal part is emitted.
func (l *lexer) scanRawString() {
	start := l.pos()
	l.next() // opening backtick
	contentStart := l.pos()

	for l.peek() != '`' {
		if l.atEOF() {
			l.errorHintf(start, 1, "raw strings are closed with a backtick",
				"unterminated raw string literal")
			l.emitRaw(start, contentStart, l.src[contentStart.Offset:l.offset])
			return
		}
		l.next()
	}
	text := l.src[contentStart.Offset:l.offset]
	l.next() // closing backtick
	l.emitRaw(start, contentStart, text)
}

// emitRaw emits a TokenRawString whose single literal part is text.
func (l *lexer) emitRaw(start, contentStart Position, text string) {
	var parts []StringPart
	if text != "" {
		parts = []StringPart{{Kind: StringPartLiteral, Value: text, Pos: contentStart}}
	}
	l.emitString(TokenRawString, start, parts)
}

// emitString builds and appends a string token from its parts. Value is the
// concatenation of the literal parts, which is the whole text whenever the
// literal contains no interpolation.
func (l *lexer) emitString(tt TokenType, start Position, parts []StringPart) {
	var value strings.Builder
	for _, p := range parts {
		if p.Kind == StringPartLiteral {
			value.WriteString(p.Value)
		}
	}
	l.emitToken(Token{
		Type:   tt,
		Lexeme: l.src[start.Offset:l.offset],
		Value:  value.String(),
		Parts:  parts,
		Pos:    start,
		End:    l.pos(),
	})
}

// scanInterpolation scans a "{ ... }" interpolation, with the cursor sitting
// on the opening brace, and appends a StringPartExpr holding the raw source
// between the braces. Nested braces are matched and braces inside nested
// string literals are ignored.
func (l *lexer) scanInterpolation(parts *[]StringPart) {
	open := l.pos()
	l.next() // '{'
	exprStart := l.pos()
	depth := 1

	for {
		switch c := l.peek(); {
		case c == eof:
			l.errorHintf(open, 1, "close the interpolation with '}'",
				"unterminated interpolation in string literal")
			l.addExprPart(parts, l.src[exprStart.Offset:l.offset], exprStart, open)
			return
		case c == '"' || c == '`':
			l.skipNestedString(c)
		case c == '{':
			depth++
			l.next()
		case c == '}':
			depth--
			if depth == 0 {
				text := l.src[exprStart.Offset:l.offset]
				l.next() // '}'
				l.addExprPart(parts, text, exprStart, open)
				return
			}
			l.next()
		default:
			l.next()
		}
	}
}

// addExprPart trims the raw interpolation source, adjusts its position to the
// first non-blank character and appends it as a StringPartExpr. An
// interpolation with no expression is reported instead.
func (l *lexer) addExprPart(parts *[]StringPart, raw string, at, open Position) {
	trimmed := strings.TrimLeft(raw, " \t\r\n")
	if strings.TrimRight(trimmed, " \t\r\n") == "" {
		l.errorHintf(open, 2, "write an expression, as in \"value: {x}\"",
			"empty interpolation expression")
		return
	}
	pos := advancePosition(at, raw[:len(raw)-len(trimmed)])
	*parts = append(*parts, StringPart{
		Kind:  StringPartExpr,
		Value: strings.TrimRight(trimmed, " \t\r\n"),
		Pos:   pos,
	})
}

// skipNestedString consumes a string literal nested inside an interpolation
// so that braces within it do not affect brace matching. Double-quoted
// strings honour backslash escapes and end at a newline; raw strings may span
// lines.
func (l *lexer) skipNestedString(quote rune) {
	l.next() // opening quote
	for {
		c := l.peek()
		switch {
		case c == eof:
			return
		case quote == '"' && c == '\n':
			return
		case quote == '"' && c == '\\':
			l.next()
			l.next()
			continue
		case c == quote:
			l.next()
			return
		}
		l.next()
	}
}

// decodeEscape decodes the escape sequence at the cursor, which must sit on a
// backslash, and returns the rune it stands for. The second result is false
// when nothing should be written (the escape was cut short by the end of the
// file). An unknown escape is reported and decodes to the escaped character
// itself so that scanning can continue.
func (l *lexer) decodeEscape() (rune, bool) {
	start := l.pos()
	l.next() // backslash
	if l.atEOF() {
		l.errorHintf(start, 1, "escape sequences are written as '\\n', '\\t', ...",
			"unterminated escape sequence")
		return 0, false
	}

	switch c := l.next(); c {
	case 'n':
		return '\n', true
	case 't':
		return '\t', true
	case 'r':
		return '\r', true
	case '0':
		return 0, true
	case '\\', '"', '`', '\'':
		return c, true
	case 'x':
		return l.decodeHexEscape(start)
	case 'u':
		return l.decodeUnicodeEscape(start)
	default:
		l.errorHintf(start, l.offset-start.Offset,
			"valid escapes are \\n \\t \\r \\0 \\\\ \\\" \\` \\xNN \\u{...}",
			"unknown escape sequence '\\%c'", c)
		return c, true
	}
}

// decodeHexEscape decodes the two hexadecimal digits of a "\xNN" escape.
func (l *lexer) decodeHexEscape(start Position) (rune, bool) {
	value := rune(0)
	for i := 0; i < 2; i++ {
		d, ok := hexValue(l.peek())
		if !ok {
			l.errorHintf(start, l.offset-start.Offset,
				"write exactly two hexadecimal digits, e.g. \\x41",
				"invalid '\\x' escape")
			return 0, false
		}
		l.next()
		value = value*16 + rune(d)
	}
	return value, true
}

// decodeUnicodeEscape decodes a "\u{1F600}" escape into its code point.
func (l *lexer) decodeUnicodeEscape(start Position) (rune, bool) {
	const hint = "write the code point in braces, e.g. \\u{1F600}"
	if l.peek() != '{' {
		l.errorHintf(start, l.offset-start.Offset, hint, "invalid '\\u' escape")
		return 0, false
	}
	l.next()

	value := rune(0)
	digits := 0
	for {
		d, ok := hexValue(l.peek())
		if !ok {
			break
		}
		l.next()
		digits++
		if digits <= 6 {
			value = value*16 + rune(d)
		}
	}
	if digits == 0 || digits > 6 || l.peek() != '}' {
		if l.peek() == '}' {
			l.next()
		}
		l.errorHintf(start, l.offset-start.Offset, hint, "invalid '\\u' escape")
		return 0, false
	}
	l.next() // '}'
	if value > utf8.MaxRune || (value >= 0xD800 && value <= 0xDFFF) {
		l.errorHintf(start, l.offset-start.Offset,
			"code points range from 0 to 10FFFF and exclude D800-DFFF",
			"invalid Unicode code point in '\\u' escape")
		return 0, false
	}
	return value, true
}

// hexValue returns the numeric value of a hexadecimal digit.
func hexValue(r rune) (int, bool) {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0'), true
	case r >= 'a' && r <= 'f':
		return int(r-'a') + 10, true
	case r >= 'A' && r <= 'F':
		return int(r-'A') + 10, true
	}
	return 0, false
}
