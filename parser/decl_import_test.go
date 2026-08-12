package parser

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/ast"
)

func TestImportDecl_Simple(t *testing.T) {
	prog := mustParse(t, "import math_utils\n")
	im, ok := prog.Decls[0].(*ast.ImportDecl)
	if !ok {
		t.Fatalf("decl = %T, want ImportDecl", prog.Decls[0])
	}
	if len(im.Path) != 1 || im.Path[0] != "math_utils" {
		t.Errorf("Path = %v, want [math_utils]", im.Path)
	}
}

func TestImportDecl_Dotted(t *testing.T) {
	prog := mustParse(t, "import net.http\n")
	im := prog.Decls[0].(*ast.ImportDecl)
	if im.PathString() != "net.http" {
		t.Errorf("PathString = %q, want net.http", im.PathString())
	}
}

func TestImportDecl_Alias(t *testing.T) {
	prog := mustParse(t, "import net.http as web\n")
	im := prog.Decls[0].(*ast.ImportDecl)
	if im.Alias != "web" {
		t.Errorf("Alias = %q, want web", im.Alias)
	}
}

func TestImportDecl_From(t *testing.T) {
	prog := mustParse(t, "from os.fs import read_file, write_file\n")
	im := prog.Decls[0].(*ast.ImportDecl)
	if !im.From {
		t.Errorf("From = false, want true")
	}
	if len(im.Path) != 2 || im.Path[0] != "os" || im.Path[1] != "fs" {
		t.Errorf("Path = %v, want [os fs]", im.Path)
	}
	if len(im.Names) != 2 {
		t.Fatalf("Names = %d, want 2", len(im.Names))
	}
	if im.Names[0].Name != "read_file" {
		t.Errorf("name[0] = %q, want read_file", im.Names[0].Name)
	}
}

func TestImportDecl_FromWithAlias(t *testing.T) {
	prog := mustParse(t, "from os import fs as myfs\n")
	im := prog.Decls[0].(*ast.ImportDecl)
	if len(im.Names) != 1 {
		t.Fatalf("Names = %d, want 1", len(im.Names))
	}
	if im.Names[0].Alias != "myfs" {
		t.Errorf("name[0].Alias = %q, want myfs", im.Names[0].Alias)
	}
}

func TestDecl_AllConstructsParse(t *testing.T) {
	// Sanity: a file with one of each decl kind parses with no errors.
	src := strings.Join([]string{
		"import a.b",
		"const X = 1",
		"fn f() { }",
		"struct S { x int }",
		"enum E { A B }",
		"trait T { fn m(self) -> int }",
		"impl S { fn new() -> S { } }",
		"impl T for S { fn m(self) -> int = 0 }",
	}, "\n") + "\n"
	prog := mustParse(t, src)
	if len(prog.Decls) != 8 {
		t.Errorf("got %d decls, want 8", len(prog.Decls))
	}
}
