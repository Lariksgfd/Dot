package lexer

import (
	"os"
	"strings"
	"testing"

	doterr "github.com/dotlang/dot/errors"
)

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

// lex tokenizes src and fails the test when any diagnostic is reported.
func lex(t *testing.T, src string) []Token {
	t.Helper()
	toks, err := Tokenize(src, "test.dot")
	if err != nil {
		t.Fatalf("Tokenize(%q): unexpected error:\n%v", src, err)
	}
	return toks
}

// lexBad tokenizes src and fails the test when no diagnostic is reported.
func lexBad(t *testing.T, src string) ([]Token, *doterr.ErrorList) {
	t.Helper()
	toks, err := Tokenize(src, "test.dot")
	if err == nil {
		t.Fatalf("Tokenize(%q): expected an error, got none (tokens=%v)", src, typeNames(toks))
	}
	list, ok := err.(*doterr.ErrorList)
	if !ok {
		t.Fatalf("Tokenize(%q): error has type %T, want *errors.ErrorList", src, err)
	}
	return toks, list
}

// typeNames renders the token types of toks for failure messages.
func typeNames(toks []Token) string {
	parts := make([]string, len(toks))
	for i, tok := range toks {
		parts[i] = tok.Type.String()
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// assertTypes compares the token stream with want, to which the mandatory
// trailing TokenNewline and TokenEOF are appended automatically.
func assertTypes(t *testing.T, src string, got []Token, want ...TokenType) {
	t.Helper()
	want = append(want, TokenNewline, TokenEOF)
	if len(got) != len(want) {
		t.Fatalf("Tokenize(%q): got %d tokens %s, want %d %v",
			src, len(got), typeNames(got), len(want), want)
	}
	for i := range want {
		if got[i].Type != want[i] {
			t.Errorf("Tokenize(%q): token %d is %v, want %v (full stream %s)",
				src, i, got[i].Type, want[i], typeNames(got))
		}
	}
}

// firstError returns the first diagnostic of list as a *errors.LexError.
func firstError(t *testing.T, list *doterr.ErrorList) *doterr.LexError {
	t.Helper()
	if list.Len() == 0 {
		t.Fatal("error list is empty")
	}
	le, ok := list.Errors[0].(*doterr.LexError)
	if !ok {
		t.Fatalf("first error has type %T, want *errors.LexError", list.Errors[0])
	}
	return le
}

// ---------------------------------------------------------------------------
// comments
// ---------------------------------------------------------------------------

func TestTokenize_Comments(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []TokenType
	}{
		{"line comment only", "// nothing here", []TokenType{TokenComment}},
		{"leading line comment", "// hi\nx", []TokenType{TokenComment, TokenNewline, TokenIdent}},
		{"trailing line comment keeps newline", "x // hi\ny",
			[]TokenType{TokenIdent, TokenComment, TokenNewline, TokenIdent}},
		{"comment at EOF without newline", "x // done", []TokenType{TokenIdent, TokenComment}},
		{"triple slash is a line comment", "/// doc\nx", []TokenType{TokenComment, TokenNewline, TokenIdent}},
		{"block comment inline", "x /* c */ y", []TokenType{TokenIdent, TokenComment, TokenIdent}},
		{"block comment only", "/* c */", []TokenType{TokenComment}},
		{"block comment at EOF", "x /* c */", []TokenType{TokenIdent, TokenComment}},
		{"nested block comment", "/* a /* b */ c */ x", []TokenType{TokenComment, TokenIdent}},
		{"deeply nested block comment", "/* /* /* x */ */ */ y", []TokenType{TokenComment, TokenIdent}},
		{"multi-line block comment yields a newline", "x /* a\nb */ y",
			[]TokenType{TokenIdent, TokenComment, TokenNewline, TokenIdent}},
		{"block comment containing slashes", "/* // not a line comment */ x",
			[]TokenType{TokenComment, TokenIdent}},
		{"line comment containing block open", "// /* unclosed\nx", []TokenType{TokenComment, TokenNewline, TokenIdent}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertTypes(t, tc.src, lex(t, tc.src), tc.want...)
		})
	}
}

func TestTokenize_UnterminatedBlockComment(t *testing.T) {
	src := "x /* never closed\nmore"
	toks, list := lexBad(t, src)
	if list.Len() != 1 {
		t.Fatalf("got %d errors, want 1:\n%v", list.Len(), list)
	}
	le := firstError(t, list)
	if !strings.Contains(le.Message, "unterminated block comment") {
		t.Errorf("Message = %q, want it to mention 'unterminated block comment'", le.Message)
	}
	if le.Line != 1 || le.Column != 3 {
		t.Errorf("position = %d:%d, want 1:3", le.Line, le.Column)
	}
	assertTypes(t, src, toks, TokenIdent)
}

// ---------------------------------------------------------------------------
// positions
// ---------------------------------------------------------------------------

func TestTokenize_Positions(t *testing.T) {
	src := "a bb\n  ccc\n\nd"
	toks := lex(t, src)
	want := []struct {
		typ  TokenType
		line int
		col  int
		off  int
	}{
		{TokenIdent, 1, 1, 0},
		{TokenIdent, 1, 3, 2},
		{TokenNewline, 1, 5, 4},
		{TokenIdent, 2, 3, 7},
		{TokenNewline, 2, 6, 10},
		{TokenIdent, 4, 1, 12},
	}
	if len(toks) != len(want)+2 {
		t.Fatalf("got %d tokens %s, want %d", len(toks), typeNames(toks), len(want)+2)
	}
	for i, w := range want {
		got := toks[i]
		if got.Type != w.typ || got.Pos.Line != w.line || got.Pos.Column != w.col || got.Pos.Offset != w.off {
			t.Errorf("token %d = %v at %d:%d (offset %d), want %v at %d:%d (offset %d)",
				i, got.Type, got.Pos.Line, got.Pos.Column, got.Pos.Offset, w.typ, w.line, w.col, w.off)
		}
	}
}

func TestTokenize_PositionsAfterMultilineConstructs(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		lastIdent string
		line      int
		col       int
	}{
		{"after raw string", "x = `one\ntwo\nthree`\nafter", "after", 4, 1},
		{"after raw string same line", "x = `a\nb` after", "after", 2, 4},
		{"after block comment", "/*\n\n*/x", "x", 3, 3},
		{"after nested block comment", "/* a\n/* b */\n*/ z", "z", 3, 4},
		{"after unicode identifier", "日本語 x", "x", 1, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			toks := lex(t, tc.src)
			var found *Token
			for i := range toks {
				if toks[i].Type == TokenIdent && toks[i].Lexeme == tc.lastIdent {
					found = &toks[i]
				}
			}
			if found == nil {
				t.Fatalf("identifier %q not found in %s", tc.lastIdent, typeNames(toks))
			}
			if found.Pos.Line != tc.line || found.Pos.Column != tc.col {
				t.Errorf("%q at %d:%d, want %d:%d",
					tc.lastIdent, found.Pos.Line, found.Pos.Column, tc.line, tc.col)
			}
		})
	}
}

func TestTokenize_TokenEndPosition(t *testing.T) {
	toks := lex(t, "foo bar")
	if toks[0].End.Column != 4 || toks[0].End.Offset != 3 {
		t.Errorf("End of 'foo' = %d:%d offset %d, want column 4 offset 3",
			toks[0].End.Line, toks[0].End.Column, toks[0].End.Offset)
	}
}

func TestPosition_StringAndIsValid(t *testing.T) {
	p := Position{File: "main.dot", Line: 3, Column: 14, Offset: 42}
	if got := p.String(); got != "main.dot:3:14" {
		t.Errorf("String() = %q, want %q", got, "main.dot:3:14")
	}
	if !p.IsValid() {
		t.Error("IsValid() = false, want true")
	}
	if (Position{}).IsValid() {
		t.Error("zero Position.IsValid() = true, want false")
	}
}

func TestTokenize_DefaultFilename(t *testing.T) {
	toks, err := Tokenize("x", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if toks[0].Pos.File != "<input>" {
		t.Errorf("File = %q, want %q", toks[0].Pos.File, "<input>")
	}
}

// ---------------------------------------------------------------------------
// EOF invariant
// ---------------------------------------------------------------------------

func TestTokenize_EOFInvariant(t *testing.T) {
	sources := []string{
		"", "\n", "\n\n\n", "   ", "// only a comment", "/* only a comment */",
		"x", "x\n", "x\n\n\n", "x +", "x = ", "(", "(\n", "[\n1", "{", "}",
		"fn main() {\n}\n", "\n\n// lead\nx\n\n", "`unterminated raw",
		"\"unterminated", "0x", ";", "@",
	}
	for _, src := range sources {
		t.Run(strings.ReplaceAll(src, "\n", "\\n"), func(t *testing.T) {
			toks, _ := Tokenize(src, "test.dot")
			if toks == nil {
				t.Fatal("Tokenize returned a nil slice")
			}
			if len(toks) < 2 {
				t.Fatalf("got %d tokens %s, want at least 2", len(toks), typeNames(toks))
			}
			if toks[len(toks)-1].Type != TokenEOF {
				t.Errorf("last token is %v, want TokenEOF (%s)", toks[len(toks)-1].Type, typeNames(toks))
			}
			if toks[len(toks)-2].Type != TokenNewline {
				t.Errorf("second-to-last token is %v, want TokenNewline (%s)",
					toks[len(toks)-2].Type, typeNames(toks))
			}
			for i, tok := range toks[:len(toks)-1] {
				if tok.Type == TokenEOF {
					t.Errorf("TokenEOF at index %d, want it only at the end (%s)", i, typeNames(toks))
				}
			}
			if len(toks) >= 3 && toks[len(toks)-3].Type == TokenNewline {
				t.Errorf("two newlines before EOF, want exactly one (%s)", typeNames(toks))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// error collection
// ---------------------------------------------------------------------------

func TestTokenize_UnexpectedCharacter(t *testing.T) {
	src := "a § b"
	toks, list := lexBad(t, src)
	le := firstError(t, list)
	if !strings.Contains(le.Message, "unexpected character") {
		t.Errorf("Message = %q, want it to mention 'unexpected character'", le.Message)
	}
	assertTypes(t, src, toks, TokenIdent, TokenIllegal, TokenIdent)
}

func TestTokenize_CollectsMultipleErrors(t *testing.T) {
	src := "a ; b ; c && d"
	_, list := lexBad(t, src)
	if list.Len() != 3 {
		t.Errorf("got %d errors, want 3:\n%v", list.Len(), list)
	}
	if list.Truncated {
		t.Error("Truncated = true, want false")
	}
}

func TestTokenize_ErrorCapAt100(t *testing.T) {
	src := strings.Repeat(";", 250)
	_, list := lexBad(t, src)
	if list.Len() != doterr.MaxErrors {
		t.Errorf("got %d errors, want %d", list.Len(), doterr.MaxErrors)
	}
	if !list.Truncated {
		t.Error("Truncated = false, want true")
	}
}

// ---------------------------------------------------------------------------
// Token.String
// ---------------------------------------------------------------------------

func TestToken_String(t *testing.T) {
	toks := lex(t, "foo")
	if got, want := toks[0].String(), "TokenIdent(foo) at test.dot:1:1"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	toks = lex(t, "fn")
	if got, want := toks[0].String(), "TokenFn at test.dot:1:1"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// golden sample program
// ---------------------------------------------------------------------------

func TestTokenize_SampleProgram(t *testing.T) {
	const path = "../testdata/lexer_sample.dot"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	toks, lexErr := Tokenize(string(data), "lexer_sample.dot")
	if lexErr != nil {
		t.Fatalf("%s must lex without errors, got:\n%v", path, lexErr)
	}
	for i, tok := range toks {
		if tok.Type == TokenIllegal {
			t.Errorf("TokenIllegal at index %d (%s), lexeme %q", i, tok.Pos, tok.Lexeme)
		}
	}
	if len(toks) < 200 {
		t.Errorf("got only %d tokens, expected a substantial program", len(toks))
	}
	if toks[len(toks)-1].Type != TokenEOF || toks[len(toks)-2].Type != TokenNewline {
		t.Errorf("stream does not end with NEWLINE EOF: %v %v",
			toks[len(toks)-2].Type, toks[len(toks)-1].Type)
	}
}

func TestTokenize_SmokeProgram(t *testing.T) {
	const path = "../testdata/smoke.dot"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("reading %s: %v", path, err)
	}
	if _, lexErr := Tokenize(string(data), "smoke.dot"); lexErr != nil {
		t.Errorf("%s must lex without errors, got:\n%v", path, lexErr)
	}
}
