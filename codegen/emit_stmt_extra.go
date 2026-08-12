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
		g.emitBlockStmts(x.Body)
		g.indent--
		g.line("}")
	case ast.ForCond:
		cond := g.emitExpr(x.Cond)
		g.line(fmt.Sprintf("while (%s) {", cond))
		g.indent++
		g.emitBlockStmts(x.Body)
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
	g.emitBlockStmts(x.Body)
	g.indent--
	g.line("}")
}

// emitForIterator emits `for i, item in items` as an iterator-based loop.
func (g *generator) emitForIterator(x *ast.ForStmt) {
	iterable := g.emitExpr(x.Iterable)
	vn := varName(g, x.Value)
	kn := "_"
	if x.Key != nil {
		kn = varName(g, x.Key)
	}
	keyExpr := "NULL"
	if x.Key != nil {
		keyExpr = "&" + kn
	}
	itIdx := g.unusedIdx
	g.unusedIdx++
	g.line(fmt.Sprintf("for (DotIter _dot_it_%d = dot_iter(%s); dot_iter_next(&_dot_it_%d, &%s, %s); ) {",
		itIdx, iterable, itIdx, vn, keyExpr))
	g.indent++
	g.emitBlockStmts(x.Body)
	g.indent--
	g.line("}")
}

// emitReturnStmt emits a return statement. Defers run before the return;
// the value is retained if it is a heap type.
func (g *generator) emitReturnStmt(x *ast.ReturnStmt) {
	g.emitDefers()
	if len(x.Values) == 0 {
		g.line("return;")
		return
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
			val := g.emitExpr(x.Values[0])
			t := g.info.TypeOf(x.Values[0])
			if isEnumType(t) {
				g.line(fmt.Sprintf("return %s;", val))
				return
			}
			g.line(fmt.Sprintf("return %s;", emitRetain(g, val, t)))
			return
		}
	}
	// Multi-value return: pack into a tuple struct.
	elems := make([]types.Type, len(values))
	fields := make([]string, len(values))
	for i, v := range values {
		vt := g.info.TypeOf(v)
		elems[i] = vt
		fields[i] = fmt.Sprintf(". _%d = %s", i, emitRetain(g, g.emitExpr(v), vt))
	}
	tupName := cTupleName(g, &types.Tuple{Elems: elems})
	g.line(fmt.Sprintf("return (%s){%s };", tupName, strings.Join(fields, ", ")))
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
	g.emitBlockStmts(x.Block)
	g.indent--
	g.line(fmt.Sprintf("%s; }", emitArenaExit(g)))
}

// emitBlockStmt emits a brace-delimited block with scope cleanup at the end.
func (g *generator) emitBlockStmt(x *ast.BlockStmt) {
	g.line("{")
	g.indent++
	saved := g.scopeVars
	g.scopeVars = nil
	for _, s := range x.Stmts {
		g.emitStmt(s)
	}
	if len(g.scopeVars) > 0 {
		cleanup := emitScopeCleanup(g, g.scopeVars)
		for _, cl := range strings.Split(strings.TrimRight(cleanup, "\n"), "\n") {
			if cl != "" {
				g.line(cl)
			}
		}
	}
	g.scopeVars = saved
	g.indent--
	g.line("}")
}

// emitBlockStmts emits the statements of a block without wrapping braces.
func (g *generator) emitBlockStmts(b *ast.BlockStmt) {
	for _, s := range b.Stmts {
		g.emitStmt(s)
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
