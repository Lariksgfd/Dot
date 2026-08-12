package lexer

import "testing"

// ---------------------------------------------------------------------------
// keywords and reserved words
// ---------------------------------------------------------------------------

var keywordCases = []struct {
	src  string
	want TokenType
}{
	{"fn", TokenFn}, {"struct", TokenStruct}, {"trait", TokenTrait},
	{"impl", TokenImpl}, {"enum", TokenEnum}, {"if", TokenIf},
	{"else", TokenElse}, {"for", TokenFor}, {"in", TokenIn},
	{"match", TokenMatch}, {"return", TokenReturn}, {"break", TokenBreak},
	{"continue", TokenContinue}, {"import", TokenImport}, {"from", TokenFrom},
	{"const", TokenConst}, {"mut", TokenMut}, {"self", TokenSelf},
	{"Self", TokenSelfType}, {"spawn", TokenSpawn}, {"async", TokenAsync},
	{"await", TokenAwait}, {"defer", TokenDefer}, {"embed", TokenEmbed},
	{"pub", TokenPub}, {"true", TokenTrue}, {"false", TokenFalse},
	{"nil", TokenNil}, {"and", TokenAnd}, {"or", TokenOr},
	{"not", TokenNot}, {"as", TokenAs}, {"is", TokenIs},
	{"dyn", TokenDyn}, {"weak", TokenWeak},
}

var reservedCases = []struct {
	src  string
	want TokenType
}{
	{"macro", TokenMacro}, {"where", TokenWhere}, {"yield", TokenYield},
	{"type", TokenTypeKw}, {"module", TokenModule}, {"unsafe", TokenUnsafe},
	{"extern", TokenExtern}, {"static", TokenStatic}, {"virtual", TokenVirtual},
	{"override", TokenOverride},
}

func TestTokenize_Keywords(t *testing.T) {
	for _, tc := range keywordCases {
		t.Run(tc.src, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, tc.want)
			if toks[0].Lexeme != tc.src {
				t.Errorf("Lexeme = %q, want %q", toks[0].Lexeme, tc.src)
			}
			if !tc.want.IsKeyword() {
				t.Errorf("%v.IsKeyword() = false, want true", tc.want)
			}
			if tc.want.IsReserved() {
				t.Errorf("%v.IsReserved() = true, want false", tc.want)
			}
			if tc.want.Literal() != tc.src {
				t.Errorf("%v.Literal() = %q, want %q", tc.want, tc.want.Literal(), tc.src)
			}
			if got, ok := LookupKeyword(tc.src); !ok || got != tc.want {
				t.Errorf("LookupKeyword(%q) = (%v, %v), want (%v, true)", tc.src, got, ok, tc.want)
			}
		})
	}
}

func TestTokenize_ReservedWords(t *testing.T) {
	for _, tc := range reservedCases {
		t.Run(tc.src, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, tc.want)
			if !tc.want.IsReserved() {
				t.Errorf("%v.IsReserved() = false, want true", tc.want)
			}
			if tc.want.IsKeyword() {
				t.Errorf("%v.IsKeyword() = true, want false", tc.want)
			}
		})
	}
}

func TestLookupKeyword_NotAKeyword(t *testing.T) {
	for _, s := range []string{"foo", "Fn", "FN", "iff", "_", "returns", "selfish"} {
		if got, ok := LookupKeyword(s); ok || got != TokenIdent {
			t.Errorf("LookupKeyword(%q) = (%v, %v), want (TokenIdent, false)", s, got, ok)
		}
	}
}

func TestTokenType_IsOperator_KeywordOperators(t *testing.T) {
	for _, tt := range []TokenType{TokenAnd, TokenOr, TokenNot} {
		if !tt.IsOperator() {
			t.Errorf("%v.IsOperator() = false, want true", tt)
		}
		if !tt.IsKeyword() {
			t.Errorf("%v.IsKeyword() = false, want true", tt)
		}
	}
	for _, tt := range []TokenType{TokenInt, TokenFloat, TokenString, TokenRawString,
		TokenTrue, TokenFalse, TokenNil} {
		if !tt.IsLiteral() {
			t.Errorf("%v.IsLiteral() = false, want true", tt)
		}
	}
	if TokenIdent.IsLiteral() {
		t.Error("TokenIdent.IsLiteral() = true, want false")
	}
}

// ---------------------------------------------------------------------------
// identifiers
// ---------------------------------------------------------------------------

func TestTokenize_Identifiers(t *testing.T) {
	cases := []struct {
		src  string
		want TokenType
	}{
		{"foo", TokenIdent},
		{"Point", TokenIdent},
		{"to_string", TokenIdent},
		{"_private", TokenIdent},
		{"_", TokenUnderscore},
		{"__", TokenIdent},
		{"_1", TokenIdent},
		{"x1", TokenIdent},
		{"a_b_c9", TokenIdent},
		{"café", TokenIdent},
		{"переменная", TokenIdent},
		{"日本語", TokenIdent},
		{"Ω", TokenIdent},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, tc.want)
			if toks[0].Lexeme != tc.src {
				t.Errorf("Lexeme = %q, want %q", toks[0].Lexeme, tc.src)
			}
			if toks[0].Value != tc.src {
				t.Errorf("Value = %q, want %q", toks[0].Value, tc.src)
			}
		})
	}
}

func TestTokenize_UnderscorePatterns(t *testing.T) {
	src := "_, err = f(_)"
	toks := lex(t, src)
	assertTypes(t, src, toks,
		TokenUnderscore, TokenComma, TokenIdent, TokenAssign,
		TokenIdent, TokenLParen, TokenUnderscore, TokenRParen)
}
