// Package codegen translates a typed Dot AST into C source code.
//
// This file implements statement codegen: var/const declarations, assignments,
// if/match/spawn in statement position, for loops, return, break/continue,
// defer, perf blocks and nested block statements with scope cleanup.
package codegen

import (
	"fmt"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
	"github.com/dotlang/dot/types"
)

// emitStmt dispatches statement codegen by AST node kind, writing C directly
// to g.buf. Statements produce no value; they emit side effects only.
func (g *generator) emitStmt(s ast.Stmt) {
	switch x := s.(type) {
	case *ast.VarDecl:
		g.emitVarDecl(x)
	case *ast.ExprStmt:
		g.emitExprStmt(x)
	case *ast.ReturnStmt:
		g.emitReturnStmt(x)
	case *ast.ForStmt:
		g.emitForStmt(x)
	case *ast.BreakStmt:
		g.emitBreakStmt(x)
	case *ast.ContinueStmt:
		g.emitContinueStmt(x)
	case *ast.DeferStmt:
		g.emitDeferStmt(x)
	case *ast.PerfBlock:
		g.emitPerfBlock(x)
	case *ast.BlockStmt:
		g.emitBlockStmt(x)
	case *ast.BadStmt:
		g.line("/* bad stmt */")
	}
}

// emitVarDecl emits a variable declaration. Const declarations become a
// #define macro; regular declarations emit a C variable with an optional
// retain for heap-allocated initialisers. Names the checker treated as
// reassignments (D53) become plain assignments instead of declarations.
func (g *generator) emitVarDecl(x *ast.VarDecl) {
	if len(x.Names) > 1 && len(x.Values) == 1 {
		g.emitTupleDecl(x)
		return
	}
	// Pure multi-name reassignment (`a, b = b, a`): evaluate every
	// right-hand side into a temporary first so the swap is correct (D11).
	if len(x.Names) > 1 && len(x.Names) == len(x.Values) && !x.Const && g.allReassigned(x) {
		g.emitMultiAssign(x)
		return
	}
	for i, name := range x.Names {
		vn := varName(g, name)
		var val string
		if i < len(x.Values) {
			val = g.emitExpr(x.Values[i])
		}
		declType := g.info.TypeOf(name)
		if x.Const {
			g.line(fmt.Sprintf("#define %s %s", vn, val))
			continue
		}
		// A name the checker treated as a reassignment (D53) has no Defs
		// entry: emit a plain assignment, not a C declaration.
		if id, isIdent := name.(*ast.Ident); isIdent && g.info.Defs[id] == nil && val != "" {
			var doRetain bool
			if i < len(x.Values) {
				doRetain = isLValue(x.Values[i])
			}

			if types.IsHeap(declType) {
				// Evaluate the new value exactly once into a temporary: the
				// old code re-emitted the RHS inside the comparison, inside
				// the release and again in the assignment, so a value built
				// from a call was freed (and recomputed) between its uses.
				tmp := fmt.Sprintf("_dot_new_%d", g.unusedIdx)
				g.unusedIdx++
				g.line(fmt.Sprintf("%s %s = %s;", cType(g, declType), tmp, val))
				g.line(fmt.Sprintf("if ((void*)(%s) != (void*)(%s)) {", vn, tmp))
				g.indent++
				g.line(fmt.Sprintf("if (%s != NULL) { dot_release((DotRefcnt*)(%s)); }", vn, vn))
				if doRetain {
					g.line(fmt.Sprintf("%s = %s;", vn, emitRetain(g, tmp, declType)))
				} else {
					g.line(fmt.Sprintf("%s = %s;", vn, tmp))
				}
				g.indent--
				g.line("}")
			} else {
				if doRetain {
					g.line(fmt.Sprintf("%s = %s;", vn, emitDeepRetain(g, val, declType)))
				} else {
					g.line(fmt.Sprintf("%s = %s;", vn, val))
				}
			}
			continue
		}
		ct := cType(g, declType)
		isEnum := false
		if isEnumType(declType) {
			isEnum = true
			ct += "*"
		}
		if val == "" {
			g.line(fmt.Sprintf("%s %s;", ct, vn))
			g.scopeVars = append(g.scopeVars, scopeVar{name: vn, typ: declType})
			continue
		}
		if i < len(x.Values) {
			if esc := g.info.Escapes[x.Values[i]]; esc == types.Heap {
				g.line(fmt.Sprintf("%s %s = dot_alloc(sizeof(%s));", ct, vn, cType(g, declType)))
				g.line(fmt.Sprintf("*%s = %s;", vn, val))
				g.scopeVars = append(g.scopeVars, scopeVar{name: vn, typ: declType})
				continue
			}
		}
		var doRetain bool
		if i < len(x.Values) {
			doRetain = isLValue(x.Values[i])
		}

		if isEnum {
			if doRetain {
				g.line(fmt.Sprintf("%s %s = %s;", ct, vn, emitDeepRetain(g, val, declType)))
			} else {
				g.line(fmt.Sprintf("%s %s = %s;", ct, vn, val))
			}
			g.scopeVars = append(g.scopeVars, scopeVar{name: vn, typ: declType})
		} else if types.IsHeap(declType) {
			if doRetain {
				g.line(fmt.Sprintf("%s %s = %s;", ct, vn, emitRetain(g, val, declType)))
			} else {
				g.line(fmt.Sprintf("%s %s = %s;", ct, vn, val))
			}
			g.scopeVars = append(g.scopeVars, scopeVar{name: vn, typ: declType})
		} else {
			if doRetain {
				g.line(fmt.Sprintf("%s %s = %s;", ct, vn, emitDeepRetain(g, val, declType)))
			} else {
				g.line(fmt.Sprintf("%s %s = %s;", ct, vn, val))
			}
			g.scopeVars = append(g.scopeVars, scopeVar{name: vn, typ: declType})
		}
	}
}

// allReassigned reports whether every name of a declaration was treated by
// the checker as a reassignment of an existing binding (no Defs entry).
func (g *generator) allReassigned(x *ast.VarDecl) bool {
	for _, name := range x.Names {
		id, ok := name.(*ast.Ident)
		if !ok || g.info.Defs[id] != nil {
			return false
		}
	}
	return true
}

// emitMultiAssign emits a multi-name reassignment: all right-hand sides are
// evaluated into temporaries first, then assigned left to right, so that
// `a, b = b, a` swaps (SPEC §3, D11).
func (g *generator) emitMultiAssign(x *ast.VarDecl) {
	temps := make([]string, 0, len(x.Values))
	for _, v := range x.Values {
		tmp := fmt.Sprintf("_dot_ma_%d", g.unusedIdx)
		g.unusedIdx++
		g.line(fmt.Sprintf("%s %s = %s;", cType(g, g.info.TypeOf(v)), tmp, g.emitExpr(v)))
		temps = append(temps, tmp)
	}
	for i, name := range x.Names {
		vn := varName(g, name)
		declType := g.info.TypeOf(name)
		if types.IsHeap(declType) {
			// Transfer the temporary's reference to the target.
			g.line(fmt.Sprintf("if ((void*)(%s) != (void*)(%s)) {", vn, temps[i]))
			g.indent++
			g.line(fmt.Sprintf("if (%s != NULL) { dot_release((DotRefcnt*)(%s)); }", vn, vn))
			g.line(fmt.Sprintf("%s = %s;", vn, temps[i]))
			g.indent--
			g.line("}")
		} else {
			g.line(fmt.Sprintf("%s = %s;", vn, emitDeepRetain(g, temps[i], declType)))
		}
	}
}

// emitTupleDecl emits a multi-name declaration initialised by one multi-value
// call: a temporary tuple struct holds the result and each name is bound to
// the matching field.
func (g *generator) emitTupleDecl(x *ast.VarDecl) {
	val := g.emitExpr(x.Values[0])
	t := g.info.TypeOf(x.Values[0])
	tup, ok := t.(*types.Tuple)
	if !ok {
		g.line(fmt.Sprintf("/* multi-name decl of %s */ %s;", cType(g, t), val))
		return
	}
	tupName := cTupleName(g, tup)
	tmp := fmt.Sprintf("_dot_tup_%d", g.unusedIdx)
	g.unusedIdx++
	g.line(fmt.Sprintf("%s %s = %s;", tupName, tmp, val))
	for i, name := range x.Names {
		if _, ok := name.(*ast.UnderscoreExpr); ok {
			continue
		}
		vn := varName(g, name)
		declType := g.info.TypeOf(name)
		// Reassigned name (D53): plain assignment from the tuple field.
		if id, isIdent := name.(*ast.Ident); isIdent && g.info.Defs[id] == nil {
			if types.IsHeap(declType) {
				newTmp := fmt.Sprintf("_dot_new_%d", g.unusedIdx)
				g.unusedIdx++
				g.line(fmt.Sprintf("%s %s = %s._%d;", cType(g, declType), newTmp, tmp, i))
				g.line(fmt.Sprintf("if ((void*)(%s) != (void*)(%s)) {", vn, newTmp))
				g.indent++
				g.line(fmt.Sprintf("if (%s != NULL) { dot_release((DotRefcnt*)(%s)); }", vn, vn))
				g.line(fmt.Sprintf("%s = %s;", vn, emitRetain(g, newTmp, declType)))
				g.indent--
				g.line("}")
			} else {
				g.line(fmt.Sprintf("%s = %s;", vn, emitDeepRetain(g, fmt.Sprintf("%s._%d", tmp, i), declType)))
			}
			continue
		}
		ct := cType(g, declType)
		g.line(fmt.Sprintf("%s %s = %s;", ct, vn, emitDeepRetain(g, fmt.Sprintf("%s._%d", tmp, i), declType)))
		g.scopeVars = append(g.scopeVars, scopeVar{name: vn, typ: declType})
	}
}

// emitExprStmt emits an expression in statement position. If/match/spawn are
// dispatched to their statement forms; assignments emit release/retain for
// heap targets; other expressions emit a trailing semicolon.
func (g *generator) emitExprStmt(x *ast.ExprStmt) {
	switch e := x.X.(type) {
	case *ast.IfExpr:
		g.emitIfStmt(e)
	case *ast.MatchExpr:
		g.emitMatchStmt(e)
	case *ast.SpawnExpr:
		g.line(fmt.Sprintf("%s;", g.emitExpr(e)))
	case *ast.AssignExpr:
		g.emitAssignStmt(e)
	default:
		g.line(fmt.Sprintf("%s;", g.emitExpr(x.X)))
	}
}

// emitAssignStmt emits an assignment expression in statement position. Heap
// targets are released before assignment and the new value is retained.
func (g *generator) emitAssignStmt(x *ast.AssignExpr) {
	for i, tgt := range x.Targets {
		if i >= len(x.Values) {
			break
		}
		tname := g.emitExpr(tgt)
		tgtType := g.info.TypeOf(tgt)
		val := g.emitExpr(x.Values[i])
		valType := g.info.TypeOf(x.Values[i])
		switch x.Op {
		case lexer.TokenPlusAssign, lexer.TokenMinusAssign,
			lexer.TokenStarAssign, lexer.TokenSlashAssign:
			g.line(fmt.Sprintf("%s %s %s;", tname, x.Op.Literal(), val))
		default:
			if idxExpr, ok := tgt.(*ast.IndexExpr); ok {
				if t := g.info.TypeOf(idxExpr.X); t != nil {
					if _, isSlice := t.(*types.Slice); isSlice {
						if isStructOrEnum(valType) {
							val = emitBoxValue(g, val, valType)
						}
						g.line(fmt.Sprintf("dot_slice_set(%s, %s, (DotAny)(intptr_t)(%s));",
							g.emitExpr(idxExpr.X), g.emitExpr(idxExpr.Indices[0]), val))
						continue
					}
				}
			}

			doRetain := isLValue(x.Values[i])
			// Enums are always pointers in C (locals, params and fields), so
			// an enum assignment is a plain pointer assignment: heap enums
			// take the release/retain path below, unit enums the simple one.
			// (The old memcpy variant treated the target as a value struct,
			// which produced invalid initializers and corrupted payloads.)

			if types.IsHeap(tgtType) {
				// Evaluate the RHS once into a temporary so the comparison
				// and the assignment see the same value.
				tmp := fmt.Sprintf("_dot_new_%d", g.unusedIdx)
				g.unusedIdx++
				g.line(fmt.Sprintf("%s %s = %s;", cType(g, tgtType), tmp, val))
				g.line(fmt.Sprintf("if ((void*)(%s) != (void*)(%s)) {", tname, tmp))
				g.indent++
				g.line(fmt.Sprintf("if (%s != NULL) { dot_release((DotRefcnt*)(%s)); }", tname, tname))
				if doRetain {
					g.line(fmt.Sprintf("%s = %s;", tname, emitRetain(g, tmp, valType)))
				} else {
					g.line(fmt.Sprintf("%s = %s;", tname, tmp))
				}
				g.indent--
				g.line("}")
			} else {
				if doRetain {
					g.line(fmt.Sprintf("%s = %s;", tname, emitDeepRetain(g, val, valType)))
				} else {
					g.line(fmt.Sprintf("%s = %s;", tname, val))
				}
			}
		}
	}
}

// emitIfStmt emits an if-else chain from an IfExpr used in statement position.
func (g *generator) emitIfStmt(x *ast.IfExpr) {
	cond, bindings := g.emitIfCond(x)
	g.line(fmt.Sprintf("if (%s) {", cond))
	g.indent++
	if bindings != "" {
		g.line(bindings)
	}
	g.emitScopedBlockStmts(x.Then)
	g.indent--
	g.emitIfTail(x)
}

// emitIfTail emits the closing brace and any else / else-if tail.
func (g *generator) emitIfTail(x *ast.IfExpr) {
	if x.ElseIf != nil {
		cond, bindings := g.emitIfCond(x.ElseIf)
		g.line(fmt.Sprintf("} else if (%s) {", cond))
		g.indent++
		if bindings != "" {
			g.line(bindings)
		}
		g.emitScopedBlockStmts(x.ElseIf.Then)
		g.indent--
		g.emitIfTail(x.ElseIf)
		return
	}
	if x.Else != nil {
		g.line("} else {")
		g.indent++
		g.emitScopedBlockStmts(x.Else)
		g.indent--
	}
	g.line("}")
}

// emitIfCond returns the C condition and the pattern-binding declarations
// for an if header. Plain conditions emit the boolean expression; the if-let
// form (`if p = opt {`) emits a success-tag check plus payload bindings.
func (g *generator) emitIfCond(x *ast.IfExpr) (cond string, bindings string) {
	if x.Bind == nil {
		return g.emitExpr(x.Cond), ""
	}
	subj := g.emitExpr(x.Cond)
	return emitIfLet(g, x.Bind, subj, g.info.TypeOf(x.Cond))
}

// emitMatchStmt emits a match expression in statement position. Enum matches
// become a switch on the tag; literal matches become an if/else chain.
func (g *generator) emitMatchStmt(x *ast.MatchExpr) {
	subj := g.emitExpr(x.Subject)
	subjType := g.info.TypeOf(x.Subject)
	if subjType != nil && isEnumType(subjType) {
		g.line(fmt.Sprintf("switch ((%s)->tag) {", subj))
		for i, arm := range x.Arms {
			// Each case gets its own braces so arm-local declarations (pattern
			// bindings and body locals) do not leak into sibling cases.
			g.line(fmt.Sprintf("case %d: {", i))
			g.indent++
			if bindings := patternBindings(g, subj, arm.Pattern, subjType); bindings != "" {
				g.line(bindings)
			}
			g.emitMatchArmBody(arm)
			g.indent--
			g.line("}")
		}
		g.line("}")
		return
	}
	// Literal match: if/else chain.
	for i, arm := range x.Arms {
		pcond := patternCond(g, subj, arm.Pattern)
		if i == 0 {
			g.line(fmt.Sprintf("if (%s) {", pcond))
		} else {
			g.line(fmt.Sprintf("} else if (%s) {", pcond))
		}
		g.indent++
		if bindings := patternBindings(g, subj, arm.Pattern, subjType); bindings != "" {
			g.line(bindings)
		}
		g.emitMatchArmBody(arm)
		g.indent--
	}
	if len(x.Arms) > 0 {
		g.line("}")
	}
}

// emitMatchArmBody emits the body of a match arm in statement position. Arm
// bodies are their own scope level: locals declared in an arm are released
// before the arm's closing brace.
func (g *generator) emitMatchArmBody(arm *ast.MatchArm) {
	if be, ok := arm.Body.(*ast.BlockExpr); ok {
		g.emitScopedBlockStmts(be.Block)
		return
	}
	g.line(fmt.Sprintf("%s;", g.emitExpr(arm.Body)))
}
