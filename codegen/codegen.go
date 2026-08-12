// Package codegen translates a typed Dot AST into C source code.
package codegen

import (
	"fmt"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/types"
)

// generator holds the state for one C code generation pass.
type generator struct {
	info       *types.Info
	prog       *ast.Program
	buf        *strings.Builder
	indent     int
	arenaDepth int
	closureIdx int
	closures   []string
	defers     []string
	scopeVars  []scopeVar
	unusedIdx  int
}

// Generate translates a typed Dot program into a complete C source string.
// info is the result of types.Check; prog is the AST root.
// Returns the C source code or an error if codegen fails.
func Generate(info *types.Info, prog *ast.Program) (string, error) {
	if info == nil {
		return "", fmt.Errorf("codegen: nil types.Info")
	}
	if prog == nil {
		return "", fmt.Errorf("codegen: nil AST program")
	}
	types.AnalyzeEscapes(prog, info)
	var b strings.Builder
	g := &generator{
		info: info,
		prog: prog,
		buf:  &b,
	}
	if err := g.emitAll(); err != nil {
		return "", err
	}
	return b.String(), nil
}

// emitAll runs every codegen phase in order and returns the first error.
func (g *generator) emitAll() error {
	phases := []func() error{
		g.emitHeader,
		g.emitTypeDefs,
		g.emitFuncDecls,
		g.emitFuncDefs,
		g.emitMain,
	}
	for _, phase := range phases {
		if err := phase(); err != nil {
			return err
		}
	}
	return nil
}

// emitHeader writes the C includes that every generated file needs.
// Extern headers discovered by neededRuntimeIncludes are appended after the
// standard runtime includes.
func (g *generator) emitHeader() error {
	g.line(`#include "dot_runtime.h"`)
	g.line(`#include <stdint.h>`)
	g.line(`#include <stdbool.h>`)
	g.line(`#include <stdlib.h>`)
	for _, h := range neededRuntimeIncludes(g.prog) {
		g.line("#include \"" + h + "\"")
	}
	for _, m := range g.info.NeededRuntime {
		g.line("#include \"" + m + ".h\"")
	}
	g.line("")
	return nil
}

// emitMain wraps top-level statements inside C main().
func (g *generator) emitMain() error {
	hasMain := false
	for _, decl := range g.prog.Decls {
		if fn, ok := decl.(*ast.FnDecl); ok && fn.Name == "main" {
			hasMain = true
			break
		}
	}
	g.line("int main(void) {")
	g.line("    async_init();")
	if hasMain {
		g.line("    Dot_main();")
	} else {
		g.line("    /* no main function defined */")
	}
	g.line("    async_run();")
	g.line("    dot_atexit_cleanup();")
	g.line("    return 0;")
	g.line("}")
	return nil
}

// resultType returns the result type of a FnSig, looking it up in
// info.Types when available, otherwise defaulting to types.Void.
func resultType(fn *ast.FnDecl) types.Type {
	if fn.Sig == nil {
		return types.Void
	}
	return types.Void
}

// line appends s to the output buffer followed by a newline.
func (g *generator) line(s string) {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
	g.buf.WriteString(s)
	g.buf.WriteByte('\n')
}

// nextClosure returns a fresh closure index and increments the counter.
func (g *generator) nextClosure() int {
	n := g.closureIdx
	g.closureIdx++
	return n
}

// addClosure records a closure source string for later emission.
func (g *generator) addClosure(src string) {
	g.closures = append(g.closures, src)
}
