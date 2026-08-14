// Package codegen translates a typed Dot AST into C source code.
//
// This file implements ARC (Automatic Reference Counting) emission helpers:
// retain/release for heap-allocated types, scope cleanup for local variables,
// defer registration and execution, and arena management for @perf blocks.
package codegen

import (
	"fmt"
	"strings"

	"github.com/dotlang/dot/types"
)

// scopeVar tracks a local variable's name and type so that emitScopeCleanup
// can release heap values when the scope exits.
type scopeVar struct {
	name string
	typ  types.Type
}

// emitRetain wraps expr in a dot_retain call when t is a heap type.
// Non-heap values are returned unchanged.
// Enum values are returned unchanged because they're already pointer types.
func emitRetain(g *generator, expr string, t types.Type) string {
	if types.IsHeap(t) {
		if isStructOrEnum(t) {
			return fmt.Sprintf("dot_retain((DotRefcnt*)(&(%s)))", expr)
		}
		return fmt.Sprintf("dot_retain((DotRefcnt*)(%s))", expr)
	}
	return expr
}

// emitRelease returns a dot_release call for heap types, or an empty string
// for non-heap values (nothing to release).
func emitRelease(g *generator, expr string, t types.Type) string {
	if types.IsHeap(t) {
		if isStructOrEnum(t) {
			return fmt.Sprintf("if (&(%s) != NULL) { dot_release((DotRefcnt*)(&(%s))); }", expr, expr)
		}
		return fmt.Sprintf("if (%s != NULL) { dot_release((DotRefcnt*)(%s)); }", expr, expr)
	}
	return ""
}

// emitScopeCleanup emits a release call for every heap-allocated local in scopeVars.
// It returns a newline-separated string of release statements (possibly empty).
func emitScopeCleanup(g *generator, scopeVars []scopeVar) string {
	var b strings.Builder
	for _, sv := range scopeVars {
		if rel := emitDeepRelease(g, sv.name, sv.typ); rel != "" {
			b.WriteString(rel)
			if !strings.HasSuffix(rel, ";\n") {
				b.WriteString(";\n")
			}
		}
	}
	return b.String()
}

// emitDeepRetain returns a newline-separated string of retain calls for all
// heap-allocated types within the given value expression, OR returns the original expression
// wrapped in dot_retain if it is a simple heap type.
// For value types (structs, tuples) that contain references, it returns a C block expression
// or comma expression that performs the retains and evaluates to the original expression.
func emitDeepRetain(g *generator, expr string, t types.Type) string {
	if t == nil {
		return expr
	}
	under := types.Underlying(t)
	switch x := under.(type) {
	case *types.Struct:
		var b strings.Builder
		tmp := fmt.Sprintf("_dot_rtn_%d", g.unusedIdx)
		g.unusedIdx++
		b.WriteString(fmt.Sprintf("({ %s %s = %s; ", cType(g, t), tmp, expr))
		for _, f := range x.Fields {
			fExpr := fmt.Sprintf("%s.%s", tmp, cFieldName(f.Name))
			ret := emitDeepRetain(g, fExpr, f.Type)
			if ret != fExpr {
				b.WriteString(fmt.Sprintf("%s; ", ret))
			}
		}
		b.WriteString(fmt.Sprintf("%s; })", tmp))
		// If nothing was actually retained, avoid the temporary block
		if b.String() == fmt.Sprintf("({ %s %s = %s; %s; })", cType(g, t), tmp, expr, tmp) {
			return expr
		}
		return b.String()
	case *types.Tuple:
		var b strings.Builder
		tmp := fmt.Sprintf("_dot_rtn_%d", g.unusedIdx)
		g.unusedIdx++
		b.WriteString(fmt.Sprintf("({ %s %s = %s; ", cType(g, t), tmp, expr))
		for i, e := range x.Elems {
			fExpr := fmt.Sprintf("%s._%d", tmp, i)
			ret := emitDeepRetain(g, fExpr, e)
			if ret != fExpr {
				b.WriteString(fmt.Sprintf("%s; ", ret))
			}
		}
		b.WriteString(fmt.Sprintf("%s; })", tmp))
		if b.String() == fmt.Sprintf("({ %s %s = %s; %s; })", cType(g, t), tmp, expr, tmp) {
			return expr
		}
		return b.String()
	case *types.Enum:
		if !types.IsHeap(x) {
			return expr
		}
		// Enums are pointers in locals, but by-value in structs?
		// Actually, enums are value types (structs in C), but IsHeap returns true if they have payload.
		// Wait, emitRetain always uses dot_retain for Enum! 
		return emitRetain(g, expr, t)
	default:
		return emitRetain(g, expr, t)
	}
}

// emitDeepRelease returns a newline-separated string of release calls for all
// heap-allocated types within the given value expression. For structs, tuples,
// and payload-carrying enums, it recursively walks the fields. For plain heap
// types, it delegates to emitRelease.
func emitDeepRelease(g *generator, expr string, t types.Type) string {
	if t == nil {
		return ""
	}
	t = types.Underlying(t)
	switch x := t.(type) {
	case *types.Struct:
		var b strings.Builder
		for _, f := range x.Fields {
			fExpr := fmt.Sprintf("%s.%s", expr, cFieldName(f.Name))
			if rel := emitDeepRelease(g, fExpr, f.Type); rel != "" {
				b.WriteString(rel)
				if !strings.HasSuffix(rel, ";\n") {
					b.WriteString(";\n")
				}
			}
		}
		return b.String()
	case *types.Tuple:
		var b strings.Builder
		for i, e := range x.Elems {
			fExpr := fmt.Sprintf("%s._%d", expr, i)
			if rel := emitDeepRelease(g, fExpr, e); rel != "" {
				b.WriteString(rel)
				if !strings.HasSuffix(rel, ";\n") {
					b.WriteString(";\n")
				}
			}
		}
		return b.String()
	case *types.Enum:
		if !types.IsHeap(x) {
			return ""
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("if (%s != NULL) {\n", expr))
		b.WriteString(fmt.Sprintf("switch ((%s)->tag) {\n", expr))
		for _, v := range x.Variants {
			if len(v.Fields) == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf("case %d: {\n", v.Tag))
			for _, f := range v.Fields {
				fExpr := fmt.Sprintf("(%s)->%s", expr, cFieldName(f.Name))
				if rel := emitDeepRelease(g, fExpr, f.Type); rel != "" {
					b.WriteString(rel)
					if !strings.HasSuffix(rel, ";\n") {
						b.WriteString(";\n")
					}
				}
			}
			b.WriteString("break;\n}\n")
		}
		b.WriteString("}\n")
		b.WriteString(fmt.Sprintf("dot_release((DotRefcnt*)(%s));\n", expr))
		b.WriteString("}\n")
		return b.String()
	default:
		if types.IsHeap(t) {
			rel := emitRelease(g, expr, t)
			if rel != "" {
				return rel + ";\n"
			}
		}
	}
	return ""
}


// emitDeferRegister emits a defer registration call. The deferred call is
// registered on the current scope's defer stack so it runs at scope exit.
func emitDeferRegister(g *generator, call string) string {
	return fmt.Sprintf("dot_defer_register(scope, %s)", call)
}

// emitDeferRun emits a call to execute all registered defers in LIFO order.
func emitDeferRun(g *generator) string {
	return "dot_defer_run(scope)"
}

// emitArenaEnter begins a @perf block by allocating a new arena with the
// given initial size in bytes.
func emitArenaEnter(g *generator) string {
	depth := g.arenaDepth
	g.arenaDepth++
	return fmt.Sprintf("DotArena* _arena_%d = dot_arena_new(1024)", depth)
}

// emitArenaExit ends a @perf block by freeing the arena.
func emitArenaExit(g *generator) string {
	g.arenaDepth--
	depth := g.arenaDepth
	return fmt.Sprintf("dot_arena_free(_arena_%d)", depth)
}

// isInArena reports whether the generator is currently inside a @perf block.
func isInArena(g *generator) bool {
	return g.arenaDepth > 0
}
