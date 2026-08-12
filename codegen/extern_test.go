package codegen

import (
	"testing"

	"github.com/dotlang/dot/ast"
)

func makeStringLit(s string) *ast.StringLit {
	return &ast.StringLit{
		Parts: []ast.StringPart{
			{Kind: ast.PartText, Text: s},
		},
	}
}

func makeAnnotatedFn(name string, annots []*ast.Annotation) *ast.FnDecl {
	return &ast.FnDecl{
		Name:        name,
		Annotations: annots,
	}
}

func TestIsExtern(t *testing.T) {
	tests := []struct {
		name string
		fn   *ast.FnDecl
		want bool
	}{
		{
			name: "extern C annotation",
			fn: makeAnnotatedFn("foo", []*ast.Annotation{
				{Name: "extern", Args: []ast.Expr{makeStringLit("C")}},
			}),
			want: true,
		},
		{
			name: "no annotations",
			fn:   makeAnnotatedFn("bar", nil),
			want: false,
		},
		{
			name: "empty annotations",
			fn:   makeAnnotatedFn("baz", []*ast.Annotation{}),
			want: false,
		},
		{
			name: "different annotation",
			fn: makeAnnotatedFn("qux", []*ast.Annotation{
				{Name: "inline"},
			}),
			want: false,
		},
		{
			name: "test annotation",
			fn: makeAnnotatedFn("testFn", []*ast.Annotation{
				{Name: "test"},
			}),
			want: false,
		},
		{
			name: "extern with non-C arg",
			fn: makeAnnotatedFn("bad", []*ast.Annotation{
				{Name: "extern", Args: []ast.Expr{makeStringLit("Rust")}},
			}),
			want: false,
		},
		{
			name: "extern with no args",
			fn: makeAnnotatedFn("noarg", []*ast.Annotation{
				{Name: "extern"},
			}),
			want: false,
		},
		{
			name: "extern with multiple args",
			fn: makeAnnotatedFn("multi", []*ast.Annotation{
				{Name: "extern", Args: []ast.Expr{makeStringLit("C"), makeStringLit("X")}},
			}),
			want: false,
		},
		{
			name: "extern C as second annotation",
			fn: makeAnnotatedFn("multiAnnot", []*ast.Annotation{
				{Name: "inline"},
				{Name: "extern", Args: []ast.Expr{makeStringLit("C")}},
			}),
			want: true,
		},
		{
			name: "extern with integer arg",
			fn: makeAnnotatedFn("intarg", []*ast.Annotation{
				{Name: "extern", Args: []ast.Expr{&ast.IntLit{Value: 42}}},
			}),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isExtern(tt.fn)
			if got != tt.want {
				t.Errorf("isExtern(%q) = %v, want %v", tt.fn.Name, got, tt.want)
			}
		})
	}
}

func TestNeededRuntimeIncludes(t *testing.T) {
	externFn := makeAnnotatedFn("externRead", []*ast.Annotation{
		{Name: "extern", Args: []ast.Expr{makeStringLit("C")}},
	})

	regularFn := &ast.FnDecl{Name: "regular"}

	makeImport := func(path string) *ast.ImportDecl {
		segments := splitPath(path)
		return &ast.ImportDecl{Path: segments}
	}

	tests := []struct {
		name  string
		decls []ast.Decl
		want  []string
	}{
		{
			name:  "empty program",
			decls: nil,
			want:  nil,
		},
		{
			name: "no extern functions",
			decls: []ast.Decl{
				regularFn,
				makeImport("std.io"),
			},
			want: nil,
		},
		{
			name: "single extern with import",
			decls: []ast.Decl{
				externFn,
				makeImport("std.io"),
			},
			want: []string{"io.h"},
		},
		{
			name: "extern with multiple imports",
			decls: []ast.Decl{
				externFn,
				makeImport("std.io"),
				makeImport("std.math"),
				makeImport("std.fs"),
			},
			want: []string{"io.h", "math.h", "fs.h"},
		},
		{
			name: "duplicate imports deduplicated",
			decls: []ast.Decl{
				externFn,
				makeImport("std.io"),
				makeImport("std.io"),
				makeImport("std.math"),
			},
			want: []string{"io.h", "math.h"},
		},
		{
			name: "extern with no imports",
			decls: []ast.Decl{
				externFn,
				regularFn,
			},
			want: nil,
		},
		{
			name: "extern with empty path import",
			decls: []ast.Decl{
				externFn,
				&ast.ImportDecl{Path: nil},
			},
			want: nil,
		},
		{
			name: "extern with top-level import (single segment)",
			decls: []ast.Decl{
				externFn,
				&ast.ImportDecl{Path: []string{"foo"}},
			},
			want: []string{"foo.h"},
		},
		{
			name: "mixed: non-extern import after extern",
			decls: []ast.Decl{
				regularFn,
				makeImport("std.time"),
				externFn,
				makeImport("std.os"),
			},
			want: []string{"time.h", "os.h"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog := &ast.Program{Decls: tt.decls}
			got := neededRuntimeIncludes(prog)

			if tt.want == nil && got != nil {
				t.Fatalf("neededRuntimeIncludes() = %v, want nil", got)
			}
			if tt.want != nil && got == nil {
				t.Fatalf("neededRuntimeIncludes() = nil, want %v", tt.want)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("neededRuntimeIncludes() = %v (len=%d), want %v (len=%d)", got, len(got), tt.want, len(tt.want))
			}
			for i, h := range got {
				if h != tt.want[i] {
					t.Errorf("neededRuntimeIncludes()[%d] = %q, want %q", i, h, tt.want[i])
				}
			}
		})
	}
}

func splitPath(s string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func TestStringLitValue(t *testing.T) {
	tests := []struct {
		name  string
		expr  ast.Expr
		val   string
		valid bool
	}{
		{
			name:  "simple string",
			expr:  makeStringLit("hello"),
			val:   "hello",
			valid: true,
		},
		{
			name:  "empty string",
			expr:  makeStringLit(""),
			val:   "",
			valid: true,
		},
		{
			name:  "int lit not string",
			expr:  &ast.IntLit{Value: 42},
			val:   "",
			valid: false,
		},
		{
			name: "interpolated string",
			expr: &ast.StringLit{
				Parts: []ast.StringPart{
					{Kind: ast.PartText, Text: "hello "},
					{Kind: ast.PartExpr},
					{Kind: ast.PartText, Text: " world"},
				},
			},
			val:   "",
			valid: false,
		},
		{
			name:  "nil expr",
			expr:  nil,
			val:   "",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := stringLitValue(tt.expr)
			if ok != tt.valid {
				t.Errorf("stringLitValue() ok = %v, want %v", ok, tt.valid)
			}
			if val != tt.val {
				t.Errorf("stringLitValue() val = %q, want %q", val, tt.val)
			}
		})
	}
}
