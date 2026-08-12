package lexer

import "strings"

// baseName returns the human readable name of a numeric base, used in
// diagnostics ("hexadecimal", "binary", "octal", "decimal").
func baseName(base int) string {
	switch base {
	case 16:
		return "hexadecimal"
	case 8:
		return "octal"
	case 2:
		return "binary"
	default:
		return "decimal"
	}
}

// baseDigits describes the digits accepted by a base, for use in hints.
func baseDigits(base int) string {
	switch base {
	case 2:
		return "0 and 1"
	case 8:
		return "0-7"
	case 16:
		return "0-9, a-f and A-F"
	default:
		return "0-9"
	}
}

// isBaseDigit reports whether r is a valid digit in the given base.
func isBaseDigit(r rune, base int) bool {
	switch base {
	case 2:
		return r == '0' || r == '1'
	case 8:
		return r >= '0' && r <= '7'
	case 16:
		return isASCIIDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
	default:
		return isASCIIDigit(r)
	}
}

// basePrefix maps the letter following a leading "0" to its base, returning
// 0 when the letter does not introduce a base prefix.
func basePrefix(r rune) int {
	switch r {
	case 'x', 'X':
		return 16
	case 'b', 'B':
		return 2
	case 'o', 'O':
		return 8
	}
	return 0
}

// scanNumber scans an integer or floating point literal starting at the
// cursor, which must sit on an ASCII digit.
//
// Value holds the literal with every "_" separator removed and, for prefixed
// literals, without the "0x"/"0b"/"0o" prefix (design decision D3); Lexeme
// keeps the exact source text. Base is 16, 2, 8 or 10, and Suffix carries a
// trailing unit such as the "mb" of 1_mb.
//
// When afterDot is true the previous token was a TokenDot, so the literal is
// a tuple index (t.0.1) and neither a fraction nor an exponent is consumed.
// Malformed literals are reported and then emitted with a best-effort value
// so that scanning can continue (design decision D6).
func (l *lexer) scanNumber(afterDot bool) {
	start := l.pos()
	base := 10
	isFloat := false
	var digits strings.Builder

	if l.peek() == '0' && basePrefix(l.peekAt(1)) != 0 {
		base = basePrefix(l.peekAt(1))
		l.next()
		l.next()
		if l.scanDigits(&digits, base) == 0 {
			l.errorHintf(start, l.offset-start.Offset,
				"write the digits directly after the base prefix, e.g. 0xFF",
				"%s literal has no digits", baseName(base))
			digits.WriteByte('0')
		}
	} else {
		l.scanDigits(&digits, base)
		if !afterDot {
			isFloat = l.scanFraction(&digits, start)
			if l.scanExponent(&digits, start) {
				isFloat = true
			}
		}
	}

	l.checkStrayDigits(base)
	suffix := l.scanNumberSuffix(base)

	tt := TokenInt
	if isFloat {
		tt = TokenFloat
	}
	l.emitToken(Token{
		Type:   tt,
		Lexeme: l.src[start.Offset:l.offset],
		Value:  digits.String(),
		Base:   base,
		Suffix: suffix,
		Pos:    start,
		End:    l.pos(),
	})
}

// scanFraction consumes a decimal point and the digits after it, but only
// when the "." is immediately followed by an ASCII digit, so that "0..10"
// stays a range and "x.0" stays a field access (design decision D3). It
// reports whether a fraction was consumed.
func (l *lexer) scanFraction(b *strings.Builder, start Position) bool {
	if l.peek() != '.' || !isASCIIDigit(l.peekAt(1)) {
		return false
	}
	l.next()
	b.WriteByte('.')
	l.scanDigits(b, 10)
	for l.peek() == '.' && isASCIIDigit(l.peekAt(1)) {
		l.next()
		l.errorHintf(start, l.offset-start.Offset,
			"a float has at most one decimal point",
			"malformed number: more than one decimal point")
		var extra strings.Builder
		l.scanDigits(&extra, 10)
	}
	return true
}

// scanDigits consumes a run of digits valid in base, writing them to b and
// dropping "_" separators. It returns the number of digits written.
//
// A "_" separates two digits; a "_" followed by a letter starts a unit suffix
// and is left for scanNumberSuffix. Leading, doubled and trailing separators
// are reported and skipped.
func (l *lexer) scanDigits(b *strings.Builder, base int) int {
	count := 0
	for {
		r := l.peek()
		if isBaseDigit(r, base) {
			b.WriteRune(r)
			l.next()
			count++
			continue
		}
		if r != '_' {
			return count
		}

		switch nxt := l.peekAt(1); {
		case nxt == '_':
			l.errorHintf(l.pos(), 2, "use a single '_' between digits",
				"digit separator '_' repeated")
			for l.peek() == '_' {
				l.next()
			}
		case isBaseDigit(nxt, base):
			if count == 0 {
				l.errorHintf(l.pos(), 1, "a number may not start with '_'",
					"digit separator '_' must appear between digits")
			}
			l.next()
		case isIdentStart(nxt):
			return count // "1_mb": the underscore introduces a unit suffix
		default:
			l.errorHintf(l.pos(), 1, "a number may not end with '_'",
				"digit separator '_' must appear between digits")
			l.next()
			return count
		}
	}
}

// scanExponent consumes an "e"/"E" exponent with an optional sign, appending
// it to b, and reports whether the literal is therefore a float. An "e"
// followed by identifier characters rather than digits is left alone: it
// starts a unit suffix such as the "em" of 2em.
func (l *lexer) scanExponent(b *strings.Builder, start Position) bool {
	if r := l.peek(); r != 'e' && r != 'E' {
		return false
	}
	sign := l.peekAt(1)
	signed := sign == '+' || sign == '-'

	switch {
	case isASCIIDigit(sign):
	case signed && isASCIIDigit(l.peekAt(2)):
	case signed:
		l.next()
		l.next()
		l.emptyExponent(b, start)
		return true
	case isIdentPart(sign):
		return false
	default:
		l.next()
		l.emptyExponent(b, start)
		return true
	}

	b.WriteByte('e')
	l.next()
	if signed {
		b.WriteRune(sign)
		l.next()
	}
	l.scanDigits(b, 10)
	return true
}

// emptyExponent reports an exponent without digits and substitutes "e0" so
// the emitted token still carries a parsable value.
func (l *lexer) emptyExponent(b *strings.Builder, start Position) {
	l.errorHintf(start, l.offset-start.Offset,
		"write at least one digit after the exponent, e.g. 1e10",
		"exponent has no digits")
	b.WriteString("e0")
}

// checkStrayDigits reports digits that follow the literal but are not valid
// in its base, for example the "2" of 0b12. The offending run is consumed so
// that scanning resumes cleanly.
func (l *lexer) checkStrayDigits(base int) {
	if base == 16 || !isASCIIDigit(l.peek()) {
		return
	}
	bad := l.pos()
	for isASCIIDigit(l.peek()) {
		l.next()
	}
	l.errorHintf(bad, l.offset-bad.Offset,
		"valid digits for a "+baseName(base)+" literal are "+baseDigits(base),
		"invalid digit for a %s literal", baseName(base))
}

// scanNumberSuffix consumes a unit suffix directly attached to a numeric
// literal ("10kb", "1_mb") and returns it without the optional separating
// underscore. It returns "" when there is no suffix.
func (l *lexer) scanNumberSuffix(base int) string {
	if l.peek() == '_' && isIdentStart(l.peekAt(1)) {
		l.next()
	}
	if !isIdentStart(l.peek()) {
		return ""
	}
	from := l.offset
	for isIdentPart(l.peek()) {
		l.next()
	}
	return l.src[from:l.offset]
}
