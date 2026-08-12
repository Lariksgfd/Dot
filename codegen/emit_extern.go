package codegen

import (
	"github.com/dotlang/dot/ast"
)

// isExtern reports whether fn carries the @extern("C") annotation,
// meaning it is declared in Dot but its body is provided by external C code.
func isExtern(fn *ast.FnDecl) bool {
	for _, ann := range fn.Annotations {
		if ann.Name != "extern" || len(ann.Args) != 1 {
			continue
		}
		if s, ok := stringLitValue(ann.Args[0]); ok && s == "C" {
			return true
		}
	}
	return false
}

// stringLitValue returns the decoded string value of e when it is a simple
// *ast.StringLit consisting of a single PartText chunk (no interpolation).
func stringLitValue(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.StringLit)
	if !ok || len(lit.Parts) != 1 || lit.Parts[0].Kind != ast.PartText {
		return "", false
	}
	return lit.Parts[0].Text, true
}

// neededRuntimeIncludes scans the program for @extern("C") functions and
// returns the list of C runtime header names that must be included.
// Each imported module that provides extern functions contributes its
// last path segment as a header name (e.g. "std.io" produces "io.h").
func neededRuntimeIncludes(prog *ast.Program) []string {
	hasExtern := false
	for _, decl := range prog.Decls {
		if fn, ok := decl.(*ast.FnDecl); ok && isExtern(fn) {
			hasExtern = true
			break
		}
	}
	if !hasExtern {
		return nil
	}
	seen := make(map[string]bool)
	var headers []string
	for _, decl := range prog.Decls {
		imp, ok := decl.(*ast.ImportDecl)
		if !ok {
			continue
		}
		n := len(imp.Path)
		if n == 0 {
			continue
		}
		h := imp.Path[n-1] + ".h"
		if seen[h] {
			continue
		}
		seen[h] = true
		headers = append(headers, h)
	}
	return headers
}
