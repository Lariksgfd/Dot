package parser

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// parseStringLit parses src as a string literal (wrapped in a VarDecl) and
// returns the *ast.StringLit.
func parseStringLit(t *testing.T, src string) *ast.StringLit {
	t.Helper()
	prog := mustParse(t, "s = "+src+"\n")
	vd := prog.Decls[0].(*ast.VarDecl)
	sl, ok := vd.Values[0].(*ast.StringLit)
	if !ok {
		t.Fatalf("%s: value = %T, want StringLit", src, vd.Values[0])
	}
	return sl
}

func TestInterpolation_Simple(t *testing.T) {
	sl := parseStringLit(t, `"hello {name}"`)
	if len(sl.Parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(sl.Parts))
	}
	if sl.Parts[0].Kind != ast.PartText || sl.Parts[0].Text != "hello " {
		t.Errorf("part[0] = %+v, want PartText 'hello '", sl.Parts[0])
	}
	if sl.Parts[1].Kind != ast.PartExpr {
		t.Errorf("part[1].Kind = %v, want PartExpr", sl.Parts[1].Kind)
	}
	if sl.Parts[1].Expr == nil {
		t.Errorf("part[1].Expr is nil")
	}
	if id, ok := sl.Parts[1].Expr.(*ast.Ident); !ok || id.Name != "name" {
		t.Errorf("part[1].Expr = %#v, want Ident(name)", sl.Parts[1].Expr)
	}
}

func TestInterpolation_AbsolutePosition(t *testing.T) {
	// The interpolated expression's position is absolute in the source file.
	// The full source is: s = "a{b}c"
	// col:               123456789...
	// The string starts at col 5; "a"=5, "{"=6, "b"=7. The expr `b` is at col 7.
	src := `"a{b}c"`
	sl := parseStringLit(t, src)
	if len(sl.Parts) != 3 {
		t.Fatalf("parts = %d, want 3", len(sl.Parts))
	}
	exprPart := sl.Parts[1]
	if exprPart.Kind != ast.PartExpr {
		t.Fatalf("part[1] is not PartExpr")
	}
	id, ok := exprPart.Expr.(*ast.Ident)
	if !ok {
		t.Fatalf("expr = %T, want Ident", exprPart.Expr)
	}
	if id.Pos().Column != 8 {
		t.Errorf("expr column = %d, want 8", id.Pos().Column)
	}
	if id.Pos().Line != 1 {
		t.Errorf("expr line = %d, want 1", id.Pos().Line)
	}
}

func TestInterpolation_ExprWithCall(t *testing.T) {
	sl := parseStringLit(t, `"value: {f(x)}"`)
	if len(sl.Parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(sl.Parts))
	}
	call, ok := sl.Parts[1].Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expr = %T, want CallExpr", sl.Parts[1].Expr)
	}
	if _, ok := call.Fn.(*ast.Ident); !ok {
		t.Errorf("call.Fn = %T, want Ident", call.Fn)
	}
	if len(call.Args) != 1 {
		t.Errorf("call.Args = %d, want 1", len(call.Args))
	}
}

func TestInterpolation_ExprWithField(t *testing.T) {
	sl := parseStringLit(t, `"{obj.field}"`)
	if len(sl.Parts) != 1 {
		t.Fatalf("parts = %d, want 1", len(sl.Parts))
	}
	fe, ok := sl.Parts[0].Expr.(*ast.FieldExpr)
	if !ok {
		t.Fatalf("expr = %T, want FieldExpr", sl.Parts[0].Expr)
	}
	if fe.Name != "field" {
		t.Errorf("field = %q, want field", fe.Name)
	}
}

func TestInterpolation_EscapedBraces(t *testing.T) {
	// {{ and }} decode to literal { and }.
	sl := parseStringLit(t, `"a{{b}}c"`)
	if len(sl.Parts) != 1 {
		t.Fatalf("parts = %d, want 1", len(sl.Parts))
	}
	if sl.Parts[0].Kind != ast.PartText {
		t.Fatalf("part[0].Kind = %v, want PartText", sl.Parts[0].Kind)
	}
	if sl.Parts[0].Text != "a{b}c" {
		t.Errorf("part[0].Text = %q, want a{b}c", sl.Parts[0].Text)
	}
}

func TestInterpolation_EscapedNewline(t *testing.T) {
	// A string with an escaped \n (single source line) and an interpolation.
	// Source: s = "x\n{a}"
	// The interpolation starts after the \n escape, still on line 1.
	src := "\"x\\n{a}\"\n"
	sl := parseStringLit(t, src)
	if len(sl.Parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(sl.Parts))
	}
	exprPart := sl.Parts[1]
	if exprPart.Kind != ast.PartExpr {
		t.Fatalf("part[1] is not PartExpr")
	}
	if exprPart.Pos.Line != 1 {
		t.Errorf("expr line = %d, want 1", exprPart.Pos.Line)
	}
}

func TestInterpolation_Nested(t *testing.T) {
	// Nested interpolation: "a{ "b{x}" }".
	sl := parseStringLit(t, `"a{ "b{x}" }"`)
	if len(sl.Parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(sl.Parts))
	}
	inner, ok := sl.Parts[1].Expr.(*ast.StringLit)
	if !ok {
		t.Fatalf("expr = %T, want StringLit", sl.Parts[1].Expr)
	}
	if len(inner.Parts) != 2 {
		t.Fatalf("inner parts = %d, want 2", len(inner.Parts))
	}
	if _, ok := inner.Parts[1].Expr.(*ast.Ident); !ok {
		t.Errorf("inner expr = %T, want Ident", inner.Parts[1].Expr)
	}
}

func TestInterpolation_SyntaxErrorPosition(t *testing.T) {
	// A syntax error inside an interpolation produces a diagnostic at the
	// right absolute position.
	src := "x = \"{1 +}\"\n"
	_, err := lexer.Tokenize(src, "test.dot")
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	_, errs := mustParseWithErrs(t, src)
	if len(errs) == 0 {
		t.Fatalf("expected diagnostic for syntax error inside interpolation")
	}
	// The diagnostic should reference line 1, a column inside the string.
	found := false
	for _, m := range errs {
		if strings.Contains(m, "1:") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a diagnostic referencing line 1, got: %v", errs)
	}
}

func TestInterpolation_Arithmetic(t *testing.T) {
	sl := parseStringLit(t, `"{a + b}"`)
	if len(sl.Parts) != 1 {
		t.Fatalf("parts = %d, want 1", len(sl.Parts))
	}
	be, ok := sl.Parts[0].Expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expr = %T, want BinaryExpr", sl.Parts[0].Expr)
	}
	if be.Op != lexer.TokenPlus {
		t.Errorf("op = %v, want TokenPlus", be.Op)
	}
}

func TestRawString_NoInterpolation(t *testing.T) {
	prog := mustParse(t, "s = `raw {x}`\n")
	vd := prog.Decls[0].(*ast.VarDecl)
	rs, ok := vd.Values[0].(*ast.RawStringLit)
	if !ok {
		t.Fatalf("value = %T, want RawStringLit", vd.Values[0])
	}
	if rs.Value != "raw {x}" {
		t.Errorf("value = %q, want 'raw {x}'", rs.Value)
	}
}
