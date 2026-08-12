package types

import (
	"testing"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/errors"
	"github.com/dotlang/dot/parser"
)

func TestBorrow_BasicDecl(t *testing.T) {
	src := `fn main() {
	x = 42
	print(x)
}`
	prog, err := parser.ParseFile(src, "<test>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	borrows := AnalyzeBorrows(prog, nil)
	if len(borrows) == 0 {
		t.Fatal("expected at least one borrow record")
	}
	found := false
	for _, b := range borrows {
		if b.Name == "x" {
			found = true
			if b.DeclaredAt == (ast.Position{}) {
				t.Error("DeclaredAt not set")
			}
			if b.LastUsedAt == (ast.Position{}) {
				t.Error("LastUsedAt not set")
			}
		}
	}
	if !found {
		t.Error("variable 'x' not tracked")
	}
}

func TestBorrow_MoveDetection(t *testing.T) {
	src := `fn main() {
	x = 42
	y = x
	print(x)
}`
	prog, err := parser.ParseFile(src, "<test>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := newInfo(NewUniverse())
	AnalyzeBorrows(prog, info)
	if info.Diagnostics.Len() > 0 {
		t.Logf("diagnostics: %s", info.Diagnostics.Error())
	}
}

func TestBorrow_NoMoveNoError(t *testing.T) {
	src := `fn main() {
	x = 42
	print(x)
}`
	prog, err := parser.ParseFile(src, "<test>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := newInfo(NewUniverse())
	AnalyzeBorrows(prog, info)
	for _, e := range info.Diagnostics.Errors {
		if d, ok := e.(*errors.Diagnostic); ok && d.Message == "variable x used after move" {
			t.Errorf("unexpected move error: %s", d.Message)
		}
	}
}

func TestBorrow_ReturnMove(t *testing.T) {
	src := `fn make() -> int {
	x = 42
	return x
}`
	prog, err := parser.ParseFile(src, "<test>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	borrows := AnalyzeBorrows(prog, nil)
	found := false
	for _, b := range borrows {
		if b.Name == "x" {
			found = true
			if !b.moved() {
				t.Error("expected x to be marked as moved")
			}
			if b.MovedTo != "return" {
				t.Errorf("expected MovedTo='return', got %q", b.MovedTo)
			}
		}
	}
	if !found {
		t.Error("variable 'x' not tracked")
	}
}

func TestBorrow_IsMut(t *testing.T) {
	src := `fn main() {
	x = 42
	x = 100
}`
	prog, err := parser.ParseFile(src, "<test>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	borrows := AnalyzeBorrows(prog, nil)
	for _, b := range borrows {
		if b.Name == "x" && !b.IsMut {
			t.Error("expected x to be marked as mut")
		}
	}
}

func TestBorrow_NilProgram(t *testing.T) {
	borrows := AnalyzeBorrows(nil, nil)
	if borrows != nil {
		t.Error("expected nil for nil program")
	}
}

func TestBorrow_MultipleVars(t *testing.T) {
	src := `fn main() {
	a = 1
	b = 2
	c = a + b
}`
	prog, err := parser.ParseFile(src, "<test>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	borrows := AnalyzeBorrows(prog, nil)
	names := map[string]bool{}
	for _, b := range borrows {
		names[b.Name] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !names[want] {
			t.Errorf("variable %q not tracked", want)
		}
	}
}
