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
		if isStructOrEnum(elemType) {
			res = fmt.Sprintf("(*(%s*)%s)", cType(g, elemType), res)
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
	fmt.Printf("DEBUG emitIndex: type=%T, obj=%s\n", t, obj)
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
		return fmt.Sprintf("({ typeof(%s) _t = %s; if (_t.tag == 1) return DotNone; _t.value; })", inner, inner)
	}
	if isResultType(t) {
		return fmt.Sprintf("({ typeof(%s) _t = %s; if (_t.tag == 1) return dot_result_err(_t.error); _t.value; })", inner, inner)
	}
	return inner
}

// emitBlockExpr emits a block expression as a GCC statement expression.
func emitBlockExpr(g *generator, x *ast.BlockExpr) string {
	var b strings.Builder
	b.WriteString("({ ")
	for _, s := range x.Block.Stmts {
		switch st := s.(type) {
		case *ast.ExprStmt:
			b.WriteString(g.emitExpr(st.X))
			b.WriteString("; ")
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
	b.WriteString("})")
	return b.String()
}

// emitFnLit emits a lambda as a closure struct constructor.
func emitFnLit(g *generator, x *ast.FnLit) string {
	name := fmt.Sprintf("_dot_closure_%d", g.nextClosure())
	params := "void* _env"
	fnType := g.info.TypeOf(x)
	var fnParams []types.Param
	if ft, ok := fnType.(*types.Fn); ok {
		fnParams = ft.Params
	}
	for i, p := range x.Sig.Params {
		params += ", "
		var pt types.Type
		if i < len(fnParams) {
			pt = fnParams[i].Type
		} else {
			pt = types.Int
		}
		params += cType(g, pt) + " " + p.Name
	}

	captures := g.info.Captures[x]
	envName := fmt.Sprintf("%s_env", name)

	var envStruct strings.Builder
	if len(captures) > 0 {
		envStruct.WriteString(fmt.Sprintf("typedef struct { "))
		for i, cap := range captures {
			envStruct.WriteString(fmt.Sprintf("%s v%d; ", cType(g, cap.Type), i))
		}
		envStruct.WriteString(fmt.Sprintf("} %s;", envName))
		g.addClosure(envStruct.String())
	}

	body := "0"
	if x.ExprBody != nil {
		body = g.emitExpr(x.ExprBody)
	} else if x.Body != nil {
		body = emitBlockExpr(g, &ast.BlockExpr{Block: x.Body})
	}

	var retType string
	if ft, ok := fnType.(*types.Fn); ok {
		retType = cType(g, ft.Result)
	} else {
		retType = "void*"
	}
	result := cType(g, fnType)
	var funcCode strings.Builder
	funcCode.WriteString(fmt.Sprintf("%s %s(%s) { ", retType, name, params))
	if len(captures) > 0 {
		funcCode.WriteString(fmt.Sprintf("%s* env = (%s*)_env; ", envName, envName))
		for i, cap := range captures {
			funcCode.WriteString(fmt.Sprintf("%s %s = env->v%d; ", cType(g, cap.Type), cap.Name, i))
		}
	}
	funcCode.WriteString(fmt.Sprintf("return %s; }", body))
	g.addClosure(funcCode.String())

	if len(captures) > 0 {
		var init strings.Builder
		init.WriteString(fmt.Sprintf("({ %s* env = malloc(sizeof(%s)); ", envName, envName))
		for i, cap := range captures {
			init.WriteString(fmt.Sprintf("env->v%d = %s; ", i, emitIdent(g, &ast.Ident{Name: cap.Name})))
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
func emitArrayLit(g *generator, x *ast.ArrayLit) string {
	var elems []string
	for _, e := range x.Elems {
		elems = append(elems, g.emitExpr(e))
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
		fields = append(fields, fmt.Sprintf(". %s = %s", cFieldName(f.Name), g.emitExpr(f.Value)))
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
	return fmt.Sprintf("dot_slice_sub(%s, %s, %s, %s)", obj, lo, hi, inclusive)
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
	for _, stmt := range x.Block.Stmts {
		g.emitStmt(stmt)
	}
	body := b.String()
	g.buf = oldBuf
	g.indent = oldIndent
	
	src := fmt.Sprintf("void %s(void) {\n%s}", name, body)
	g.addClosure(src)
	
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

// isPointerType reports whether t is a *types.Pointer.
func isPointerType(t types.Type) bool {
	if t == nil {
		return false
	}
	switch t.(type) {
	case *types.Pointer:
		return true
	case *types.Enum:
		// Enums with payloads are heap-allocated, so they're pointer types in C.
		return true
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
