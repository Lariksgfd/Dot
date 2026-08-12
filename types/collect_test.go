package types

import (
	"testing"

	"github.com/dotlang/dot/parser"
)

func TestCollectDecls_Struct(t *testing.T) {
	src := `struct Foo {
    x int
    y string
}

fn main() {}`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	sym, ok := info.FileScope.Lookup("Foo")
	if !ok {
		t.Fatal("Foo not declared")
	}
	if sym.Kind != SymType {
		t.Errorf("kind = %s, want SymType", sym.Kind)
	}
	if sym.Named == nil || sym.Named.Name != "Foo" {
		t.Error("Named does not match")
	}
	s, ok := sym.Named.Underlying.(*Struct)
	if !ok {
		t.Fatalf("underlying = %T, want *Struct", sym.Named.Underlying)
	}
	if len(s.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(s.Fields))
	}
	if s.Fields[0].Name != "x" || s.Fields[1].Name != "y" {
		t.Errorf("unexpected field names: %v", []string{s.Fields[0].Name, s.Fields[1].Name})
	}
	if info.Diagnostics.Len() != 0 {
		t.Errorf("expected 0 diagnostics, got %d", info.Diagnostics.Len())
	}
}

func TestCollectDecls_Enum(t *testing.T) {
	src := `enum Color {
    Red
    Green
    Blue
}

fn main() {}`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	sym, ok := info.FileScope.Lookup("Color")
	if !ok {
		t.Fatal("Color not declared")
	}
	enum, ok := sym.Named.Underlying.(*Enum)
	if !ok {
		t.Fatalf("underlying = %T, want *Enum", sym.Named.Underlying)
	}
	if len(enum.Variants) != 3 {
		t.Errorf("expected 3 variants, got %d", len(enum.Variants))
	}
	for _, v := range []string{"Red", "Green", "Blue"} {
		if _, found := enum.Variant(v); !found {
			t.Errorf("variant %q not found in enum byName", v)
		}
	}
}

func TestCollectDecls_Function(t *testing.T) {
	src := `fn add(a int, b int) -> int {
    return a + b
}

fn main() {}`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	sym, ok := info.FileScope.Lookup("add")
	if !ok {
		t.Fatal("add not declared")
	}
	if sym.Kind != SymFunc {
		t.Errorf("kind = %s, want SymFunc", sym.Kind)
	}
	if info.Diagnostics.Len() != 0 {
		t.Errorf("expected 0 diagnostics, got %d", info.Diagnostics.Len())
	}
}

func TestCollectDecls_InherentImpl(t *testing.T) {
	src := `struct Point {
    x int
}

impl Point {
    fn origin() -> Point { return Point { x: 0 } }
}

fn main() {}`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info.Diagnostics.Len() != 0 {
		t.Errorf("expected 0 diagnostics, got %d", info.Diagnostics.Len())
	}
}

func TestCollectDecls_TraitImpl(t *testing.T) {
	src := `trait Printable {
    fn to_string(self) -> string
}

struct Point {
    x int
}

impl Printable for Point {
    fn to_string(self) -> string = "point"
}

fn main() {}`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info.Diagnostics.Len() != 0 {
		t.Errorf("expected 0 diagnostics, got %d", info.Diagnostics.Len())
	}
}

func TestCollectDecls_TopLevelVar(t *testing.T) {
	src := `x = 42

fn main() { print(x) }`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	sym, ok := info.FileScope.Lookup("x")
	if !ok {
		t.Fatal("x not declared")
	}
	if sym.Kind != SymVar {
		t.Errorf("kind = %s, want SymVar", sym.Kind)
	}
}

func TestCollectDecls_TopLevelConst(t *testing.T) {
	src := `const PI = 3.14159

fn main() { print(PI) }`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	sym, ok := info.FileScope.Lookup("PI")
	if !ok {
		t.Fatal("PI not declared")
	}
	if sym.Kind != SymConst {
		t.Errorf("kind = %s, want SymConst", sym.Kind)
	}
}

func TestCollectDecls_OptionResult(t *testing.T) {
	src := `fn find(id int) -> Option[int] {
    if id == 0 {
        return None
    }
    return Some(42)
}

fn main() {}`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info.Diagnostics.Len() != 0 {
		t.Errorf("expected 0 diagnostics, got %d", info.Diagnostics.Len())
	}
}

func TestCollectDecls_MultipleTypes(t *testing.T) {
	src := `struct A {
    x int
}

enum B {
    One
    Two
}

struct C {
    val string
}

fn main() {}`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	for _, name := range []string{"A", "B", "C"} {
		if _, ok := info.FileScope.Lookup(name); !ok {
			t.Errorf("%q not declared", name)
		}
	}
}

func TestCollectDecls_FunctionWithDefaults(t *testing.T) {
	src := "fn connect(host string, port int = 8080) -> string { return \"a\" }\nfn main() {}"
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	sym, ok := info.FileScope.Lookup("connect")
	if !ok {
		t.Fatal("connect not declared")
	}
	fn, ok := sym.Type.(*Fn)
	if !ok {
		t.Fatal("type is not Fn")
	}
	if len(fn.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(fn.Params))
	}
	if !fn.Params[1].HasDflt {
		t.Error("second param should have default")
	}
}

func TestCollectDecls_MatchExprBody(t *testing.T) {
	src := `fn double(x int) -> int = x * 2

fn main() {}`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info.Diagnostics.Len() != 0 {
		t.Errorf("expected 0 diagnostics, got %d", info.Diagnostics.Len())
	}
}

func TestCollectDecls_EmbeddedField(t *testing.T) {
	src := `struct Inner {
    y int
}

struct Outer {
    x int
    embed Inner
}

fn main() {}`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info.Diagnostics.Len() != 0 {
		t.Errorf("expected 0 diagnostics, got %d", info.Diagnostics.Len())
	}
	sym, ok := info.FileScope.Lookup("Outer")
	if !ok {
		t.Fatal("Outer not declared")
	}
	st := sym.Named.Underlying.(*Struct)
	if st.Flat == nil {
		t.Fatal("Flat not populated")
	}
	if _, ok := st.Flat["y"]; !ok {
		t.Error("embedded field 'y' should be reachable via Flat")
	}
}
