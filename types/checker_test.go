package types

import (
	"testing"

	"github.com/dotlang/dot/parser"
)

func TestCheck_MainFunction(t *testing.T) {
	src := "fn main() {}"
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info == nil {
		t.Fatal("info is nil")
	}
	if info.FileScope == nil {
		t.Error("FileScope is nil")
	}
	if info.Defs == nil || info.Types == nil || info.Uses == nil || info.Diagnostics == nil {
		t.Error("Info maps not initialized")
	}
}

func TestCheck_MainWithReturn(t *testing.T) {
	src := "fn main() -> int { return 42 }"
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info == nil {
		t.Fatal("info is nil")
	}
	if info.Diagnostics.Len() != 0 {
		t.Errorf("expected 0 diagnostics, got %d", info.Diagnostics.Len())
	}
}

func TestCheck_Variable(t *testing.T) {
	src := `fn main() {
    x = 5
    print(x)
}`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info == nil {
		t.Fatal("info is nil")
	}
}

func TestCheck_StructDeclaration(t *testing.T) {
	src := `struct Point {
    x int
    y int
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
	sym, ok := info.FileScope.Lookup("Point")
	if !ok {
		t.Fatal("Point not declared")
	}
	if sym.Kind != SymType {
		t.Errorf("Point kind = %s, want SymType", sym.Kind)
	}
}

func TestCheck_EnumDeclaration(t *testing.T) {
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
	if sym.Kind != SymType {
		t.Errorf("Color kind = %s, want SymType", sym.Kind)
	}
	if sym.Named == nil || sym.Named.Underlying == nil {
		t.Fatal("Named/Underlying not set")
	}
	e, ok := sym.Named.Underlying.(*Enum)
	if !ok {
		t.Fatalf("underlying = %T, want *Enum", sym.Named.Underlying)
	}
	if len(e.Variants) != 3 {
		t.Errorf("expected 3 variants, got %d", len(e.Variants))
	}
}

func TestCheck_FunctionCall(t *testing.T) {
	src := `fn add(a int, b int) -> int {
    return a + b
}

fn main() {
    add(1, 2)
}`
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

func TestCheck_UndefinedVariable(t *testing.T) {
	src := "fn main() { print(undefinedVar) }"
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err == nil {
		t.Fatal("expected error for undefined variable, got nil")
	}
	if info == nil {
		t.Fatal("info is nil")
	}
}

func TestCheck_IfStatement(t *testing.T) {
	src := "fn main() { if true { print(1) } }"
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

func TestCheck_ImplBlock(t *testing.T) {
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

func TestCheck_TraitDeclaration(t *testing.T) {
	src := `trait Printable {
    fn to_string(self) -> string
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
	sym, ok := info.FileScope.Lookup("Printable")
	if !ok {
		t.Fatal("Printable not declared")
	}
	if sym.Kind != SymTrait {
		t.Errorf("Printable kind = %s, want SymTrait", sym.Kind)
	}
}

func TestCheck_TypeAnnotation(t *testing.T) {
	src := "fn main() { x int = 42 }"
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

func TestCheck_ConstDeclaration(t *testing.T) {
	src := `const X = 42

fn main() { print(X) }`
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	sym, ok := info.FileScope.Lookup("X")
	if !ok {
		t.Fatal("X not declared")
	}
	if sym.Kind != SymConst {
		t.Errorf("X kind = %s, want SymConst", sym.Kind)
	}
}

func TestCheck_NilProgram(t *testing.T) {
	info, err := Check(nil, "empty.dot")
	if err != nil {
		t.Fatalf("Check(nil) failed: %v", err)
	}
	if info == nil {
		t.Fatal("Check(nil) should return non-nil Info")
	}
}

func TestCheck_ErrorDiagnostics(t *testing.T) {
	src := "fn main() { return 42 }"
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "test.dot")
	if err == nil {
		t.Fatal("expected error for return in void function")
	}
	if info == nil {
		t.Fatal("info is nil")
	}
	if info.Diagnostics.Len() == 0 {
		t.Error("expected at least 1 diagnostic")
	}
}

func TestCheck_MatchStatement(t *testing.T) {
	src := `fn classify(n int) -> string {
    return match n {
        1 => "one"
        _ => "other"
    }
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

func TestCheck_ForLoop(t *testing.T) {
	src := "fn main() { for i in 0..10 { print(i) } }"
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

func TestCheck_Assignment(t *testing.T) {
	src := `fn main() {
    x = 1
    x = 2
}`
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

func TestCheck_ReturnVoid(t *testing.T) {
	src := `fn f() { return }

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

func TestCheck_OptionPattern(t *testing.T) {
	src := "fn main() { opt = Some(42) }"
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

func TestCheck_WithImports_Basic(t *testing.T) {
	src := "fn main() {}"
	prog, err := parser.ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := CheckWithImports(prog, "test.dot", "")
	if err != nil {
		t.Fatalf("CheckWithImports failed: %v", err)
	}
	if info == nil {
		t.Fatal("info is nil")
	}
	if info.Diagnostics.Len() != 0 {
		t.Errorf("expected 0 diagnostics, got %d", info.Diagnostics.Len())
	}
}
