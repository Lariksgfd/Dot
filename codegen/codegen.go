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
	// scopeStack holds the scopeVars lists of enclosing scopes while a block
	// (or function) body is emitted, deepest level at the top. A return
	// statement walks the stack to release every live scope, and block exits
	// pop their level so each variable is cleaned exactly where declared.
	scopeStack [][]scopeVar
	unusedIdx  int
	tupleDefs  map[string]*tupleInfo
	tupleList  []*tupleInfo
	// closureDecls holds prototypes (and env typedefs) of closure functions
	// and spawn fibers, discovered while emitting function bodies. They are
	// printed before the bodies that reference them.
	closureDecls []string
	// genericTypes maps the C name of a generic Named type instantiation
	// that has no Info.InstanceList entry to its instantiated *Named, in
	// deterministic order (genericTypeList).
	genericTypes    map[string]*types.Named
	genericTypeList []string
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
		g.emitMonomorphisedTypeDecls,
		g.emitTypeDefs,
		g.emitMonomorphisedDecls,
		g.emitTopLevelConsts,
		g.emitFuncDecls,
		g.emitFunctionBodies,
		g.emitMain,
	}
	for _, phase := range phases {
		if err := phase(); err != nil {
			return err
		}
	}
	return nil
}

// emitFunctionBodies emits the monomorphised and user function bodies. While
// bodies are emitted, closures and spawn fibers are discovered; their
// prototypes and env typedefs are printed first, then the bodies, then the
// helper definitions, so that every reference has a prior declaration.
func (g *generator) emitFunctionBodies() error {
	var bodies strings.Builder
	saved := g.buf
	g.buf = &bodies
	errMono := g.emitMonomorphised()
	errDefs := g.emitFuncDefs()
	g.buf = saved
	if errMono != nil {
		return errMono
	}
	if errDefs != nil {
		return errDefs
	}
	for _, d := range g.closureDecls {
		g.line(d)
	}
	if len(g.closureDecls) > 0 {
		g.line("")
	}
	g.buf.WriteString(bodies.String())
	for _, src := range g.closures {
		g.buf.WriteString(src)
		g.buf.WriteByte('\n')
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
	// Tuple typedefs are emitted by emitSortedTypeDefs (emit_types.go) in
	// dependency order together with the named types; predeclareTuples only
	// registers the shapes early so they are known before any C is rendered.
	g.predeclareTuples()
	return nil
}

// predeclareTuples walks every function signature before any C is emitted so
// that tuple typedefs are declared in the header even though tuple shapes are
// only discovered when types are rendered.
func (g *generator) predeclareTuples() {
	for _, decl := range g.prog.Decls {
		switch d := decl.(type) {
		case *ast.FnDecl:
			if ft := g.fnType(d); ft != nil {
				registerTupleTypes(g, ft.Result)
				for _, p := range ft.Params {
					registerTupleTypes(g, p.Type)
				}
			}
		case *ast.ImplDecl:
			recv := g.implRecvType(d)
			for _, m := range d.Methods {
				ft := g.fnType(m)
				if ft == nil && recv != nil {
					ft = g.methodType(m, recv)
				}
				if ft == nil {
					continue
				}
				registerTupleTypes(g, ft.Result)
				for _, p := range ft.Params {
					registerTupleTypes(g, p.Type)
				}
			}
		}
	}
}

// registerTupleTypes registers the tuple shapes mentioned by t, recursively
// for the composite types that can contain them.
func registerTupleTypes(g *generator, t types.Type) {
	switch x := t.(type) {
	case *types.Tuple:
		cTupleName(g, x)
	case *types.Slice:
		registerTupleTypes(g, x.Elem)
	case *types.Array:
		registerTupleTypes(g, x.Elem)
	case *types.Map:
		registerTupleTypes(g, x.Key)
		registerTupleTypes(g, x.Value)
	case *types.Pointer:
		registerTupleTypes(g, x.Elem)
	case *types.Weak:
		registerTupleTypes(g, x.Elem)
	case *types.Fn:
		for _, p := range x.Params {
			registerTupleTypes(g, p.Type)
		}
		registerTupleTypes(g, x.Result)
	case *types.Named:
		for _, a := range x.TypeArgs {
			registerTupleTypes(g, a)
		}
	}
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
