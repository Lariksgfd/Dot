package types

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/parser"
)

// checkCG37 parses and checks src, returning the program, the info and the
// raw Check error. The raw error is returned instead of failing so tests can
// inspect warning-only diagnostics (Check returns a non-nil ErrorList even
// when it holds only warnings).
func checkCG37(t *testing.T, src string) (*ast.Program, *Info, error) {
	t.Helper()
	prog, err := parser.ParseFile(src, "<test>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "<test>")
	if info == nil {
		t.Fatal("Check returned nil info")
	}
	return prog, info, err
}

// loopBodyVarDecl returns the first VarDecl inside the body of the ForStmt
// stmt, or fails the test.
func loopBodyVarDecl(t *testing.T, forStmt *ast.ForStmt) *ast.VarDecl {
	t.Helper()
	for _, s := range forStmt.Body.Stmts {
		if vd, ok := s.(*ast.VarDecl); ok {
			return vd
		}
	}
	t.Fatalf("no VarDecl in loop body")
	return nil
}

func identOf(t *testing.T, e ast.Expr) *ast.Ident {
	t.Helper()
	id, ok := e.(*ast.Ident)
	if !ok {
		t.Fatalf("expected *ast.Ident, got %T", e)
	}
	return id
}

// assignTargetIdent returns the target identifier of a statement that assigns
// to a plain name. The parser emits a VarDecl for the first occurrence and an
// ExprStmt(AssignExpr) for later ones in the same scope (D53).
func assignTargetIdent(t *testing.T, s ast.Stmt) *ast.Ident {
	t.Helper()
	switch stmt := s.(type) {
	case *ast.VarDecl:
		return identOf(t, stmt.Names[0])
	case *ast.ExprStmt:
		ae, ok := stmt.X.(*ast.AssignExpr)
		if !ok {
			t.Fatalf("expected AssignExpr in ExprStmt, got %T", stmt.X)
		}
		return identOf(t, ae.Targets[0])
	}
	t.Fatalf("expected VarDecl or ExprStmt, got %T", s)
	return nil
}

// TestCG37_ReassignLoopFindsFunctionVar checks that `s = s + i` inside a loop
// body reassigns the function-body local instead of shadowing it: the name
// has no Defs entry and is recorded as a use of the outer symbol (CG-37).
func TestCG37_ReassignLoopFindsFunctionVar(t *testing.T) {
	src := `fn f(n int) -> int {
    s = 10
    for i in 1..n {
        s = s + i
    }
    return s
}

fn main() {}`
	prog, info, err := checkCG37(t, src)
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	fn := prog.Decls[0].(*ast.FnDecl)
	outerDecl := fn.Body.Stmts[0].(*ast.VarDecl)
	outerIdent := identOf(t, outerDecl.Names[0])
	outerSym := info.Defs[outerIdent]
	if outerSym == nil {
		t.Fatal("outer s has no Defs entry")
	}

	forStmt, ok := fn.Body.Stmts[1].(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected ForStmt, got %T", fn.Body.Stmts[1])
	}
	innerDecl := loopBodyVarDecl(t, forStmt)
	innerIdent := identOf(t, innerDecl.Names[0])

	if def := info.Defs[innerIdent]; def != nil {
		t.Errorf("loop-body reassignment recorded a Defs entry, want nil (reassignment, not declaration)")
	}
	if use := info.Uses[innerIdent]; use != outerSym {
		t.Errorf("loop-body reassignment Uses = %v, want the function-body symbol %v", use, outerSym)
	}
	if use := info.Uses[innerIdent]; use == nil || use.Type != Int {
		t.Errorf("loop-body reassignment symbol type = %v, want int", use.Type)
	}

	// The RHS `s` of `s = s + i` must also resolve to the outer symbol.
	bin, ok := innerDecl.Values[0].(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", innerDecl.Values[0])
	}
	if use := info.Uses[identOf(t, bin.X)]; use != outerSym {
		t.Errorf("RHS s resolves to %v, want the function-body symbol %v", use, outerSym)
	}
}

// TestCG37_ReassignLoopFindsMatchArmVar checks the arm-local variant: a loop
// nested inside a block match arm reassigns the variable declared in that arm
// (a loop body is not a shadowing scope, but arm blocks still are).
func TestCG37_ReassignLoopFindsMatchArmVar(t *testing.T) {
	src := `fn f(n int) -> int {
    match n {
        0 => 0
        _ => {
            s = 100
            for i in 1..n {
                s = s + i
            }
            return s
        }
    }
}

fn main() {}`
	prog, info, err := checkCG37(t, src)
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	fn := prog.Decls[0].(*ast.FnDecl)
	matchExpr := fn.Body.Stmts[0].(*ast.ExprStmt).X.(*ast.MatchExpr)
	armBlock := matchExpr.Arms[1].Body.(*ast.BlockExpr)

	armDecl := armBlock.Block.Stmts[0].(*ast.VarDecl)
	armIdent := identOf(t, armDecl.Names[0])
	armSym := info.Defs[armIdent]
	if armSym == nil {
		t.Fatal("arm-local s has no Defs entry")
	}

	forStmt := armBlock.Block.Stmts[1].(*ast.ForStmt)
	innerDecl := loopBodyVarDecl(t, forStmt)
	innerIdent := identOf(t, innerDecl.Names[0])

	if def := info.Defs[innerIdent]; def != nil {
		t.Errorf("loop-body reassignment recorded a Defs entry, want nil")
	}
	if use := info.Uses[innerIdent]; use != armSym {
		t.Errorf("loop-body reassignment Uses = %v, want the match-arm symbol %v", use, armSym)
	}
}

// TestCG37_ReassignLoopFirstDeclIsLoopLocal checks that the first assignment
// of a brand-new name inside a loop body declares it in the loop scope, not
// in the enclosing function scope.
func TestCG37_ReassignLoopFirstDeclIsLoopLocal(t *testing.T) {
	src := `fn main() {
    for i in 0..3 {
        s = 1
    }
}`
	prog, info, err := checkCG37(t, src)
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	fn := prog.Decls[0].(*ast.FnDecl)
	forStmt := fn.Body.Stmts[0].(*ast.ForStmt)
	innerDecl := loopBodyVarDecl(t, forStmt)
	innerIdent := identOf(t, innerDecl.Names[0])

	def := info.Defs[innerIdent]
	if def == nil {
		t.Fatal("first loop-body assignment should declare a symbol, Defs is nil")
	}
	if def.Name != "s" {
		t.Errorf("declared name = %q, want %q", def.Name, "s")
	}
	if use := info.Uses[innerIdent]; use != nil {
		t.Errorf("fresh declaration should have no Uses entry, got %v", use)
	}

	loopScope, ok := info.Scopes[forStmt]
	if !ok {
		t.Fatal("loop scope not recorded")
	}
	if sym, ok := loopScope.LookupLocal("s"); !ok || sym != def {
		t.Errorf("loop scope does not own the declared symbol (sym=%v ok=%v)", sym, ok)
	}
	fnScope, ok := info.Scopes[fn]
	if !ok {
		t.Fatal("function scope not recorded")
	}
	if sym, ok := fnScope.LookupLocal("s"); ok {
		t.Errorf("function scope unexpectedly owns %q (%v), want it declared in the loop scope", "s", sym)
	}
}

// TestCG37_ReassignLoopShadowsGlobal checks that the outward walk from a loop
// body stops at the function scope: a name that exists only at file scope is
// shadowed (D53 rule S), not reassigned.
func TestCG37_ReassignLoopShadowsGlobal(t *testing.T) {
	src := `g = 100

fn main() {
    for i in 0..1 {
        g = 1
    }
}`
	prog, info, err := checkCG37(t, src)
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	globalDecl := prog.Decls[0].(*ast.VarDecl)
	globalIdent := identOf(t, globalDecl.Names[0])
	globalSym := info.Defs[globalIdent]
	if globalSym == nil {
		t.Fatal("global g has no Defs entry")
	}

	fn := prog.Decls[1].(*ast.FnDecl)
	forStmt := fn.Body.Stmts[0].(*ast.ForStmt)
	innerDecl := loopBodyVarDecl(t, forStmt)
	innerIdent := identOf(t, innerDecl.Names[0])

	def := info.Defs[innerIdent]
	if def == nil {
		t.Fatal("loop-body g should shadow the global with a fresh declaration, Defs is nil")
	}
	if def == globalSym {
		t.Error("loop-body g reassigned the global; the walk must stop at the function scope")
	}
	if use := info.Uses[innerIdent]; use != nil {
		t.Errorf("fresh shadow should have no Uses entry, got %v", use)
	}
}

// TestCG37_MatchNeverArmMergesWithValue checks that a match whose one arm is
// a block ending in return (type Never) and whose other arm yields a value
// types as the value type, with no diagnostics and no missing-return.
func TestCG37_MatchNeverArmMergesWithValue(t *testing.T) {
	src := `fn f(x int) -> int {
    match x {
        0 => 0
        _ => {
            return 42
        }
    }
}

fn main() {}`
	prog, info, err := checkCG37(t, src)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if info.Diagnostics.Len() != 0 {
		t.Errorf("expected 0 diagnostics, got %d", info.Diagnostics.Len())
	}

	fn := prog.Decls[0].(*ast.FnDecl)
	matchExpr := fn.Body.Stmts[0].(*ast.ExprStmt).X.(*ast.MatchExpr)
	if got := info.TypeOf(matchExpr); got != Int {
		t.Errorf("match type = %v, want int", got)
	}
	blockArm := matchExpr.Arms[1].Body.(*ast.BlockExpr)
	if got := info.TypeOf(blockArm); !IsNever(got) {
		t.Errorf("return-ending arm block type = %v, want Never", got)
	}
}

// TestCG37_MatchBlockArmTrailingExpr checks the idiomatic form: a block arm
// whose last statement is a value expression, with a reassigning loop in the
// middle. The match yields the arm-local's value and the loop mutates it.
func TestCG37_MatchBlockArmTrailingExpr(t *testing.T) {
	src := `fn sum_to(n int) -> int {
    match n {
        0 => 0
        _ => {
            s = 0
            for i in 1..n {
                s = s + i
            }
            s
        }
    }
}

fn main() {}`
	prog, info, err := checkCG37(t, src)
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	fn := prog.Decls[0].(*ast.FnDecl)
	matchExpr := fn.Body.Stmts[0].(*ast.ExprStmt).X.(*ast.MatchExpr)
	if got := info.TypeOf(matchExpr); got != Int {
		t.Errorf("match type = %v, want int", got)
	}
	armBlock := matchExpr.Arms[1].Body.(*ast.BlockExpr)
	if got := info.TypeOf(armBlock); got != Int {
		t.Errorf("value-ending arm block type = %v, want int", got)
	}

	armDecl := armBlock.Block.Stmts[0].(*ast.VarDecl)
	armSym := info.Defs[identOf(t, armDecl.Names[0])]
	forStmt := armBlock.Block.Stmts[1].(*ast.ForStmt)
	innerIdent := identOf(t, loopBodyVarDecl(t, forStmt).Names[0])
	if use := info.Uses[innerIdent]; use != armSym {
		t.Errorf("loop reassign Uses = %v, want arm symbol %v", use, armSym)
	}
}

// TestCG37_DeadCodeCheckedAfterReturn checks that statements after a
// terminating statement are still checked (their Defs/Uses get recorded) and
// reported as one warning, without any errors.
func TestCG37_DeadCodeCheckedAfterReturn(t *testing.T) {
	src := `fn main() {
    print("before")
    return
    x = 1
    x = x + 1
}`
	prog, info, _ := checkCG37(t, src)
	if n := info.Diagnostics.ErrorCount(); n != 0 {
		t.Errorf("ErrorCount = %d, want 0", n)
	}
	if n := info.Diagnostics.WarningCount(); n != 1 {
		t.Errorf("WarningCount = %d, want 1", n)
	}
	if !strings.Contains(info.Diagnostics.Error(), "unreachable code") {
		t.Errorf("expected an 'unreachable code' warning, got %q", info.Diagnostics.Error())
	}

	fn := prog.Decls[0].(*ast.FnDecl)
	deadDecl := fn.Body.Stmts[2].(*ast.VarDecl)
	deadIdent := identOf(t, deadDecl.Names[0])
	if def := info.Defs[deadIdent]; def == nil {
		t.Error("dead declaration after return was not checked: no Defs entry")
	}
	reassignIdent := assignTargetIdent(t, fn.Body.Stmts[3])
	if use := info.Uses[reassignIdent]; use != info.Defs[deadIdent] {
		t.Errorf("dead reassignment was not checked: Uses = %v, want %v", use, info.Defs[deadIdent])
	}
}

// TestCG37_TrailingDeadReturnTerminates checks that a function body ending in
// a dead return statement still terminates (no missing-return error).
func TestCG37_TrailingDeadReturnTerminates(t *testing.T) {
	src := `fn g() -> int {
    return 7
    return 8
}

fn main() {}`
	_, info, _ := checkCG37(t, src)
	if n := info.Diagnostics.ErrorCount(); n != 0 {
		t.Errorf("ErrorCount = %d, want 0 (trailing dead return must terminate the body)", n)
	}
	if n := info.Diagnostics.WarningCount(); n != 1 {
		t.Errorf("WarningCount = %d, want 1", n)
	}
}

// TestCG37_MatchArmDeadStmtsNoCrash checks that a match arm block with dead
// statements after a return does not crash the checker and that the dead
// statements are still checked (Defs/Uses recorded).
func TestCG37_MatchArmDeadStmtsNoCrash(t *testing.T) {
	src := `fn f(x int) -> int {
    match x {
        0 => 0
        _ => {
            return 42
            y = 1
            y = y + 1
        }
    }
}

fn main() {}`
	prog, info, _ := checkCG37(t, src)

	fn := prog.Decls[0].(*ast.FnDecl)
	matchExpr := fn.Body.Stmts[0].(*ast.ExprStmt).X.(*ast.MatchExpr)
	armBlock := matchExpr.Arms[1].Body.(*ast.BlockExpr)

	deadDecl := armBlock.Block.Stmts[1].(*ast.VarDecl)
	deadIdent := identOf(t, deadDecl.Names[0])
	if def := info.Defs[deadIdent]; def == nil {
		t.Error("dead declaration inside a match arm was not checked: no Defs entry")
	}
	reassignIdent := assignTargetIdent(t, armBlock.Block.Stmts[2])
	if use := info.Uses[reassignIdent]; use != info.Defs[deadIdent] {
		t.Errorf("dead reassignment inside a match arm was not checked: Uses = %v, want %v",
			use, info.Defs[deadIdent])
	}
}
