package parser

import (
	"testing"

	"github.com/dotlang/dot/ast"
)

// exprNode parses src as an expression via a discard assignment and returns
// the raw expression node, for structural assertions.
func exprNode(t *testing.T, src string) ast.Expr {
	t.Helper()
	prog := mustParse(t, "_ = "+src+"\n")
	vd, ok := prog.Decls[0].(*ast.VarDecl)
	if !ok {
		t.Fatalf("_ = %q: first decl is %T, want VarDecl", src, prog.Decls[0])
	}
	if len(vd.Values) == 0 {
		t.Fatalf("_ = %q: VarDecl has no values", src)
	}
	return vd.Values[0]
}

// TestArrayLit_ClosureIndexCall covers PA-03: a bare array literal of
// closures immediately followed by `[i]` must parse as postfix indexing, not
// as a typed array literal (looksLikeArrayTypePrefix must yield to indexing
// when `[` follows the closing `]`).
func TestArrayLit_ClosureIndexCall(t *testing.T) {
	for _, src := range []string{
		"[fn(x int) -> int = x + 1][0](41)",
		"[fn(x int) -> int = x + 1, fn(x int) -> int = x * 2][1](41)",
	} {
		e := exprNode(t, src)
		call, ok := e.(*ast.CallExpr)
		if !ok {
			t.Fatalf("%s: root is %T, want CallExpr", src, e)
		}
		idx, ok := call.Fn.(*ast.IndexExpr)
		if !ok {
			t.Fatalf("%s: callee is %T, want IndexExpr", src, call.Fn)
		}
		lit, ok := idx.X.(*ast.ArrayLit)
		if !ok {
			t.Fatalf("%s: indexed expr is %T, want ArrayLit", src, idx.X)
		}
		if lit.Type != nil {
			t.Errorf("%s: ArrayLit.Type = %T, want nil (bare literal)", src, lit.Type)
		}
		if len(lit.Elems) == 0 {
			t.Fatalf("%s: ArrayLit has no elements", src)
		}
		for i, el := range lit.Elems {
			if _, ok := el.(*ast.FnLit); !ok {
				t.Errorf("%s: elem %d is %T, want FnLit", src, i, el)
			}
		}
		if len(idx.Indices) != 1 {
			t.Errorf("%s: Indices = %d, want 1", src, len(idx.Indices))
		}
		if len(call.Args) != 1 {
			t.Errorf("%s: Args = %d, want 1", src, len(call.Args))
		}
	}

	// The whole expression must print as CallExpr(IndexExpr(ArrayLit, i), arg).
	want := "CallExpr\n" +
		"  IndexExpr\n" +
		"    ArrayLit\n" +
		"      <nil>\n" +
		"      FnLit\n" +
		"        FnSig\n" +
		"          <nil>\n" +
		"          Param name=x\n" +
		"            NamedType name=int\n" +
		"            <nil>\n" +
		"          NamedType name=int\n" +
		"        <nil>\n" +
		"        BinaryExpr op=+\n" +
		"          Ident name=x\n" +
		"          IntLit value=1 raw=1 base=10\n" +
		"    IntLit value=0 raw=0 base=10\n" +
		"  Arg\n" +
		"    IntLit value=41 raw=41 base=10\n"
	if got := expr(t, "[fn(x int) -> int = x + 1][0](41)"); got != want {
		t.Errorf("printed tree:\n got:\n%s\nwant:\n%s", got, want)
	}
}

// TestArrayLit_TypedFormsStillParse is the PA-03 regression: typed forms
// `[]T{...}` and `[N]T{...}` (and a bare `[]T`) must still parse as typed
// literals with a non-nil Type.
func TestArrayLit_TypedFormsStillParse(t *testing.T) {
	t.Run("slice", func(t *testing.T) {
		e := exprNode(t, "[]int{1, 2}")
		lit, ok := e.(*ast.ArrayLit)
		if !ok {
			t.Fatalf("[]int{1, 2}: root is %T, want ArrayLit", e)
		}
		sty, ok := lit.Type.(*ast.SliceType)
		if !ok {
			t.Fatalf("[]int{1, 2}: Type = %T, want SliceType", lit.Type)
		}
		if _, ok := sty.Elem.(*ast.NamedType); !ok {
			t.Errorf("[]int{1, 2}: Elem = %T, want NamedType", sty.Elem)
		}
		if len(lit.Elems) != 2 {
			t.Errorf("[]int{1, 2}: Elems = %d, want 2", len(lit.Elems))
		}
	})

	t.Run("fixed", func(t *testing.T) {
		e := exprNode(t, "[5]int{1, 2}")
		lit, ok := e.(*ast.ArrayLit)
		if !ok {
			t.Fatalf("[5]int{1, 2}: root is %T, want ArrayLit", e)
		}
		aty, ok := lit.Type.(*ast.ArrayType)
		if !ok {
			t.Fatalf("[5]int{1, 2}: Type = %T, want ArrayType", lit.Type)
		}
		if aty.Len == nil {
			t.Errorf("[5]int{1, 2}: Len is nil")
		}
		if _, ok := aty.Elem.(*ast.NamedType); !ok {
			t.Errorf("[5]int{1, 2}: Elem = %T, want NamedType", aty.Elem)
		}
		if len(lit.Elems) != 2 {
			t.Errorf("[5]int{1, 2}: Elems = %d, want 2", len(lit.Elems))
		}
	})

	t.Run("slice_no_elems", func(t *testing.T) {
		// Bare `[]T` without braces stays a typed literal (Type, no Elems).
		e := exprNode(t, "[]int")
		lit, ok := e.(*ast.ArrayLit)
		if !ok {
			t.Fatalf("[]int: root is %T, want ArrayLit", e)
		}
		if _, ok := lit.Type.(*ast.SliceType); !ok {
			t.Errorf("[]int: Type = %T, want SliceType", lit.Type)
		}
		if len(lit.Elems) != 0 {
			t.Errorf("[]int: Elems = %d, want 0", len(lit.Elems))
		}
	})
}

// TestArrayLit_EmptyBareIndex documents the known remaining PA-03 edge: an
// EMPTY bare literal `[]` followed by `[i]` still misparses as a typed form,
// so indexing an empty literal is not yet supported.
func TestArrayLit_EmptyBareIndex(t *testing.T) {
	t.Skip("known edge: `[][0](41)` — empty bare literal still takes the typed-array branch in looksLikeArrayTypePrefix")
}
