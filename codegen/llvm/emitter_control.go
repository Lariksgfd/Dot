package llvm

import (
	"github.com/dotlang/dot/ast"
)

func (e *emitter) emitIfExpr(ex *ast.IfExpr) string {
	condVal := e.emitExpr(ex.Cond)
	thenBlock := e.nextLabel("if.then")
	elseBlock := e.nextLabel("if.else")
	endBlock := e.nextLabel("if.end")

	hasElse := ex.Else != nil || ex.ElseIf != nil
	var resPtr string
	if hasElse {
		resPtr = "%" + e.nextLabel("if.res")
		e.emit("  %s = alloca i32", resPtr)
	}

	if hasElse {
		e.emit("  br i1 %s, label %%%s, label %%%s", condVal, thenBlock, elseBlock)
	} else {
		e.emit("  br i1 %s, label %%%s, label %%%s", condVal, thenBlock, endBlock)
	}

	e.emit("\n%s:", thenBlock)
	var thenVal string
	if ex.Then != nil {
		var lastVal string
		for _, stmt := range ex.Then.Stmts {
			if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
				lastVal = e.emitExpr(exprStmt.X)
			} else {
				e.emitStmt(stmt)
				lastVal = ""
			}
		}
		thenVal = lastVal
	}
	if hasElse && thenVal != "" {
		e.emit("  store i32 %s, ptr %s", thenVal, resPtr)
	}
	e.emit("  br label %%%s", endBlock)

	if hasElse {
		e.emit("\n%s:", elseBlock)
		var elseVal string
		if ex.Else != nil {
			var lastVal string
			for _, stmt := range ex.Else.Stmts {
				if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
					lastVal = e.emitExpr(exprStmt.X)
				} else {
					e.emitStmt(stmt)
					lastVal = ""
				}
			}
			elseVal = lastVal
		} else if ex.ElseIf != nil {
			elseVal = e.emitExpr(ex.ElseIf)
		}
		if elseVal != "" {
			e.emit("  store i32 %s, ptr %s", elseVal, resPtr)
		}
		e.emit("  br label %%%s", endBlock)
	}

	e.emit("\n%s:", endBlock)
	if hasElse {
		tmp := e.nextTmp()
		e.emit("  %s = load i32, ptr %s", tmp, resPtr)
		return tmp
	}
	return "0"
}

func (e *emitter) emitBlockExpr(ex *ast.BlockExpr) string {
	var lastVal string
	if ex.Block != nil {
		for _, stmt := range ex.Block.Stmts {
			if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
				lastVal = e.emitExpr(exprStmt.X)
			} else {
				e.emitStmt(stmt)
				lastVal = ""
			}
		}
	}
	if lastVal == "" {
		lastVal = "0"
	}
	return lastVal
}

func (e *emitter) emitForCond(s *ast.ForStmt) {
	condBlock := e.nextLabel("for.cond")
	bodyBlock := e.nextLabel("for.body")
	endBlock := e.nextLabel("for.end")

	e.emit("  br label %%%s", condBlock)

	e.emit("\n%s:", condBlock)
	if s.Cond != nil {
		condVal := e.emitExpr(s.Cond)
		e.emit("  br i1 %s, label %%%s, label %%%s", condVal, bodyBlock, endBlock)
	} else {
		e.emit("  br label %%%s", bodyBlock)
	}

	e.emit("\n%s:", bodyBlock)
	if s.Body != nil {
		for _, bs := range s.Body.Stmts {
			e.emitStmt(bs)
		}
	}
	e.emit("  br label %%%s", condBlock)

	e.emit("\n%s:", endBlock)
}

func (e *emitter) emitForIn(s *ast.ForStmt) {
	rangeExpr, ok := s.Iterable.(*ast.RangeExpr)
	if !ok {
		e.emitForCond(s)
		return
	}

	loopVar := ""
	if ident, ok := s.Value.(*ast.Ident); ok {
		loopVar = ident.Name
	} else {
		loopVar = e.nextLabel("for.it")
	}

	e.emit("  %%%s = alloca i32", loopVar)

	loVal := "0"
	if rangeExpr.Low != nil {
		loVal = e.emitExpr(rangeExpr.Low)
	}
	e.emit("  store i32 %s, ptr %%%s", loVal, loopVar)

	condBlock := e.nextLabel("for.cond")
	bodyBlock := e.nextLabel("for.body")
	stepBlock := e.nextLabel("for.step")
	endBlock := e.nextLabel("for.end")

	e.emit("  br label %%%s", condBlock)

	e.emit("\n%s:", condBlock)
	curVal := e.nextTmp()
	e.emit("  %s = load i32, ptr %%%s", curVal, loopVar)

	hiVal := "0"
	if rangeExpr.High != nil {
		hiVal = e.emitExpr(rangeExpr.High)
	}

	cmpOp := "slt"
	if rangeExpr.Inclusive {
		cmpOp = "sle"
	}
	condVal := e.nextTmp()
	e.emit("  %s = icmp %s i32 %s, %s", condVal, cmpOp, curVal, hiVal)
	e.emit("  br i1 %s, label %%%s, label %%%s", condVal, bodyBlock, endBlock)

	e.emit("\n%s:", bodyBlock)
	if s.Body != nil {
		for _, bs := range s.Body.Stmts {
			e.emitStmt(bs)
		}
	}
	e.emit("  br label %%%s", stepBlock)

	e.emit("\n%s:", stepBlock)
	stepVal := e.nextTmp()
	e.emit("  %s = load i32, ptr %%%s", stepVal, loopVar)
	nextVal := e.nextTmp()
	e.emit("  %s = add i32 %s, 1", nextVal, stepVal)
	e.emit("  store i32 %s, ptr %%%s", nextVal, loopVar)
	e.emit("  br label %%%s", condBlock)

	e.emit("\n%s:", endBlock)
}
