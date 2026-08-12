package codegen

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/types"
)

// TestStringify_Empty verifies that an empty string literal stringifies to
// the empty text, not to its raw source with quotes ("" has zero parts).
func TestStringify_Empty(t *testing.T) {
	lit := &ast.StringLit{Raw: `""`}
	if got := stringify(lit); got != "" {
		t.Errorf("stringify(`\"\"`) = %q, want \"\"", got)
	}
}

// TestStringify_NonEmpty verifies that a plain non-interpolated literal
// stringifies to its decoded text.
func TestStringify_NonEmpty(t *testing.T) {
	lit := &ast.StringLit{Parts: []ast.StringPart{{Kind: ast.PartText, Text: "ok"}}}
	if got := stringify(lit); got != "ok" {
		t.Errorf("stringify(non-empty) = %q, want %q", got, "ok")
	}
}

// TestCLiteral_EmptyString verifies that an empty string literal emits a
// dot_string_from_lit call with an empty payload and zero length.
func TestCLiteral_EmptyString(t *testing.T) {
	lit := &ast.StringLit{Raw: `""`}
	got := cLiteral(nil, lit, types.String_)
	if want := `dot_string_from_lit("", 0)`; !strings.Contains(got, want) {
		t.Errorf("cLiteral(`\"\"`) = %q, want it to contain %q", got, want)
	}
}
