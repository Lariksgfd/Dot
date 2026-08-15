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
			return emitUnitVariantAlloc(g, cname, sym.Variant.Tag)
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
		// A field or identifier callee may denote a static method
		// (Type.method, IsMethod is false because the signature has no self
		// receiver), a builtin method (items.push) or an enum payload
		// constructor (Some(x), Shape.Circle(r)). All three have a *types.Fn
		// type but must NOT take the indirect-closure path below: static and
		// builtin methods are direct calls, constructors build a boxed union.
		if fn, ok := x.Fn.(*ast.FieldExpr); ok {
			if sel, ok := g.info.Selections[fn]; ok {
				switch sel.Kind {
				case types.SelectMethod:
					return emitMethodCall(g, x, info)
				case types.SelectBuiltinMethod:
					return emitMethodCall(g, x, info)
				case types.SelectVariant:
					return emitVariantCall(g, x, info, sel.Variant)
				}
			}
		}
		// Unqualified payload constructor: Some(x), Ok(v), Err(e).
		if ident, ok := x.Fn.(*ast.Ident); ok {
			if sym, ok := g.info.Uses[ident]; ok && sym.Kind == types.SymVariant && sym.Variant != nil {
				return emitVariantCall(g, x, info, sym.Variant)
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
			// The callee expression is materialised into a C temporary once
			// (CG-18): fn and env must come from the same evaluation of the
			// callee, and the arguments are evaluated afterwards, at the
			// call site. A callee that is not an lvalue transfers its env
			// ownership into the temporary, so the env is released after
			// the call; capture-returning bodies deep-retain their result,
			// which keeps the result alive independently of the env.
			fnExpr := g.emitExpr(x.Fn)
			args := emitCallArgs(g, x, info)
			tmp := fmt.Sprintf("_dot_fn_%d", g.unusedIdx)
			g.unusedIdx++
			callExpr := fmt.Sprintf("%s.fn(%s.env", tmp, tmp)
			if len(args) > 0 {
				callExpr += ", " + strings.Join(args, ", ")
			}
			callExpr += ")"
			if !isLValue(x.Fn) {
				resType := g.info.TypeOf(x)
				if resType != nil && resType != types.Invalid && resType.Kind() != types.KindVoid {
					resTmp := fmt.Sprintf("_dot_call_%d", g.unusedIdx)
					g.unusedIdx++
					return fmt.Sprintf("({ %s %s = %s; %s %s = %s; if (%s.env != NULL) { dot_release((DotRefcnt*)(%s.env)); } %s; })",
						cType(g, fnType), tmp, fnExpr, cFieldType(g, resType), resTmp, callExpr, tmp, tmp, resTmp)
				}
				return fmt.Sprintf("({ %s %s = %s; %s; if (%s.env != NULL) { dot_release((DotRefcnt*)(%s.env)); } })",
					cType(g, fnType), tmp, fnExpr, callExpr, tmp, tmp)
			}
			return fmt.Sprintf("({ %s %s = %s; %s; })", cType(g, fnType), tmp, fnExpr, callExpr)
		}

		return emitMangledCall(g, callFuncName(g, x), x, info)
	}
	return fmt.Sprintf("/* TODO: call %T */", x)
}

// optionResultBuiltin returns inlined C for an Option/Result builtin method
// call (is_some/is_none/is_ok/is_err/unwrap/unwrap_err), or "" when the
// receiver is neither an Option nor a Result.
func optionResultBuiltin(g *generator, recvExpr string, recvType types.Type, name string) string {
	isOpt := isOptionType(recvType)
	isRes := isResultType(recvType)
	if !isOpt && !isRes {
		return ""
	}
	switch name {
	case "is_some", "is_ok":
		return fmt.Sprintf("((%s) != NULL && (%s)->tag == 0)", recvExpr, recvExpr)
	case "is_none", "is_err":
		return fmt.Sprintf("((%s) == NULL || (%s)->tag == 1)", recvExpr, recvExpr)
	case "unwrap":
		if isOpt {
			return fmt.Sprintf("(%s)->%s", recvExpr, variantFieldName("Some", "value"))
		}
		return fmt.Sprintf("(%s)->%s", recvExpr, variantFieldName("Ok", "value"))
	case "unwrap_err":
		if isRes {
			return fmt.Sprintf("(%s)->%s", recvExpr, variantFieldName("Err", "error"))
		}
	}
	return ""
}

func isStructOrEnum(t types.Type) bool {
	switch x := t.(type) {
	case *types.Struct, *types.Enum: return true
	case *types.Named: return isStructOrEnum(x.Underlying)
	case *types.TypeVar:
		if x.Bound != nil {
			return isStructOrEnum(x.Bound)
		}
	}
	return false
}

// isBoxedElem reports whether values of t are stored in DotAny slots (slice
// elements, iterator results) inside a dot_box_struct box: structs and enums
// (existing behaviour) plus closure values, whose two-field struct does not
// fit a DotAny slot and is therefore stored as a box pointer (CG-22).
func isBoxedElem(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, isFn := t.(*types.Fn); isFn {
		return true
	}
	return isStructOrEnum(t)
}

func isEnumType(t types.Type) bool {
	switch x := t.(type) {
	case *types.Enum: return true
	case *types.Named: return isEnumType(x.Underlying)
	case *types.TypeVar:
		if x.Bound != nil {
			return isEnumType(x.Bound)
		}
	}
	return false
}

// emitMethodCall emits a method call with the receiver as the first argument.
func emitMethodCall(g *generator, call *ast.CallExpr, info *types.CallInfo) string {
	sel := g.info.Selections[call.Fn.(*ast.FieldExpr)]
	recv := g.emitExpr(call.Fn.(*ast.FieldExpr).X)
	name := methodName(sel)
	// Option/Result methods are inlined: the runtime has no helpers for them
	// and the tagged-union layout is known here.
	if inline := optionResultBuiltin(g, recv, sel.Recv, call.Fn.(*ast.FieldExpr).Name); inline != "" {
		return inline
	}
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
			if isBoxedElem(t) {
				// Box the value: structs are copied into the box, enums are
				// copied from the object their pointer designates, closures
				// are stored boxed with a deep-retained env. emitBoxValue
				// deep-retains the boxed copy itself.
				arg = emitBoxValue(g, arg, t)
			} else {
				arg = emitRetain(g, arg, t)
			}
			args = append(args, fmt.Sprintf("(DotAny)(intptr_t)(%s)", arg))
		}
		return fmt.Sprintf("%s(%s)", name, strings.Join(args, ", "))
	}
	var args []string
	if sel.Method != nil && !sel.Method.Static {
		recvExpr := call.Fn.(*ast.FieldExpr).X
		recvType := sel.Recv
		if isPointerLike(recvType) {
			if !isPointerType(g.info.TypeOf(recvExpr)) && !isSelfParam(g, recvExpr) {
				recv = "&" + recv
			}
		}
		args = append(args, recv)
	}
	args = append(args, emitCallArgs(g, call, info)...)
	return fmt.Sprintf("%s(%s)", name, strings.Join(args, ", "))
}

// emitUnitVariantAlloc emits a statement expression constructing a fresh
// unit enum variant: a dot_alloc'd heap object with its ARC header
// initialised (refcount 1, no dtor) and its tag set to the variant's real
// discriminant. dot_alloc zeroes the memory and initialises the DotRefcnt
// header, so the result is safe to dot_release.
func emitUnitVariantAlloc(g *generator, cname string, tag int) string {
	tmp := fmt.Sprintf("_dot_v_%d", g.unusedIdx)
	g.unusedIdx++
	return fmt.Sprintf("({ %s* %s = (%s*)dot_alloc(sizeof(%s)); %s->tag = %d; %s; })",
		cname, tmp, cname, cname, tmp, tag, tmp)
}

// emitVariantCall emits an enum payload constructor call such as Some(x) or
// Shape.Circle(r): a heap-allocated tagged union whose tag is the variant's
// and whose union members receive the payload arguments in field order.
func emitVariantCall(g *generator, call *ast.CallExpr, info *types.CallInfo, variant *types.Variant) string {
	enumType := info.Result
	if enumType == nil || enumType == types.Invalid {
		enumType = g.info.TypeOf(call)
	}
	cname := cType(g, enumType)
	args := emitCallArgs(g, call, info)
	tmp := fmt.Sprintf("_dot_v_%d", g.unusedIdx)
	g.unusedIdx++
	var b strings.Builder
	b.WriteString(fmt.Sprintf("({ %s* %s = (%s*)dot_alloc(sizeof(%s)); ", cname, tmp, cname, cname))
	b.WriteString(fmt.Sprintf("%s->tag = %d; ", tmp, variant.Tag))
	for i, f := range variant.Fields {
		var arg string
		if i < len(args) {
			arg = args[i]
		} else {
			arg = zeroValue(f.Type)
		}
		b.WriteString(fmt.Sprintf("%s->%s = %s; ", tmp, variantFieldName(variant.Name, f.Name), arg))
	}
	b.WriteString(fmt.Sprintf("%s; })", tmp))
	return b.String()
}

// isSelfParam reports whether e refers to the method receiver `self`. In C
// `self` is always a pointer (methods are declared with `DotX* self`), even
// though its Dot type is the plain struct, so a receiver address must not be
// taken again.
func isSelfParam(g *generator, e ast.Expr) bool {
	switch id := e.(type) {
	case *ast.SelfExpr:
		return true
	case *ast.Ident:
		sym, ok := g.info.Uses[id]
		return ok && sym != nil && sym.Kind == types.SymParam && sym.Name == "self"
	}
	return false
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
		return emitUnitVariantAlloc(g, cname, sel.Variant.Tag)
	}
	return fmt.Sprintf("/* variant %s */", x.Name)
}

// emitIfExpr emits an if-else expression. When the result type is a value,
// a GCC statement expression binds the chosen branch into a result temporary
// (`({ T _tmp; if (c) { _tmp = <then>; } else { _tmp = <else>; } _tmp; })`):
// a bare `({ if (c) ...; else ...; })` has no value and GCC rejects it as a
// void expression. Void-typed if-expressions keep the value-less form.
func emitIfExpr(g *generator, x *ast.IfExpr) string {
	resType := g.info.TypeOf(x)
	if resType == nil || resType == types.Invalid || resType.Kind() == types.KindVoid {
		return emitIfExprVoid(g, x)
	}
	tmp := fmt.Sprintf("_dot_if_%d", g.unusedIdx)
	g.unusedIdx++
	var b strings.Builder
	b.WriteString(fmt.Sprintf("({ %s %s; ", cFieldType(g, resType), tmp))
	var writeIf func(cond string, ifx *ast.IfExpr)
	writeIf = func(cond string, ifx *ast.IfExpr) {
		b.WriteString(fmt.Sprintf("if (%s) { ", cond))
		writeIfBranch(&b, g, ifx.Then, tmp)
		b.WriteString(" }")
		if ifx.ElseIf != nil {
			b.WriteString(" else ")
			writeIf(g.emitExpr(ifx.ElseIf.Cond), ifx.ElseIf)
		} else if ifx.Else != nil {
			b.WriteString(" else { ")
			writeIfBranch(&b, g, ifx.Else, tmp)
			b.WriteString(" }")
		}
	}
	writeIf(g.emitExpr(x.Cond), x)
	b.WriteString(fmt.Sprintf(" %s; })", tmp))
	return b.String()
}

// writeIfBranch renders one if-expr branch inside a statement expression:
// every statement is emitted as a statement, and the branch's trailing
// expression is assigned to the result temporary.
func writeIfBranch(b *strings.Builder, g *generator, blk *ast.BlockStmt, tmp string) {
	for i, s := range blk.Stmts {
		last := i == len(blk.Stmts)-1
		switch st := s.(type) {
		case *ast.ExprStmt:
			if last {
				b.WriteString(fmt.Sprintf("%s = %s; ", tmp, g.emitExpr(st.X)))
			} else {
				b.WriteString(g.emitExpr(st.X))
				b.WriteString("; ")
			}
		case *ast.ReturnStmt:
			b.WriteString("return ")
			for i, v := range st.Values {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(g.emitExpr(v))
			}
			b.WriteString("; ")
		default:
			b.WriteString(fmt.Sprintf("/* stmt %T */ ", st))
		}
	}
}

// emitIfExprVoid emits a value-less if-else statement expression for
// void-typed if-expressions.
func emitIfExprVoid(g *generator, x *ast.IfExpr) string {
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
// a switch (for enums) or an if/else chain (for literals). The result
// temporary is typed from the match's result type, not the subject's: arms
// may produce a different type than the scrutinee (e.g. matching a string to
// produce a TokenType).
func emitMatch(g *generator, x *ast.MatchExpr) string {
	subj := g.emitExpr(x.Subject)
	resType := g.info.TypeOf(x)
	subjType := g.info.TypeOf(x.Subject)
	if subjType != nil && isEnumType(subjType) {
		return emitEnumMatch(g, subj, subjType, resType, x)
	}
	return emitLiteralMatch(g, subj, resType, x)
}

// matchResultType renders the C type of the temporary holding a match result.
// The match's own result type wins; the subject's type is the fallback for
// the rare case where the checker did not record a result.
func matchResultType(g *generator, subj string, resType types.Type) string {
	if resType == nil || resType == types.Invalid {
		return fmt.Sprintf("__typeof__(%s)", subj)
	}
	return cFieldType(g, resType)
}

// emitEnumMatch emits a switch on the enum tag for each arm. EnumPattern
// arms become case labels carrying the variant's REAL discriminant (not the
// arm's index); wildcard/binding arms become a default case; literal,
// guarded and other non-variant patterns are dispatched inside default via
// an if/else chain, never as case indices.
func emitEnumMatch(g *generator, subj string, subjType types.Type, resType types.Type, x *ast.MatchExpr) string {
	var b strings.Builder
	tmp := fmt.Sprintf("_dot_match_%d", g.unusedIdx)
	g.unusedIdx++
	b.WriteString("({ ")
	b.WriteString(cFieldType(g, subjType))
	b.WriteString(" ")
	b.WriteString(tmp)
	b.WriteString(" = ")
	b.WriteString(subj)
	b.WriteString("; ")
	b.WriteString(matchResultType(g, subj, resType))
	b.WriteString(" _match_res; switch (")
	b.WriteString(tmp)
	b.WriteString("->tag) { ")
	var fallback []*ast.MatchArm
	for _, arm := range x.Arms {
		if tag, ok := enumVariantTag(g, subjType, arm.Pattern); ok {
			b.WriteString("case ")
			b.WriteString(strconv.Itoa(tag))
			b.WriteString(": { ")
			b.WriteString(patternBindings(g, tmp, arm.Pattern, subjType))
			b.WriteString("_match_res = ")
			b.WriteString(g.emitExpr(arm.Body))
			b.WriteString("; break; } ")
		} else {
			fallback = append(fallback, arm)
		}
	}
	if len(fallback) > 0 {
		b.WriteString("default: { ")
		writeMatchFallbackExprs(&b, g, tmp, subjType, fallback)
		b.WriteString(" } ")
	}
	b.WriteString("} _match_res; })")
	return b.String()
}

// emitLiteralMatch emits an if/else chain comparing the subject to each arm.
func emitLiteralMatch(g *generator, subj string, resType types.Type, x *ast.MatchExpr) string {
	subjType := g.info.TypeOf(x.Subject)
	var b strings.Builder
	b.WriteString("({ ")
	b.WriteString(matchResultType(g, subj, resType))
	b.WriteString(" _match_res; ")
	writeMatchFallbackExprs(&b, g, subj, subjType, x.Arms)
	b.WriteString(" _match_res; })")
	return b.String()
}

// enumVariantTag returns the tag of the variant matched by an unguarded
// EnumPattern arm. ok is false for any other arm kind (wildcard, literal,
// guarded, ...), which must be dispatched through the default branch.
func enumVariantTag(g *generator, subjType types.Type, pat ast.Pattern) (int, bool) {
	ep, ok := pat.(*ast.EnumPattern)
	if !ok {
		return 0, false
	}
	en := enumUnderlying(subjType)
	if en == nil {
		return 0, false
	}
	v, ok := en.Variant(ep.Variant)
	if !ok {
		return 0, false
	}
	return v.Tag, true
}

// matchArmCond returns the C dispatch condition, the pattern-binding
// declarations and the guard expression ("" when absent) for one match arm
// pattern matched against subj of type subjType. Enum patterns compare the
// subject's tag to the variant's real discriminant; wildcard and binding
// patterns always match; or-patterns join their alternatives; everything
// else falls back to patternCond.
func matchArmCond(g *generator, subj string, subjType types.Type, pat ast.Pattern) (cond, bindings, guard string) {
	switch p := pat.(type) {
	case *ast.GuardedPattern:
		c, b, _ := matchArmCond(g, subj, subjType, p.Pattern)
		return c, b, g.emitExpr(p.Guard)
	case *ast.EnumPattern:
		if tag, ok := enumVariantTag(g, subjType, pat); ok {
			return fmt.Sprintf("((%s) != NULL && (%s)->tag == %d)", subj, subj, tag),
				patternBindings(g, subj, pat, subjType), ""
		}
		// Unknown variant: the arm can never match.
		return "false", "", ""
	case *ast.OrPattern:
		conds := make([]string, 0, len(p.Alts))
		for _, alt := range p.Alts {
			c, _, _ := matchArmCond(g, subj, subjType, alt)
			conds = append(conds, "("+c+")")
		}
		return strings.Join(conds, " || "), "", ""
	}
	return patternCond(g, subj, pat), patternBindings(g, subj, pat, subjType), ""
}

// writeMatchFallbackExprs renders an if/else chain of match arms in
// expression position, assigning the arm body to _match_res. Catch-all arms
// end the chain; guarded arms nest the rest of the chain in the guard's else
// branch so a failed guard tries the next arm.
func writeMatchFallbackExprs(b *strings.Builder, g *generator, subj string, subjType types.Type, arms []*ast.MatchArm) {
	var emitChain func(i int)
	emitChain = func(i int) {
		if i >= len(arms) {
			return
		}
		arm := arms[i]
		cond, bindings, guard := matchArmCond(g, subj, subjType, arm.Pattern)
		last := i == len(arms)-1
		if cond == "true" && guard == "" && last {
			b.WriteString(bindings)
			b.WriteString("_match_res = ")
			b.WriteString(g.emitExpr(arm.Body))
			b.WriteString("; ")
			return
		}
		b.WriteString("if (")
		b.WriteString(cond)
		b.WriteString(") { ")
		b.WriteString(bindings)
		if guard != "" {
			b.WriteString("if (")
			b.WriteString(guard)
			b.WriteString(") { ")
			b.WriteString("_match_res = ")
			b.WriteString(g.emitExpr(arm.Body))
			b.WriteString("; } else { ")
			emitChain(i + 1)
			b.WriteString(" }")
		} else {
			b.WriteString("_match_res = ")
			b.WriteString(g.emitExpr(arm.Body))
			b.WriteString("; ")
		}
		b.WriteString("}")
		if !last {
			b.WriteString(" else { ")
			emitChain(i + 1)
			b.WriteString("}")
		}
	}
	emitChain(0)
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
	case *ast.OrPattern:
		conds := make([]string, 0, len(p.Alts))
		for _, alt := range p.Alts {
			conds = append(conds, patternCond(g, subj, alt))
		}
		return "(" + strings.Join(conds, " || ") + ")"
	case *ast.TuplePattern:
		return "true"
	}
	return "true"
}

// patternBindings emits the C declarations for the variables bound by a
// match arm pattern, so the arm body can reference them. Enum patterns bind
// each sub-pattern to the matching variant field; a bare identifier binds
// the whole subject. Bindings borrow the subject's values (no retain).
func patternBindings(g *generator, subj string, pat ast.Pattern, subjType types.Type) string {
	var b strings.Builder
	writePatternBindings(&b, g, subj, pat, subjType)
	return b.String()
}

// writePatternBindings appends binding declarations to b.
func writePatternBindings(b *strings.Builder, g *generator, subj string, pat ast.Pattern, subjType types.Type) {
	switch p := pat.(type) {
	case *ast.IdentPattern:
		b.WriteString(fmt.Sprintf("%s %s = %s; ", cFieldType(g, subjType), p.Name, subj))
	case *ast.EnumPattern:
		en := enumUnderlying(subjType)
		var v *types.Variant
		if en != nil {
			v, _ = en.Variant(p.Variant)
		}
		if v == nil || len(p.Args) == 0 {
			return
		}
		for i, arg := range p.Args {
			if i >= len(v.Fields) {
				break
			}
			f := v.Fields[i]
			fExpr := fmt.Sprintf("(%s)->%s", subj, variantFieldName(v.Name, f.Name))
			writeArgBinding(b, g, arg, fExpr, f.Type)
		}
	}
}

// writeArgBinding emits the binding for one variant-field sub-pattern.
func writeArgBinding(b *strings.Builder, g *generator, arg ast.Pattern, fExpr string, fType types.Type) {
	switch a := arg.(type) {
	case *ast.IdentPattern:
		b.WriteString(fmt.Sprintf("%s %s = %s; ", cFieldType(g, fType), a.Name, fExpr))
	case *ast.WildcardPattern:
		// nothing to bind
	case *ast.EnumPattern:
		writePatternBindings(b, g, fExpr, a, fType)
	}
}

// enumUnderlying returns the *types.Enum behind t (through Named wrappers),
// or nil when t is not an enum.
func enumUnderlying(t types.Type) *types.Enum {
	switch x := t.(type) {
	case *types.Enum:
		return x
	case *types.Named:
		return enumUnderlying(x.Underlying)
	}
	return nil
}

// ensure emit_expr_extra.go helpers are referenced.
var _ = strconv.Itoa





