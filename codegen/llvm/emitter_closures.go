package llvm

import (
	"github.com/dotlang/dot/ast"
)

func (e *emitter) emitFnLit(ex *ast.FnLit) string {
	// closures are generated as function pointers or {env, func} depending on captures.
	// For now, return a null pointer or unimplemented
	return "null"
}
