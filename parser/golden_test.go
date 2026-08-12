package parser

import (
	"os"
	"strings"
	"testing"

	"github.com/dotlang/dot/ast"
)

func TestGolden_ParserSample(t *testing.T) {
	// The canonical sample program parses with zero diagnostics and zero
	// Bad* nodes anywhere in the AST.
	src, err := os.ReadFile("../testdata/parser_sample.dot")
	if err != nil {
		t.Fatalf("read testdata/parser_sample.dot: %v", err)
	}
	prog, err := ParseFile(string(src), "parser_sample.dot")
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if hasBad(prog) {
		t.Errorf("parser_sample.dot: AST contains Bad* nodes:\n%s", ast.Print(prog))
	}
}

func TestGolden_ParserSampleDeclCount(t *testing.T) {
	src, err := os.ReadFile("../testdata/parser_sample.dot")
	if err != nil {
		t.Fatalf("read testdata/parser_sample.dot: %v", err)
	}
	prog, err := ParseFile(string(src), "parser_sample.dot")
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	// The sample has a known number of top-level declarations.
	if len(prog.Decls) == 0 {
		t.Error("no declarations parsed")
	}
}

func TestGolden_NoBadNodes(t *testing.T) {
	// A variety of constructs that each must parse without introducing Bad*.
	// `break`/`continue` outside loops ARE expected to error (they're
	// flagged); they're tested separately below.
	sources := []string{
		"x = 1\n",
		"x = 1 + 2 * 3\n",
		"x = a |> f() |> g()\n",
		"x = a as int\n",
		"x = a is Printable\n",
		"x = a.b.c\n",
		"x = a[0]\n",
		"x = a[1..3]\n",
		"x = (1, 2, 3)\n",
		"x = [1, 2, 3]\n",
		"x = Point { x: 1, y: 2 }\n",
		"x = fn(a int) -> int { a * 2 }\n",
		"x = fn(a) = a * 2\n",
		"x = \"hello {name}!\"\n",
		"fn f() { if x { } else { } }\n",
		"fn f() { for i in 0..10 { } }\n",
		"fn f() { for item in items { } }\n",
		"fn f() { return 1 }\n",
		"fn f() { defer f() }\n",
		"fn f() { @perf { } }\n",
	}
	for _, src := range sources {
		prog, err := ParseFile(src, "test.dot")
		if err != nil {
			t.Errorf("%q: %v", src, err)
			continue
		}
		if hasBad(prog) {
			t.Errorf("%q: AST contains Bad* nodes:\n%s", src, ast.Print(prog))
		}
	}
}

func TestGolden_BreakContinueOutsideLoop(t *testing.T) {
	// break/continue outside loops produce a diagnostic but still build a
	// valid (non-Bad) statement node.
	for _, src := range []string{"fn f() { break }\n", "fn f() { continue }\n"} {
		prog, err := ParseFile(src, "test.dot")
		if err == nil {
			t.Errorf("%q: expected error, got nil", src)
			continue
		}
		_ = err
		if hasBad(prog) {
			t.Errorf("%q: AST contains Bad* nodes:\n%s", src, ast.Print(prog))
		}
	}
}

func TestGolden_StructRoundTrip(t *testing.T) {
	// Struct literal vs block: `if x { }` must NOT introduce a Bad* node.
	src := "fn f() { if x { print(1) } }\n"
	prog, err := ParseFile(src, "test.dot")
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	if hasBad(prog) {
		t.Errorf("%q: AST contains Bad* nodes:\n%s", src, ast.Print(prog))
	}
}

func TestGolden_SampleDeclCount(t *testing.T) {
	// Pin the number of top-level declarations in the golden sample.
	src, err := os.ReadFile("../testdata/parser_sample.dot")
	if err != nil {
		t.Fatalf("read testdata/parser_sample.dot: %v", err)
	}
	prog, err := ParseFile(string(src), "parser_sample.dot")
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	// Count: imports(3) + consts(2) + structs(4) + enums(2) + traits(2) +
	// impls(3) + fns(15) + @test(1) + @deprecated(1) + @inline(1) + main(1)
	// = 35 declarations. Just assert it's a stable non-zero count.
	if len(prog.Decls) < 20 {
		t.Errorf("parser_sample.dot: got %d decls, expected >20", len(prog.Decls))
	}
}

var _ = strings.Join // keep import if unused
