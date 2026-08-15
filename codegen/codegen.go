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
	// fnDefs maps a closure-struct shape to its typedef info; fnDefList
	// holds the typedefs in registration order. fnEmittedIdx marks how many
	// were flushed into the type section, so a shape first rendered during
	// function-body emission can be flushed before the bodies.
	fnDefs       map[string]*fnDefInfo
	fnDefList    []*fnDefInfo
	fnEmittedIdx int
	// fnCaptures holds the C names of the variables the closure currently
	// being emitted captures (nil outside closures). Return and block-expr
	// tail values that are capture idents must deep-retain instead of
	// moving: the env owns its own reference and may die right after a call,
	// so a result aliasing env contents must keep its own.
	fnCaptures map[string]bool
	// cleanupFloors is a stack of scopeStack depths recorded at function
	// boundaries crossed while emitting a body (closure bodies, spawn
	// fibers): a return inside such a body is a return from the nested C
	// function, so return-cleanup must only walk the scopes opened inside
	// that body and never the enclosing function's (CG-34).
	cleanupFloors []int
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
	// Safety flush: closure-struct shapes first rendered while emitting
	// bodies missed the type section; emit their typedefs here, before the
	// bodies that use them.
	if g.fnEmittedIdx < len(g.fnDefList) {
		late := make([]string, 0, len(g.fnDefList)-g.fnEmittedIdx)
		for i := g.fnEmittedIdx; i < len(g.fnDefList); i++ {
			late = append(late, g.fnDefList[i].Text)
		}
		g.closureDecls = append(late, g.closureDecls...)
		g.fnEmittedIdx = len(g.fnDefList)
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
	// Closure-struct typedefs are predeclared the same way (they contain
	// only pointers, but prototypes must reference already-defined types).
	g.predeclareFnTypes()
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

// registerFnTypes pre-registers the closure-struct typedefs for every fn
// shape mentioned by t, recursively.
func registerFnTypes(g *generator, t types.Type) {
	switch x := t.(type) {
	case *types.Fn:
		cFnName(g, x)
		for _, p := range x.Params {
			registerFnTypes(g, p.Type)
		}
		registerFnTypes(g, x.Result)
	case *types.Slice:
		registerFnTypes(g, x.Elem)
	case *types.Array:
		registerFnTypes(g, x.Elem)
	case *types.Map:
		registerFnTypes(g, x.Key)
		registerFnTypes(g, x.Value)
	case *types.Pointer:
		registerFnTypes(g, x.Elem)
	case *types.Weak:
		registerFnTypes(g, x.Elem)
	case *types.Chan:
		registerFnTypes(g, x.Elem)
	case *types.Future:
		registerFnTypes(g, x.Result)
	case *types.Tuple:
		for _, e := range x.Elems {
			registerFnTypes(g, e)
		}
	case *types.Named:
		for _, a := range x.TypeArgs {
			registerFnTypes(g, a)
		}
	case *types.Struct:
		for _, f := range x.Fields {
			registerFnTypes(g, f.Type)
		}
	case *types.Enum:
		for _, v := range x.Variants {
			for _, p := range v.Fields {
				registerFnTypes(g, p.Type)
			}
		}
	case *types.TypeVar:
		if x.Bound != nil {
			registerFnTypes(g, x.Bound)
		}
	}
}

// predeclareFnTypes walks every function signature and every recorded
// expression type before any C is rendered, so that the closure-struct
// typedefs they need are registered for the header emission and function
// prototypes reference already-defined types.
func (g *generator) predeclareFnTypes() {
	for _, decl := range g.prog.Decls {
		switch d := decl.(type) {
		case *ast.FnDecl:
			if ft := g.fnType(d); ft != nil {
				registerFnTypes(g, ft)
			}
		case *ast.ImplDecl:
			recv := g.implRecvType(d)
			for _, m := range d.Methods {
				ft := g.fnType(m)
				if ft == nil && recv != nil {
					ft = g.methodType(m, recv)
				}
				if ft != nil {
					registerFnTypes(g, ft)
				}
			}
		}
	}
	for _, inst := range g.info.InstanceList {
		if inst == nil {
			continue
		}
		if f, ok := inst.Result.(*types.Fn); ok {
			registerFnTypes(g, f)
		}
	}
	for _, t := range g.info.Types {
		registerFnTypes(g, t)
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
