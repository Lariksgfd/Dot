package lexer

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// numCase describes one expected numeric token.
type numCase struct {
	src    string
	typ    TokenType
	value  string
	base   int
	suffix string
}

func checkNumber(t *testing.T, tc numCase, tok Token) {
	t.Helper()
	if tok.Type != tc.typ {
		t.Errorf("Type = %v, want %v", tok.Type, tc.typ)
	}
	if tok.Value != tc.value {
		t.Errorf("Value = %q, want %q", tok.Value, tc.value)
	}
	if tok.Base != tc.base {
		t.Errorf("Base = %d, want %d", tok.Base, tc.base)
	}
	if tok.Suffix != tc.suffix {
		t.Errorf("Suffix = %q, want %q", tok.Suffix, tc.suffix)
	}
	if tok.Lexeme != tc.src {
		t.Errorf("Lexeme = %q, want %q", tok.Lexeme, tc.src)
	}
}

func TestScanNumber_IntegerLiterals(t *testing.T) {
	cases := []numCase{
		{"0", TokenInt, "0", 10, ""},
		{"42", TokenInt, "42", 10, ""},
		{"9223372036854775807", TokenInt, "9223372036854775807", 10, ""},
		{"0xFF", TokenInt, "FF", 16, ""},
		{"0XFF", TokenInt, "FF", 16, ""},
		{"0xdeadBEEF", TokenInt, "deadBEEF", 16, ""},
		{"0b1010", TokenInt, "1010", 2, ""},
		{"0B1010", TokenInt, "1010", 2, ""},
		{"0o755", TokenInt, "755", 8, ""},
		{"0O77", TokenInt, "77", 8, ""},
		{"1_000_000", TokenInt, "1000000", 10, ""},
		{"0xFF_FF", TokenInt, "FFFF", 16, ""},
		{"0b1010_1010", TokenInt, "10101010", 2, ""},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, tc.typ)
			checkNumber(t, tc, toks[0])
		})
	}
}

func TestScanNumber_UnitSuffixes(t *testing.T) {
	cases := []numCase{
		{"1_mb", TokenInt, "1", 10, "mb"},
		{"10kb", TokenInt, "10", 10, "kb"},
		{"2em", TokenInt, "2", 10, "em"},
		{"1_000_kb", TokenInt, "1000", 10, "kb"},
		{"3ms", TokenInt, "3", 10, "ms"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, tc.typ)
			checkNumber(t, tc, toks[0])
		})
	}
}

func TestScanNumber_FloatLiterals(t *testing.T) {
	cases := []struct {
		src  string
		num  float64
		want string // exact expected Value, "" to skip the exact check
	}{
		{"3.14", 3.14, "3.14"},
		{"0.0", 0, "0.0"},
		{"1.0e10", 1.0e10, "1.0e10"},
		{"1e-9", 1e-9, "1e-9"},
		{"1e10", 1e10, "1e10"},
		{"2.5E+3", 2.5e3, ""},
		{"1_000.5", 1000.5, "1000.5"},
		{"1.5e0", 1.5, "1.5e0"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, TokenFloat)
			tok := toks[0]
			if tok.Base != 10 {
				t.Errorf("Base = %d, want 10", tok.Base)
			}
			if tok.Lexeme != tc.src {
				t.Errorf("Lexeme = %q, want %q", tok.Lexeme, tc.src)
			}
			got, err := strconv.ParseFloat(tok.Value, 64)
			if err != nil {
				t.Fatalf("Value %q is not a parsable float: %v", tok.Value, err)
			}
			if math.Abs(got-tc.num) > 1e-12*math.Max(1, math.Abs(tc.num)) {
				t.Errorf("Value = %q (%v), want %v", tok.Value, got, tc.num)
			}
			if tc.want != "" && tok.Value != tc.want {
				t.Errorf("Value = %q, want %q", tok.Value, tc.want)
			}
		})
	}
}

// TestScanNumber_LeadingDotIsNotAFloat pins design decision D3: ".5" is not a
// valid Dot float literal, it lexes as TokenDot followed by TokenInt.
func TestScanNumber_LeadingDotIsNotAFloat(t *testing.T) {
	src := ".5"
	toks := lex(t, src)
	assertTypes(t, src, toks, TokenDot, TokenInt)
	if toks[1].Value != "5" {
		t.Errorf("Value = %q, want %q", toks[1].Value, "5")
	}
}

func TestScanNumber_RangeVsFloat(t *testing.T) {
	cases := []struct {
		src    string
		types  []TokenType
		values []string
	}{
		{"0..10", []TokenType{TokenInt, TokenDotDot, TokenInt}, []string{"0", "..", "10"}},
		{"0..=10", []TokenType{TokenInt, TokenDotDotEq, TokenInt}, []string{"0", "..=", "10"}},
		{"1.5", []TokenType{TokenFloat}, []string{"1.5"}},
		{"2.sqrt()", []TokenType{TokenInt, TokenDot, TokenIdent, TokenLParen, TokenRParen},
			[]string{"2", ".", "sqrt", "(", ")"}},
		{"1.", []TokenType{TokenInt, TokenDot}, []string{"1", "."}},
		{"1...5", []TokenType{TokenInt, TokenEllipsis, TokenInt}, []string{"1", "...", "5"}},
		{"for i in 0..n", []TokenType{TokenFor, TokenIdent, TokenIn, TokenInt, TokenDotDot, TokenIdent},
			nil},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, tc.types...)
			for i, want := range tc.values {
				if toks[i].Value != want {
					t.Errorf("token %d Value = %q, want %q", i, toks[i].Value, want)
				}
			}
		})
	}
}

func TestScanNumber_TupleIndexing(t *testing.T) {
	cases := []struct {
		src    string
		types  []TokenType
		values []string
	}{
		{"t.0", []TokenType{TokenIdent, TokenDot, TokenInt}, []string{"t", ".", "0"}},
		{"t.0.1", []TokenType{TokenIdent, TokenDot, TokenInt, TokenDot, TokenInt},
			[]string{"t", ".", "0", ".", "1"}},
		{"t.1.0.2", []TokenType{TokenIdent, TokenDot, TokenInt, TokenDot, TokenInt,
			TokenDot, TokenInt}, nil},
		// afterDot suppresses the exponent, so "e5" becomes a unit suffix.
		{"t.0e5", []TokenType{TokenIdent, TokenDot, TokenInt}, []string{"t", ".", "0"}},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			toks := lex(t, tc.src)
			assertTypes(t, tc.src, toks, tc.types...)
			for i, want := range tc.values {
				if toks[i].Value != want {
					t.Errorf("token %d Value = %q, want %q", i, toks[i].Value, want)
				}
			}
		})
	}
}

func TestScanNumber_InExpressions(t *testing.T) {
	src := "x = 1 + 2 * 0xFF - 3.5"
	toks := lex(t, src)
	assertTypes(t, src, toks,
		TokenIdent, TokenAssign, TokenInt, TokenPlus, TokenInt, TokenStar,
		TokenInt, TokenMinus, TokenFloat)
}

func TestScanNumber_Errors(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		message string
		nerr    int
	}{
		{"binary digit out of range", "0b12", "invalid digit for a binary literal", 1},
		{"octal digit out of range", "0o78", "invalid digit for a octal literal", 1},
		{"hex prefix without digits", "0x", "hexadecimal literal has no digits", 1},
		{"binary prefix without digits", "0b", "binary literal has no digits", 1},
		{"octal prefix without digits", "0o", "octal literal has no digits", 1},
		{"doubled separator", "1__0", "digit separator '_' repeated", 1},
		{"trailing separator", "1_", "digit separator '_' must appear between digits", 1},
		{"exponent without digits", "1e", "exponent has no digits", 1},
		{"signed exponent without digits", "1e+", "exponent has no digits", 1},
		{"two decimal points", "1.2.3", "more than one decimal point", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			toks, list := lexBad(t, tc.src)
			if list.Len() != tc.nerr {
				t.Errorf("got %d errors, want %d:\n%v", list.Len(), tc.nerr, list)
			}
			le := firstError(t, list)
			if !strings.Contains(le.Message, tc.message) {
				t.Errorf("Message = %q, want it to contain %q", le.Message, tc.message)
			}
			// Recovery: a numeric token is still emitted.
			if toks[0].Type != TokenInt && toks[0].Type != TokenFloat {
				t.Errorf("first token is %v, want a numeric token (%s)", toks[0].Type, typeNames(toks))
			}
		})
	}
}

func TestScanNumber_ErrorRecoveryContinuesScanning(t *testing.T) {
	src := "a = 0b12\nb = 1__0\nc = 42"
	toks, list := lexBad(t, src)
	if list.Len() != 2 {
		t.Errorf("got %d errors, want 2:\n%v", list.Len(), list)
	}
	assertTypes(t, src, toks,
		TokenIdent, TokenAssign, TokenInt, TokenNewline,
		TokenIdent, TokenAssign, TokenInt, TokenNewline,
		TokenIdent, TokenAssign, TokenInt)
	if last := toks[10]; last.Value != "42" {
		t.Errorf("last int Value = %q, want %q", last.Value, "42")
	}
}

func TestScanNumber_ZeroBaseForNonNumbers(t *testing.T) {
	toks := lex(t, `foo "bar"`)
	for _, tok := range toks {
		if tok.Base != 0 {
			t.Errorf("%v has Base = %d, want 0", tok.Type, tok.Base)
		}
	}
}
