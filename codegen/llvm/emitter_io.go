package llvm

import (
	"fmt"

	"github.com/dotlang/dot/ast"
)

// formatString registers a printf/puts format or constant string global and
// returns its @.str.N name.
func (e *emitter) formatString(content string) string {
	if name, ok := e.stringLits[content]; ok {
		return name
	}
	name := fmt.Sprintf("@.str.%d", len(e.stringLits))
	e.stringLits[content] = name
	return name
}

// emitPrint emits a call for the builtin print. Strings go through puts,
// scalars through printf with a format picked by the argument type.
func (e *emitter) emitPrint(ex *ast.CallExpr) string {
	arg := ex.Args[0].Value
	t := e.exprType(arg)

	if t == "{ ptr, i32 }" {
		v := e.emitExpr(arg)
		charPtr := e.nextTmp()
		e.emit("  %s = extractvalue { ptr, i32 } %s, 0", charPtr, v)
		callTmp := e.nextTmp()
		e.emit("  %s = call i32 @puts(ptr %s)", callTmp, charPtr)
		tmp := e.nextTmp()
		e.emit("  %s = zext i32 %s to i64", tmp, callTmp)
		return tmp
	}

	val := e.emitExpr(arg)
	passed := val
	passedType := t
	fmtContent := "%lld\n"

	switch t {
	case "double":
		fmtContent = "%g\n"
	case "float":
		fmtContent = "%g\n"
		passedType = "double"
		ext := e.nextTmp()
		e.emit("  %s = fpext float %s to double", ext, val)
		passed = ext
	case "i1":
		fmtContent = "%d\n"
		passedType = "i32"
		ext := e.nextTmp()
		e.emit("  %s = zext i1 %s to i32", ext, val)
		passed = ext
	default:
		if t != "i64" {
			passedType = "i64"
			ext := e.nextTmp()
			e.emit("  %s = zext %s %s to i64", ext, t, val)
			passed = ext
		}
	}

	fmtName := e.formatString(fmtContent)
	callTmp := e.nextTmp()
	e.emit("  %s = call i32 (ptr, ...) @printf(ptr %s, %s %s)", callTmp, fmtName, passedType, passed)
	tmp := e.nextTmp()
	e.emit("  %s = zext i32 %s to i64", tmp, callTmp)
	return tmp
}
