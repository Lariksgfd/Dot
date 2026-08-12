package parser

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/ast"
)

// expr parses src as an expression by wrapping it in an assignment to a
// discard target and printing the value. This gives us a clean Pratt-parser
// entry for arbitrary expression sources.
func expr(t *testing.T, src string) string {
	t.Helper()
	// The leading newline ensures the wrapped "_" is on its own logical line
	// and acts as a plain assignment target that parseSimpleStmt recognizes.
	prog := mustParse(t, "_ = "+src+"\n")
	vd, ok := prog.Decls[0].(*ast.VarDecl)
	if !ok {
		t.Fatalf("_ = %q: first decl is %T, want VarDecl", src, prog.Decls[0])
	}
	if len(vd.Values) == 0 {
		t.Fatalf("_ = %q: VarDecl has no values", src)
	}
	return ast.Print(vd.Values[0])
}

// parseDeclStmt parses src as a single top-level declaration and returns its
// printed form.
func parseDecl(t *testing.T, src string) string {
	t.Helper()
	prog := mustParse(t, src+"\n")
	if len(prog.Decls) != 1 {
		t.Fatalf("%q: got %d decls, want 1", src, len(prog.Decls))
	}
	return ast.Print(prog.Decls[0])
}

func TestPrecedence_FullLadder(t *testing.T) {
	// From SYNTAX.md, tightest-to-loosest inside the expression, the Pratt
	// parser produces this exact nesting. See D27.
	// Note the printer renders `==` as "op===" because Literal() returns "=="
	// and the format is "op=" + literal.
	want := "BinaryExpr op=or\n" +
		"  Ident name=a\n" +
		"  BinaryExpr op=and\n" +
		"    Ident name=b\n" +
		"    BinaryExpr op===\n" +
		"      Ident name=c\n" +
		"      BinaryExpr op=|\n" +
		"        Ident name=d\n" +
		"        BinaryExpr op=^\n" +
		"          Ident name=e\n" +
		"          BinaryExpr op=&\n" +
		"            Ident name=f\n" +
		"            BinaryExpr op=<<\n" +
		"              Ident name=g\n" +
		"              BinaryExpr op=+\n" +
		"                Ident name=h\n" +
		"                BinaryExpr op=*\n" +
		"                  Ident name=i\n" +
		"                  BinaryExpr op=**\n" +
		"                    Ident name=j\n" +
		"                    Ident name=k\n"
	got := expr(t, "a or b and c == d | e ^ f & g << h + i * j ** k")
	if got != want {
		t.Errorf("precedence ladder:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrecedence_RightAssocPower(t *testing.T) {
	// ** is right-associative: a ** b ** c == a ** (b ** c).
	want := "BinaryExpr op=**\n" +
		"  IntLit value=2 raw=2 base=10\n" +
		"  BinaryExpr op=**\n" +
		"    IntLit value=3 raw=3 base=10\n" +
		"    IntLit value=2 raw=2 base=10\n"
	got := expr(t, "2 ** 3 ** 2")
	if got != want {
		t.Errorf("2**3**2:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrecedence_RightAssocAssign(t *testing.T) {
	// = is right-associative at the expression level: a = b = c == a = (b = c).
	want := "AssignExpr op==\n" +
		"  Ident name=a\n" +
		"  AssignExpr op==\n" +
		"    Ident name=b\n" +
		"    Ident name=c\n"
	got := expr(t, "a = b = c")
	if got != want {
		t.Errorf("a=b=c:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrecedence_UnaryVsPower(t *testing.T) {
	// Per SYNTAX.md and D27, unary binds tighter than **: -2**2 == (-2)**2.
	want := "BinaryExpr op=**\n" +
		"  UnaryExpr op=-\n" +
		"    IntLit value=2 raw=2 base=10\n" +
		"  IntLit value=2 raw=2 base=10\n"
	got := expr(t, "-2 ** 2")
	if got != want {
		t.Errorf("-2**2:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrecedence_UnaryNotVsAnd(t *testing.T) {
	// not binds tighter than and: not a and b == (not a) and b.
	want := "BinaryExpr op=and\n" +
		"  UnaryExpr op=not\n" +
		"    Ident name=a\n" +
		"  Ident name=b\n"
	got := expr(t, "not a and b")
	if got != want {
		t.Errorf("not a and b:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRange_NonAssoc(t *testing.T) {
	// 1..2..3 must be a diagnostic. We still expect the parser to return a
	// tree (recovery). The test only asserts the diagnostic exists.
	src := "1..2..3"
	_, msgs := mustParseWithErrs(t, "_ = "+src+"\n")
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "range operators cannot be chained") {
			found = true
		}
	}
	if !found {
		t.Errorf("%q: expected 'range operators cannot be chained' diagnostic, got: %v", src, msgs)
	}
}

func TestCast_Precedence(t *testing.T) {
	// `as` is between * and **: x as float / 2 == (x as float) / 2.
	want := "BinaryExpr op=/\n" +
		"  CastExpr\n" +
		"    Ident name=x\n" +
		"    NamedType name=float\n" +
		"  IntLit value=2 raw=2 base=10\n"
	got := expr(t, "x as float / 2")
	if got != want {
		t.Errorf("x as float / 2:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCast_AfterUnary(t *testing.T) {
	// Prefix - binds tighter than `as`: -x as int == (-x) as int.
	want := "CastExpr\n" +
		"  UnaryExpr op=-\n" +
		"    Ident name=x\n" +
		"  NamedType name=int\n"
	got := expr(t, "-x as int")
	if got != want {
		t.Errorf("-x as int:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestIs_Precedence(t *testing.T) {
	// `is` sits between * and **: x is Printable and y == (x is Printable) and y.
	want := "BinaryExpr op=and\n" +
		"  IsExpr\n" +
		"    Ident name=x\n" +
		"    NamedType name=Printable\n" +
		"  Ident name=y\n"
	got := expr(t, "x is Printable and y")
	if got != want {
		t.Errorf("x is Printable and y:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPipe_ChainLeftAssoc(t *testing.T) {
	// |> is left-associative: a |> f() |> g() == (a |> f()) |> g().
	want := "PipeExpr\n" +
		"  PipeExpr\n" +
		"    Ident name=a\n" +
		"    CallExpr\n" +
		"      Ident name=f\n" +
		"  CallExpr\n" +
		"    Ident name=g\n"
	got := expr(t, "a |> f() |> g()")
	if got != want {
		t.Errorf("a |> f() |> g():\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPipe_AcrossNewlines(t *testing.T) {
	// The lexer suppresses newlines after |>, so a multi-line pipe chain parses
	// as a single expression.
	src := "data\n  |> filter(f)\n  |> map(g)\n"
	want := "PipeExpr\n" +
		"  PipeExpr\n" +
		"    Ident name=data\n" +
		"    CallExpr\n" +
		"      Ident name=filter\n" +
		"      Arg\n" +
		"        Ident name=f\n" +
		"  CallExpr\n" +
		"    Ident name=map\n" +
		"    Arg\n" +
		"      Ident name=g\n"
	got := expr(t, src)
	if got != want {
		t.Errorf("multi-line pipe:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestTry_Postfix(t *testing.T) {
	// a.b(c)[d]?.e — the postfix ? binds tighter than the trailing .e field.
	want := "FieldExpr name=e\n" +
		"  TryExpr\n" +
		"    IndexExpr\n" +
		"      CallExpr\n" +
		"        FieldExpr name=b\n" +
		"          Ident name=a\n" +
		"        Arg\n" +
		"          Ident name=c\n" +
		"      Ident name=d\n"
	got := expr(t, "a.b(c)[d]?.e")
	if got != want {
		t.Errorf("a.b(c)[d]?.e:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCall_NamedArgs(t *testing.T) {
	want := "CallExpr\n" +
		"  Ident name=f\n" +
		"  Arg name=port\n" +
		"    IntLit value=3000 raw=3000 base=10\n"
	got := expr(t, "f(port: 3000)")
	if got != want {
		t.Errorf("f(port: 3000):\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCall_Spread(t *testing.T) {
	want := "CallExpr\n" +
		"  Ident name=f\n" +
		"  Arg spread=true\n" +
		"    Ident name=xs\n"
	got := expr(t, "f(xs...)")
	if got != want {
		t.Errorf("f(xs...):\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCall_Nested(t *testing.T) {
	// Outer call whose argument is itself a call.
	want := "CallExpr\n" +
		"  Ident name=f\n" +
		"  Arg\n" +
		"    CallExpr\n" +
		"      Ident name=g\n" +
		"      Arg\n" +
		"        Ident name=x\n"
	got := expr(t, "f(g(x))")
	if got != want {
		t.Errorf("f(g(x)):\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestIndex_GenericVsIndex(t *testing.T) {
	// Stack[int] is generic instantiation (IndexExpr with one index).
	wantGeneric := "IndexExpr\n" +
		"  Ident name=Stack\n" +
		"  Ident name=int\n"
	if got := expr(t, "Stack[int]"); got != wantGeneric {
		t.Errorf("Stack[int]:\n got:\n%s\nwant:\n%s", got, wantGeneric)
	}

	// items[0] is an index.
	wantIndex := "IndexExpr\n" +
		"  Ident name=items\n" +
		"  IntLit value=0 raw=0 base=10\n"
	if got := expr(t, "items[0]"); got != wantIndex {
		t.Errorf("items[0]:\n got:\n%s\nwant:\n%s", got, wantIndex)
	}

	// Result[int, Error] has two indices.
	wantMulti := "IndexExpr\n" +
		"  Ident name=Result\n" +
		"  Ident name=int\n" +
		"  Ident name=Error\n"
	if got := expr(t, "Result[int, Error]"); got != wantMulti {
		t.Errorf("Result[int, Error]:\n got:\n%s\nwant:\n%s", got, wantMulti)
	}
}

func TestSlice_AllForms(t *testing.T) {
	cases := map[string]string{
		"a[1..3]": "SliceExpr\n" +
			"  Ident name=a\n" +
			"  IntLit value=1 raw=1 base=10\n" +
			"  IntLit value=3 raw=3 base=10\n",
		"a[..3]": "SliceExpr\n" +
			"  Ident name=a\n" +
			"  <nil>\n" +
			"  IntLit value=3 raw=3 base=10\n",
		"a[1..]": "SliceExpr\n" +
			"  Ident name=a\n" +
			"  IntLit value=1 raw=1 base=10\n" +
			"  <nil>\n",
		"a[..]": "SliceExpr\n" +
			"  Ident name=a\n" +
			"  <nil>\n" +
			"  <nil>\n",
		"a[1..=3]": "SliceExpr inclusive=true\n" +
			"  Ident name=a\n" +
			"  IntLit value=1 raw=1 base=10\n" +
			"  IntLit value=3 raw=3 base=10\n",
	}
	for src, want := range cases {
		if got := expr(t, src); got != want {
			t.Errorf("%s:\n got:\n%s\nwant:\n%s", src, got, want)
		}
	}
}

func TestTupleIndex(t *testing.T) {
	// t.0 is a tuple index.
	want := "FieldExpr name=0 tuple=true\n" +
		"  Ident name=t\n"
	if got := expr(t, "t.0"); got != want {
		t.Errorf("t.0:\n got:\n%s\nwant:\n%s", got, want)
	}

	// t.0.1 chains a second tuple index.
	wantChain := "FieldExpr name=1 tuple=true\n" +
		"  FieldExpr name=0 tuple=true\n" +
		"    Ident name=t\n"
	if got := expr(t, "t.0.1"); got != wantChain {
		t.Errorf("t.0.1:\n got:\n%s\nwant:\n%s", got, wantChain)
	}
}

func TestLambda_Block(t *testing.T) {
	// Param prints its Type and Default children, both nil for an untyped
	// parameter with no default.
	want := "FnLit\n" +
		"  FnSig\n" +
		"    <nil>\n" +
		"    Param name=x\n" +
		"      <nil>\n" +
		"      <nil>\n" +
		"    <nil>\n" +
		"  BlockStmt\n" +
		"    ExprStmt\n" +
		"      Ident name=x\n" +
		"  <nil>\n"
	if got := expr(t, "fn(x) { x }"); got != want {
		t.Errorf("fn(x){x}:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestLambda_TypedWithResult(t *testing.T) {
	// FnSig prints Recv, each Param (with Type and Default children), and
	// Result. Param always prints both Type and Default (nil when absent).
	want := "FnLit\n" +
		"  FnSig\n" +
		"    <nil>\n" +
		"    Param name=x\n" +
		"      NamedType name=int\n" +
		"      <nil>\n" +
		"    NamedType name=int\n" +
		"  BlockStmt\n" +
		"    ExprStmt\n" +
		"      BinaryExpr op=*\n" +
		"        Ident name=x\n" +
		"        IntLit value=2 raw=2 base=10\n" +
		"  <nil>\n"
	if got := expr(t, "fn(x int) -> int { x * 2 }"); got != want {
		t.Errorf("fn(x int)->int{x*2}:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestLambda_ShortForm(t *testing.T) {
	want := "FnLit\n" +
		"  FnSig\n" +
		"    <nil>\n" +
		"    Param name=x\n" +
		"      <nil>\n" +
		"      <nil>\n" +
		"    <nil>\n" +
		"  <nil>\n" +
		"  BinaryExpr op=*\n" +
		"    Ident name=x\n" +
		"    IntLit value=2 raw=2 base=10\n"
	if got := expr(t, "fn(x) = x * 2"); got != want {
		t.Errorf("fn(x)=x*2:\n got:\n%s\nwant:\n%s", got, want)
	}
}
