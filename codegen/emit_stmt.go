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
// retain for heap-allocated initialisers.
func (g *generator) emitVarDecl(x *ast.VarDecl) {
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
		ct := cType(g, declType)
		isEnum := false
		if _, ok := declType.(*types.Enum); ok {
			isEnum = true
			ct += "*"
		}
		if val == "" {
			g.line(fmt.Sprintf("%s %s;", ct, vn))
			continue
		}
		if i < len(x.Values) {
			if esc := g.info.Escapes[x.Values[i]]; esc == types.Heap {
				g.line(fmt.Sprintf("%s %s = dot_alloc(sizeof(%s));", ct, vn, cType(g, declType)))
				g.line(fmt.Sprintf("*%s = %s;", vn, val))
				continue
			}
		}
		if isEnum {
			g.line(fmt.Sprintf("%s %s = %s;", ct, vn, val))
			g.scopeVars = append(g.scopeVars, scopeVar{name: vn, typ: declType})
		} else if types.IsHeap(declType) {
			g.line(fmt.Sprintf("%s %s = %s;", ct, vn, emitRetain(g, val, declType)))
			g.scopeVars = append(g.scopeVars, scopeVar{name: vn, typ: declType})
		} else {
			g.line(fmt.Sprintf("%s %s = %s;", ct, vn, val))
		}
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
							val = fmt.Sprintf("dot_box_struct(sizeof(%s), &(%s))", cType(g, valType), val)
						}
						g.line(fmt.Sprintf("dot_slice_set(%s, %s, (DotAny)(intptr_t)(%s));",
							g.emitExpr(idxExpr.X), g.emitExpr(idxExpr.Indices[0]), val))
						continue
					}
				}
			}

			if isEnum := isEnumType(tgtType); isEnum {
				g.line(fmt.Sprintf("memcpy(%s, %s, sizeof(%s));", tname, val, cType(g, tgtType)))
				continue
			}

			if types.IsHeap(tgtType) {
				g.line(fmt.Sprintf("dot_release((DotRefcnt*)(%s));", tname))
				g.line(fmt.Sprintf("%s = %s;", tname, emitRetain(g, val, valType)))
			} else {
				g.line(fmt.Sprintf("%s = %s;", tname, val))
			}
		}
	}
}

// emitIfStmt emits an if-else chain from an IfExpr used in statement position.
func (g *generator) emitIfStmt(x *ast.IfExpr) {
	cond := g.emitExpr(x.Cond)
	g.line(fmt.Sprintf("if (%s) {", cond))
	g.indent++
	g.emitBlockStmts(x.Then)
	g.indent--
	g.emitIfTail(x)
}

// emitIfTail emits the closing brace and any else / else-if tail.
func (g *generator) emitIfTail(x *ast.IfExpr) {
	if x.ElseIf != nil {
		g.line(fmt.Sprintf("} else if (%s) {", g.emitExpr(x.ElseIf.Cond)))
		g.indent++
		g.emitBlockStmts(x.ElseIf.Then)
		g.indent--
		g.emitIfTail(x.ElseIf)
		return
	}
	if x.Else != nil {
		g.line("} else {")
		g.indent++
		g.emitBlockStmts(x.Else)
		g.indent--
	}
	g.line("}")
}

// emitMatchStmt emits a match expression in statement position. Enum matches
// become a switch on the tag; literal matches become an if/else chain.
func (g *generator) emitMatchStmt(x *ast.MatchExpr) {
	subj := g.emitExpr(x.Subject)
	subjType := g.info.TypeOf(x.Subject)
	if subjType != nil && subjType.Kind() == types.KindEnum {
		g.line(fmt.Sprintf("switch ((%s)->tag) {", subj))
		for i, arm := range x.Arms {
			g.line(fmt.Sprintf("case %d:", i))
			g.indent++
			g.emitMatchArmBody(arm)
			g.indent--
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
		g.emitMatchArmBody(arm)
		g.indent--
	}
	if len(x.Arms) > 0 {
		g.line("}")
	}
}

// emitMatchArmBody emits the body of a match arm in statement position.
func (g *generator) emitMatchArmBody(arm *ast.MatchArm) {
	if be, ok := arm.Body.(*ast.BlockExpr); ok {
		g.emitBlockStmts(be.Block)
		return
	}
	g.line(fmt.Sprintf("%s;", g.emitExpr(arm.Body)))
}
