package lexer

import "unicode"

// isIdentStart reports whether r may begin an identifier: a Unicode letter or
// an underscore.
func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

// isIdentPart reports whether r may continue an identifier: a Unicode letter,
// a Unicode digit or an underscore.
func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isASCIIDigit reports whether r is one of '0'..'9'.
func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }

// scanIdentOrKeyword scans an identifier, a keyword, a reserved word or the
// lone underscore.
//
// Keyword lookup is suppressed when the previous token is TokenAt, so an
// annotation such as @extern("C") yields TokenAt + TokenIdent even though
// "extern" is a reserved word (design decision D4).
func (l *lexer) scanIdentOrKeyword() {
	start := l.pos()
	for isIdentPart(l.peek()) {
		l.next()
	}
	text := l.src[start.Offset:l.offset]

	if text == "_" {
		l.emit(TokenUnderscore, start, text)
		return
	}
	if l.prevType() == TokenAt {
		l.emit(TokenIdent, start, text)
		return
	}
	if tt, ok := LookupKeyword(text); ok {
		l.emit(tt, start, text)
		return
	}
	l.emit(TokenIdent, start, text)
}

// scanAt scans the "@" that introduces an annotation, an arena block or a
// loop label. The identifier must follow immediately (design decision D4).
func (l *lexer) scanAt(start Position) {
	l.next()
	l.emit(TokenAt, start, "@")
	if !isIdentStart(l.peek()) {
		l.errorHintf(start, 1,
			"annotations and labels are written as '@name' with no space",
			"expected identifier after '@'")
		return
	}
	l.scanIdentOrKeyword()
}
