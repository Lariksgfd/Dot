package types

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dotlang/dot/ast"
)

func projectRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(f))
}

func stdlibDir() string {
	return filepath.Join(projectRoot(), "stdlib")
}

func TestStdlibModules(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"std.io", []string{"io"}},
		{"std.fs", []string{"fs"}},
		{"std.os", []string{"os"}},
		{"std.math", []string{"math"}},
		{"std.time", []string{"time"}},
		{"std.unknown", nil},
		{"std.collections", nil},
		{"", nil},
		{"net.http", nil},
		{"something.else", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := stdlibModules(tt.path)
			if tt.want == nil && got != nil {
				t.Fatalf("stdlibModules(%q) = %v, want nil", tt.path, got)
			}
			if tt.want != nil && got == nil {
				t.Fatalf("stdlibModules(%q) = nil, want %v", tt.path, tt.want)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("stdlibModules(%q) = %v (len=%d), want %v (len=%d)", tt.path, got, len(got), tt.want, len(tt.want))
			}
			for i, m := range got {
				if m != tt.want[i] {
					t.Errorf("stdlibModules(%q)[%d] = %q, want %q", tt.path, i, m, tt.want[i])
				}
			}
		})
	}
}

func TestCheckWithImports_HelloStdlib(t *testing.T) {
	prog := &ast.Program{
		Decls: []ast.Decl{
			&ast.ImportDecl{Path: []string{"std", "io"}},
			&ast.FnDecl{
				Name: "main",
				Sig: &ast.FnSig{
					Params: nil,
					Result: nil,
				},
				Body: &ast.BlockStmt{
					Stmts: []ast.Stmt{
						&ast.ExprStmt{
							X: &ast.CallExpr{
								Fn: &ast.Ident{Name: "print_str"},
								Args: []ast.Arg{
									{Value: &ast.StringLit{
										Parts: []ast.StringPart{
											{Kind: ast.PartText, Text: "Hello, stdlib!"},
										},
									}},
								},
							},
						},
					},
				},
			},
		},
	}

	info, err := CheckWithImports(prog, "test.dot", stdlibDir())
	if err != nil {
		t.Logf("CheckWithImports produced diagnostics (may be expected for unresolved extern): %v", err)
	}
	if info == nil {
		t.Fatal("CheckWithImports returned nil Info")
	}

	hasIO := false
	for _, m := range info.NeededRuntime {
		if m == "io" {
			hasIO = true
			break
		}
	}
	if !hasIO {
		t.Errorf("expected NeededRuntime to contain 'io', got %v", info.NeededRuntime)
	}
}

func TestCheckWithImports_NoImports(t *testing.T) {
	prog := &ast.Program{
		Decls: []ast.Decl{
			&ast.FnDecl{
				Name: "main",
				Sig: &ast.FnSig{
					Params: nil,
					Result: nil,
				},
				Body: &ast.BlockStmt{
					Stmts: []ast.Stmt{
						&ast.ExprStmt{
							X: &ast.CallExpr{
								Fn: &ast.Ident{Name: "print"},
								Args: []ast.Arg{
									{Value: &ast.StringLit{
										Parts: []ast.StringPart{
											{Kind: ast.PartText, Text: "hello"},
										},
									}},
								},
							},
						},
					},
				},
			},
		},
	}

	info, err := CheckWithImports(prog, "test_noimports.dot", stdlibDir())
	if err != nil {
		t.Fatalf("CheckWithImports failed: %v", err)
	}
	if info == nil {
		t.Fatal("CheckWithImports returned nil Info")
	}
	if len(info.NeededRuntime) != 0 {
		t.Errorf("expected empty NeededRuntime, got %v", info.NeededRuntime)
	}
}

func TestCheck_Basic(t *testing.T) {
	prog := &ast.Program{
		Decls: []ast.Decl{
			&ast.FnDecl{
				Name: "main",
				Sig: &ast.FnSig{
					Params: nil,
					Result: nil,
				},
				Body: &ast.BlockStmt{
					Stmts: []ast.Stmt{},
				},
			},
		},
	}

	info, err := Check(prog, "test_basic.dot")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info == nil {
		t.Fatal("Check returned nil Info")
	}
}

func TestImportValidation_UnsupportedPath(t *testing.T) {
	tests := []struct {
		name string
		path []string
	}{
		{"non-std", []string{"net", "http"}},
		{"single segment", []string{"foo"}},
		{"empty", nil},
		{"deep path", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog := &ast.Program{
				Decls: []ast.Decl{
					&ast.ImportDecl{Path: tt.path},
					&ast.FnDecl{
						Name: "main",
						Sig:  &ast.FnSig{Params: nil, Result: nil},
						Body: &ast.BlockStmt{Stmts: nil},
					},
				},
			}
			info, err := CheckWithImports(prog, "test_val.dot", stdlibDir())
			if info == nil {
				t.Fatal("CheckWithImports returned nil Info")
			}
			if err == nil {
				t.Errorf("expected error for invalid import path %v, got nil", tt.path)
			}
		})
	}
}

func TestImportValidation_FromImport(t *testing.T) {
	prog := &ast.Program{
		Decls: []ast.Decl{
			&ast.ImportDecl{
				From: true,
				Path: []string{"std", "io"},
				Names: []ast.ImportName{
					{Name: "print_str"},
				},
			},
			&ast.FnDecl{
				Name: "main",
				Sig:  &ast.FnSig{Params: nil, Result: nil},
				Body: &ast.BlockStmt{Stmts: nil},
			},
		},
	}
	info, err := CheckWithImports(prog, "test_from.dot", stdlibDir())
	if info == nil {
		t.Fatal("CheckWithImports returned nil Info")
	}
	if err == nil {
		t.Errorf("expected error for 'from ... import', got nil")
	}
}

func TestResolveStdlibImport_MissingFile(t *testing.T) {
	imp := &ast.ImportDecl{Path: []string{"std", "nonexistent_xyz"}}
	_, _, err := resolveStdlibImport(imp, stdlibDir())
	if err == nil {
		t.Errorf("expected error for missing stdlib file, got nil")
	}
}

func TestIsExternFn(t *testing.T) {
	tests := []struct {
		name string
		fn   *ast.FnDecl
		want bool
	}{
		{
			name: "extern C",
			fn: &ast.FnDecl{
				Name: "f",
				Annotations: []*ast.Annotation{
					{Name: "extern", Args: []ast.Expr{&ast.StringLit{
						Parts: []ast.StringPart{{Kind: ast.PartText, Text: "C"}},
					}}},
				},
			},
			want: true,
		},
		{
			name: "no annotations",
			fn:   &ast.FnDecl{Name: "g"},
			want: false,
		},
		{
			name: "different annotation",
			fn: &ast.FnDecl{
				Name: "h",
				Annotations: []*ast.Annotation{
					{Name: "test"},
				},
			},
			want: false,
		},
		{
			name: "extern with wrong arg",
			fn: &ast.FnDecl{
				Name: "i",
				Annotations: []*ast.Annotation{
					{Name: "extern", Args: []ast.Expr{&ast.StringLit{
						Parts: []ast.StringPart{{Kind: ast.PartText, Text: "Go"}},
					}}},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isExternFn(tt.fn)
			if got != tt.want {
				t.Errorf("isExternFn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringLitValue_TypeCheck(t *testing.T) {
	sl := &ast.StringLit{
		Parts: []ast.StringPart{
			{Kind: ast.PartText, Text: "hello"},
		},
	}
	val, ok := stringLitValue(sl)
	if !ok {
		t.Fatal("stringLitValue returned false for simple string")
	}
	if val != "hello" {
		t.Errorf("stringLitValue() = %q, want %q", val, "hello")
	}

	interpSl := &ast.StringLit{
		Parts: []ast.StringPart{
			{Kind: ast.PartText, Text: "a"},
			{Kind: ast.PartExpr},
		},
	}
	_, ok = stringLitValue(interpSl)
	if ok {
		t.Error("stringLitValue returned true for interpolated string")
	}

	_, ok = stringLitValue(&ast.IntLit{Value: 42})
	if ok {
		t.Error("stringLitValue returned true for IntLit")
	}
}

func TestMergeImportedDecls_NoOp(t *testing.T) {
	info := &Info{}
	prog := &ast.Program{
		Decls: []ast.Decl{
			&ast.FnDecl{Name: "hi"},
		},
	}
	mergeImportedDecls(info, prog, "testmod")
}

func TestIsExternFn_DuplicateInCodegen(t *testing.T) {
	sl := &ast.StringLit{
		Parts: []ast.StringPart{{Kind: ast.PartText, Text: "C"}},
	}
	fn := &ast.FnDecl{
		Name: "close",
		Annotations: []*ast.Annotation{
			{Name: "extern", Args: []ast.Expr{sl}},
		},
	}
	if !isExternFn(fn) {
		t.Error("isExternFn should return true for @extern(\"C\")")
	}
}

func TestAppendUnique(t *testing.T) {
	tests := []struct {
		name string
		dst  []string
		s    string
		want []string
	}{
		{"empty dst", nil, "a", []string{"a"}},
		{"new element", []string{"a", "b"}, "c", []string{"a", "b", "c"}},
		{"duplicate", []string{"a", "b"}, "a", []string{"a", "b"}},
		{"duplicate single", []string{"x"}, "x", []string{"x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendUnique(tt.dst, tt.s)
			if len(got) != len(tt.want) {
				t.Fatalf("appendUnique() len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("appendUnique()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
