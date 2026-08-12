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
	resType := e.exprType(ex)
	if resType == "void" {
		resType = "i64"
	}
	var resPtr string
	if hasElse {
		resPtr = "%" + e.nextLabel("if.res")
		e.emit("  %s = alloca %s", resPtr, resType)
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
		e.emit("  store %s %s, ptr %s", resType, thenVal, resPtr)
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
			e.emit("  store %s %s, ptr %s", resType, elseVal, resPtr)
		}
		e.emit("  br label %%%s", endBlock)
	}

	e.emit("\n%s:", endBlock)
	if hasElse {
		tmp := e.nextTmp()
		e.emit("  %s = load %s, ptr %s", tmp, resType, resPtr)
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

	loopType := "i64"
	if rangeExpr.Low != nil {
		loopType = e.exprType(rangeExpr.Low)
	} else if rangeExpr.High != nil {
		loopType = e.exprType(rangeExpr.High)
	}
	e.varTypes[loopVar] = loopType

	e.emit("  %%%s = alloca %s", loopVar, loopType)

	loVal := "0"
	if rangeExpr.Low != nil {
		loVal = e.emitExpr(rangeExpr.Low)
	}
	e.emit("  store %s %s, ptr %%%s", loopType, loVal, loopVar)

	condBlock := e.nextLabel("for.cond")
	bodyBlock := e.nextLabel("for.body")
	stepBlock := e.nextLabel("for.step")
	endBlock := e.nextLabel("for.end")

	e.emit("  br label %%%s", condBlock)

	e.emit("\n%s:", condBlock)
	curVal := e.nextTmp()
	e.emit("  %s = load %s, ptr %%%s", curVal, loopType, loopVar)

	hiVal := "0"
	if rangeExpr.High != nil {
		hiVal = e.emitExpr(rangeExpr.High)
	}

	cmpOp := "slt"
	if rangeExpr.Inclusive {
		cmpOp = "sle"
	}
	condVal := e.nextTmp()
	e.emit("  %s = icmp %s %s %s, %s", condVal, cmpOp, loopType, curVal, hiVal)
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
	e.emit("  %s = load %s, ptr %%%s", stepVal, loopType, loopVar)
	nextVal := e.nextTmp()
	e.emit("  %s = add %s %s, 1", nextVal, loopType, stepVal)
	e.emit("  store %s %s, ptr %%%s", loopType, nextVal, loopVar)
	e.emit("  br label %%%s", condBlock)

	e.emit("\n%s:", endBlock)
}
