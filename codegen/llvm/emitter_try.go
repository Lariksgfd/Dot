package llvm

import "github.com/dotlang/dot/ast"

func (e *emitter) emitTryExpr(ex *ast.TryExpr) string {
	// A basic implementation of TryExpr (?) that evaluates the expression,
	// checks the tag (0 for Ok/Some, 1 for Err/None), and early returns if not 0.
	recv := e.emitExpr(ex.X)
	
	enumName := e.enumNameOfExpr(ex.X)
	if enumName == "" {
		return "0"
	}
	
	tagVal := e.emitEnumTag(enumName, recv)
	
	okBlock := e.nextLabel("try.ok")
	errBlock := e.nextLabel("try.err")
	
	cond := e.nextTmp()
	e.emit("  %s = icmp eq i32 %s, 0", cond, tagVal) // Ok or Some is usually tag 0
	e.emit("  br i1 %s, label %%%s, label %%%s", cond, okBlock, errBlock)
	
	e.emit("\n%s:", errBlock)
	// Return the error/none value. For now, since they're pointers to enum,
	// if the return type matches, we can just return it.
	// A proper implementation needs to match the function's return type.
	e.emit("  ret ptr %s", recv)
	
	e.emit("\n%s:", okBlock)
	// Return the unwrapped payload
	ft, idx := e.optionPayloadInfo(enumName)
	if ft == "" { // maybe it's Result?
		ft, idx = e.resultPayloadInfo(enumName)
	}
	
	if ft != "" {
		payloadPtr := e.nextTmp()
		e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", payloadPtr, enumName, recv, idx)
		payloadVal := e.nextTmp()
		e.emit("  %s = load %s, ptr %s", payloadVal, ft, payloadPtr)
		return payloadVal
	}
	
	return "0"
}

func (e *emitter) resultPayloadInfo(enumName string) (string, int) {
	if vmap, ok := e.enumLayouts[enumName]; ok {
		if l, ok := vmap["Ok"]; ok && len(l.FieldTypes) > 0 {
			return l.FieldTypes[0], l.FieldIdx[0]
		}
	}
	return "", 0
}
