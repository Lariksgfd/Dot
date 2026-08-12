package lexer

import "testing"

// partSpec is the expected shape of one StringPart.
type partSpec struct {
	kind  StringPartKind
	value string
	line  int
	col   int
}

// checkParts compares a token's parts with the expected specs. A line of 0
// means "do not check the position".
func checkParts(t *testing.T, tok Token, want []partSpec) {
	t.Helper()
	if len(tok.Parts) != len(want) {
		t.Fatalf("got %d parts %v, want %d %v", len(tok.Parts), dumpParts(tok.Parts), len(want), want)
	}
	for i, w := range want {
		got := tok.Parts[i]
		if got.Kind != w.kind {
			t.Errorf("part %d Kind = %v, want %v", i, got.Kind, w.kind)
		}
		if got.Value != w.value {
			t.Errorf("part %d Value = %q, want %q", i, got.Value, w.value)
		}
		if w.line != 0 && (got.Pos.Line != w.line || got.Pos.Column != w.col) {
			t.Errorf("part %d Pos = %d:%d, want %d:%d",
				i, got.Pos.Line, got.Pos.Column, w.line, w.col)
		}
	}
}

func dumpParts(parts []StringPart) []string {
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = p.Kind.String() + "(" + p.Value + ")@" + p.Pos.String()
	}
	return out
}

// ---------------------------------------------------------------------------
// plain strings and escapes
// ---------------------------------------------------------------------------

func TestScanString_Plain(t *testing.T) {
	cases := []struct {
		src   string
		value string
	}{
		{`""`, ""},
		{`"hello"`, "hello"},
		{`"hello world"`, "hello world"},
		{`"  spaced  "`, "  spaced  "},
		{`"日本語"`, "日本語"},
		{`"a/b*c//d"`, "a/b*c//d"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, TokenString)
			if toks[0].Value != tc.value {
				t.Errorf("Value = %q, want %q", toks[0].Value, tc.value)
			}
			if toks[0].Lexeme != tc.src {
				t.Errorf("Lexeme = %q, want %q", toks[0].Lexeme, tc.src)
			}
		})
	}
}

func TestScanString_Escapes(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		bytes []byte
	}{
		{"newline", `"a\nb"`, []byte("a\nb")},
		{"tab", `"a\tb"`, []byte("a\tb")},
		{"carriage return", `"a\rb"`, []byte("a\rb")},
		{"backslash", `"a\\b"`, []byte(`a\b`)},
		{"quote", `"a\"b"`, []byte(`a"b`)},
		{"backtick", "\"a\\`b\"", []byte("a`b")},
		{"nul", `"a\0b"`, []byte{'a', 0, 'b'}},
		{"hex", `"\x41"`, []byte("A")},
		{"hex lowercase digits", `"\x7f"`, []byte{0x7f}},
		{"unicode BMP", `"\u{263A}"`, []byte("\u263A")},
		{"unicode emoji", `"\u{1F600}"`, []byte("\U0001F600")},
		{"unicode single digit", `"\u{41}"`, []byte("A")},
		{"all together", `"n:\n t:\t h:\x41"`, []byte("n:\n t:\t h:A")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, TokenString)
			if got := []byte(toks[0].Value); string(got) != string(tc.bytes) {
				t.Errorf("Value bytes = %v (%q), want %v (%q)",
					got, toks[0].Value, tc.bytes, string(tc.bytes))
			}
		})
	}
}

func TestScanString_BraceEscapes(t *testing.T) {
	cases := []struct {
		src   string
		value string
	}{
		{`"{{"`, "{"},
		{`"}}"`, "}"},
		{`"{{x}}"`, "{x}"},
		{`"a{{b}}c"`, "a{b}c"},
		{`"{{{{"`, "{{"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, TokenString)
			if toks[0].Value != tc.value {
				t.Errorf("Value = %q, want %q", toks[0].Value, tc.value)
			}
			for _, p := range toks[0].Parts {
				if p.Kind != StringPartLiteral {
					t.Errorf("got %v part, want only literal parts in %v",
						p.Kind, dumpParts(toks[0].Parts))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// interpolation (D1)
// ---------------------------------------------------------------------------

func TestScanString_Interpolation(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		parts []partSpec
	}{
		{
			name: "trailing expression",
			src:  `"hi {name}"`,
			parts: []partSpec{
				{StringPartLiteral, "hi ", 1, 2},
				{StringPartExpr, "name", 1, 6},
			},
		},
		{
			name: "expression only",
			src:  `"{x}"`,
			parts: []partSpec{
				{StringPartExpr, "x", 1, 3},
			},
		},
		{
			name: "two expressions",
			src:  `"{a + b} and {c}"`,
			parts: []partSpec{
				{StringPartExpr, "a + b", 1, 3},
				{StringPartLiteral, " and ", 1, 9},
				{StringPartExpr, "c", 1, 15},
			},
		},
		{
			name: "whitespace trimmed, position adjusted",
			src:  `"{   a + b   }"`,
			parts: []partSpec{
				{StringPartExpr, "a + b", 1, 6},
			},
		},
		{
			name: "nested braces in a map literal",
			src:  `"{ f({x: 1}) }"`,
			parts: []partSpec{
				{StringPartExpr, "f({x: 1})", 1, 4},
			},
		},
		{
			name: "braces inside a nested string",
			src:  `"{ g("}") }"`,
			parts: []partSpec{
				{StringPartExpr, `g("}")`, 1, 4},
			},
		},
		{
			name: "escaped brace inside a nested string",
			src:  `"{ g("\"}") }"`,
			parts: []partSpec{
				{StringPartExpr, `g("\"}")`, 1, 4},
			},
		},
		{
			name: "nested raw string with a brace",
			src:  "\"{ g(`}`) }\"",
			parts: []partSpec{
				{StringPartExpr, "g(`}`)", 1, 4},
			},
		},
		{
			name: "field access and method call",
			src:  `"({self.x}, {self.y})"`,
			parts: []partSpec{
				{StringPartLiteral, "(", 1, 2},
				{StringPartExpr, "self.x", 1, 4},
				{StringPartLiteral, ", ", 1, 11},
				{StringPartExpr, "self.y", 1, 14},
				{StringPartLiteral, ")", 1, 21},
			},
		},
		{
			name: "literal braces around an interpolation",
			src:  `"{{{x}}}"`,
			parts: []partSpec{
				{StringPartLiteral, "{", 1, 2},
				{StringPartExpr, "x", 1, 5},
				{StringPartLiteral, "}", 1, 7},
			},
		},
		{
			name: "escapes around an interpolation",
			src:  `"a\t{x}\nb"`,
			parts: []partSpec{
				{StringPartLiteral, "a\t", 1, 2},
				{StringPartExpr, "x", 1, 6},
				{StringPartLiteral, "\nb", 1, 8},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, TokenString)
			checkParts(t, toks[0], tc.parts)
			if toks[0].Lexeme != tc.src {
				t.Errorf("Lexeme = %q, want %q", toks[0].Lexeme, tc.src)
			}
		})
	}
}

func TestScanString_InterpolationPositionOnLaterLine(t *testing.T) {
	src := "x = 1\ny = \"v={a}\""
	toks := lex(t, src)
	var str *Token
	for i := range toks {
		if toks[i].Type == TokenString {
			str = &toks[i]
		}
	}
	if str == nil {
		t.Fatalf("no string token in %s", typeNames(toks))
	}
	checkParts(t, *str, []partSpec{
		{StringPartLiteral, "v=", 2, 6},
		{StringPartExpr, "a", 2, 9},
	})
}

// TestScanString_MultiLineInterpolation pins design decision D1: a newline
// inside "{ ... }" is allowed, only EOF terminates the interpolation early.
func TestScanString_MultiLineInterpolation(t *testing.T) {
	src := "\"{a +\nb} tail\"\nz"
	toks := lex(t, src)
	assertTypes(t, src, toks, TokenString, TokenNewline, TokenIdent)
	checkParts(t, toks[0], []partSpec{
		{StringPartExpr, "a +\nb", 1, 3},
		{StringPartLiteral, " tail", 2, 3},
	})
}

func TestScanString_ValueIsLiteralPartsOnly(t *testing.T) {
	toks := lex(t, `"a{b}c"`)
	if got, want := toks[0].Value, "ac"; got != want {
		t.Errorf("Value = %q, want %q", got, want)
	}
}
