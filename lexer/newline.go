package lexer

// filterNewlines applies the newline rules D2.1 to D2.6 to a token stream and
// returns a new slice; the input is never modified.
//
// A TokenNewline is dropped when the previous significant token cannot end a
// statement (any operator or opener, see danglesStatement) or when the next
// significant token continues the previous line (see continuesStatement).
// Runs of newlines collapse into one, newlines at the start of the file are
// dropped, and the stream is terminated with exactly one TokenNewline
// immediately before TokenEOF.
//
// Newlines inside "(" and "[" are already suppressed while scanning, so this
// pass only has to stay consistent with that behaviour; it is a pure function
// so it can be exercised on hand-written token slices as well.
func filterNewlines(tokens []Token) []Token {
	out := make([]Token, 0, len(tokens))
	var pending Token
	havePending := false

	for _, tok := range tokens {
		switch {
		case tok.Type == TokenNewline:
			if len(out) == 0 || danglesStatement(out[len(out)-1].Type) {
				continue // rule 5 (leading) and rule 3 (trailing operator)
			}
			if !havePending { // rule 5: collapse a run into its first newline
				pending, havePending = tok, true
			}
		case tok.Type == TokenEOF:
			return terminate(out, pending, havePending, tok)
		default:
			if continuesStatement(tok.Type) {
				havePending = false // rule 4
			}
			if havePending {
				out = append(out, pending)
				havePending = false
			}
			out = append(out, tok)
		}
	}

	if havePending {
		out = append(out, pending)
	}
	return out
}

// terminate closes the stream with exactly one TokenNewline followed by the
// end-of-file token (rule 6). The pending newline is reused when there is
// one, otherwise a zero-width newline is synthesized at the EOF position.
func terminate(out []Token, pending Token, havePending bool, eofTok Token) []Token {
	if !havePending {
		pending = Token{
			Type:   TokenNewline,
			Lexeme: "\n",
			Value:  "\n",
			Pos:    eofTok.Pos,
			End:    eofTok.Pos,
		}
	}
	return append(out, pending, eofTok)
}

// danglesStatement reports whether a statement cannot end with a token of
// this type, so that a following newline is a line continuation (rule 3).
//
// Operators, openers, ",", ":", "@" and "." dangle. "?", ")", "]", "}",
// "return", "break", "continue", identifiers and literals do not.
func danglesStatement(t TokenType) bool {
	switch t {
	case TokenQuestion:
		return false
	case TokenDot, TokenComma, TokenColon, TokenAt,
		TokenLParen, TokenLBracket, TokenLBrace:
		return true
	}
	return t.IsOperator()
}

// continuesStatement reports whether a token of this type continues the
// previous line, so that any pending newline before it is dropped (rule 4).
// This is what makes hanging method chains, pipelines and "else" on its own
// line work.
func continuesStatement(t TokenType) bool {
	switch t {
	case TokenDot, TokenPipeArrow, TokenElse, TokenFatArrow,
		TokenRParen, TokenRBracket, TokenComma:
		return true
	}
	return false
}
