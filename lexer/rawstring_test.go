package lexer

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// raw strings
// ---------------------------------------------------------------------------

func TestScanRawString(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		value string
	}{
		{"plain", "`hello`", "hello"},
		{"empty", "``", ""},
		{"no escape processing", "`raw \\n text`", `raw \n text`},
		{"no interpolation", "`hi {name}`", "hi {name}"},
		{"quotes inside", "`say \"hi\"`", `say "hi"`},
		{"multi-line", "`one\ntwo\nthree`", "one\ntwo\nthree"},
		{"backslash at end", "`a\\`", `a\`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, TokenRawString)
			if toks[0].Value != tc.value {
				t.Errorf("Value = %q, want %q", toks[0].Value, tc.value)
			}
			if toks[0].Lexeme != tc.src {
				t.Errorf("Lexeme = %q, want %q", toks[0].Lexeme, tc.src)
			}
			if tc.value == "" {
				return
			}
			if len(toks[0].Parts) != 1 {
				t.Fatalf("got %d parts %v, want 1", len(toks[0].Parts), dumpParts(toks[0].Parts))
			}
			if toks[0].Parts[0].Kind != StringPartLiteral {
				t.Errorf("part Kind = %v, want StringPartLiteral", toks[0].Parts[0].Kind)
			}
			if toks[0].Parts[0].Pos.Column != 2 || toks[0].Parts[0].Pos.Line != 1 {
				t.Errorf("part Pos = %d:%d, want 1:2",
					toks[0].Parts[0].Pos.Line, toks[0].Parts[0].Pos.Column)
			}
		})
	}
}

func TestScanRawString_PositionAfter(t *testing.T) {
	src := "a = `x\ny\nz` + b"
	toks := lex(t, src)
	last := toks[len(toks)-3] // before NEWLINE EOF
	if last.Type != TokenIdent || last.Lexeme != "b" {
		t.Fatalf("last significant token is %v(%q), want TokenIdent(b)", last.Type, last.Lexeme)
	}
	if last.Pos.Line != 3 || last.Pos.Column != 6 {
		t.Errorf("'b' at %d:%d, want 3:6", last.Pos.Line, last.Pos.Column)
	}
}

// ---------------------------------------------------------------------------
// string errors
// ---------------------------------------------------------------------------

func TestScanString_Errors(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		message string
		line    int
		col     int
		typ     TokenType
	}{
		{"unterminated string", `x = "hello`, "unterminated string literal", 1, 5, TokenString},
		{"string broken by newline", "x = \"hello\ny = 1", "unterminated string literal", 1, 5, TokenString},
		{"unterminated raw string", "x = `hello", "unterminated raw string literal", 1, 5, TokenRawString},
		{"unknown escape", `"a\qb"`, "unknown escape sequence", 1, 3, TokenString},
		{"empty interpolation", `"{}"`, "empty interpolation expression", 1, 2, TokenString},
		{"blank interpolation", `"{   }"`, "empty interpolation expression", 1, 2, TokenString},
		{"unmatched closing brace", `"a } b"`, "unmatched '}' in string literal", 1, 4, TokenString},
		{"unterminated interpolation", `"a {b`, "unterminated interpolation", 1, 4, TokenString},
		{"bad hex escape", `"\xZZ"`, "invalid '\\x' escape", 1, 2, TokenString},
		{"short hex escape", `"\x4"`, "invalid '\\x' escape", 1, 2, TokenString},
		{"bad unicode escape", `"\u41"`, "invalid '\\u' escape", 1, 2, TokenString},
		{"empty unicode escape", `"\u{}"`, "invalid '\\u' escape", 1, 2, TokenString},
		{"out of range unicode escape", `"\u{110000}"`, "invalid Unicode code point", 1, 2, TokenString},
		{"surrogate unicode escape", `"\u{D800}"`, "invalid Unicode code point", 1, 2, TokenString},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			toks, list := lexBad(t, tc.src)
			le := firstError(t, list)
			if !strings.Contains(le.Message, tc.message) {
				t.Errorf("Message = %q, want it to contain %q", le.Message, tc.message)
			}
			if le.Line != tc.line || le.Column != tc.col {
				t.Errorf("position = %d:%d, want %d:%d", le.Line, le.Column, tc.line, tc.col)
			}
			var found bool
			for _, tok := range toks {
				if tok.Type == tc.typ {
					found = true
				}
			}
			if !found {
				t.Errorf("no %v emitted after recovery (%s)", tc.typ, typeNames(toks))
			}
		})
	}
}

func TestScanString_UnterminatedRecoversAtNewline(t *testing.T) {
	src := "a = \"oops\nb = 2"
	toks, _ := lexBad(t, src)
	assertTypes(t, src, toks,
		TokenIdent, TokenAssign, TokenString, TokenNewline,
		TokenIdent, TokenAssign, TokenInt)
}

func TestStringPartKind_String(t *testing.T) {
	if got := StringPartLiteral.String(); got != "StringPartLiteral" {
		t.Errorf("StringPartLiteral.String() = %q", got)
	}
	if got := StringPartExpr.String(); got != "StringPartExpr" {
		t.Errorf("StringPartExpr.String() = %q", got)
	}
}
