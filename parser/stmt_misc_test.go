package parser

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/ast"
)

func TestStructLitVsBlock_IfCond(t *testing.T) {
	src := "fn f() { if x { } }"
	d := parseFirstDecl(t, src)
	es := d.(*ast.FnDecl).Body.Stmts[0].(*ast.ExprStmt)
	ie := es.X.(*ast.IfExpr)
	if ie.Cond == nil {
		t.Errorf("if x {}: Cond is nil")
	}
	if _, ok := ie.Cond.(*ast.Ident); !ok {
		t.Errorf("if x {}: Cond = %T, want Ident (not struct lit)", ie.Cond)
	}
}

func TestStructLitVsBlock_Assign(t *testing.T) {
	vd := varDecl(t, "p = Point { x: 1 }")
	if _, ok := vd.Values[0].(*ast.StructLit); !ok {
		t.Errorf("p = Point {x:1}: value = %T, want StructLit", vd.Values[0])
	}
}

func TestStructLitVsBlock_IfEqStruct(t *testing.T) {
	// Per D30, a struct literal may not appear at the top level of an `if`
	// condition. The user must parenthesise: `if (Point{x:1}) == p`.
	src := "fn f() { if (Point { x: 1 }) == p { } }"
	d := parseFirstDecl(t, src)
	es := d.(*ast.FnDecl).Body.Stmts[0].(*ast.ExprStmt)
	ie := es.X.(*ast.IfExpr)
	be, ok := ie.Cond.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("if (Point{x:1}) == p: Cond = %T, want BinaryExpr", ie.Cond)
	}
	// (Point { x: 1 }) is a ParenExpr wrapping a StructLit.
	lhs, ok := be.X.(*ast.ParenExpr)
	if !ok {
		t.Fatalf("if (Point{x:1}) == p: LHS = %T, want ParenExpr", be.X)
	}
	sl, ok := lhs.X.(*ast.StructLit)
	if !ok {
		t.Fatalf("if (Point{x:1}) == p: inner = %T, want StructLit", lhs.X)
	}
	if sl.Type == nil {
		t.Errorf("if (Point{x:1}) == p: StructLit.Type is nil")
	}
}

func TestPerfBlock_InsideFn(t *testing.T) {
	src := "fn f() { @perf { print(1) } }"
	d := parseFirstDecl(t, src)
	pb, ok := d.(*ast.FnDecl).Body.Stmts[0].(*ast.PerfBlock)
	if !ok {
		t.Fatalf("@perf {}: stmt is %T, want PerfBlock", d.(*ast.FnDecl).Body.Stmts[0])
	}
	if pb.Block == nil {
		t.Errorf("@perf {}: Block is nil")
	}
}

func TestPerfBlock_TopLevel(t *testing.T) {
	_, msgs := mustParseWithErrs(t, "@perf { }\n")
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "@perf blocks are only allowed inside a function body") {
			found = true
		}
	}
	if !found {
		t.Errorf("@perf at top level: expected diagnostic, got: %v", msgs)
	}
}
