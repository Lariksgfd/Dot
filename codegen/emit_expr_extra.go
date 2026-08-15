// Package codegen translates a typed Dot AST into C source code.
//
// This file holds expression-codegen helpers that are too large or less
// central for emit_expr.go: unary, index, cast, try, block, fn-lit,
// tuple, array, map, struct-lit codegen.
package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
	"github.com/dotlang/dot/types"
)

// emitUnary emits C code for a unary expression.
func emitUnary(g *generator, x *ast.UnaryExpr) string {
	inner := g.emitExpr(x.X)
	switch x.Op {
	case lexer.TokenMinus:
		return fmt.Sprintf("(-%s)", inner)
	case lexer.TokenNot:
		return fmt.Sprintf("(!%s)", inner)
	case lexer.TokenTilde:
		return fmt.Sprintf("(~%s)", inner)
	case lexer.TokenPlus:
		return fmt.Sprintf("(%s)", inner)
	}
	return fmt.Sprintf("(%s%s) /* unknown unary */", x.Op.Literal(), inner)
}

// emitIndex emits C code for an index expression: slice get, array access,
// or generic instantiation lookup.
func emitIndex(g *generator, x *ast.IndexExpr) string {
	if inst, ok := g.info.Instances[x]; ok && inst.Mangled != "" {
		return inst.Mangled
	}
	obj := g.emitExpr(x.X)
	t := g.info.TypeOf(x.X)
	if t == nil {
		t = g.info.TypeOf(x)
	}
	switch tt := t.(type) {
	case *types.Slice:
		res := fmt.Sprintf("dot_slice_get(%s, %s)", obj, g.emitExpr(x.Indices[0]))
		elemType := tt.Elem
		if isBoxedElem(elemType) {
			// Boxed elements (structs, enums, closures) carry a DotRefcnt
			// header before the payload (dot_box_struct), so the payload is
			// reached by skipping sizeof(DotRefcnt); cFieldType renders the
			// storage shape (enums become pointers), so the deref yields the
			// right kind.
			res = fmt.Sprintf("(*(%s*)dot_box_payload(%s))", cFieldType(g, elemType), res)
		} else if !types.IsHeap(elemType) && !isPointerLike(elemType) {
			res = fmt.Sprintf("((%s)(intptr_t)%s)", cType(g, elemType), res)
		}
		return res
	case *types.Array:
		return fmt.Sprintf("%s.data[%s]", obj, g.emitExpr(x.Indices[0]))
	case *types.Map:
		return fmt.Sprintf("dot_map_get(%s, %s)", obj, g.emitExpr(x.Indices[0]))
	case *types.Basic:
		if tt.Kind() == types.KindString {
			return fmt.Sprintf("((DotString*)%s)->data[%s]", obj, g.emitExpr(x.Indices[0]))
		}
	}
	if len(x.Indices) == 1 {
		return fmt.Sprintf("%s[%s]", obj, g.emitExpr(x.Indices[0]))
	}
	return fmt.Sprintf("/* index %T */", x)
}

// emitCast emits a cast expression. Numeric casts use C casts; string and
// byte-slice conversions use runtime calls; trait-object upcasts build a
// fat pointer.
func emitCast(g *generator, x *ast.CastExpr) string {
	inner := g.emitExpr(x.X)
	from := g.info.TypeOf(x.X)
	to := g.info.TypeOf(x)
	if types.IsNumeric(from) && types.IsNumeric(to) {
		return fmt.Sprintf("((%s)%s)", cType(g, to), inner)
	}
	if isStringType(from) && isByteSliceType(to) {
		return fmt.Sprintf("dot_string_to_bytes(%s)", inner)
	}
	if isByteSliceType(from) && isStringType(to) {
		return fmt.Sprintf("dot_string_from_bytes(%s)", inner)
	}
	if isDynType(to) && !isDynType(from) {
		return emitDynCast(g, inner, from, to)
	}
	return fmt.Sprintf("((%s)%s)", cType(g, to), inner)
}

// emitDynCast builds a fat pointer for a trait-object upcast.
func emitDynCast(g *generator, inner string, from, to types.Type) string {
	traitName := ""
	if d, ok := to.(*types.Dyn); ok && d.Trait != nil {
		traitName = d.Trait.Name
	}
	vtable := "Dot" + traitName + "_vtable"
	return fmt.Sprintf("((DotDyn*){ .data = dot_retain(%s), .vtable = &%s })", inner, vtable)
}

// emitTry emits a try (?) expression. On Option it returns None on the
// None tag; on Result it returns Err on the Err tag.
func emitTry(g *generator, x *ast.TryExpr) string {
	inner := g.emitExpr(x.X)
	t := g.info.TypeOf(x.X)
	if isOptionType(t) {
		return fmt.Sprintf("({ typeof(%s) _t = %s; if (_t.tag == 1) return DotNone; _t.%s; })", inner, inner, variantFieldName("Some", "value"))
	}
	if isResultType(t) {
		return fmt.Sprintf("({ typeof(%s) _t = %s; if (_t.tag == 1) return dot_result_err(_t.%s); _t.%s; })", inner, inner, variantFieldName("Err", "error"), variantFieldName("Ok", "value"))
	}
	return inner
}

// emitBlockExpr emits a block expression as a GCC statement expression using
// the full statement machinery (declarations, control flow, scope cleanup),
// so a block-expr behaves like a function body. The trailing expression is
// the block's value (D18): it is materialised into a temporary before scope
// cleanup runs, following the return-statement ownership rules.
func emitBlockExpr(g *generator, x *ast.BlockExpr) string {
	return emitBlockExprAs(g, x.Block, g.info.TypeOf(x))
}

// emitBlockExprAs is emitBlockExpr for blocks that have no BlockExpr node of
// their own (closure bodies, if-else branches) and therefore no recorded
// type: want is the expected result type, and Void (or nil) makes the block
// a plain void statement expression with the trailing expression evaluated
// for its side effects only.
func emitBlockExprAs(g *generator, blk *ast.BlockStmt, want types.Type) string {
	var b strings.Builder
	oldBuf := g.buf
	oldIndent := g.indent
	oldDefers := g.defers
	g.buf = &b
	g.indent = 0
	g.defers = nil

	g.pushScope()
	stmts := blk.Stmts
	n := len(stmts)

	var tail *ast.ExprStmt
	tmp := ""
	moved := make(map[string]bool)
	if want != nil && want != types.Invalid && want.Kind() != types.KindVoid && n > 0 {
		if es, ok := stmts[n-1].(*ast.ExprStmt); ok {
			vt := g.info.TypeOf(es.X)
			if vt != nil && vt != types.Invalid && vt.Kind() != types.KindVoid {
				tail = es
				tmp = fmt.Sprintf("_dot_block_%d", g.unusedIdx)
				g.unusedIdx++
				// A trailing identifier is moved out of the block: its
				// ownership transfers to the result, so scope cleanup
				// must skip it. Capture idents are NOT moved (the env
				// keeps its own reference) and are deep-retained below.
				if id, ok := es.X.(*ast.Ident); ok && !g.fnCaptures[varName(g, id)] {
					moved[varName(g, id)] = true
				}
			}
		}
	}

	for i, s := range stmts {
		if i == n-1 && tail != nil {
			vt := g.info.TypeOf(tail.X)
			val := g.emitExpr(tail.X)
			if id, ok := tail.X.(*ast.Ident); ok && g.fnCaptures[varName(g, id)] {
				val = emitDeepRetain(g, val, vt)
			} else if isLValue(tail.X) && !isEnumType(vt) {
				if _, isFn := vt.(*types.Fn); isFn {
					val = emitFnRetain(g, val, vt)
				} else {
					val = emitRetain(g, val, vt)
				}
			}
			g.line(fmt.Sprintf("%s %s = %s;", cFieldType(g, vt), tmp, val))
			continue
		}
		g.emitStmt(s)
	}

	g.emitDefers()
	g.defers = oldDefers

	var popped []scopeVar
	if !scopeBlockEndsInReturn(stmts) {
		popped = g.popScope()
	} else {
		g.popScope()
	}
	cleanup := make([]scopeVar, 0, len(popped))
	for _, sv := range popped {
		if !moved[sv.name] {
			cleanup = append(cleanup, sv)
		}
	}
	emitScopeCleanupStmts(g, cleanup)

	g.buf = oldBuf
	g.indent = oldIndent

	var out strings.Builder
	out.WriteString("({ ")
	out.WriteString(b.String())
	if tmp != "" {
		out.WriteString(fmt.Sprintf(" %s; ", tmp))
	}
	out.WriteString("})")
	return out.String()
}

// emitFnLit emits a lambda as a closure struct constructor.
//
// [CG-16 SOLUTION] env lifecycle: the environment is a malloc'd struct whose
// first member is a DotRefcnt header (like dot_box_struct). Every capture is
// deep-retained into the env at creation; the env's dtor (registered in the
// header) releases the captures, and dot_release frees the env itself. The
// closure value stays a plain C struct (not ARC-managed itself); references
// to it are ARC-managed through the env: emitDeepRetain/emitDeepRelease
// retain/release `value.env`, closure locals are released by scope cleanup,
// and assignment/return copy paths retain the env, so the env (and the
// captured state) dies when the last closure reference is released.
func emitFnLit(g *generator, x *ast.FnLit) string {
	name := fmt.Sprintf("_dot_closure_%d", g.nextClosure())
	params := "void* _env"
	fnType := g.info.TypeOf(x)
	var fnParams []types.Param
	var resultWant types.Type
	if ft, ok := fnType.(*types.Fn); ok {
		fnParams = ft.Params
		resultWant = ft.Result
	}
	for i, p := range x.Sig.Params {
		params += ", "
		var pt types.Type
		if i < len(fnParams) {
			pt = fnParams[i].Type
		} else {
			pt = types.Int
		}
		ct := cType(g, pt)
		if ct == "void" {
			ct = "void*"
		}
		params += ct + " " + p.Name
	}

	captures := g.info.Captures[x]
	envName := fmt.Sprintf("%s_env", name)
	dtorName := fmt.Sprintf("%s_drop", name)

	if len(captures) > 0 {
		var envStruct strings.Builder
		envStruct.WriteString("typedef struct { DotRefcnt rc; ")
		for i, cap := range captures {
			envStruct.WriteString(fmt.Sprintf("%s v%d; ", cFieldType(g, cap.Type), i))
		}
		envStruct.WriteString(fmt.Sprintf("} %s;", envName))
		// The env typedef must precede the function bodies that use it.
		g.closureDecls = append(g.closureDecls, envStruct.String())
	}

	savedCaptures := g.fnCaptures
	g.fnCaptures = nil
	for _, cap := range captures {
		if g.fnCaptures == nil {
			g.fnCaptures = make(map[string]bool)
		}
		g.fnCaptures[cap.Name] = true
	}
	// The closure body is a separate C function: a return inside it must
	// only clean scopes opened within the body (CG-34).
	g.pushCleanupFloor()
	body := "0"
	if x.ExprBody != nil {
		body = g.emitExpr(x.ExprBody)
		if id, ok := x.ExprBody.(*ast.Ident); ok && g.fnCaptures[varName(g, id)] {
			body = emitDeepRetain(g, body, g.info.TypeOf(x.ExprBody))
		}
	} else if x.Body != nil {
		body = emitBlockExprAs(g, x.Body, resultWant)
	}
	g.popCleanupFloor()
	g.fnCaptures = savedCaptures

	var retType string
	if ft, ok := fnType.(*types.Fn); ok {
		retType = cType(g, ft.Result)
	} else {
		retType = "void*"
	}
	// Prototype emitted before the bodies so call sites see a declaration.
	g.closureDecls = append(g.closureDecls, fmt.Sprintf("%s %s(%s);", retType, name, params))
	result := cType(g, fnType)
	var funcCode strings.Builder
	funcCode.WriteString(fmt.Sprintf("%s %s(%s) { ", retType, name, params))
	if len(captures) > 0 {
		funcCode.WriteString(fmt.Sprintf("%s* env = (%s*)_env; ", envName, envName))
		for i, cap := range captures {
			// Captures are borrowed views of env-owned references: the
			// closure body must not release them.
			funcCode.WriteString(fmt.Sprintf("%s %s = env->v%d; ", cFieldType(g, cap.Type), cap.Name, i))
		}
	}
	// A body ending in a return diverges: its statement expression is void,
	// so it is emitted as bare statements (the return inside it does the
	// returning) instead of `return <stmt-expr>;`, which would reject a void
	// value in a non-void function.
	if x.Body != nil && scopeBlockEndsInReturn(x.Body.Stmts) {
		funcCode.WriteString(body)
		funcCode.WriteString("; ")
	} else {
		funcCode.WriteString(fmt.Sprintf("return %s; ", body))
	}
	funcCode.WriteString("}")
	g.addClosure(funcCode.String())

	if len(captures) > 0 {
		g.closureDecls = append(g.closureDecls, fmt.Sprintf("void %s(void* p);", dtorName))
		var drop strings.Builder
		drop.WriteString(fmt.Sprintf("void %s(void* p) { ", dtorName))
		drop.WriteString(fmt.Sprintf("%s* e = (%s*)p; ", envName, envName))
		for i, cap := range captures {
			rel := emitDeepRelease(g, fmt.Sprintf("e->v%d", i), cap.Type)
			if rel != "" {
				drop.WriteString(rel)
				if !strings.HasSuffix(rel, ";\n") {
					drop.WriteString(";\n")
				}
			}
		}
		drop.WriteString(" }")
		g.addClosure(drop.String())

		var init strings.Builder
		init.WriteString(fmt.Sprintf("({ %s* env = (%s*)malloc(sizeof(%s)); ", envName, envName, envName))
		init.WriteString("if (!env) abort(); ")
		init.WriteString("atomic_init(&env->rc.count, 1); ")
		init.WriteString(fmt.Sprintf("env->rc.dtor = %s; ", dtorName))
		for i, cap := range captures {
			init.WriteString(fmt.Sprintf("env->v%d = %s; ", i, emitDeepRetain(g, cap.Name, cap.Type)))
		}
		init.WriteString(fmt.Sprintf("((%s){ .fn = %s, .env = env }); })", result, name))
		return init.String()
	}

	return fmt.Sprintf("((%s){ .fn = %s, .env = NULL })", result, name)
}

// emitTupleLit emits a tuple literal as a compound literal.
func emitTupleLit(g *generator, x *ast.TupleLit) string {
	t := g.info.TypeOf(x)
	cname := cType(g, t)
	var fields []string
	for i, e := range x.Elems {
		fields = append(fields, fmt.Sprintf(". _%d = %s", i, g.emitExpr(e.Value)))
	}
	return fmt.Sprintf("((%s){%s })", cname, strings.Join(fields, ", "))
}

// emitArrayLit emits an array/slice literal as a runtime construction call.
// Struct and enum elements are boxed so the DotAny slots hold either a struct
// copy or a pointer (DotAny cannot hold a struct value directly).
func emitArrayLit(g *generator, x *ast.ArrayLit) string {
	var elems []string
	for _, e := range x.Elems {
		val := g.emitExpr(e)
		t := g.info.TypeOf(e)
		if isBoxedElem(t) {
			val = fmt.Sprintf("(DotAny)(intptr_t)(%s)", emitBoxValue(g, val, t))
		} else if t != nil && !isPointerLike(t) {
			val = fmt.Sprintf("(DotAny)(intptr_t)(%s)", val)
		}
		elems = append(elems, val)
	}
	return fmt.Sprintf("dot_slice_from_array(%d, (DotAny[]){%s})", len(elems), strings.Join(elems, ", "))
}

// emitMapLit emits a map literal (empty for v0.1; TODO for populated).
func emitMapLit(g *generator, x *ast.MapLit) string {
	if len(x.Entries) == 0 {
		return "dot_map_new()"
	}
	return fmt.Sprintf("/* map literal with %d entries */ dot_map_new()", len(x.Entries))
}

// emitStructLit emits a struct literal as a compound literal.
func emitStructLit(g *generator, x *ast.StructLit) string {
	t := g.info.TypeOf(x)
	cname := cType(g, t)
	var fields []string
	for _, f := range x.Fields {
		val := g.emitExpr(f.Value)
		valType := g.info.TypeOf(f.Value)
		if isLValue(f.Value) {
			fields = append(fields, fmt.Sprintf(". %s = %s", cFieldName(f.Name), emitDeepRetain(g, val, valType)))
		} else {
			fields = append(fields, fmt.Sprintf(". %s = %s", cFieldName(f.Name), val))
		}
	}
	return fmt.Sprintf("((%s){%s })", cname, strings.Join(fields, ", "))
}

// emitCaptures emits the capture list for a lambda (empty for v0.1).
func emitCaptures(g *generator, x *ast.FnLit) string {
	return "NULL, 0"
}

// isStringType reports whether t is the string type.
func isStringType(t types.Type) bool {
	if t == nil {
		return false
	}
	if b, ok := t.(*types.Basic); ok {
		return b.Kind() == types.KindString
	}
	return false
}

// isByteSliceType reports whether t is []byte.
func isByteSliceType(t types.Type) bool {
	if t == nil {
		return false
	}
	if sl, ok := t.(*types.Slice); ok {
		if b, ok := sl.Elem.(*types.Basic); ok {
			return b.Kind() == types.KindUint8
		}
	}
	return false
}

// isResultType reports whether t is a Result type.
func isResultType(t types.Type) bool {
	if t == nil {
		return false
	}
	if n, ok := t.(*types.Named); ok {
		return n.Name == "Result"
	}
	return false
}

// isDynType reports whether t is a dyn Trait type.
func isDynType(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.(*types.Dyn)
	return ok
}

// emitSlice emits a slice operation arr[lo..hi] as a runtime call.
func emitSlice(g *generator, x *ast.SliceExpr) string {
	obj := g.emitExpr(x.X)
	lo := "0"
	if x.Low != nil {
		lo = g.emitExpr(x.Low)
	}
	hi := "dot_slice_len(" + obj + ")"
	if x.High != nil {
		hi = g.emitExpr(x.High)
	}
	inclusive := "false"
	if x.Inclusive {
		inclusive = "true"
	}
	sub := fmt.Sprintf("dot_slice_sub(%s, %s, %s, %s)", obj, lo, hi, inclusive)
	if isStringExpr(g, x.X) {
		// Slicing a string yields a string: take its bytes, sub-slice them
		// and rebuild a DotString (there is no string-specific sub helper in
		// the runtime).
		return fmt.Sprintf("dot_string_from_bytes(dot_slice_sub(dot_string_to_bytes(%s), %s, %s, %s))",
			obj, lo, hi, inclusive)
	}
	return sub
}

// emitAwait emits an await expression as a runtime future-get call.
func emitAwait(g *generator, x *ast.AwaitExpr) string {
	return fmt.Sprintf("dot_future_get(%s)", g.emitExpr(x.X))
}

// emitSpawn emits a spawn expression as a runtime spawn call.
func emitSpawn(g *generator, x *ast.SpawnExpr) string {
	name := fmt.Sprintf("_dot_fiber_%d", g.nextClosure())

	oldBuf := g.buf
	oldIndent := g.indent
	var b strings.Builder
	g.buf = &b
	g.indent = 1
	// The spawn body is a separate C function (fiber): returns inside it
	// must not clean the enclosing function's scopes (CG-34).
	g.pushCleanupFloor()
	for _, stmt := range x.Block.Stmts {
		g.emitStmt(stmt)
	}
	g.popCleanupFloor()
	body := b.String()
	g.buf = oldBuf
	g.indent = oldIndent

	src := fmt.Sprintf("void %s(void) {\n%s}", name, body)
	g.addClosure(src)
	// Prototype emitted before the bodies so spawn call sites see a
	// declaration.
	g.closureDecls = append(g.closureDecls, fmt.Sprintf("void %s(void);", name))

	if x.IsThread {
		return fmt.Sprintf("(dot_thread_spawn(%s), 0)", name)
	}
	return fmt.Sprintf("(async_spawn(%s), 0)", name)
}

// emitRange emits a range expression (used in for loops, not standalone).
func emitRange(g *generator, x *ast.RangeExpr) string {
	lo := "?"
	if x.Low != nil {
		lo = g.emitExpr(x.Low)
	}
	hi := "?"
	if x.High != nil {
		hi = g.emitExpr(x.High)
	}
	suffix := ""
	if x.Inclusive {
		suffix = "="
	}
	return fmt.Sprintf("/* range %s..%s%s */", lo, suffix, hi)
}

// emitAssignExpr emits an assignment in expression position.
func emitAssignExpr(g *generator, x *ast.AssignExpr) string {
	if len(x.Targets) == 1 && len(x.Values) == 1 {
		t := g.emitExpr(x.Targets[0])
		v := g.emitExpr(x.Values[0])
		switch x.Op {
		case lexer.TokenPlusAssign:
			return fmt.Sprintf("(%s += %s)", t, v)
		case lexer.TokenMinusAssign:
			return fmt.Sprintf("(%s -= %s)", t, v)
		case lexer.TokenStarAssign:
			return fmt.Sprintf("(%s *= %s)", t, v)
		case lexer.TokenSlashAssign:
			return fmt.Sprintf("(%s /= %s)", t, v)
		}
		return fmt.Sprintf("(%s = %s)", t, v)
	}
	return fmt.Sprintf("/* multi-assign %T */", x)
}

// emitPipeExpr emits a pipe expression (should have been rewritten by Phase 4).
func emitPipeExpr(g *generator, x *ast.PipeExpr) string {
	return fmt.Sprintf("/* pipe: %T */", x)
}

// emitIsExpr emits a type test as always-true (RTTI not in v0.1).
func emitIsExpr(g *generator, x *ast.IsExpr) string {
	return "true"
}

// isPointerType reports whether t is a pointer-like C type: an explicit
// pointer, an enum (heap-allocated), or a named enum such as Option[T]/Result.
func isPointerType(t types.Type) bool {
	if t == nil {
		return false
	}
	switch x := t.(type) {
	case *types.Pointer:
		return true
	case *types.Enum:
		// Enums with payloads are heap-allocated, so they're pointer types in C.
		return true
	case *types.Named, *types.TypeVar:
		// Named enums (e.g. Option[int], user-declared enums) and bound type
		// variables resolving to enums are pointers too.
		return isEnumType(x)
	}
	return false
}

// isPointerLike reports whether a method receiver type expects a pointer arg.
func isPointerLike(t types.Type) bool {
	if t == nil {
		return false
	}
	switch t.(type) {
	case *types.Pointer, *types.Named:
		return true
	}
	return types.IsHeap(t)
}

// isOptionType reports whether t is an Option type (Named named "Option").
func isOptionType(t types.Type) bool {
	if t == nil {
		return false
	}
	if n, ok := t.(*types.Named); ok {
		return n.Name == "Option"
	}
	return false
}

// ensure strconv is used.
var _ = strconv.Itoa
