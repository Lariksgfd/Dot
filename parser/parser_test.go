package parser

import (
	"strings"
	"testing"
	"time"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/errors"
)

// mustParse parses source and returns the program. It fails the test if there
// are any diagnostics.
func mustParse(t *testing.T, src string) *ast.Program {
	t.Helper()
	prog, err := ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("ParseFile(%q) returned error: %v", src, err)
	}
	return prog
}

// parseErrs parses source and returns the collected diagnostic messages.
func parseErrs(t *testing.T, src string) []string {
	t.Helper()
	_, err := ParseFile(src, "test.dot")
	if err == nil {
		return nil
	}
	list, ok := err.(*errors.ErrorList)
	if !ok {
		return []string{err.Error()}
	}
	var msgs []string
	for _, e := range list.Errors {
		msgs = append(msgs, e.Error())
	}
	return msgs
}

// mustParseWithErrs parses source expecting at least one diagnostic. It fails
// if there are no errors.
func mustParseWithErrs(t *testing.T, src string) (*ast.Program, []string) {
	t.Helper()
	prog, err := ParseFile(src, "test.dot")
	if err == nil {
		t.Fatalf("ParseFile(%q) expected errors, got none", src)
	}
	list, ok := err.(*errors.ErrorList)
	if !ok {
		t.Fatalf("ParseFile(%q) returned non-ErrorList: %T", src, err)
	}
	var msgs []string
	for _, e := range list.Errors {
		msgs = append(msgs, e.Error())
	}
	return prog, msgs
}

// terminatesWithin runs f and reports whether it returned before the timeout.
// Used to guarantee pathological inputs do not hang the parser.
func terminatesWithin(t *testing.T, name string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not terminate within 5s (possible infinite loop)", name)
	}
}

// exprTree parses src as a complete expression (wrapped so it is a valid
// top-level statement) and returns the printed AST of the expression.
func exprTree(t *testing.T, src string) string {
	t.Helper()
	prog := mustParse(t, src+"\n")
	if len(prog.Decls) == 0 {
		t.Fatalf("no declarations parsed from %q", src)
	}
	vd, ok := prog.Decls[0].(*ast.VarDecl)
	if !ok {
		t.Fatalf("first decl of %q is %T, expected VarDecl", src, prog.Decls[0])
	}
	if len(vd.Values) == 0 {
		t.Fatalf("VarDecl from %q has no values", src)
	}
	return ast.Print(vd.Values[0])
}

// countBad walks n and counts every *BadExpr/*BadStmt/*BadType/*BadPattern.
func countBad(n ast.Node) (cnt int) {
	ast.Inspect(n, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.BadExpr, *ast.BadStmt, *ast.BadType, *ast.BadPattern:
			cnt++
		}
		return true
	})
	return
}

// hasBad reports whether n contains any Bad* node.
func hasBad(n ast.Node) bool {
	return countBad(n) > 0
}

// expectTree parses src as an expression and asserts its printed AST equals
// want.
func expectTree(t *testing.T, src, want string) {
	t.Helper()
	got := exprTree(t, src)
	if got != want {
		t.Errorf("expression %q:\n got:\n%s\nwant:\n%s", src, got, want)
	}
}

func TestParseFile_Empty(t *testing.T) {
	prog, err := ParseFile("", "test.dot")
	if err != nil {
		t.Fatalf("empty file returned error: %v", err)
	}
	if len(prog.Decls) != 0 {
		t.Errorf("empty file: got %d decls, want 0", len(prog.Decls))
	}
}

func TestParseFile_OnlyComments(t *testing.T) {
	src := "// a comment\n/* block */\n"
	prog, err := ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("comments-only file returned error: %v", err)
	}
	if len(prog.Decls) != 0 {
		t.Errorf("comments-only file: got %d decls, want 0", len(prog.Decls))
	}
}

func TestParseFile_OnlyImports(t *testing.T) {
	src := "import a\nimport b.c\nfrom d.e import f, g\n"
	prog := mustParse(t, src)
	if len(prog.Decls) != 3 {
		t.Fatalf("import-only file: got %d decls, want 3", len(prog.Decls))
	}
	for i, d := range prog.Decls {
		if _, ok := d.(*ast.ImportDecl); !ok {
			t.Errorf("decl %d is %T, expected ImportDecl", i, d)
		}
	}
}

func TestParseFile_MixedDecls(t *testing.T) {
	src := "const X = 1\nfn f() {}\nstruct S {}\n"
	prog := mustParse(t, src)
	if len(prog.Decls) != 3 {
		t.Fatalf("mixed decls: got %d, want 3", len(prog.Decls))
	}
	if _, ok := prog.Decls[0].(*ast.VarDecl); !ok {
		t.Errorf("decl 0 = %T, want VarDecl", prog.Decls[0])
	}
	if _, ok := prog.Decls[1].(*ast.FnDecl); !ok {
		t.Errorf("decl 1 = %T, want FnDecl", prog.Decls[1])
	}
	if _, ok := prog.Decls[2].(*ast.StructDecl); !ok {
		t.Errorf("decl 2 = %T, want StructDecl", prog.Decls[2])
	}
}

func TestStmtTerminator_MissingNewline(t *testing.T) {
	// Two statements on one line without a terminator: the second is a
	// diagnostic.
	_, msgs := mustParseWithErrs(t, "x = 1 y = 2\n")
	if len(msgs) == 0 {
		t.Fatal("expected a diagnostic for missing statement terminator")
	}
}

func TestStmtTerminator_NewlineOK(t *testing.T) {
	mustParse(t, "x = 1\ny = 2\n")
}

func TestErrorRecovery_MultipleDiagnostics(t *testing.T) {
	// Several broken declarations in a row should each produce a diagnostic,
	// not just the first.
	src := "fn { }\nfn { }\nfn { }\n"
	_, msgs := mustParseWithErrs(t, src)
	if len(msgs) < 3 {
		t.Errorf("expected at least 3 diagnostics, got %d: %v", len(msgs), msgs)
	}
}

func TestErrorRecovery_ReturnsProgram(t *testing.T) {
	// After a broken fn declaration, the parser should recover and parse the
	// following valid declarations. NOTE: this test currently FAILS due to a
	// syncDecl bug — see the bug report. It is kept as a regression test.
	src := "fn { }\nx = 1\n"
	prog, msgs := mustParseWithErrs(t, src)
	if prog == nil {
		t.Fatal("parser returned nil program on error")
	}
	if len(msgs) == 0 {
		t.Fatal("expected at least one diagnostic")
	}
	// The valid declaration after the error should still be present.
	if len(prog.Decls) < 2 {
		t.Errorf("expected at least 2 decls after recovery, got %d", len(prog.Decls))
	}
}

func TestErrorRecovery_VarDeclRecovered(t *testing.T) {
	// A broken fn followed by a const should recover (const is a decl-sync
	// keyword).
	src := "fn { }\nconst Y = 2\n"
	prog, msgs := mustParseWithErrs(t, src)
	if prog == nil {
		t.Fatal("parser returned nil program on error")
	}
	if len(msgs) == 0 {
		t.Fatal("expected at least one diagnostic")
	}
	if len(prog.Decls) < 2 {
		t.Errorf("expected at least 2 decls, got %d", len(prog.Decls))
	}
}

func TestNoInfiniteLoop_Braces(t *testing.T) {
	terminatesWithin(t, "}}}}", func() {
		ParseFile("}}}}\n", "test.dot")
	})
}

func TestNoInfiniteLoop_Parens(t *testing.T) {
	terminatesWithin(t, "((((", func() {
		ParseFile("((((\n", "test.dot")
	})
}

func TestNoInfiniteLoop_Keywords(t *testing.T) {
	terminatesWithin(t, "fn fn fn", func() {
		ParseFile("fn fn fn\n", "test.dot")
	})
}

func TestNoInfiniteLoop_EveryPrefix(t *testing.T) {
	// Every prefix of the sample file must terminate.
	const src = "import a\nconst X = 1\nfn f() {}\n"
	for i := 1; i <= len(src); i++ {
		prefix := src[:i]
		terminatesWithin(t, "prefix", func() {
			ParseFile(prefix, "test.dot")
		})
	}
}

func TestParseFile_ZeroImports(t *testing.T) {
	prog := mustParse(t, "x = 1\n")
	if len(prog.Imports()) != 0 {
		t.Errorf("got %d imports, want 0", len(prog.Imports()))
	}
}

func TestParseFile_ImportsProgram(t *testing.T) {
	prog := mustParse(t, "import a.b\nimport c as d\n")
	imports := prog.Imports()
	if len(imports) != 2 {
		t.Fatalf("got %d imports, want 2", len(imports))
	}
	if imports[0].PathString() != "a.b" {
		t.Errorf("import 0 path = %q, want a.b", imports[0].PathString())
	}
	if imports[1].Alias != "d" {
		t.Errorf("import 1 alias = %q, want d", imports[1].Alias)
	}
}

func TestParseErrs_HasMessages(t *testing.T) {
	msgs := parseErrs(t, "fn { }\n")
	if len(msgs) == 0 {
		t.Fatal("expected error messages, got none")
	}
	for i, m := range msgs {
		if m == "" {
			t.Errorf("message %d is empty", i)
		}
	}
}

func TestParseFile_RawString(t *testing.T) {
	// Raw strings may span lines and carry no escapes.
	prog := mustParse(t, "x = `line1\nline2`\n")
	vd := prog.Decls[0].(*ast.VarDecl)
	if len(vd.Values) != 1 {
		t.Fatalf("got %d values, want 1", len(vd.Values))
	}
	if _, ok := vd.Values[0].(*ast.RawStringLit); !ok {
		t.Errorf("value is %T, want RawStringLit", vd.Values[0])
	}
}

func TestParseFile_StructLiteralInAssign(t *testing.T) {
	prog := mustParse(t, "p = Point { x: 1, y: 2 }\n")
	vd := prog.Decls[0].(*ast.VarDecl)
	if _, ok := vd.Values[0].(*ast.StructLit); !ok {
		t.Errorf("value is %T, want StructLit", vd.Values[0])
	}
}

func TestParseFile_ExprPrintNonEmpty(t *testing.T) {
	prog := mustParse(t, "x = 1 + 2\n")
	out := ast.Print(prog)
	if out == "" {
		t.Fatal("Print returned empty string")
	}
	if !strings.Contains(out, "VarDecl") {
		t.Errorf("Print output missing VarDecl:\n%s", out)
	}
}
