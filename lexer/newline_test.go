package lexer

import (
	"strings"
	"testing"
)

// nlCase is a newline-filtering scenario expressed as source text.
type nlCase struct {
	name string
	src  string
	want []TokenType
}

func runNLCases(t *testing.T, cases []nlCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertTypes(t, tc.src, lex(t, tc.src), tc.want...)
		})
	}
}

// ---------------------------------------------------------------------------
// D2.1 — bracket suppression
// ---------------------------------------------------------------------------

func TestFilterNewlines_SuppressedInsideParensAndBrackets(t *testing.T) {
	runNLCases(t, []nlCase{
		{"empty parens over lines", "(\n)", []TokenType{TokenLParen, TokenRParen}},
		{"call arguments", "f(\n1,\n2\n)", []TokenType{
			TokenIdent, TokenLParen, TokenInt, TokenComma, TokenInt, TokenRParen}},
		{"parenthesized expression", "x = (\n1 +\n2\n)", []TokenType{
			TokenIdent, TokenAssign, TokenLParen, TokenInt, TokenPlus, TokenInt, TokenRParen}},
		{"slice literal", "[\n1,\n2\n]", []TokenType{
			TokenLBracket, TokenInt, TokenComma, TokenInt, TokenRBracket}},
		{"generic type argument list", "Stack[\nT\n]", []TokenType{
			TokenIdent, TokenLBracket, TokenIdent, TokenRBracket}},
		{"nested brackets", "f(g[\n0\n],\nh(\n1))", []TokenType{
			TokenIdent, TokenLParen, TokenIdent, TokenLBracket, TokenInt, TokenRBracket,
			TokenComma, TokenIdent, TokenLParen, TokenInt, TokenRParen, TokenRParen}},
	})
}

// TestFilterNewlines_NotSuppressedInsideBraces pins rule D2.2: braces delimit
// blocks, where newlines are the statement terminators.
func TestFilterNewlines_NotSuppressedInsideBraces(t *testing.T) {
	runNLCases(t, []nlCase{
		{"block body", "{\na\nb\n}", []TokenType{
			TokenLBrace, TokenIdent, TokenNewline, TokenIdent, TokenNewline, TokenRBrace}},
		{"function body", "fn f() {\nx = 1\n}", []TokenType{
			TokenFn, TokenIdent, TokenLParen, TokenRParen, TokenLBrace,
			TokenIdent, TokenAssign, TokenInt, TokenNewline, TokenRBrace}},
		{"struct literal", "Point {\nx: 1\ny: 2\n}", []TokenType{
			TokenIdent, TokenLBrace, TokenIdent, TokenColon, TokenInt, TokenNewline,
			TokenIdent, TokenColon, TokenInt, TokenNewline, TokenRBrace}},
		{"brace block inside a call", "f(fn() {\na\n})", []TokenType{
			TokenIdent, TokenLParen, TokenFn, TokenLParen, TokenRParen, TokenLBrace,
			TokenIdent, TokenNewline, TokenRBrace, TokenRParen}},
	})
}

// ---------------------------------------------------------------------------
// D2.3 — trailing-operator suppression
// ---------------------------------------------------------------------------

func TestFilterNewlines_SuppressedAfterDanglingOperator(t *testing.T) {
	runNLCases(t, []nlCase{
		{"plus", "a +\nb", []TokenType{TokenIdent, TokenPlus, TokenIdent}},
		{"assign", "a =\nb", []TokenType{TokenIdent, TokenAssign, TokenIdent}},
		{"comma", "a,\nb", []TokenType{TokenIdent, TokenComma, TokenIdent}},
		{"arrow", "fn f() ->\nint {}", []TokenType{
			TokenFn, TokenIdent, TokenLParen, TokenRParen, TokenArrow, TokenIdent,
			TokenLBrace, TokenRBrace}},
		{"fat arrow", "0 =>\n1", []TokenType{TokenInt, TokenFatArrow, TokenInt}},
		{"pipe arrow", "a |>\nf", []TokenType{TokenIdent, TokenPipeArrow, TokenIdent}},
		{"dot", "a.\nb", []TokenType{TokenIdent, TokenDot, TokenIdent}},
		{"colon", "x:\n1", []TokenType{TokenIdent, TokenColon, TokenInt}},
		{"and", "a and\nb", []TokenType{TokenIdent, TokenAnd, TokenIdent}},
		{"or", "a or\nb", []TokenType{TokenIdent, TokenOr, TokenIdent}},
		{"not", "not\na", []TokenType{TokenNot, TokenIdent}},
		{"star", "a *\nb", []TokenType{TokenIdent, TokenStar, TokenIdent}},
		{"eqeq", "a ==\nb", []TokenType{TokenIdent, TokenEqEq, TokenIdent}},
		{"range", "a ..\nb", []TokenType{TokenIdent, TokenDotDot, TokenIdent}},
		{"open brace", "{\na", []TokenType{TokenLBrace, TokenIdent}},
		{"open paren", "f(\na", []TokenType{TokenIdent, TokenLParen, TokenIdent}},
	})
}

// TestFilterNewlines_SuppressedAfterAt covers the "@" case of rule D2.3 on a
// hand-written stream: "@" followed by a newline is a lex error in real
// source (design decision D4), so it cannot be produced by Tokenize.
func TestFilterNewlines_SuppressedAfterAt(t *testing.T) {
	in := []Token{
		{Type: TokenAt, Lexeme: "@"},
		{Type: TokenNewline, Lexeme: "\n"},
		{Type: TokenIdent, Lexeme: "test"},
		{Type: TokenEOF},
	}
	got := filterNewlines(in)
	want := []TokenType{TokenAt, TokenIdent, TokenNewline, TokenEOF}
	if len(got) != len(want) {
		t.Fatalf("got %s, want %v", typeNames(got), want)
	}
	for i := range want {
		if got[i].Type != want[i] {
			t.Fatalf("got %s, want %v", typeNames(got), want)
		}
	}
}

func TestFilterNewlines_KeptAfterStatementEnders(t *testing.T) {
	runNLCases(t, []nlCase{
		{"question mark", "a?\nb", []TokenType{TokenIdent, TokenQuestion, TokenNewline, TokenIdent}},
		{"close brace", "}\na", []TokenType{TokenRBrace, TokenNewline, TokenIdent}},
		{"return", "return\na", []TokenType{TokenReturn, TokenNewline, TokenIdent}},
		{"break", "break\na", []TokenType{TokenBreak, TokenNewline, TokenIdent}},
		{"continue", "continue\na", []TokenType{TokenContinue, TokenNewline, TokenIdent}},
		{"identifier", "a\nb", []TokenType{TokenIdent, TokenNewline, TokenIdent}},
		{"int literal", "1\n2", []TokenType{TokenInt, TokenNewline, TokenInt}},
		{"string literal", "\"a\"\n\"b\"", []TokenType{TokenString, TokenNewline, TokenString}},
	})
}

// ---------------------------------------------------------------------------
// D2.4 — leading-continuation suppression
// ---------------------------------------------------------------------------

func TestFilterNewlines_SuppressedBeforeContinuationTokens(t *testing.T) {
	runNLCases(t, []nlCase{
		{"dot", "a\n.b", []TokenType{TokenIdent, TokenDot, TokenIdent}},
		{"pipe arrow", "a\n|> f", []TokenType{TokenIdent, TokenPipeArrow, TokenIdent}},
		{"else", "}\nelse {", []TokenType{TokenRBrace, TokenElse, TokenLBrace}},
		{"fat arrow", "0\n=> 1", []TokenType{TokenInt, TokenFatArrow, TokenInt}},
		{"comma", "a\n, b", []TokenType{TokenIdent, TokenComma, TokenIdent}},
	})
}

// TestFilterNewlines_LeadingRParenAndRBracket exercises the ")" and "]" cases
// of rule D2.4 directly, because the scanner already suppresses newlines while
// those brackets are open.
func TestFilterNewlines_LeadingRParenAndRBracket(t *testing.T) {
	cases := []struct {
		name string
		in   []TokenType
		want []TokenType
	}{
		{"before rparen", []TokenType{TokenIdent, TokenNewline, TokenRParen},
			[]TokenType{TokenIdent, TokenRParen, TokenNewline, TokenEOF}},
		{"before rbracket", []TokenType{TokenIdent, TokenNewline, TokenRBracket},
			[]TokenType{TokenIdent, TokenRBracket, TokenNewline, TokenEOF}},
		{"before rbrace is kept", []TokenType{TokenIdent, TokenNewline, TokenRBrace},
			[]TokenType{TokenIdent, TokenNewline, TokenRBrace, TokenNewline, TokenEOF}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := make([]Token, 0, len(tc.in)+1)
			for _, tt := range tc.in {
				in = append(in, Token{Type: tt, Lexeme: tt.Literal()})
			}
			in = append(in, Token{Type: TokenEOF})
			got := filterNewlines(in)
			gotTypes := make([]TokenType, len(got))
			for i, tok := range got {
				gotTypes[i] = tok.Type
			}
			if len(gotTypes) != len(tc.want) {
				t.Fatalf("got %v, want %v", gotTypes, tc.want)
			}
			for i := range tc.want {
				if gotTypes[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", gotTypes, tc.want)
				}
			}
		})
	}
}

func TestFilterNewlines_DoesNotMutateInput(t *testing.T) {
	in := []Token{
		{Type: TokenIdent, Lexeme: "a"},
		{Type: TokenNewline, Lexeme: "\n"},
		{Type: TokenNewline, Lexeme: "\n"},
		{Type: TokenIdent, Lexeme: "b"},
		{Type: TokenEOF},
	}
	before := make([]Token, len(in))
	copy(before, in)
	filterNewlines(in)
	for i := range in {
		if in[i].Type != before[i].Type || in[i].Lexeme != before[i].Lexeme {
			t.Fatalf("input token %d was modified: %v -> %v", i, before[i], in[i])
		}
	}
}

// ---------------------------------------------------------------------------
// D2.5 / D2.6 — collapsing, leading drop, termination
// ---------------------------------------------------------------------------

func TestFilterNewlines_CollapseAndTerminate(t *testing.T) {
	runNLCases(t, []nlCase{
		{"run collapsed", "a\n\n\n\nb", []TokenType{TokenIdent, TokenNewline, TokenIdent}},
		{"blank lines with spaces", "a\n   \n\t\nb", []TokenType{TokenIdent, TokenNewline, TokenIdent}},
		{"leading newlines dropped", "\n\n\na", []TokenType{TokenIdent}},
		{"leading comment lines dropped", "// c\n\n// d\na", []TokenType{TokenComment, TokenNewline, TokenComment, TokenNewline, TokenIdent}},
		{"trailing newlines collapse to one", "a\n\n\n", []TokenType{TokenIdent}},
		{"no trailing newline", "a", []TokenType{TokenIdent}},
		{"only newlines", "\n\n\n", nil},
		{"empty source", "", nil},
	})
}

func TestFilterNewlines_SynthesizedNewlinePosition(t *testing.T) {
	toks := lex(t, "abc")
	nl := toks[len(toks)-2]
	eof := toks[len(toks)-1]
	if nl.Type != TokenNewline {
		t.Fatalf("second-to-last token is %v, want TokenNewline", nl.Type)
	}
	if nl.Pos != eof.Pos {
		t.Errorf("synthesized newline at %v, want the EOF position %v", nl.Pos, eof.Pos)
	}
}

func TestFilterNewlines_CollapsedRunKeepsFirstPosition(t *testing.T) {
	toks := lex(t, "a\n\n\nb")
	if toks[1].Type != TokenNewline {
		t.Fatalf("token 1 is %v, want TokenNewline", toks[1].Type)
	}
	if toks[1].Pos.Line != 1 {
		t.Errorf("collapsed newline at line %d, want 1", toks[1].Pos.Line)
	}
}

// ---------------------------------------------------------------------------
// realistic multi-line code
// ---------------------------------------------------------------------------

func TestFilterNewlines_PipeChain(t *testing.T) {
	src := strings.Join([]string{
		"result = data",
		"    |> filter(f)",
		"    .map(g)",
		"    |> collect()",
	}, "\n")
	assertTypes(t, src, lex(t, src),
		TokenIdent, TokenAssign, TokenIdent,
		TokenPipeArrow, TokenIdent, TokenLParen, TokenIdent, TokenRParen,
		TokenDot, TokenIdent, TokenLParen, TokenIdent, TokenRParen,
		TokenPipeArrow, TokenIdent, TokenLParen, TokenRParen)
}

func TestFilterNewlines_HangingElse(t *testing.T) {
	src := strings.Join([]string{
		"if x > 0 {",
		"    a()",
		"}",
		"else if x < 0 {",
		"    b()",
		"}",
		"else {",
		"    c()",
		"}",
	}, "\n")
	assertTypes(t, src, lex(t, src),
		TokenIf, TokenIdent, TokenGt, TokenInt, TokenLBrace,
		TokenIdent, TokenLParen, TokenRParen, TokenNewline, TokenRBrace,
		TokenElse, TokenIf, TokenIdent, TokenLt, TokenInt, TokenLBrace,
		TokenIdent, TokenLParen, TokenRParen, TokenNewline, TokenRBrace,
		TokenElse, TokenLBrace,
		TokenIdent, TokenLParen, TokenRParen, TokenNewline, TokenRBrace)
}

func TestFilterNewlines_MatchArms(t *testing.T) {
	src := strings.Join([]string{
		"match code {",
		"    200 => \"OK\"",
		"    404 => \"Not Found\"",
		"    _ => \"Unknown\"",
		"}",
	}, "\n")
	assertTypes(t, src, lex(t, src),
		TokenMatch, TokenIdent, TokenLBrace,
		TokenInt, TokenFatArrow, TokenString, TokenNewline,
		TokenInt, TokenFatArrow, TokenString, TokenNewline,
		TokenUnderscore, TokenFatArrow, TokenString, TokenNewline,
		TokenRBrace)
}

func TestFilterNewlines_MultiLineCallWithTrailingComma(t *testing.T) {
	src := "connect(\n    host,\n    port,\n)"
	assertTypes(t, src, lex(t, src),
		TokenIdent, TokenLParen, TokenIdent, TokenComma,
		TokenIdent, TokenComma, TokenRParen)
}
