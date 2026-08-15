// Package codegen translates a typed Dot AST into C source code.
//
// This file implements ARC (Automatic Reference Counting) emission helpers:
// retain/release for heap-allocated types, scope cleanup for local variables,
// defer registration and execution, and arena management for @perf blocks.
package codegen

import (
	"fmt"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/types"
)

// scopeVar tracks a local variable's name and type so that emitScopeCleanup
// can release heap values when the scope exits.
type scopeVar struct {
	name string
	typ  types.Type
}

// pushScope starts a new scope level for a function or block body. The
// current scopeVars list is saved on the scope stack and reset so that
// declarations inside the new scope are tracked separately.
func (g *generator) pushScope() {
	g.scopeStack = append(g.scopeStack, g.scopeVars)
	g.scopeVars = nil
}

// popScope ends the innermost scope level, restoring the enclosing scopeVars
// list. The popped level's own variables are returned so the caller can emit
// their cleanup inside the scope's own C block.
func (g *generator) popScope() []scopeVar {
	popped := g.scopeVars
	g.scopeVars = g.scopeStack[len(g.scopeStack)-1]
	g.scopeStack = g.scopeStack[:len(g.scopeStack)-1]
	return popped
}

// shadowsEnclosing reports whether a C variable of this name is already
// visible in any enclosing scope level. A fresh declaration that shadows an
// enclosing local must evaluate its initialiser before the declaration,
// because in C the new name is already in scope inside its own initialiser.
func (g *generator) shadowsEnclosing(name string) bool {
	for _, sv := range g.scopeVars {
		if sv.name == name {
			return true
		}
	}
	for _, level := range g.scopeStack {
		for _, sv := range level {
			if sv.name == name {
				return true
			}
		}
	}
	return false
}

// emitScopeCleanupStmts emits release statements for scopeVars into the
// output buffer, one per line. It is a no-op when scopeVars is empty.
func emitScopeCleanupStmts(g *generator, scopeVars []scopeVar) {
	if len(scopeVars) == 0 {
		return
	}
	cleanup := emitScopeCleanup(g, scopeVars)
	for _, cl := range strings.Split(strings.TrimRight(cleanup, "\n"), "\n") {
		if cl != "" {
			g.line(cl)
		}
	}
}

// scopesDeepestFirst returns every active scope level (current + scopeStack)
// ordered from the deepest to the shallowest, for return-statement cleanup.
// Levels at or below the innermost cleanup floor (a function boundary crossed
// by a closure or spawn body) are excluded: a return inside such a body
// returns from the nested C function and must not touch the enclosing
// function's variables, which are not even visible there (CG-34).
func (g *generator) scopesDeepestFirst() [][]scopeVar {
	floor := 0
	if len(g.cleanupFloors) > 0 {
		floor = g.cleanupFloors[len(g.cleanupFloors)-1]
	}
	levels := make([][]scopeVar, 0, len(g.scopeStack)-floor+1)
	levels = append(levels, g.scopeVars)
	for i := len(g.scopeStack) - 1; i > floor; i-- {
		levels = append(levels, g.scopeStack[i])
	}
	return levels
}

// pushCleanupFloor records a function-boundary crossing for the body about
// to be emitted; popCleanupFloor restores the previous boundary. Nested
// bodies (a closure inside a closure) simply push deeper floors.
func (g *generator) pushCleanupFloor() {
	g.cleanupFloors = append(g.cleanupFloors, len(g.scopeStack))
}

func (g *generator) popCleanupFloor() {
	g.cleanupFloors = g.cleanupFloors[:len(g.cleanupFloors)-1]
}

// scopeBlockEndsInReturn reports whether the last statement of a block is a
// return, in which case the block's own end-cleanup is skipped because the
// return statement already cleaned the scope.
func scopeBlockEndsInReturn(stmts []ast.Stmt) bool {
	if len(stmts) == 0 {
		return false
	}
	_, isReturn := stmts[len(stmts)-1].(*ast.ReturnStmt)
	return isReturn
}

// emitIfLet emits the success condition and the payload-binding declaration
// for an if-let header (`if p = opt { ... }`). The scrutinee must be an
// Option or Result: the condition checks the success tag (Some/Ok, tag 0)
// and the binding extracts the payload field.
func emitIfLet(g *generator, bind ast.Pattern, subj string, subjType types.Type) (cond string, bindings string) {
	cond = fmt.Sprintf("((%s) != NULL && (%s)->tag == 0)", subj, subj)
	var payloadType types.Type
	var payloadField string
	if isOptionType(subjType) {
		payloadField = variantFieldName("Some", "value")
		if n, ok := subjType.(*types.Named); ok && len(n.TypeArgs) > 0 {
			payloadType = n.TypeArgs[0]
		}
	} else if isResultType(subjType) {
		payloadField = variantFieldName("Ok", "value")
		if n, ok := subjType.(*types.Named); ok && len(n.TypeArgs) > 0 {
			payloadType = n.TypeArgs[0]
		}
	}
	if payloadField == "" {
		return cond, ""
	}
	payloadExpr := fmt.Sprintf("(%s)->%s", subj, payloadField)
	switch p := bind.(type) {
	case *ast.IdentPattern:
		return cond, fmt.Sprintf("%s %s = %s;", cFieldType(g, payloadType), p.Name, payloadExpr)
	case *ast.EnumPattern:
		if len(p.Args) == 1 {
			if id, ok := p.Args[0].(*ast.IdentPattern); ok {
				return cond, fmt.Sprintf("%s %s = %s;", cFieldType(g, payloadType), id.Name, payloadExpr)
			}
		}
	}
	return cond, ""
}

// emitRetain wraps expr in a dot_retain call when t is a heap type.
// Non-heap values are returned unchanged. Every heap value (string, slice,
// map, payload-carrying enum, dyn, chan, future) is a pointer in C, including
// enums, so no address-of is ever taken here.
func emitRetain(g *generator, expr string, t types.Type) string {
	if types.IsHeap(t) {
		return fmt.Sprintf("dot_retain((DotRefcnt*)(%s))", expr)
	}
	return expr
}

// emitFnRetain materialises a closure value into a temporary, retains its env
// (closures are ARC-managed through their env, see emitFnLit) and yields the
// value. Used on the copy paths (assignment, return, tail value of a
// block-expr) so that every live closure reference keeps the captured state
// alive; scope cleanup releases one env ref per closure reference.
func emitFnRetain(g *generator, expr string, t types.Type) string {
	tmp := fmt.Sprintf("_dot_fr_%d", g.unusedIdx)
	g.unusedIdx++
	return fmt.Sprintf("({ %s %s = %s; if (%s.env != NULL) { dot_retain((DotRefcnt*)(%s.env)); } %s; })",
		cType(g, t), tmp, expr, tmp, tmp, tmp)
}

// emitRelease returns a dot_release call for heap types, or an empty string
// for non-heap values (nothing to release). Heap values are pointers in C
// (including enums), so the NULL check and the cast apply to expr directly.
func emitRelease(g *generator, expr string, t types.Type) string {
	if types.IsHeap(t) {
		return fmt.Sprintf("if (%s != NULL) { dot_release((DotRefcnt*)(%s)); }", expr, expr)
	}
	return ""
}

// emitBoxValue boxes a struct or enum value for storage in a DotAny slot
// (slice element). dot_box_struct memcpy's from the source address: struct
// values are copied from a temporary's address so the box holds a plain
// struct value; enum values are pointers, so the box stores the pointer
// itself (the box content is the enum reference, not the object).
//
// The box must own its heap members: the boxed copy is deep-retained (struct
// members are retained, enum pointers are dot_retain'd) so that releasing the
// source expression later cannot leave the box with dangling pointers.
func emitBoxValue(g *generator, expr string, t types.Type) string {
	if isEnumType(t) {
		tmp := fmt.Sprintf("_dot_box_%d", g.unusedIdx)
		g.unusedIdx++
		return fmt.Sprintf("({ %s* %s = %s; dot_retain((DotRefcnt*)(%s)); dot_box_struct(sizeof(%s*), &%s); })",
			cType(g, t), tmp, expr, tmp, cType(g, t), tmp)
	}
	tmp := fmt.Sprintf("_dot_box_%d", g.unusedIdx)
	g.unusedIdx++
	return fmt.Sprintf("({ %s %s = %s; dot_box_struct(sizeof(%s), &%s); })",
		cType(g, t), tmp, emitDeepRetain(g, expr, t), cType(g, t), tmp)
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
		// Heap enums are pointers in C (locals, params and struct fields alike),
		// so a plain retain of the pointer is the correct deep retain: the
		// payload's own references stay owned by the object itself.
		return emitRetain(g, expr, t)
	case *types.Fn:
		// Closure values are ARC-managed through their env: retaining the
		// env keeps the captured state alive for the copied reference.
		tmp := fmt.Sprintf("_dot_rtn_%d", g.unusedIdx)
		g.unusedIdx++
		return fmt.Sprintf("({ %s %s = %s; if (%s.env != NULL) { dot_retain((DotRefcnt*)(%s.env)); } %s; })",
			cType(g, t), tmp, expr, tmp, tmp, tmp)
	default:
		return emitRetain(g, expr, t)
	}
}

// needsDeepRetain reports whether emitDeepRetain performs any retain for a
// value of type t: the type has a heap component (string, slice, map,
// payload-carrying enum, closure env) reachable by value. It mirrors
// emitDeepRetain's structure so the two can never disagree.
func needsDeepRetain(t types.Type) bool {
	if t == nil {
		return false
	}
	switch x := types.Underlying(t).(type) {
	case *types.Struct:
		for _, f := range x.Fields {
			if needsDeepRetain(f.Type) {
				return true
			}
		}
		return false
	case *types.Tuple:
		for _, e := range x.Elems {
			if needsDeepRetain(e) {
				return true
			}
		}
		return false
	case *types.Enum:
		return types.IsHeap(x)
	case *types.Fn:
		return true
	default:
		return types.IsHeap(t)
	}
}

// emitDeepReleaseLines emits the statements of a deep release for expr of
// type t, one per line. A no-op when there is nothing to release.
func emitDeepReleaseLines(g *generator, expr string, t types.Type) {
	rel := emitDeepRelease(g, expr, t)
	for _, cl := range strings.Split(strings.TrimRight(rel, "\n"), "\n") {
		if cl != "" {
			g.line(cl)
		}
	}
}

// emitDeepRelease returns a newline-separated string of release calls for all
// heap-allocated types within the given value expression. For structs, tuples,
// and payload-carrying enums, it recursively walks the fields. For plain heap
// types, it delegates to emitRelease.
//
// Recursive types (e.g. Expr -> []MatchArm -> Option[Expr] -> Expr, or
// Decl -> Option[TypeNode] -> TypeNode -> Option[TypeNode]) would make the
// walk diverge. emitDeepReleaseSeen therefore carries a path-scoped set of
// already-expanded types: when a type reappears in the expansion chain, the
// walk stops there. A repeated heap enum is still released directly (it is a
// heap-allocated pointer), but its own fields are not re-walked. This bounds
// the generated code and guarantees termination for any type graph.
func emitDeepRelease(g *generator, expr string, t types.Type) string {
	return emitDeepReleaseSeen(g, expr, t, make(map[types.Type]bool))
}

func emitDeepReleaseSeen(g *generator, expr string, t types.Type, seen map[types.Type]bool) string {
	if t == nil {
		return ""
	}
	t = types.Underlying(t)
	// Cycle guard: do not expand a type that already appeared on this path.
	if seen[t] {
		if types.IsHeap(t) {
			return fmt.Sprintf("if (%s != NULL) { dot_release((DotRefcnt*)(%s)); };\n", expr, expr)
		}
		return ""
	}
	seen[t] = true
	switch x := t.(type) {
	case *types.Struct:
		var b strings.Builder
		for _, f := range x.Fields {
			fExpr := fmt.Sprintf("%s.%s", expr, cFieldName(f.Name))
			if rel := emitDeepReleaseSeen(g, fExpr, f.Type, seen); rel != "" {
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
			if rel := emitDeepReleaseSeen(g, fExpr, e, seen); rel != "" {
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
		// Payload fields are owned by the enum object itself and must only be
		// released when this reference is the last one (the object is about to
		// die). A release of a still-referenced enum must drop the pointer
		// reference only; unwrapping an Option and pushing the payload into a
		// slice retains the payload pointer but not its fields, so releasing
		// the fields here (the old behaviour) freed them while the object was
		// still alive and pointing at them — a use-after-free behind the
		// flaky bootstrap crashes (PA-06).
		b.WriteString(fmt.Sprintf("if (((DotRefcnt*)(%s))->count == 1) {\n", expr))
		b.WriteString(fmt.Sprintf("switch ((%s)->tag) {\n", expr))
		for _, v := range x.Variants {
			if len(v.Fields) == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf("case %d: {\n", v.Tag))
			for _, f := range v.Fields {
				fExpr := fmt.Sprintf("(%s)->%s", expr, variantFieldName(v.Name, f.Name))
				if rel := emitDeepReleaseSeen(g, fExpr, f.Type, seen); rel != "" {
					b.WriteString(rel)
					if !strings.HasSuffix(rel, ";\n") {
						b.WriteString(";\n")
					}
				}
			}
			b.WriteString("break;\n}\n")
		}
		b.WriteString("}\n")
		b.WriteString("}\n")
		b.WriteString(fmt.Sprintf("dot_release((DotRefcnt*)(%s));\n", expr))
		b.WriteString("}\n")
		return b.String()
	case *types.Fn:
		// Closure values release their env (which drops the captures and
		// frees itself) - see emitFnLit.
		return fmt.Sprintf("if (%s.env != NULL) { dot_release((DotRefcnt*)(%s.env)); };\n", expr, expr)
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
