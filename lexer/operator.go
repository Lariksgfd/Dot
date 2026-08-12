package lexer

// operatorSpelling pairs a fixed source spelling with the token type it
// produces.
type operatorSpelling struct {
	text string
	tt   TokenType
}

// multiOperators lists the operator spellings of two or more characters,
// longest first, so that a linear scan implements maximal munch: "<<=" wins
// over "<<", which wins over "<" (design decision D3). They are matched
// before the rejected spellings so that "!=" is not mistaken for "!".
var multiOperators = []operatorSpelling{
	{"<<=", TokenShlAssign},
	{">>=", TokenShrAssign},
	{"...", TokenEllipsis},
	{"..=", TokenDotDotEq},

	{"**", TokenStarStar},
	{"|>", TokenPipeArrow},
	{"->", TokenArrow},
	{"=>", TokenFatArrow},
	{"==", TokenEqEq},
	{"!=", TokenBangEq},
	{"<=", TokenLtEq},
	{">=", TokenGtEq},
	{"<<", TokenShl},
	{">>", TokenShr},
	{"+=", TokenPlusAssign},
	{"-=", TokenMinusAssign},
	{"*=", TokenStarAssign},
	{"/=", TokenSlashAssign},
	{"%=", TokenPercentAssign},
	{"&=", TokenAmpAssign},
	{"|=", TokenPipeAssign},
	{"^=", TokenCaretAssign},
	{"..", TokenDotDot},
}

// singleOperators lists the one-character operator and punctuation
// spellings, tried only after multiOperators and the rejected spellings.
var singleOperators = []operatorSpelling{
	{"+", TokenPlus},
	{"-", TokenMinus},
	{"*", TokenStar},
	{"/", TokenSlash},
	{"%", TokenPercent},
	{"=", TokenAssign},
	{"<", TokenLt},
	{">", TokenGt},
	{"&", TokenAmp},
	{"|", TokenPipe},
	{"^", TokenCaret},
	{"~", TokenTilde},
	{"?", TokenQuestion},
	{".", TokenDot},
	{",", TokenComma},
	{":", TokenColon},
	{"(", TokenLParen},
	{")", TokenRParen},
	{"[", TokenLBracket},
	{"]", TokenRBracket},
	{"{", TokenLBrace},
	{"}", TokenRBrace},
}

// nonOperator describes a character sequence that looks like an operator in
// other languages but is not one in Dot.
type nonOperator struct {
	text    string
	message string
	hint    string
}

// nonOperators lists the rejected spellings, longest first, each with the
// diagnostic that explains the Dot equivalent.
var nonOperators = []nonOperator{
	{"&&", "'&&' is not a Dot operator", "use 'and' instead of '&&'"},
	{"||", "'||' is not a Dot operator", "use 'or' instead of '||'"},
	{";", "unexpected ';'", "Dot has no semicolons; statements end at a newline"},
	{"!", "'!' is not a Dot operator", "use 'not' instead of '!'"},
	{"'", "character literals are not supported", "Dot strings use double quotes"},
	{"#", "unexpected '#'", "comments start with '//'"},
	{"$", "unexpected '$'", "string interpolation is written as \"{expr}\", no '$'"},
}

// scanOperator scans one operator, punctuation mark or rejected operator-like
// spelling at the cursor.
//
// It reports true when it consumed something, including when the spelling was
// rejected (a diagnostic is recorded and a TokenIllegal is emitted so that
// scanning continues). It reports false when the cursor is not on an operator
// start at all, leaving the "unexpected character" diagnostic to the caller.
func (l *lexer) scanOperator() bool {
	start := l.pos()

	// "@" introduces an annotation, an arena block or a loop label and has
	// its own scanner because the identifier must follow immediately (D4).
	if l.peek() == '@' {
		l.scanAt(start)
		return true
	}

	for _, op := range multiOperators {
		if l.match(op.text) {
			l.emit(op.tt, start, op.text)
			return true
		}
	}
	for _, bad := range nonOperators {
		if l.matchNonOperator(bad, start) {
			return true
		}
	}
	for _, op := range singleOperators {
		if l.match(op.text) {
			l.emit(op.tt, start, op.text)
			return true
		}
	}
	return false
}

// matchNonOperator consumes a rejected spelling, records its diagnostic and
// emits a TokenIllegal. "!" is only rejected when it is not part of "!=",
// which the operator table has already matched.
func (l *lexer) matchNonOperator(bad nonOperator, start Position) bool {
	if !l.match(bad.text) {
		return false
	}
	l.errorHintf(start, len(bad.text), bad.hint, "%s", bad.message)
	l.emit(TokenIllegal, start, bad.text)
	return true
}
