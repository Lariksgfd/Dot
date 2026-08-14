// Package codegen translates a typed Dot AST into C source code.
//
// This file dispatches expression codegen by AST node type. It covers every
// expression form described in PLAN.md section 3: literals, unary, binary,
// call, field, index, slice, cast, try, await, spawn, range, if-expr, match,
// block, fn-lit and tuple expressions.
package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
	"github.com/dotlang/dot/types"
)

// emitExpr returns the C expression string for e, dispatching on the AST
// node kind. It uses g.info to look up types and resolve names.
func (g *generator) emitExpr(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.IntLit:
		return cLiteral(g, x, g.info.TypeOf(x))
	case *ast.FloatLit:
		return cLiteral(g, x, g.info.TypeOf(x))
	case *ast.BoolLit:
		return cLiteral(g, x, g.info.TypeOf(x))
	case *ast.StringLit:
		return cLiteral(g, x, g.info.TypeOf(x))
	case *ast.RawStringLit:
		return cLiteral(g, x, g.info.TypeOf(x))
	case *ast.NilLit:
		return emitNil(g, x)
	case *ast.Ident:
		return emitIdent(g, x)
	case *ast.SelfExpr:
		return "self"
	case *ast.UnderscoreExpr:
		return "_"
	case *ast.ParenExpr:
		return "(" + g.emitExpr(x.X) + ")"
	case *ast.UnaryExpr:
		return emitUnary(g, x)
	case *ast.BinaryExpr:
		return emitBinary(g, x)
	case *ast.CallExpr:
		return emitCall(g, x)
	case *ast.IndexExpr:
		return emitIndex(g, x)
	case *ast.SliceExpr:
		return emitSlice(g, x)
	case *ast.FieldExpr:
		return emitField(g, x)
	case *ast.CastExpr:
		return emitCast(g, x)
	case *ast.IsExpr:
		return emitIsExpr(g, x)
	case *ast.TryExpr:
		return emitTry(g, x)
	case *ast.AwaitExpr:
		return emitAwait(g, x)
	case *ast.SpawnExpr:
		return emitSpawn(g, x)
	case *ast.RangeExpr:
		return emitRange(g, x)
	case *ast.IfExpr:
		return emitIfExpr(g, x)
	case *ast.MatchExpr:
		return emitMatch(g, x)
	case *ast.BlockExpr:
		return emitBlockExpr(g, x)
	case *ast.FnLit:
		return emitFnLit(g, x)
	case *ast.TupleLit:
		return emitTupleLit(g, x)
	case *ast.ArrayLit:
		return emitArrayLit(g, x)
	case *ast.MapLit:
		return emitMapLit(g, x)
	case *ast.StructLit:
		return emitStructLit(g, x)
	case *ast.AssignExpr:
		return emitAssignExpr(g, x)
	case *ast.PipeExpr:
		return emitPipeExpr(g, x)
	case *ast.BadExpr:
		return "0 /* bad expr */"
	}
	return fmt.Sprintf("/* TODO: %T */", e)
}

// emitIdent resolves an identifier to its C name using info.Uses.
func emitIdent(g *generator, x *ast.Ident) string {
	if sym, ok := g.info.Uses[x]; ok {
		if sym.Kind == types.SymFunc {
			return "Dot_" + sym.Name
		}
		if sym.Kind == types.SymVariant && sym.Variant != nil {
			typ := g.info.TypeOf(x)
			cname := cType(g, typ)
			return fmt.Sprintf("((%s*)dot_box_struct(sizeof(%s), (&((%s){ .tag = %d }))))", cname, cname, cname, sym.Variant.Tag)
		}
		return sym.Name
	}
	return x.Name
}

// emitNil emits a nil literal: DotNone for Option types, NULL otherwise.
func emitNil(g *generator, x *ast.NilLit) string {
	t := g.info.TypeOf(x)
	if isOptionType(t) {
		return "DotNone"
	}
	return "NULL"
}

// emitBinary emits C code for a binary expression, mapping Dot operators to
// their C equivalents. The ** power operator becomes a runtime call.
func emitBinary(g *generator, x *ast.BinaryExpr) string {
	lhs := g.emitExpr(x.X)
	rhs := g.emitExpr(x.Y)
	
	if x.Op == lexer.TokenPlus {
		if x.X != nil {
			t := g.info.TypeOf(x.X)
			if t != nil && t.Kind() == types.KindString {
				return fmt.Sprintf("dot_string_concat(%s, %s)", lhs, rhs)
			}
		}
	}

	switch x.Op {
	case lexer.TokenPlus:
		return fmt.Sprintf("(%s + %s)", lhs, rhs)
	case lexer.TokenMinus:
		return fmt.Sprintf("(%s - %s)", lhs, rhs)
	case lexer.TokenStar:
		return fmt.Sprintf("(%s * %s)", lhs, rhs)
	case lexer.TokenSlash:
		return fmt.Sprintf("(%s / %s)", lhs, rhs)
	case lexer.TokenPercent:
		return fmt.Sprintf("(%s %% %s)", lhs, rhs)
	case lexer.TokenStarStar:
		return emitPow(g, lhs, rhs, x)
	case lexer.TokenEqEq:
		if isStringExpr(g, x.X) {
			return fmt.Sprintf("dot_string_eq(%s, %s)", lhs, rhs)
		}
		return fmt.Sprintf("(%s == %s)", lhs, rhs)
	case lexer.TokenBangEq:
		if isStringExpr(g, x.X) {
			return fmt.Sprintf("!dot_string_eq(%s, %s)", lhs, rhs)
		}
		return fmt.Sprintf("(%s != %s)", lhs, rhs)
	case lexer.TokenLt:
		return fmt.Sprintf("(%s < %s)", lhs, rhs)
	case lexer.TokenGt:
		return fmt.Sprintf("(%s > %s)", lhs, rhs)
	case lexer.TokenLtEq:
		return fmt.Sprintf("(%s <= %s)", lhs, rhs)
	case lexer.TokenGtEq:
		return fmt.Sprintf("(%s >= %s)", lhs, rhs)
	case lexer.TokenAnd:
		return fmt.Sprintf("(%s && %s)", lhs, rhs)
	case lexer.TokenOr:
		return fmt.Sprintf("(%s || %s)", lhs, rhs)
	case lexer.TokenAmp:
		return fmt.Sprintf("(%s & %s)", lhs, rhs)
	case lexer.TokenPipe:
		return fmt.Sprintf("(%s | %s)", lhs, rhs)
	case lexer.TokenCaret:
		return fmt.Sprintf("(%s ^ %s)", lhs, rhs)
	case lexer.TokenShl:
		return fmt.Sprintf("(%s << %s)", lhs, rhs)
	case lexer.TokenShr:
		return fmt.Sprintf("(%s >> %s)", lhs, rhs)
	}
	return fmt.Sprintf("(%s %s %s) /* unknown binop */", lhs, x.Op.Literal(), rhs)
}

// isStringExpr reports whether e is a string-typed expression (including
// named string types), so binary operators can pick content semantics.
func isStringExpr(g *generator, e ast.Expr) bool {
	if e == nil {
		return false
	}
	t := g.info.TypeOf(e)
	if t == nil {
		return false
	}
	if t.Kind() == types.KindString {
		return true
	}
	return types.IsStringType(types.Underlying(t))
}

// emitPow selects the correct runtime power function based on the result type.
func emitPow(g *generator, lhs, rhs string, x *ast.BinaryExpr) string {
	t := g.info.TypeOf(x)
	if t != nil {
		switch t.Kind() {
		case types.KindFloat, types.KindFloat32, types.KindUntypedFloat:
			return fmt.Sprintf("dot_pow_float(%s, %s)", lhs, rhs)
		}
	}
	return fmt.Sprintf("dot_pow_int(%s, %s)", lhs, rhs)
}

// emitCall emits C code for a call expression. It handles builtin calls,
// generic instantiations, method calls and default parameters.
func emitCall(g *generator, x *ast.CallExpr) string {
	if info, ok := g.info.Calls[x]; ok {
		if info.Builtin != types.BuiltinNone {
			return emitBuiltinCall(g, x, info)
		}
		if info.IsMethod {
			return emitMethodCall(g, x, info)
		}
		// Check if it's a builtin method (like push on slices)
		if fn, ok := x.Fn.(*ast.FieldExpr); ok {
			if sel, ok := g.info.Selections[fn]; ok && sel.Kind == types.SelectBuiltinMethod {
				return emitMethodCall(g, x, info)
			}
		}
		if inst, ok := g.info.Instances[x]; ok && inst.Mangled != "" {
			return emitMangledCall(g, inst.Mangled, x, info)
		}

		// Check if it's a direct call to a top-level function
		isDirect := false
		var directName string
		if ident, ok := x.Fn.(*ast.Ident); ok {
			if sym, ok := g.info.Uses[ident]; ok && sym.Kind == types.SymFunc {
				isDirect = true
				directName = "Dot_" + sym.Name
			}
		}

		if isDirect {
			return emitMangledCall(g, directName, x, info)
		}

		// If it's an indirect call (via closure variable or expression), we must call via the struct.
		fnType := g.info.TypeOf(x.Fn)
		if _, isFnType := fnType.(*types.Fn); isFnType {
			fnExpr := g.emitExpr(x.Fn)
			args := emitCallArgs(g, x, info)
			if len(args) == 0 {
				return fmt.Sprintf("(%s).fn((%s).env)", fnExpr, fnExpr)
			}
			return fmt.Sprintf("(%s).fn((%s).env, %s)", fnExpr, fnExpr, strings.Join(args, ", "))
		}

		return emitMangledCall(g, callFuncName(g, x), x, info)
	}
	return fmt.Sprintf("/* TODO: call %T */", x)
}

func isStructOrEnum(t types.Type) bool {
	switch x := t.(type) {
	case *types.Struct, *types.Enum: return true
	case *types.Named: return isStructOrEnum(x.Underlying)
	}
	return false
}

func isEnumType(t types.Type) bool {
	switch x := t.(type) {
	case *types.Enum: return true
	case *types.Named: return isEnumType(x.Underlying)
	}
	return false
}

// emitMethodCall emits a method call with the receiver as the first argument.
func emitMethodCall(g *generator, call *ast.CallExpr, info *types.CallInfo) string {
	sel := g.info.Selections[call.Fn.(*ast.FieldExpr)]
	recv := g.emitExpr(call.Fn.(*ast.FieldExpr).X)
	name := methodName(sel)
	// Handle builtin methods (push, pop, etc.)
	if sel.Kind == types.SelectBuiltinMethod {
		name = builtinMethodName(sel, call.Fn.(*ast.FieldExpr).Name)
		var args []string
		args = append(args, recv)
		// Cast arguments to DotAny for builtin methods
		callArgs := emitCallArgs(g, call, info)
		for i, arg := range callArgs {
			var t types.Type
			if i < len(call.Args) {
				t = g.info.TypeOf(call.Args[i].Value)
			}
			if isStructOrEnum(t) {
				// We don't retain before boxing because dot_box_struct doesn't retain internally?
				// Wait! Is boxing copying the structure? Yes!
				// But we just emitDeepRetain it before boxing.
				arg = fmt.Sprintf("dot_box_struct(sizeof(%s), &(%s))", cType(g, t), emitDeepRetain(g, arg, t))
			} else {
				arg = emitRetain(g, arg, t)
			}
			args = append(args, fmt.Sprintf("(DotAny)(intptr_t)(%s)", arg))
		}
		return fmt.Sprintf("%s(%s)", name, strings.Join(args, ", "))
	}
	var args []string
	if sel.Method != nil && !sel.Method.Static {
		recvType := sel.Recv
		if isPointerLike(recvType) {
			if !isPointerType(g.info.TypeOf(call.Fn.(*ast.FieldExpr).X)) {
				recv = "&" + recv
			}
		}
		args = append(args, recv)
	}
	args = append(args, emitCallArgs(g, call, info)...)
	return fmt.Sprintf("%s(%s)", name, strings.Join(args, ", "))
}

// emitMangledCall emits a regular function call using its C name.
func emitMangledCall(g *generator, name string, call *ast.CallExpr, info *types.CallInfo) string {
	args := emitCallArgs(g, call, info)
	return fmt.Sprintf("%s(%s)", name, strings.Join(args, ", "))
}

// emitCallArgs materialises the argument list for a call, filling defaults
// and packing variadics as directed by CallInfo.
func emitCallArgs(g *generator, call *ast.CallExpr, info *types.CallInfo) []string {
	var args []string
	for pi := 0; pi < len(info.Sig.Params); pi++ {
		if pi < len(info.ArgOrder) {
			ai := info.ArgOrder[pi]
			if ai >= 0 && ai < len(call.Args) {
				t := g.info.TypeOf(call.Args[ai].Value)
				argExpr := g.emitExpr(call.Args[ai].Value)
				args = append(args, emitDeepRetain(g, argExpr, t))
				continue
			}
		}
		if pi < len(info.Sig.Params) && info.Sig.Params[pi].HasDflt {
			t := info.Sig.Params[pi].Type
			args = append(args, emitDeepRetain(g, g.emitExpr(info.Sig.Params[pi].Default), t))
		} else if pi < len(info.Sig.Params) {
			args = append(args, zeroValue(info.Sig.Params[pi].Type))
		}
	}
	return args
}

// callFuncName derives the C function name from the callee expression.
func callFuncName(g *generator, call *ast.CallExpr) string {
	switch fn := call.Fn.(type) {
	case *ast.Ident:
		if sym, ok := g.info.Uses[fn]; ok {
			if sym.Kind == types.SymFunc {
				return "Dot_" + sym.Name
			}
			return sym.Name
		}
		return fn.Name
	case *ast.FieldExpr:
		// Static method call on a type: TypeName.method(...)
		if ident, ok := fn.X.(*ast.Ident); ok {
			if sym, ok := g.info.Uses[ident]; ok {
				if sym.Kind == types.SymType {
					return "Dot_" + sym.Name + "_" + fn.Name
				}
			}
		}
		// Also check Selections for method calls
		if sel, ok := g.info.Selections[fn]; ok {
			if sel.Method != nil && sel.Owner != nil {
				return "Dot_" + sel.Owner.Name + "_" + sel.Method.Name
			}
			// Handle builtin methods (push, pop, etc.)
			if sel.Kind == types.SelectBuiltinMethod {
				recv := sel.Recv
				if _, ok := recv.(*types.Slice); ok {
					return "dot_slice_" + fn.Name
				}
				if _, ok := recv.(*types.Map); ok {
					return "dot_map_" + fn.Name
				}
				if st, ok := recv.(*types.Basic); ok && st.Kind() == types.KindString {
					return "dot_string_" + fn.Name
				}
			}
		}
		return fn.Name
	}
	return "dot_unknown"
}

// builtinMethodName returns the C name for a builtin method on a type.
func builtinMethodName(sel *types.Selection, methodName string) string {
	recv := sel.Recv
	if _, ok := recv.(*types.Slice); ok {
		return "dot_slice_" + methodName
	}
	if _, ok := recv.(*types.Map); ok {
		return "dot_map_" + methodName
	}
	if st, ok := recv.(*types.Basic); ok && st.Kind() == types.KindString {
		return "dot_string_" + methodName
	}
	return "dot_unknown"
}

// methodName builds the C name for a method from its Selection.
func methodName(sel *types.Selection) string {
	if sel.Method != nil && sel.Owner != nil {
		return "Dot_" + sel.Owner.Name + "_" + sel.Method.Name
	}
	return "dot_method_unknown"
}

// emitField emits a field access: struct field, tuple index, enum variant
// constructor, or builtin property.
func emitField(g *generator, x *ast.FieldExpr) string {
	if x.IsTupleIndex {
		obj := g.emitExpr(x.X)
		return fmt.Sprintf("%s._%d", obj, x.Index)
	}
	if sel, ok := g.info.Selections[x]; ok {
		switch sel.Kind {
		case types.SelectField:
			return emitStructField(g, x, sel)
		case types.SelectMethod:
			return methodName(sel)
		case types.SelectBuiltinProp:
			return emitBuiltinProp(g, x, sel)
		case types.SelectBuiltinMethod:
			return methodName(sel)
		case types.SelectVariant:
			return emitVariantCtor(g, x, sel)
		case types.SelectTupleIndex:
			return fmt.Sprintf("%s._%d", g.emitExpr(x.X), sel.Field.Index)
		}
	}
	obj := g.emitExpr(x.X)
	if isPointerType(g.info.TypeOf(x.X)) {
		return fmt.Sprintf("%s->%s", obj, x.Name)
	}
	return fmt.Sprintf("%s.%s", obj, x.Name)
}

// emitStructField emits a struct field access, auto-dereferencing pointers.
func emitStructField(g *generator, x *ast.FieldExpr, sel *types.Selection) string {
	obj := g.emitExpr(x.X)
	name := cFieldName(x.Name)
	if isPointerType(g.info.TypeOf(x.X)) {
		return fmt.Sprintf("%s->%s", obj, name)
	}
	// Special case: self in a method is always a pointer
	if _, ok := x.X.(*ast.SelfExpr); ok {
		return fmt.Sprintf("%s->%s", obj, name)
	}
	return fmt.Sprintf("%s.%s", obj, name)
}

// emitBuiltinProp emits a builtin property access such as items.len.
func emitBuiltinProp(g *generator, x *ast.FieldExpr, sel *types.Selection) string {
	obj := g.emitExpr(x.X)
	// Handle string properties
	if st, ok := sel.Recv.(*types.Basic); ok {
		fmt.Printf("DEBUG emitBuiltinProp: name=%s, kind=%v\n", x.Name, st.Kind())
		if st.Kind() == types.KindString {
			switch x.Name {
			case "len":
				return fmt.Sprintf("((DotString*)%s)->len", obj)
			}
		}
	}
	switch x.Name {
	case "len":
		return fmt.Sprintf("dot_slice_len(%s)", obj)
	case "cap":
		return fmt.Sprintf("dot_slice_cap(%s)", obj)
	}
	return fmt.Sprintf("dot_builtin_%s(%s)", x.Name, obj)
}

// emitVariantCtor emits an enum variant constructor as a heap-allocated pointer.
func emitVariantCtor(g *generator, x *ast.FieldExpr, sel *types.Selection) string {
	if sel.Variant != nil {
		typ := g.info.TypeOf(x)
		cname := cType(g, typ)
		return fmt.Sprintf("((%s*)dot_box_struct(sizeof(%s), (&((%s){ .tag = %d }))))", cname, cname, cname, sel.Variant.Tag)
	}
	return fmt.Sprintf("/* variant %s */", x.Name)
}

// emitIfExpr emits an if-else expression using the ternary operator when both
// branches are simple, or a GCC statement expression for complex branches.
func emitIfExpr(g *generator, x *ast.IfExpr) string {
	cond := g.emitExpr(x.Cond)
	then := g.emitExpr(&ast.BlockExpr{Block: x.Then})
	if x.ElseIf != nil {
		return fmt.Sprintf("({ if (%s) %s; else %s; })",
			cond, then, g.emitExpr(x.ElseIf))
	}
	if x.Else != nil {
		els := g.emitExpr(&ast.BlockExpr{Block: x.Else})
		return fmt.Sprintf("({ if (%s) %s; else %s; })", cond, then, els)
	}
	return fmt.Sprintf("({ if (%s) %s; })", cond, then)
}

// emitMatch emits a match expression as a GCC statement expression containing
// a switch (for enums) or an if/else chain (for literals).
func emitMatch(g *generator, x *ast.MatchExpr) string {
	subj := g.emitExpr(x.Subject)
	subjType := g.info.TypeOf(x.Subject)
	if subjType != nil && subjType.Kind() == types.KindEnum {
		return emitEnumMatch(g, subj, subjType, x)
	}
	return emitLiteralMatch(g, subj, x)
}

// emitEnumMatch emits a switch on the enum tag for each arm.
func emitEnumMatch(g *generator, subj string, t types.Type, x *ast.MatchExpr) string {
	var b strings.Builder
	b.WriteString("({ __typeof__(")
	b.WriteString(subj)
	b.WriteString(") _match_res; switch (")
	b.WriteString(subj)
	b.WriteString("->tag) { ")
	for i, arm := range x.Arms {
		b.WriteString("case ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(": _match_res = ")
		b.WriteString(g.emitExpr(arm.Body))
		b.WriteString("; break; ")
	}
	b.WriteString("} _match_res; })")
	return b.String()
}

// emitLiteralMatch emits an if/else chain comparing the subject to each arm.
func emitLiteralMatch(g *generator, subj string, x *ast.MatchExpr) string {
	var b strings.Builder
	b.WriteString("({ __typeof__(")
	b.WriteString(subj)
	b.WriteString(") _match_res; ")
	for i, arm := range x.Arms {
		if i > 0 {
			b.WriteString(" else ")
		}
		pat := patternCond(g, subj, arm.Pattern)
		b.WriteString("if (")
		b.WriteString(pat)
		b.WriteString(") _match_res = ")
		b.WriteString(g.emitExpr(arm.Body))
		b.WriteString(";")
	}
	b.WriteString(" _match_res; })")
	return b.String()
}

// patternCond emits a C condition matching the given pattern against subj.
func patternCond(g *generator, subj string, pat ast.Pattern) string {
	switch p := pat.(type) {
	case *ast.IdentPattern:
		return "true"
	case *ast.LiteralPattern:
		val := g.emitExpr(p.Value)
		if isStringExpr(g, p.Value) {
			return fmt.Sprintf("dot_string_eq(%s, %s)", subj, val)
		}
		return fmt.Sprintf("(%s == %s)", subj, val)
	case *ast.TuplePattern:
		return "true"
	}
	return "true"
}

// ensure emit_expr_extra.go helpers are referenced.
var _ = strconv.Itoa





