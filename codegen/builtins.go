// Package codegen translates a typed Dot AST into C source code.
//
// This file maps Dot builtin functions (print, len, assert, panic, ...) to
// their C runtime counterparts and reports which names are reserved builtins.
package codegen

import (
	"fmt"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/types"
)

// emitBuiltinCall emits C code for a builtin function call and returns the
// resulting C expression string. Variadic builtins pack trailing arguments
// into a DotSlice*; statement-shaped builtins (assert, panic) use a GCC
// statement expression so they compose in expression position.
func emitBuiltinCall(g *generator, call *ast.CallExpr, info *types.CallInfo) string {
	switch info.Builtin {
	case types.BuiltinPrint:
		return emitVariadicCall(g, "dot_print", call, info)
	case types.BuiltinPrintln:
		return emitVariadicCall(g, "dot_println", call, info)
	case types.BuiltinEprint:
		return emitVariadicCall(g, "dot_eprint", call, info)
	case types.BuiltinInput:
		arg := emitArg(g, call, info, 0)
		return fmt.Sprintf("dot_input(%s)", arg)
	case types.BuiltinLen:
		arg := emitArg(g, call, info, 0)
		return fmt.Sprintf("dot_len(%s)", arg)
	case types.BuiltinTypeOf:
		arg := emitArg(g, call, info, 0)
		return fmt.Sprintf("dot_typeof(%s)", arg)
	case types.BuiltinAssert:
		cond := emitArg(g, call, info, 0)
		msg := assertMsg(g, call, info)
		return fmt.Sprintf("({ if (!(%s)) { dot_assert_fail(%s); } })", cond, msg)
	case types.BuiltinPanic:
		msg := emitArg(g, call, info, 0)
		return fmt.Sprintf("({ dot_panic(%s); })", msg)
	}
	return ""
}

// emitArg returns the C expression for the parameter at index pi, applying
// ArgOrder and Defaults from the call info. Missing args fall back to the
// zero value of the parameter type.
func emitArg(g *generator, call *ast.CallExpr, info *types.CallInfo, pi int) string {
	if pi < len(info.ArgOrder) {
		ai := info.ArgOrder[pi]
		if ai >= 0 && ai < len(call.Args) {
			return g.emitExpr(call.Args[ai].Value)
		}
	}
	if pi < len(info.Sig.Params) {
		return zeroValue(info.Sig.Params[pi].Type)
	}
	return "0"
}

// emitVariadicCall emits a call to fn with the arguments packed into a
// DotSlice* when the callee is variadic, otherwise as a plain argument list.
func emitVariadicCall(g *generator, fn string, call *ast.CallExpr, info *types.CallInfo) string {
	var args []string
	if info.VariadicFrom >= 0 {
		for i := 0; i < info.VariadicFrom && i < len(info.ArgOrder); i++ {
			ai := info.ArgOrder[i]
			if ai >= 0 && ai < len(call.Args) {
				args = append(args, g.emitExpr(call.Args[ai].Value))
			}
		}
		var variadic []string
		for i := info.VariadicFrom; i < len(call.Args); i++ {
			val := g.emitExpr(call.Args[i].Value)
			t := g.info.TypeOf(call.Args[i].Value)
			if t != nil && !isPointerLike(t) {
				val = fmt.Sprintf("(DotAny)(intptr_t)(%s)", val)
			}
			variadic = append(variadic, val)
		}
		slice := fmt.Sprintf("dot_slice_pack(%d, (DotAny[]){%s})",
			len(variadic), strings.Join(variadic, ", "))
		args = append(args, slice)
	} else {
		for i := 0; i < len(info.ArgOrder); i++ {
			ai := info.ArgOrder[i]
			if ai >= 0 && ai < len(call.Args) {
				args = append(args, g.emitExpr(call.Args[ai].Value))
			}
		}
	}
	return fmt.Sprintf("%s(%s)", fn, strings.Join(args, ", "))
}

// assertMsg returns the C expression for the assert message argument, or a
// default string literal containing the source position when the caller
// omitted it.
func assertMsg(g *generator, call *ast.CallExpr, info *types.CallInfo) string {
	if len(info.ArgOrder) > 1 {
		ai := info.ArgOrder[1]
		if ai >= 0 && ai < len(call.Args) {
			return g.emitExpr(call.Args[ai].Value)
		}
	}
	pos := call.Pos()
	return fmt.Sprintf("\"%s:%d: assertion failed\"", pos.File, pos.Line)
}

// zeroValue returns a C zero-value literal for the given Dot type.
func zeroValue(t types.Type) string {
	switch t.Kind() {
	case types.KindBool:
		return "false"
	case types.KindFloat, types.KindFloat32:
		return "0.0"
	case types.KindString:
		return "NULL"
	}
	return "0"
}

// isBuiltin reports whether name is a reserved Dot builtin function.
func isBuiltin(name string) bool {
	switch name {
	case "print", "println", "eprint", "input", "len", "type_of", "assert", "panic":
		return true
	}
	return false
}

// builtinCName maps a BuiltinID to the C runtime function name that
// implements it. It returns "" for BuiltinNone.
func builtinCName(id types.BuiltinID) string {
	switch id {
	case types.BuiltinPrint:
		return "dot_print"
	case types.BuiltinPrintln:
		return "dot_println"
	case types.BuiltinEprint:
		return "dot_eprint"
	case types.BuiltinInput:
		return "dot_input"
	case types.BuiltinLen:
		return "dot_len"
	case types.BuiltinTypeOf:
		return "dot_typeof"
	case types.BuiltinAssert:
		return "dot_assert_fail"
	case types.BuiltinPanic:
		return "dot_panic"
	}
	return ""
}
