// Package codegen translates a typed Dot AST into C source code.
//
// This file holds the heavier statement-codegen helpers that did not fit in
// emit_stmt.go: for loops (all four forms), return, break/continue, defer,
// perf blocks, block statements with scope cleanup, and small name helpers.
package codegen

import (
	"fmt"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/types"
)

// emitForStmt emits a for loop. Infinite loops become for(;;), conditional
// loops become while, and ForIn loops emit a range or iterator form. Labeled
// loops emit a continue label before and a break label after.
func (g *generator) emitForStmt(x *ast.ForStmt) {
	if x.Label != "" {
		g.line(fmt.Sprintf("DotLabel_%s_continue:", x.Label))
	}
	switch x.Kind {
	case ast.ForInfinite:
		g.line("for (;;) {")
		g.indent++
		g.emitScopedBlockStmts(x.Body)
		g.indent--
		g.line("}")
	case ast.ForCond:
		cond := g.emitExpr(x.Cond)
		g.line(fmt.Sprintf("while (%s) {", cond))
		g.indent++
		g.emitScopedBlockStmts(x.Body)
		g.indent--
		g.line("}")
	case ast.ForIn:
		g.emitForIn(x)
	}
	if x.Label != "" {
		g.line(fmt.Sprintf("DotLabel_%s_break:", x.Label))
	}
}

// emitForIn emits a range or iterator-based for loop.
func (g *generator) emitForIn(x *ast.ForStmt) {
	if rangeExpr, ok := x.Iterable.(*ast.RangeExpr); ok {
		g.emitForRange(x, rangeExpr)
		return
	}
	g.emitForIterator(x)
}

// emitForRange emits `for i in lo..hi` as a counted C for loop.
// The counter is a private C variable; the loop variable is a fresh copy
// declared per iteration, so reassigning it in the body does not affect
// iteration (D63: loop variables are fresh declarations in the loop's scope).
func (g *generator) emitForRange(x *ast.ForStmt, r *ast.RangeExpr) {
	vn := varName(g, x.Value)
	lo := "0"
	if r.Low != nil {
		lo = g.emitExpr(r.Low)
	}
	hi := "0"
	if r.High != nil {
		hi = g.emitExpr(r.High)
	}
	op := "<"
	if r.Inclusive {
		op = "<="
	}
	counter := fmt.Sprintf("_dot_range_%d", g.unusedIdx)
	g.unusedIdx++
	g.line(fmt.Sprintf("for (int64_t %s = %s; %s %s %s; %s++) {",
		counter, lo, counter, op, hi, counter))
	g.indent++
	g.line(fmt.Sprintf("int64_t %s = %s;", vn, counter))
	g.emitScopedBlockStmts(x.Body)
	g.indent--
	g.line("}")
}

// emitForIterator emits `for i, item in items` as an iterator-based loop.
// The iterator runtime writes a DotAny per element into caller-provided
// slots, so the loop variable is copied out of that slot on every iteration:
// struct/enum elements live in a box and are dereferenced, pointers and
// scalars are cast directly. The whole loop is wrapped in a block so loop
// variables do not clash across sibling loops in the same function.
func (g *generator) emitForIterator(x *ast.ForStmt) {
	iterable := g.emitExpr(x.Iterable)
	vn := varName(g, x.Value)
	iterType := g.info.TypeOf(x.Iterable)
	iterFn := "dot_iter_slice"
	if types.Underlying(iterType) != nil {
		if _, isMap := types.Underlying(iterType).(*types.Map); isMap {
			iterFn = "dot_iter_map"
		}
	}
	elemType := g.info.TypeOf(x.Value)
	if elemType == nil || elemType == types.Invalid {
		if sl, ok := types.Underlying(iterType).(*types.Slice); ok {
			elemType = sl.Elem
		}
	}

	g.line("{")
	g.indent++
	itIdx := g.unusedIdx
	g.unusedIdx++
	g.line(fmt.Sprintf("DotIter _dot_it_%d = %s(%s);", itIdx, iterFn, iterable))
	keyExpr := "NULL"
	if x.Key != nil {
		kn := varName(g, x.Key)
		keyType := g.info.TypeOf(x.Key)
		g.line(fmt.Sprintf("%s %s = 0;", cType(g, keyType), kn))
		keyExpr = "&" + kn
	}
	ivName := fmt.Sprintf("_dot_iv_%d", itIdx)
	g.line(fmt.Sprintf("DotAny %s = NULL;", ivName))
	g.line(fmt.Sprintf("while (dot_iter_next(&_dot_it_%d, %s, &%s)) {", itIdx, keyExpr, ivName))
	g.indent++
	elemCType := cFieldType(g, elemType)
	var init string
	if isBoxedElem(elemType) {
		// Boxed elements carry a DotRefcnt header before the payload
		// (dot_box_struct): skip it to reach the stored value/pointer.
		init = fmt.Sprintf("*(%s*)dot_box_payload(%s)", elemCType, ivName)
	} else {
		init = fmt.Sprintf("(%s)%s", elemCType, ivName)
	}
	g.line(fmt.Sprintf("%s %s = %s;", elemCType, vn, init))
	g.emitScopedBlockStmts(x.Body)
	g.indent--
	g.line("}")
	g.indent--
	g.line("}")
}

// emitReturnStmt emits a return statement. Defers run before the return.
// The return value is materialised into a temporary BEFORE scope cleanup
// runs: cleanup frees local heap values (enums included), and a return
// expression referencing them (a struct literal holding enum/string fields,
// a call on a local, ...) must be evaluated and take its owns while they
// are still alive.
func (g *generator) emitReturnStmt(x *ast.ReturnStmt) {
	g.emitDefers()

	// Collect returned variables to implement move-semantics (ownership transfer)
	returnedVars := make(map[string]bool)
	for _, v := range x.Values {
		if id, ok := v.(*ast.Ident); ok {
			returnedVars[varName(g, id)] = true
		}
	}

	// Emit scope cleanup for every live scope (deepest first) EXCEPT the
	// variables being returned (move semantics). Outer variables whose C name
	// is shadowed by a deeper declaration are skipped at the shallow levels:
	// at the return point the C name refers to the deepest variable, so
	// releasing it here would release the wrong object. The enclosing blocks
	// still emit their own end-cleanup for the non-return paths.
	var toCleanup []scopeVar
	shadowed := make(map[string]bool)
	for _, level := range g.scopesDeepestFirst() {
		for _, sv := range level {
			if returnedVars[sv.name] || shadowed[sv.name] {
				continue
			}
			toCleanup = append(toCleanup, sv)
		}
		for _, sv := range level {
			shadowed[sv.name] = true
		}
	}

	if len(x.Values) == 0 {
		emitScopeCleanupStmts(g, toCleanup)
		g.line("return;")
		return
	}

	// returnRetVal emits one value expression into a temp, applying the same
	// ownership rules the old code used: moved identifiers are not retained,
	// heap lvalues are retained, everything else is transferred as-is.
	// Capture idents are the exception: the env keeps its own reference and
	// may be released right after the call, so the returned value must deep-
	// retain instead of moving the env's reference.
	returnRetVal := func(v ast.Expr, useRetain func(expr string, t types.Type) string) string {
		val := g.emitExpr(v)
		t := g.info.TypeOf(v)
		if id, ok := v.(*ast.Ident); ok {
			if g.fnCaptures[varName(g, id)] {
				return emitDeepRetain(g, val, t)
			}
			if returnedVars[varName(g, id)] {
				return val
			}
		}
		return useRetain(val, t)
	}

	values := x.Values
	if len(values) == 1 {
		// A single parenthesised tuple (`return (7, "ok")`) is a TupleLit
		// (possibly wrapped in ParenExpr), not a multi-value return. Pack it
		// element-wise so heap elements are retained individually; retaining
		// the whole tuple would cast a struct value to a pointer.
		if tup, ok := returnTuple(x.Values[0]); ok {
			values = make([]ast.Expr, len(tup.Elems))
			for i, e := range tup.Elems {
				values[i] = e.Value
			}
		} else {
			t := g.info.TypeOf(x.Values[0])
			if t != nil && (t == types.Void || t.Kind() == types.KindVoid) {
				g.line(fmt.Sprintf("%s;", g.emitExpr(x.Values[0])))
				emitScopeCleanupStmts(g, toCleanup)
				g.line("return;")
				return
			}
			// Enums are pointers: the old code transferred them without an
			// extra retain regardless of the source, keep that rule.
			useRetain := func(val string, t types.Type) string {
				if isEnumType(t) {
					return val
				}
				if isLValue(x.Values[0]) {
					if _, isFn := t.(*types.Fn); isFn {
						return emitFnRetain(g, val, t)
					}
					return emitRetain(g, val, t)
				}
				return val
			}
			tmp := fmt.Sprintf("_dot_ret_%d", g.unusedIdx)
			g.unusedIdx++
			g.line(fmt.Sprintf("%s %s = %s;", cFieldType(g, t), tmp, returnRetVal(x.Values[0], useRetain)))
			emitScopeCleanupStmts(g, toCleanup)
			g.line(fmt.Sprintf("return %s;", tmp))
			return
		}
	}
	// Multi-value return: pack into a tuple struct before cleanup.
	elems := make([]types.Type, len(values))
	fields := make([]string, len(values))
	for i, v := range values {
		vt := g.info.TypeOf(v)
		elems[i] = vt
		exprVal := returnRetVal(v, func(val string, t types.Type) string {
			if isLValue(v) {
				if _, isFn := t.(*types.Fn); isFn {
					return emitFnRetain(g, val, t)
				}
				return emitRetain(g, val, t)
			}
			return val
		})
		fields[i] = fmt.Sprintf(". _%d = %s", i, exprVal)
	}
	tupName := cTupleName(g, &types.Tuple{Elems: elems})
	tmp := fmt.Sprintf("_dot_ret_%d", g.unusedIdx)
	g.unusedIdx++
	g.line(fmt.Sprintf("%s %s = (%s){%s };", tupName, tmp, tupName, strings.Join(fields, ", ")))
	emitScopeCleanupStmts(g, toCleanup)
	g.line(fmt.Sprintf("return %s;", tmp))
}

// returnTuple reports whether e is a tuple literal, unwrapping parentheses.
func returnTuple(e ast.Expr) (*ast.TupleLit, bool) {
	for {
		switch t := e.(type) {
		case *ast.TupleLit:
			return t, true
		case *ast.ParenExpr:
			e = t.X
		default:
			return nil, false
		}
	}
}

// emitBreakStmt emits break, or goto to a label for labeled breaks.
func (g *generator) emitBreakStmt(x *ast.BreakStmt) {
	if x.Label != "" {
		g.line(fmt.Sprintf("goto DotLabel_%s_break;", x.Label))
		return
	}
	g.line("break;")
}

// emitContinueStmt emits continue, or goto to a label for labeled continues.
func (g *generator) emitContinueStmt(x *ast.ContinueStmt) {
	if x.Label != "" {
		g.line(fmt.Sprintf("goto DotLabel_%s_continue;", x.Label))
		return
	}
	g.line("continue;")
}

// emitDeferStmt registers a deferred call on the current scope.
func (g *generator) emitDeferStmt(x *ast.DeferStmt) {
	call := g.emitExpr(x.Call)
	g.defers = append(g.defers, emitDeferRegister(g, call))
}

// emitDefers emits every registered defer and clears the list.
func (g *generator) emitDefers() {
	for i := len(g.defers) - 1; i >= 0; i-- {
		g.line(fmt.Sprintf("%s;", g.defers[i]))
	}
	g.defers = nil
}

// emitPerfBlock emits an @perf block: arena alloc, body, arena free.
func (g *generator) emitPerfBlock(x *ast.PerfBlock) {
	g.line(fmt.Sprintf("{ %s;", emitArenaEnter(g)))
	g.indent++
	g.emitScopedBlockStmts(x.Block)
	g.indent--
	g.line(fmt.Sprintf("%s; }", emitArenaExit(g)))
}

// emitBlockStmt emits a brace-delimited block with scope cleanup at the end.
func (g *generator) emitBlockStmt(x *ast.BlockStmt) {
	g.line("{")
	g.indent++
	g.pushScope()
	for _, s := range x.Stmts {
		g.emitStmt(s)
	}
	if !scopeBlockEndsInReturn(x.Stmts) {
		emitScopeCleanupStmts(g, g.popScope())
	} else {
		g.popScope()
	}
	g.indent--
	g.line("}")
}

// emitBlockStmts emits the statements of a block without wrapping braces.
func (g *generator) emitBlockStmts(b *ast.BlockStmt) {
	for _, s := range b.Stmts {
		g.emitStmt(s)
	}
}

// emitScopedBlockStmts emits the statements of a block inside an existing
// brace pair (if/else branches, match arms, loop bodies) as a fresh scope
// level: locals declared in the branch are released before the closing brace
// of that branch, not at the enclosing function scope. The cleanup is skipped
// when the branch ends in a return (the return already cleaned the scopes).
func (g *generator) emitScopedBlockStmts(b *ast.BlockStmt) {
	g.pushScope()
	for _, s := range b.Stmts {
		g.emitStmt(s)
	}
	if !scopeBlockEndsInReturn(b.Stmts) {
		emitScopeCleanupStmts(g, g.popScope())
	} else {
		g.popScope()
	}
}

// varName returns the C name for a variable expression. Underscore expressions
// become a compiler-generated discard name.
func varName(g *generator, e ast.Expr) string {
	switch n := e.(type) {
	case *ast.Ident:
		if sym, ok := g.info.Uses[n]; ok {
			return sym.Name
		}
		return n.Name
	case *ast.UnderscoreExpr:
		name := fmt.Sprintf("_dot_unused_%d", g.unusedIdx)
		g.unusedIdx++
		return name
	}
	return "?"
}
