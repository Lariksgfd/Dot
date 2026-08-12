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
	if _, ok := t.(*types.Enum); ok {
		return expr
	}
	if types.IsHeap(t) {
		return fmt.Sprintf("dot_retain((DotRefcnt*)(%s))", expr)
	}
	return expr
}

// emitRelease returns a dot_release call for heap types, or an empty string
// for non-heap values (nothing to release).
func emitRelease(g *generator, expr string, t types.Type) string {
	if types.IsHeap(t) {
		return fmt.Sprintf("dot_release((DotRefcnt*)(%s))", expr)
	}
	return ""
}

// emitScopeCleanup emits a release call for every heap-allocated local in scopeVars.
// It returns a newline-separated string of release statements (possibly empty).
func emitScopeCleanup(g *generator, scopeVars []scopeVar) string {
	var b strings.Builder
	for _, sv := range scopeVars {
		if rel := emitRelease(g, sv.name, sv.typ); rel != "" {
			b.WriteString(rel)
			b.WriteString(";\n")
		}
	}
	return b.String()
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
