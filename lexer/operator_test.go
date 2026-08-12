package lexer

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// operators and punctuation
// ---------------------------------------------------------------------------

func TestTokenize_Operators(t *testing.T) {
	cases := []struct {
		src  string
		want []TokenType
	}{
		{"+", []TokenType{TokenPlus}},
		{"-", []TokenType{TokenMinus}},
		{"*", []TokenType{TokenStar}},
		{"/", []TokenType{TokenSlash}},
		{"%", []TokenType{TokenPercent}},
		{"**", []TokenType{TokenStarStar}},
		{"==", []TokenType{TokenEqEq}},
		{"!=", []TokenType{TokenBangEq}},
		{"<", []TokenType{TokenLt}},
		{">", []TokenType{TokenGt}},
		{"<=", []TokenType{TokenLtEq}},
		{">=", []TokenType{TokenGtEq}},
		{"&", []TokenType{TokenAmp}},
		{"|", []TokenType{TokenPipe}},
		{"^", []TokenType{TokenCaret}},
		{"~", []TokenType{TokenTilde}},
		{"<<", []TokenType{TokenShl}},
		{">>", []TokenType{TokenShr}},
		{"=", []TokenType{TokenAssign}},
		{"+=", []TokenType{TokenPlusAssign}},
		{"-=", []TokenType{TokenMinusAssign}},
		{"*=", []TokenType{TokenStarAssign}},
		{"/=", []TokenType{TokenSlashAssign}},
		{"%=", []TokenType{TokenPercentAssign}},
		{"&=", []TokenType{TokenAmpAssign}},
		{"|=", []TokenType{TokenPipeAssign}},
		{"^=", []TokenType{TokenCaretAssign}},
		{"<<=", []TokenType{TokenShlAssign}},
		{">>=", []TokenType{TokenShrAssign}},
		{"|>", []TokenType{TokenPipeArrow}},
		{"..", []TokenType{TokenDotDot}},
		{"..=", []TokenType{TokenDotDotEq}},
		{"...", []TokenType{TokenEllipsis}},
		{"->", []TokenType{TokenArrow}},
		{"=>", []TokenType{TokenFatArrow}},
		{"?", []TokenType{TokenQuestion}},
		{".", []TokenType{TokenDot}},
		{",", []TokenType{TokenComma}},
		{":", []TokenType{TokenColon}},
		{"()", []TokenType{TokenLParen, TokenRParen}},
		{"[]", []TokenType{TokenLBracket, TokenRBracket}},
		{"{}", []TokenType{TokenLBrace, TokenRBrace}},

		// maximal munch pairs
		{"a<<=b", []TokenType{TokenIdent, TokenShlAssign, TokenIdent}},
		{"a<<b", []TokenType{TokenIdent, TokenShl, TokenIdent}},
		{"a<b", []TokenType{TokenIdent, TokenLt, TokenIdent}},
		{"a<=b", []TokenType{TokenIdent, TokenLtEq, TokenIdent}},
		{"a>>=b", []TokenType{TokenIdent, TokenShrAssign, TokenIdent}},
		{"a**b", []TokenType{TokenIdent, TokenStarStar, TokenIdent}},
		{"a*b", []TokenType{TokenIdent, TokenStar, TokenIdent}},
		{"a==b", []TokenType{TokenIdent, TokenEqEq, TokenIdent}},
		{"a=b", []TokenType{TokenIdent, TokenAssign, TokenIdent}},
		{"a=>b", []TokenType{TokenIdent, TokenFatArrow, TokenIdent}},
		{"a|>b", []TokenType{TokenIdent, TokenPipeArrow, TokenIdent}},
		{"a|b", []TokenType{TokenIdent, TokenPipe, TokenIdent}},
		{"a|=b", []TokenType{TokenIdent, TokenPipeAssign, TokenIdent}},
		{"a->b", []TokenType{TokenIdent, TokenArrow, TokenIdent}},
		{"a-b", []TokenType{TokenIdent, TokenMinus, TokenIdent}},
		{"a..b", []TokenType{TokenIdent, TokenDotDot, TokenIdent}},
		{"a..=b", []TokenType{TokenIdent, TokenDotDotEq, TokenIdent}},
		{"a...b", []TokenType{TokenIdent, TokenEllipsis, TokenIdent}},
		{"a.b", []TokenType{TokenIdent, TokenDot, TokenIdent}},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			assertTypes(t, tc.src, lex(t, tc.src), tc.want...)
		})
	}
}

// ---------------------------------------------------------------------------
// annotations and labels (D4)
// ---------------------------------------------------------------------------

func TestTokenize_Annotations(t *testing.T) {
	cases := []struct {
		src  string
		want []TokenType
	}{
		{"@test", []TokenType{TokenAt, TokenIdent}},
		{"@test fn f() {}", []TokenType{TokenAt, TokenIdent, TokenFn, TokenIdent,
			TokenLParen, TokenRParen, TokenLBrace, TokenRBrace}},
		{`@extern("C")`, []TokenType{TokenAt, TokenIdent, TokenLParen, TokenString, TokenRParen}},
		{"@type", []TokenType{TokenAt, TokenIdent}},
		{"@inline", []TokenType{TokenAt, TokenIdent}},
		{"@perf {}", []TokenType{TokenAt, TokenIdent, TokenLBrace, TokenRBrace}},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, tc.want...)
			if toks[1].Type != TokenIdent {
				t.Fatalf("token after '@' is %v, want TokenIdent", toks[1].Type)
			}
		})
	}
}

func TestTokenize_Labels(t *testing.T) {
	// The newline right after "{" is dropped by rule D2.3 (an opener dangles).
	src := "@outer for i in 0..10 {\nbreak @outer\n}"
	toks := lex(t, src)
	assertTypes(t, src, toks,
		TokenAt, TokenIdent, TokenFor, TokenIdent, TokenIn,
		TokenInt, TokenDotDot, TokenInt, TokenLBrace,
		TokenBreak, TokenAt, TokenIdent, TokenNewline, TokenRBrace)
}

func TestTokenize_AtWithoutIdentifier(t *testing.T) {
	for _, src := range []string{"@ test", "@1", "@"} {
		t.Run(src, func(t *testing.T) {
			_, list := lexBad(t, src)
			le := firstError(t, list)
			if !strings.Contains(le.Message, "expected identifier after '@'") {
				t.Errorf("Message = %q, want it to mention \"expected identifier after '@'\"", le.Message)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rejected spellings
// ---------------------------------------------------------------------------

func TestTokenize_RejectedSpellings(t *testing.T) {
	cases := []struct {
		src      string
		message  string
		hintPart string
	}{
		{";", "unexpected ';'", "no semicolons"},
		{"a && b", "'&&' is not a Dot operator", "use 'and'"},
		{"a || b", "'||' is not a Dot operator", "use 'or'"},
		{"!x", "'!' is not a Dot operator", "use 'not'"},
		{"'", "character literals are not supported", "double quotes"},
		{"#", "unexpected '#'", "//"},
		{"$", "unexpected '$'", "interpolation"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			toks, list := lexBad(t, tc.src)
			le := firstError(t, list)
			if le.Message != tc.message {
				t.Errorf("Message = %q, want %q", le.Message, tc.message)
			}
			if !strings.Contains(le.Hint, tc.hintPart) {
				t.Errorf("Hint = %q, want it to contain %q", le.Hint, tc.hintPart)
			}
			var sawIllegal bool
			for _, tok := range toks {
				if tok.Type == TokenIllegal {
					sawIllegal = true
				}
			}
			if !sawIllegal {
				t.Errorf("no TokenIllegal emitted for %q (%s)", tc.src, typeNames(toks))
			}
		})
	}
}
