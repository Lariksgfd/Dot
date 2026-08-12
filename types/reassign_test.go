package types

import (
	"testing"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/parser"
)

// TestReassign_SameScope checks that `x = expr` after `x = 42` in the same
// scope is parsed as a reassignment (ExprStmt wrapping an AssignExpr), not as
// a second VarDecl (D53).
func TestReassign_SameScope(t *testing.T) {
	prog, err := parser.ParseFile("fn main() { x = 42\n x = 43 }", "<test>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn, ok := prog.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected fn decl, got %T", prog.Decls[0])
	}
	if len(fn.Body.Stmts) != 2 {
		t.Fatalf("expected 2 stmts, got %d", len(fn.Body.Stmts))
	}
	if _, ok := fn.Body.Stmts[0].(*ast.VarDecl); !ok {
		t.Errorf("first statement should be a VarDecl, got %T", fn.Body.Stmts[0])
	}
	exprStmt, ok := fn.Body.Stmts[1].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("second statement should be an ExprStmt (reassignment), got %T", fn.Body.Stmts[1])
	}
	if _, ok := exprStmt.X.(*ast.AssignExpr); !ok {
		t.Errorf("expected AssignExpr, got %T", exprStmt.X)
	}
}

func TestReassign_InScope_TypeChecks(t *testing.T) {
	checkDot(t, "fn main() { x = 42\n x = 43 }")
}

func TestReassign_StringConcatInScope(t *testing.T) {
	checkDot(t, "fn main() { s = \"a\"\n s = s + \"b\" }")
}

func TestReassign_InLoop(t *testing.T) {
	checkDot(t, "fn main() { for i in 0..3 { i = i + 1 } }")
}

func TestReassign_Swap(t *testing.T) {
	checkDot(t, "fn main() { a = 1\n b = 2\n a, b = b, a }")
}

func TestReassign_UnderscoreMulti(t *testing.T) {
	checkDot(t, "fn f() -> (int, string) { return (1, \"x\") }\n fn main() { err = \"\"\n _, err = f() }")
}

func TestReassign_TypeMismatch(t *testing.T) {
	checkDotError(t, "fn main() { x = 42\n x = \"str\" }", "cannot use string as int in assignment")
}

func TestReassign_TypeMismatchLoopVar(t *testing.T) {
	checkDotError(t, "fn main() { for i in 0..3 { i = \"oops\" } }", "cannot use string as int in assignment")
}

func TestReassign_ShadowInnerScope(t *testing.T) {
	checkDot(t, "fn main() { x = 42\n if true { x = \"str\" } }")
}

// TestBinary_StringEqReturnsBool verifies that string == yields bool in
// info.Types.
func TestBinary_StringEqReturnsBool(t *testing.T) {
	src := "fn main() { a = \"x\"\n b = a == \"x\"\n c = a != \"y\" }"
	prog, err := parser.ParseFile(src, "<test>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "<test>")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	fn, ok := prog.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected fn decl, got %T", prog.Decls[0])
	}
	want := map[string]*ast.BinaryExpr{}
	for _, stmt := range fn.Body.Stmts {
		vd, ok := stmt.(*ast.VarDecl)
		if !ok {
			continue
		}
		id, ok := vd.Names[0].(*ast.Ident)
		if !ok {
			continue
		}
		bin, ok := vd.Values[0].(*ast.BinaryExpr)
		if !ok {
			continue
		}
		want[id.Name] = bin
	}
	for _, name := range []string{"b", "c"} {
		bin, ok := want[name]
		if !ok {
			t.Fatalf("binary expr for %s not found", name)
		}
		if got := info.Types[bin]; got != Bool {
			t.Errorf("string comparison %s should have type bool, got %v", name, got)
		}
	}
}
