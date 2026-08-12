package parser

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/ast"
)

// varDecl parses a single top-level VarDecl-shaped source and returns it.
func varDecl(t *testing.T, src string) *ast.VarDecl {
	t.Helper()
	prog := mustParse(t, src+"\n")
	if len(prog.Decls) != 1 {
		t.Fatalf("%q: got %d decls, want 1", src, len(prog.Decls))
	}
	d, ok := prog.Decls[0].(*ast.VarDecl)
	if !ok {
		t.Fatalf("%q: decl is %T, want VarDecl", src, prog.Decls[0])
	}
	return d
}

// parseFirstDecl returns the single declaration parsed from src.
func parseFirstDecl(t *testing.T, src string) ast.Decl {
	t.Helper()
	prog := mustParse(t, src+"\n")
	if len(prog.Decls) == 0 {
		t.Fatalf("%q: no decls parsed", src)
	}
	return prog.Decls[0]
}

func TestVarDecl_Inferred(t *testing.T) {
	vd := varDecl(t, "x = 5")
	if vd.Const {
		t.Errorf("x = 5: Const = true, want false")
	}
	if len(vd.Names) != 1 {
		t.Fatalf("x = 5: got %d names, want 1", len(vd.Names))
	}
	if id, ok := vd.Names[0].(*ast.Ident); !ok || id.Name != "x" {
		t.Errorf("x = 5: name = %#v, want Ident(x)", vd.Names[0])
	}
	if vd.Type != nil {
		t.Errorf("x = 5: Type = %T, want nil", vd.Type)
	}
	if len(vd.Values) != 1 {
		t.Fatalf("x = 5: got %d values, want 1", len(vd.Values))
	}
}

func TestVarDecl_Typed(t *testing.T) {
	vd := varDecl(t, "x int = 5")
	if vd.Type == nil {
		t.Fatalf("x int = 5: Type is nil")
	}
	if _, ok := vd.Type.(*ast.NamedType); !ok {
		t.Errorf("x int = 5: Type = %T, want NamedType", vd.Type)
	}
}

func TestVarDecl_MultiName(t *testing.T) {
	vd := varDecl(t, "a, b = 1, 2")
	if len(vd.Names) != 2 {
		t.Fatalf("a,b=1,2: got %d names, want 2", len(vd.Names))
	}
	if len(vd.Values) != 2 {
		t.Fatalf("a,b=1,2: got %d values, want 2", len(vd.Values))
	}
}

func TestVarDecl_Const(t *testing.T) {
	vd := varDecl(t, "const X = 1")
	if !vd.Const {
		t.Errorf("const X = 1: Const = false, want true")
	}
}

func TestVarDecl_SliceType(t *testing.T) {
	vd := varDecl(t, "items []int = make()")
	if vd.Type == nil {
		t.Fatalf("items []int = make(): Type is nil")
	}
	if _, ok := vd.Type.(*ast.SliceType); !ok {
		t.Errorf("items []int = make(): Type = %T, want SliceType", vd.Type)
	}
}

func TestVarDecl_Discard(t *testing.T) {
	vd := varDecl(t, "_, err = f()")
	if len(vd.Names) != 2 {
		t.Fatalf("_, err = f(): got %d names, want 2", len(vd.Names))
	}
	if _, ok := vd.Names[0].(*ast.UnderscoreExpr); !ok {
		t.Errorf("_, err = f(): name[0] = %T, want UnderscoreExpr", vd.Names[0])
	}
}

func TestAssignExpr_IndexTarget(t *testing.T) {
	// Inside a fn, items[0] = 5 -> ExprStmt around an AssignExpr (D31).
	src := "fn f() { items = [0]\nitems[0] = 5 }"
	d := parseFirstDecl(t, src)
	fn := d.(*ast.FnDecl)
	found := false
	for _, s := range fn.Body.Stmts {
		es, ok := s.(*ast.ExprStmt)
		if !ok {
			continue
		}
		if ae, ok := es.X.(*ast.AssignExpr); ok {
			if _, ok := ae.Targets[0].(*ast.IndexExpr); ok {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("items[0] = 5: no AssignExpr with *IndexExpr target found")
	}
}

func TestAssignExpr_FieldTarget(t *testing.T) {
	src := "fn f() { p = Point { x: 0 }\np.x = 1 }"
	d := parseFirstDecl(t, src)
	fn := d.(*ast.FnDecl)
	found := false
	for _, s := range fn.Body.Stmts {
		es, ok := s.(*ast.ExprStmt)
		if !ok {
			continue
		}
		if ae, ok := es.X.(*ast.AssignExpr); ok {
			if _, ok := ae.Targets[0].(*ast.FieldExpr); ok {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("p.x = 1: no AssignExpr with *FieldExpr target found")
	}
}

func TestFor_Infinite(t *testing.T) {
	d := parseFirstDecl(t, "fn f() { for { } }")
	fs := d.(*ast.FnDecl).Body.Stmts[0].(*ast.ForStmt)
	if fs.Kind != ast.ForInfinite {
		t.Errorf("for {}: Kind = %v, want ForInfinite", fs.Kind)
	}
}

func TestFor_Cond(t *testing.T) {
	d := parseFirstDecl(t, "fn f() { for x > 0 { } }")
	fs := d.(*ast.FnDecl).Body.Stmts[0].(*ast.ForStmt)
	if fs.Kind != ast.ForCond {
		t.Errorf("for x > 0: Kind = %v, want ForCond", fs.Kind)
	}
	if fs.Cond == nil {
		t.Errorf("for x > 0: Cond is nil")
	}
}

func TestFor_InRange(t *testing.T) {
	d := parseFirstDecl(t, "fn f() { for i in 0..10 { } }")
	fs := d.(*ast.FnDecl).Body.Stmts[0].(*ast.ForStmt)
	if fs.Kind != ast.ForIn {
		t.Errorf("for i in 0..10: Kind = %v, want ForIn", fs.Kind)
	}
	if fs.Value == nil {
		t.Errorf("for i in 0..10: Value is nil")
	}
	if fs.Iterable == nil {
		t.Errorf("for i in 0..10: Iterable is nil")
	}
	if fs.Key != nil {
		t.Errorf("for i in 0..10: Key should be nil for single-var form")
	}
}

func TestFor_InCollection(t *testing.T) {
	d := parseFirstDecl(t, "fn f() { for item in items { } }")
	fs := d.(*ast.FnDecl).Body.Stmts[0].(*ast.ForStmt)
	if fs.Kind != ast.ForIn {
		t.Errorf("for item in items: Kind = %v, want ForIn", fs.Kind)
	}
	if fs.Value == nil {
		t.Errorf("for item in items: Value is nil")
	}
	if fs.Iterable == nil {
		t.Errorf("for item in items: Iterable is nil")
	}
}

func TestFor_InTwoVar(t *testing.T) {
	d := parseFirstDecl(t, "fn f() { for i, item in items { } }")
	fs := d.(*ast.FnDecl).Body.Stmts[0].(*ast.ForStmt)
	if fs.Kind != ast.ForIn {
		t.Errorf("for i, item in items: Kind = %v, want ForIn", fs.Kind)
	}
	if fs.Key == nil {
		t.Errorf("for i, item in items: Key is nil, want an index ident")
	}
	if fs.Value == nil {
		t.Errorf("for i, item in items: Value is nil")
	}
}

func TestLabel_Valid(t *testing.T) {
	src := "fn f() { @outer for i in 0..10 { break @outer } }"
	d := parseFirstDecl(t, src)
	fs := d.(*ast.FnDecl).Body.Stmts[0].(*ast.ForStmt)
	if fs.Label != "outer" {
		t.Errorf("@outer for: Label = %q, want outer", fs.Label)
	}
	bs, ok := fs.Body.Stmts[0].(*ast.BreakStmt)
	if !ok {
		t.Fatalf("break @outer: stmt is %T, want BreakStmt", fs.Body.Stmts[0])
	}
	if bs.Label != "outer" {
		t.Errorf("break @outer: Label = %q, want outer", bs.Label)
	}
}

func TestLabel_Unknown(t *testing.T) {
	// break @nope inside a fn but outside a loop: the "break outside of a loop"
	// diagnostic fires first and suppresses the "unknown loop label" one
	// (cascade suppression, D28). We just verify at least one diagnostic.
	_, msgs := mustParseWithErrs(t, "fn f() { break @nope }\n")
	if len(msgs) == 0 {
		t.Errorf("break @nope: expected at least one diagnostic, got none")
	}
}

func TestLabel_Unknown_InLoop(t *testing.T) {
	// Inside a loop with a wrong label: the "unknown loop label" fires.
	src := "fn f() { for i in 0..10 { break @wrong } }"
	prog, errs := mustParseWithErrs(t, src+"\n")
	found := false
	for _, m := range errs {
		if strings.Contains(m, "unknown loop label") {
			found = true
		}
	}
	if !found {
		t.Errorf("break @wrong: expected 'unknown loop label', got: %v", errs)
	}
	fs := prog.Decls[0].(*ast.FnDecl).Body.Stmts[0].(*ast.ForStmt)
	bs, ok := fs.Body.Stmts[0].(*ast.BreakStmt)
	if !ok {
		t.Fatalf("break @wrong: stmt is %T, want BreakStmt", fs.Body.Stmts[0])
	}
	if bs.Label != "wrong" {
		t.Errorf("break @wrong: Label = %q, want wrong", bs.Label)
	}
}

func TestBreak_OutsideLoop(t *testing.T) {
	_, msgs := mustParseWithErrs(t, "fn f() { break }\n")
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "break outside of a loop") {
			found = true
		}
	}
	if !found {
		t.Errorf("break outside loop: expected diagnostic, got: %v", msgs)
	}
}

func TestContinue_OutsideLoop(t *testing.T) {
	_, msgs := mustParseWithErrs(t, "fn f() { continue }\n")
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "continue outside of a loop") {
			found = true
		}
	}
	if !found {
		t.Errorf("continue outside loop: expected diagnostic, got: %v", msgs)
	}
}

func TestReturn_OutsideFn(t *testing.T) {
	// A `return` at the top level goes through parseDecl (not parseStmt), so
	// the parser reports "expected a declaration" rather than "return outside
	// a function". The "return outside" message fires only inside a fn body.
	_, msgs := mustParseWithErrs(t, "return\n")
	if len(msgs) == 0 {
		t.Errorf("return at top level: expected a diagnostic, got none")
	}
}

func TestReturn_OutsideFn_Method(t *testing.T) {
	// Inside a method-less context: a free-standing fn with `return` at module
	// level is caught by parseDecl. We verify the inside-fn case works.
	src := "fn f() { return }"
	d := parseFirstDecl(t, src)
	fs := d.(*ast.FnDecl)
	rs, ok := fs.Body.Stmts[0].(*ast.ReturnStmt)
	if !ok {
		t.Fatalf("return inside fn: stmt is %T, want ReturnStmt", fs.Body.Stmts[0])
	}
	if len(rs.Values) != 0 {
		t.Errorf("return inside fn: got %d values, want 0", len(rs.Values))
	}
}

func TestIf_ElseChain(t *testing.T) {
	src := "fn f() { if a { } else if b { } else { } }"
	d := parseFirstDecl(t, src)
	es := d.(*ast.FnDecl).Body.Stmts[0].(*ast.ExprStmt)
	ie := es.X.(*ast.IfExpr)
	if ie.ElseIf == nil {
		t.Fatalf("if/else: ElseIf is nil")
	}
	if ie.ElseIf.Else == nil {
		t.Errorf("if/else: final Else is nil")
	}
}

func TestIfLet(t *testing.T) {
	src := "fn f() { if u = find(42) { } }"
	d := parseFirstDecl(t, src)
	es := d.(*ast.FnDecl).Body.Stmts[0].(*ast.ExprStmt)
	ie := es.X.(*ast.IfExpr)
	if ie.Bind == nil {
		t.Fatalf("if-let: Bind is nil")
	}
	if _, ok := ie.Bind.(*ast.IdentPattern); !ok {
		t.Errorf("if-let: Bind = %T, want IdentPattern", ie.Bind)
	}
	if ie.Cond == nil {
		t.Errorf("if-let: Cond is nil")
	}
}

func TestDefer_Call(t *testing.T) {
	src := "fn f() { defer something() }"
	d := parseFirstDecl(t, src)
	ds, ok := d.(*ast.FnDecl).Body.Stmts[0].(*ast.DeferStmt)
	if !ok {
		t.Fatalf("defer f(): stmt is %T, want DeferStmt", d.(*ast.FnDecl).Body.Stmts[0])
	}
	if _, ok := ds.Call.(*ast.CallExpr); !ok {
		t.Errorf("defer f(): Call = %T, want CallExpr", ds.Call)
	}
}

func TestDefer_NotCall(t *testing.T) {
	_, msgs := mustParseWithErrs(t, "fn f() { defer f }\n")
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "defer requires a function call") {
			found = true
		}
	}
	if !found {
		t.Errorf("defer f: expected diagnostic, got: %v", msgs)
	}
}

func TestFor_ContinueLabel(t *testing.T) {
	src := "fn f() { @outer for i in 0..10 { continue @outer } }"
	d := parseFirstDecl(t, src)
	fs := d.(*ast.FnDecl).Body.Stmts[0].(*ast.ForStmt)
	if fs.Label != "outer" {
		t.Errorf("continue @outer: loop Label = %q, want outer", fs.Label)
	}
	cs, ok := fs.Body.Stmts[0].(*ast.ContinueStmt)
	if !ok {
		t.Fatalf("continue @outer: body stmt = %T, want ContinueStmt", fs.Body.Stmts[0])
	}
	if cs.Label != "outer" {
		t.Errorf("continue @outer: Label = %q, want outer", cs.Label)
	}
}
