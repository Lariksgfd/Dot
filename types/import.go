package types

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/parser"
)

// resolveStdlibImport resolves a stdlib import path to a parsed .dot file.
// It returns the parsed AST and the list of C runtime module names needed.
// For v0.1, only "std.xxx" imports are supported.
func resolveStdlibImport(imp *ast.ImportDecl, stdlibDir string) (*ast.Program, []string, error) {
	if imp.From {
		return nil, nil, fmt.Errorf("'from ... import' is not supported yet: %s", imp.PathString())
	}
	if len(imp.Path) < 2 || imp.Path[0] != "std" {
		return nil, nil, fmt.Errorf("unsupported import path: %q (only std.* supported in v0.1)", imp.PathString())
	}
	moduleName := imp.Path[1]
	filePath := filepath.Join(stdlibDir, moduleName+".dot")
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read stdlib module %q: %w", imp.PathString(), err)
	}
	prog, err := parser.ParseFile(string(src), filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing stdlib %q: %w", imp.PathString(), err)
	}
	cModules := stdlibModules(imp.PathString())
	return prog, cModules, nil
}

// stdlibModules returns the C runtime modules required by a stdlib import path.
// Returns nil when no C runtime module is needed (tier 1 pure Dot packages).
func stdlibModules(importPath string) []string {
	switch importPath {
	case "std.io":
		return []string{"io"}
	case "std.fs":
		return []string{"fs"}
	case "std.os":
		return []string{"os"}
	case "std.math":
		return []string{"math"}
	case "std.time":
		return []string{"time"}
	case "std.async":
		return []string{"async"}
	default:
		return nil
	}
}

// mergeImportedDecls records an imported program's declarations as top-level
// symbols. For v0.1, imported declarations are appended to the main program's
// Decls and processed by the normal collectDecls / checkBodies passes. This
// function is a no-op reserved for future multi-file user imports.
func mergeImportedDecls(info *Info, imported *ast.Program, moduleName string) {
	// Reserved for future use.
}

// isExternFn reports whether fn carries the @extern("C") annotation.
func isExternFn(fn *ast.FnDecl) bool {
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
// *ast.StringLit consisting of a single PartText chunk.
func stringLitValue(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.StringLit)
	if !ok || len(lit.Parts) != 1 || lit.Parts[0].Kind != ast.PartText {
		return "", false
	}
	return lit.Parts[0].Text, true
}

// resolveImports scans prog.Decls for ImportDecl nodes, parses the
// corresponding stdlib .dot files, appends their declarations to the program
// and removes the import nodes.
func (c *Checker) resolveImports(prog *ast.Program, stdlibDir string) {
	c.resolveImportsRecursive(prog, stdlibDir, map[string]bool{c.file: true})
}

func (c *Checker) resolveImportsRecursive(prog *ast.Program, stdlibDir string, loaded map[string]bool) {
	var newDecls []ast.Decl
	for _, d := range prog.Decls {
		imp, ok := d.(*ast.ImportDecl)
		if !ok {
			newDecls = append(newDecls, d)
			continue
		}
		
		if len(imp.Path) == 0 {
			c.errorf(imp, "empty import path")
			continue
		}
		
		isStd := len(imp.Path) >= 2 && imp.Path[0] == "std"
		
		if !isStd {
			relPath := imp.Path[0]
			if len(imp.Path) > 1 {
				relPath = filepath.Join(imp.Path...)
			}
			
			// Append .dot if it doesn't have an extension
			if filepath.Ext(relPath) == "" {
				relPath += ".dot"
			}

			dir := filepath.Dir(c.file) // fallback
			if prog.File != "" {
			    dir = filepath.Dir(prog.File)
			}
			absPath, err := filepath.Abs(filepath.Join(dir, relPath))
			if err != nil {
				c.errorf(imp, "invalid import path %q: %v", relPath, err)
				continue
			}
			if loaded[absPath] {
				continue
			}
			loaded[absPath] = true
			
			src, err := os.ReadFile(absPath)
			if err != nil {
				c.errorf(imp, "cannot read imported file %q: %v", relPath, err)
				continue
			}
			imported, err := parser.ParseFile(string(src), absPath)
			if err != nil {
				c.errorf(imp, "parsing imported file %q: %v", relPath, err)
				continue
			}
			c.resolveImportsRecursive(imported, stdlibDir, loaded)
			for _, childDecl := range imported.Decls {
				if _, isImp := childDecl.(*ast.ImportDecl); !isImp {
					newDecls = append(newDecls, childDecl)
				}
			}
			
		} else {
			stdPath := imp.PathString()
			if loaded[stdPath] {
				continue
			}
			loaded[stdPath] = true
			
			imported, cMods, err := resolveStdlibImport(imp, stdlibDir)
			if err != nil {
				c.errorf(imp, "%s", err.Error())
				continue
			}
			for _, m := range cMods {
				c.info.NeededRuntime = appendUnique(c.info.NeededRuntime, m)
			}
			for _, childDecl := range imported.Decls {
				if _, isImp := childDecl.(*ast.ImportDecl); !isImp {
					newDecls = append(newDecls, childDecl)
				}
			}
		}
	}
	prog.Decls = newDecls
}

// appendUnique appends s to dst if it is not already present.
func appendUnique(dst []string, s string) []string {
	for _, e := range dst {
		if e == s {
			return dst
		}
	}
	return append(dst, s)
}
