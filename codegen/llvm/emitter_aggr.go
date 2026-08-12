// Package llvm implements the LLVM IR backend for Dot.
//
// This file holds the aggregate-value codegen for part 2 of the LLVM
// backend: strings (fat pointers { ptr, i32 }), arrays/slices, structs,
// bounds checks, iteration over collections and the self-contained string
// runtime helpers.
package llvm

import (
	"fmt"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
	"github.com/dotlang/dot/types"
)

// isNamedAgg reports whether an LLVM type string denotes a user-declared
// aggregate (struct or enum): "%Point". Values of such types always flow as
// pointers in this backend.
func isNamedAgg(t string) bool {
	return strings.HasPrefix(t, "%")
}

// isFatAgg reports whether t is one of the inline composite LLVM types that
// flow by value: the string fat pointer and the slice header.
func isFatAgg(t string) bool {
	return t == "{ ptr, i32 }" || t == "{ ptr, i32, i32 }"
}

// sizeOf returns the in-memory size in bytes of an LLVM type string. Only
// the shapes this backend produces are covered; unknown types default to 8.
func sizeOf(t string) int64 {
	switch t {
	case "i1", "i8":
		return 1
	case "i16":
		return 2
	case "i32", "float":
		return 4
	case "i64", "double", "ptr":
		return 8
	case "{ ptr, i32 }":
		return 16
	case "{ ptr, i32, i32 }":
		return 16
	}
	if isNamedAgg(t) {
		return 8
	}
	return 8
}

// namedTypeName returns the name of the declared type of t (struct or enum),
// or "" when t is not a named type.
func (e *emitter) namedTypeName(t types.Type) string {
	if t == nil {
		return ""
	}
	if n, ok := t.(*types.Named); ok {
		return n.Name
	}
	if n, ok := types.Underlying(t).(*types.Named); ok {
		return n.Name
	}
	return ""
}

// namedTypeOf resolves the named type of an expression, or "".
func (e *emitter) namedTypeOf(expr ast.Expr) string {
	if e.info == nil {
		return ""
	}
	return e.namedTypeName(e.info.TypeOf(expr))
}

// emitRuntimeHelpers emits the self-contained string helpers (equality,
// concatenation) and the bounds-check panic. They operate on the
// { ptr, i32 } fat-pointer string representation, so no C runtime layout
// assumptions leak into the IR; only libc malloc/puts/exit are used.
func (e *emitter) emitRuntimeHelpers() {
	oobName := e.formatString("index out of range")

	e.emit("")
	e.emit("define internal void @dot_bounds_panic() {")
	e.emit("entry:")
	e.emit("  %s = call i32 @puts(ptr %s)", e.nextTmp(), oobName)
	e.emit("  call void @exit(i32 1)")
	e.emit("  unreachable")
	e.emit("}")
	e.emit("")

	e.emit("define internal i1 @dot_str_eq({ ptr, i32 } %sa, { ptr, i32 } %sb) {", "%", "%")
	e.emit("entry:")
	ap := e.nextTmp()
	e.emit("  %s = extractvalue { ptr, i32 } %sa, 0", ap, "%")
	al := e.nextTmp()
	e.emit("  %s = extractvalue { ptr, i32 } %sa, 1", al, "%")
	bp := e.nextTmp()
	e.emit("  %s = extractvalue { ptr, i32 } %sb, 0", bp, "%")
	bl := e.nextTmp()
	e.emit("  %s = extractvalue { ptr, i32 } %sb, 1", bl, "%")
	same := e.nextTmp()
	e.emit("  %s = icmp eq ptr %s, %s", same, ap, bp)
	e.emit("  br i1 %s, label %%%s, label %%%s", same, "eq.true", "eq.lencmp")
	e.emit("")
	e.emit("eq.lencmp:")
	sameLen := e.nextTmp()
	e.emit("  %s = icmp eq i32 %s, %s", sameLen, al, bl)
	e.emit("  br i1 %s, label %%%s, label %%%s", sameLen, "eq.loop", "eq.false")
	e.emit("")
	e.emit("eq.loop:")
	idx := e.nextTmp()
	e.emit("  %s = phi i32 [ 0, %%%s ], [ %s, %%%s ]", idx, "eq.lencmp", "%eq.next", "eq.step")
	more := e.nextTmp()
	e.emit("  %s = icmp slt i32 %s, %s", more, idx, al)
	e.emit("  br i1 %s, label %%%s, label %%%s", more, "eq.body", "eq.true")
	e.emit("")
	e.emit("eq.body:")
	aPtr := e.nextTmp()
	e.emit("  %s = getelementptr i8, ptr %s, i32 %s", aPtr, ap, idx)
	bPtr := e.nextTmp()
	e.emit("  %s = getelementptr i8, ptr %s, i32 %s", bPtr, bp, idx)
	av := e.nextTmp()
	e.emit("  %s = load i8, ptr %s", av, aPtr)
	bv := e.nextTmp()
	e.emit("  %s = load i8, ptr %s", bv, bPtr)
	neq := e.nextTmp()
	e.emit("  %s = icmp ne i8 %s, %s", neq, av, bv)
	e.emit("  br i1 %s, label %%%s, label %%%s", neq, "eq.false", "eq.step")
	e.emit("")
	e.emit("eq.step:")
	e.emit("  %s = add i32 %s, 1", "%eq.next", idx)
	e.emit("  br label %%%s", "eq.loop")
	e.emit("")
	e.emit("eq.true:")
	e.emit("  ret i1 true")
	e.emit("")
	e.emit("eq.false:")
	e.emit("  ret i1 false")
	e.emit("}")
	e.emit("")

	e.emit("define internal { ptr, i32 } @dot_str_concat({ ptr, i32 } %sa, { ptr, i32 } %sb) {", "%", "%")
	e.emit("entry:")
	cap_ := e.nextTmp()
	e.emit("  %s = extractvalue { ptr, i32 } %sa, 0", cap_, "%")
	cal := e.nextTmp()
	e.emit("  %s = extractvalue { ptr, i32 } %sa, 1", cal, "%")
	cbp := e.nextTmp()
	e.emit("  %s = extractvalue { ptr, i32 } %sb, 0", cbp, "%")
	cbl := e.nextTmp()
	e.emit("  %s = extractvalue { ptr, i32 } %sb, 1", cbl, "%")
	ctotal := e.nextTmp()
	e.emit("  %s = add i32 %s, %s", ctotal, cal, cbl)
	csz := e.nextTmp()
	e.emit("  %s = zext i32 %s to i64", csz, ctotal)
	csz1 := e.nextTmp()
	e.emit("  %s = add i64 %s, 1", csz1, csz)
	cbuf := e.nextTmp()
	e.emit("  %s = call ptr @malloc(i64 %s)", cbuf, csz1)
	e.emit("  br label %%%s", "cat.a")
	e.emit("")
	e.emit("cat.a:")
	ci := e.nextTmp()
	e.emit("  %s = phi i32 [ 0, %%%s ], [ %s, %%%s ]", ci, "entry", "%cat.next1", "cat.astep")
	cmore := e.nextTmp()
	e.emit("  %s = icmp slt i32 %s, %s", cmore, ci, cal)
	e.emit("  br i1 %s, label %%%s, label %%%s", cmore, "cat.abody", "cat.b")
	e.emit("")
	e.emit("cat.abody:")
	csp := e.nextTmp()
	e.emit("  %s = getelementptr i8, ptr %s, i32 %s", csp, cap_, ci)
	csv := e.nextTmp()
	e.emit("  %s = load i8, ptr %s", csv, csp)
	cdp := e.nextTmp()
	e.emit("  %s = getelementptr i8, ptr %s, i32 %s", cdp, cbuf, ci)
	e.emit("  store i8 %s, ptr %s", csv, cdp)
	e.emit("  br label %%%s", "cat.astep")
	e.emit("")
	e.emit("cat.astep:")
	e.emit("  %s = add i32 %s, 1", "%cat.next1", ci)
	e.emit("  br label %%%s", "cat.a")
	e.emit("")
	e.emit("cat.b:")
	cj := e.nextTmp()
	e.emit("  %s = phi i32 [ 0, %%%s ], [ %s, %%%s ]", cj, "cat.a", "%cat.next2", "cat.bstep")
	cmore2 := e.nextTmp()
	e.emit("  %s = icmp slt i32 %s, %s", cmore2, cj, cbl)
	e.emit("  br i1 %s, label %%%s, label %%%s", cmore2, "cat.bbody", "cat.finish")
	e.emit("")
	e.emit("cat.bbody:")
	csp2 := e.nextTmp()
	e.emit("  %s = getelementptr i8, ptr %s, i32 %s", csp2, cbp, cj)
	csv2 := e.nextTmp()
	e.emit("  %s = load i8, ptr %s", csv2, csp2)
	coff := e.nextTmp()
	e.emit("  %s = add i32 %s, %s", coff, cal, cj)
	cdp2 := e.nextTmp()
	e.emit("  %s = getelementptr i8, ptr %s, i32 %s", cdp2, cbuf, coff)
	e.emit("  store i8 %s, ptr %s", csv2, cdp2)
	e.emit("  br label %%%s", "cat.bstep")
	e.emit("")
	e.emit("cat.bstep:")
	e.emit("  %s = add i32 %s, 1", "%cat.next2", cj)
	e.emit("  br label %%%s", "cat.b")
	e.emit("")
	e.emit("cat.finish:")
	cterm := e.nextTmp()
	e.emit("  %s = getelementptr i8, ptr %s, i32 %s", cterm, cbuf, ctotal)
	e.emit("  store i8 0, ptr %s", cterm)
	cr1 := e.nextTmp()
	e.emit("  %s = insertvalue { ptr, i32 } undef, ptr %s, 0", cr1, cbuf)
	cr2 := e.nextTmp()
	e.emit("  %s = insertvalue { ptr, i32 } %s, i32 %s, 1", cr2, cr1, ctotal)
	e.emit("  ret { ptr, i32 } %s", cr2)
	e.emit("}")
	e.emit("")
}

// emitStringEq emits a call to the inline string equality helper and
// returns the i1 result register.
func (e *emitter) emitStringEq(left, right string) string {
	tmp := e.nextTmp()
	e.emit("  %s = call i1 @dot_str_eq({ ptr, i32 } %s, { ptr, i32 } %s)", tmp, left, right)
	return tmp
}

// emitStringConcat emits a call to the inline string concatenation helper
// and returns the { ptr, i32 } result register.
func (e *emitter) emitStringConcat(left, right string) string {
	tmp := e.nextTmp()
	e.emit("  %s = call { ptr, i32 } @dot_str_concat({ ptr, i32 } %s, { ptr, i32 } %s)", tmp, left, right)
	return tmp
}

// materializeStruct copies the struct pointed to by val into a fresh heap
// allocation and returns the heap pointer. Used when a function returns an
// aggregate: the stack alloca would dangle after the return.
func (e *emitter) materializeStruct(structName, val string) string {
	fieldTypes := e.structFieldTypes[structName]
	total := int64(0)
	for _, ft := range fieldTypes {
		total += sizeOf(ft)
	}
	heap := e.nextTmp()
	e.emit("  %s = call ptr @malloc(i64 %d)", heap, total)
	for i, ft := range fieldTypes {
		src := e.nextTmp()
		e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", src, structName, val, i)
		v := e.nextTmp()
		e.emit("  %s = load %s, ptr %s", v, ft, src)
		dst := e.nextTmp()
		e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", dst, structName, heap, i)
		e.emit("  store %s %s, ptr %s", ft, v, dst)
	}
	return heap
}

// llvmAggType converts a checked type to an LLVM value type, extended with
// the part-2 aggregates: named structs, arrays and slices.
func (e *emitter) llvmAggType(t types.Type) string {
	switch x := t.(type) {
	case *types.Named:
		return "%" + x.Name
	case *types.Array:
		return fmt.Sprintf("[%d x %s]", x.Len, e.llvmType(x.Elem))
	case *types.Slice:
		return "{ ptr, i32, i32 }"
	case *types.Pointer:
		return "ptr"
	}
	return e.llvmType(t)
}

// structFieldLLVMType returns the LLVM field type used inside a struct
// definition for a Dot field type. Named aggregates and slice headers are
// stored as pointers (pointer-flow); strings stay inline fat pointers.
func (e *emitter) structFieldLLVMType(fieldType ast.Type) string {
	ft := e.resolveType(fieldType)
	if isNamedAgg(ft) || ft == "{ ptr, i32, i32 }" {
		return "ptr"
	}
	return ft
}

// skipARC reports whether a heap-typed value should not get dot_retain/
// dot_release calls in this backend. Strings are value fat pointers
// ({ ptr, i32 }) and slices are stack headers; both are stack values, so
// ARC calls on their allocas would corrupt memory.
func (e *emitter) skipARC(t types.Type) bool {
	switch types.Underlying(t).(type) {
	case *types.Basic:
		return t.Kind() == types.KindString
	case *types.Slice:
		return true
	}
	return false
}

// structNameFor resolves the declared struct name for a receiver expression,
// falling back to a field-name scan over the declared structs.
func (e *emitter) structNameFor(expr ast.Expr, fieldName string) string {
	if name := e.namedTypeOf(expr); name != "" {
		if _, ok := e.structFields[name]; ok {
			return name
		}
	}
	if ident, ok := expr.(*ast.Ident); ok {
		if vt, ok := e.varTypes[ident.Name]; ok && isNamedAgg(vt) {
			name := strings.TrimPrefix(vt, "%")
			if _, ok := e.structFields[name]; ok {
				return name
			}
		}
	}
	for sName, fMap := range e.structFields {
		if _, ok := fMap[fieldName]; ok {
			return sName
		}
	}
	return "Unknown"
}

// fieldInfo returns the field index and LLVM field type for a field of the
// given struct, defaulting to (0, i64).
func (e *emitter) fieldInfo(structName, fieldName string) (int, string) {
	if fMap, ok := e.structFields[structName]; ok {
		if idx, ok := fMap[fieldName]; ok {
			ft := "i64"
			if fts, ok := e.structFieldTypes[structName]; ok && idx < len(fts) {
				ft = fts[idx]
			}
			return idx, ft
		}
	}
	return 0, "i64"
}

// emitStructLitAggr emits a struct literal as an alloca plus field stores.
// The result is a pointer to the alloca (named aggregates flow as pointers).
func (e *emitter) emitStructLitAggr(ex *ast.StructLit) string {
	tmp := e.nextTmp()
	structName := "Unknown"
	if named, ok := ex.Type.(*ast.NamedType); ok {
		structName = named.Name
	}
	e.emit("  %s = alloca %%%s", tmp, structName)
	for _, field := range ex.Fields {
		val := e.emitExpr(field.Value)
		idx, ftype := e.fieldInfo(structName, field.Name)
		fieldPtr := e.nextTmp()
		e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", fieldPtr, structName, tmp, idx)
		e.emit("  store %s %s, ptr %s", ftype, val, fieldPtr)
	}
	return tmp
}

// storeToField emits a store into a struct field: self.x = ...
func (e *emitter) storeToField(fe *ast.FieldExpr, val string) {
	base := e.emitExpr(fe.X)
	structName := e.structNameFor(fe.X, fe.Name)
	idx, ftype := e.fieldInfo(structName, fe.Name)
	fieldPtr := e.nextTmp()
	e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", fieldPtr, structName, base, idx)
	e.emit("  store %s %s, ptr %s", ftype, val, fieldPtr)
}

// emitFieldExpr emits a field access: a builtin property (len/cap) on
// collections, or a struct field load.
func (e *emitter) emitFieldExpr(ex *ast.FieldExpr) string {
	if ex.Name == "len" || ex.Name == "cap" {
		if v := e.emitLenProp(ex); v != "" {
			return v
		}
	}
	base := e.emitExpr(ex.X)
	structName := e.structNameFor(ex.X, ex.Name)
	idx, ftype := e.fieldInfo(structName, ex.Name)
	fieldPtr := e.nextTmp()
	e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", fieldPtr, structName, base, idx)
	tmp := e.nextTmp()
	e.emit("  %s = load %s, ptr %s", tmp, ftype, fieldPtr)
	return tmp
}

// emitLenProp emits the .len/.cap property for slices, arrays and strings.
// It returns "" when the receiver is not a collection, so callers can fall
// back to a struct field access.
func (e *emitter) emitLenProp(ex *ast.FieldExpr) string {
	if e.info == nil {
		return ""
	}
	t := e.info.TypeOf(ex.X)
	switch tt := types.Underlying(t).(type) {
	case *types.Array:
		return fmt.Sprintf("%d", tt.Len)
	case *types.Slice:
		base := e.emitExpr(ex.X)
		f := e.nextTmp()
		e.emit("  %s = getelementptr { ptr, i32, i32 }, ptr %s, i32 0, i32 1", f, base)
		v := e.nextTmp()
		e.emit("  %s = load i32, ptr %s", v, f)
		z := e.nextTmp()
		e.emit("  %s = zext i32 %s to i64", z, v)
		return z
	}
	if b, ok := t.(*types.Basic); ok && b.Kind() == types.KindString {
		v := e.emitExpr(ex.X)
		l := e.nextTmp()
		e.emit("  %s = extractvalue { ptr, i32 } %s, 1", l, v)
		z := e.nextTmp()
		e.emit("  %s = zext i32 %s to i64", z, l)
		return z
	}
	return ""
}

// emitCallExpr emits a regular function call or a method call. Named
// aggregate parameters and results flow as pointers, so both the call
// instruction and the callee use ptr for those positions.
func (e *emitter) emitCallExpr(ex *ast.CallExpr) string {
	if ident, ok := ex.Fn.(*ast.Ident); ok && ident.Name == "print" && len(ex.Args) > 0 {
		return e.emitPrint(ex)
	}
	if fe, ok := ex.Fn.(*ast.FieldExpr); ok && e.info != nil {
		if sel, ok := e.info.Selections[fe]; ok && sel.Kind == types.SelectMethod && sel.Method != nil {
			return e.emitMethodCall(ex, fe, sel)
		}
	}
	fnName := ""
	if ident, ok := ex.Fn.(*ast.Ident); ok {
		fnName = ident.Name
	}
	if e.info != nil && e.info.Instances != nil {
		if inst, ok := e.info.Instances[ex]; ok {
			fnName = inst.Mangled
		}
	}
	var args []string
	for _, arg := range ex.Args {
		at := e.exprType(arg.Value)
		if isNamedAgg(at) {
			at = "ptr"
		}
		args = append(args, fmt.Sprintf("%s %s", at, e.emitExpr(arg.Value)))
	}
	resType := e.exprType(ex)
	if isNamedAgg(resType) {
		resType = "ptr"
	}
	if resType == "void" {
		e.emit("  call void @%s(%s)", fnName, strings.Join(args, ", "))
		return "0"
	}
	tmp := e.nextTmp()
	e.emit("  %s = call %s @%s(%s)", tmp, resType, fnName, strings.Join(args, ", "))
	return tmp
}

// emitMethodCall emits a method call. The receiver (self) is passed as a
// pointer; static methods skip the receiver argument.
func (e *emitter) emitMethodCall(ex *ast.CallExpr, fe *ast.FieldExpr, sel *types.Selection) string {
	owner := ""
	if sel.Owner != nil {
		owner = sel.Owner.Name
	}
	name := "Dot_" + owner + "_" + sel.Method.Name
	var args []string
	if !sel.Method.Static {
		recv := e.emitExpr(fe.X)
		args = append(args, fmt.Sprintf("ptr %s", recv))
	}
	for _, arg := range ex.Args {
		at := e.exprType(arg.Value)
		if isNamedAgg(at) {
			at = "ptr"
		}
		args = append(args, fmt.Sprintf("%s %s", at, e.emitExpr(arg.Value)))
	}
	resType := e.exprType(ex)
	if isNamedAgg(resType) {
		resType = "ptr"
	}
	if resType == "void" {
		e.emit("  call void @%s(%s)", name, strings.Join(args, ", "))
		return "0"
	}
	tmp := e.nextTmp()
	e.emit("  %s = call %s @%s(%s)", tmp, resType, name, strings.Join(args, ", "))
	return tmp
}

// emitAssignExpr emits an assignment expression: x = v, p.x = v, a[i] = v
// and the compound forms += -= *= /= %=.
func (e *emitter) emitAssignExpr(ex *ast.AssignExpr) string {
	if len(ex.Targets) != 1 || len(ex.Values) != 1 {
		return "0"
	}
	target := ex.Targets[0]
	val := e.emitExpr(ex.Values[0])

	if ex.Op != lexer.TokenAssign {
		var cur string
		var t string
		switch tgt := target.(type) {
		case *ast.Ident:
			t = "i64"
			if vt, ok := e.varTypes[tgt.Name]; ok {
				t = vt
			}
			tmp := e.nextTmp()
			if e.ptrVars[tgt.Name] {
				e.emit("  %s = load ptr, ptr %%%s", tmp, tgt.Name)
			} else {
				e.emit("  %s = load %s, ptr %%%s", tmp, t, tgt.Name)
			}
			cur = tmp
		case *ast.FieldExpr:
			cur = e.emitFieldExpr(tgt)
			_, ft := e.fieldInfo(e.structNameFor(tgt.X, tgt.Name), tgt.Name)
			t = ft
		case *ast.IndexExpr:
			cur = e.emitIndexExpr(tgt)
			t = e.infoElemLLVM(tgt)
		default:
			return val
		}
		val = e.emitArith(compoundToBin(ex.Op), t, cur, val)
	}

	switch tgt := target.(type) {
	case *ast.Ident:
		if e.ptrVars[tgt.Name] {
			e.emit("  store ptr %s, ptr %%%s", val, tgt.Name)
		} else {
			t := "i64"
			if vt, ok := e.varTypes[tgt.Name]; ok {
				t = vt
			}
			e.emit("  store %s %s, ptr %%%s", t, val, tgt.Name)
		}
	case *ast.FieldExpr:
		e.storeToField(tgt, val)
	case *ast.IndexExpr:
		e.storeToIndex(tgt, val)
	}
	return val
}

// infoElemLLVM returns the LLVM element type of an index expression's
// container, defaulting to i64.
func (e *emitter) infoElemLLVM(ie *ast.IndexExpr) string {
	if e.info == nil {
		return "i64"
	}
	switch tt := types.Underlying(e.info.TypeOf(ie.X)).(type) {
	case *types.Slice:
		return e.llvmType(tt.Elem)
	case *types.Array:
		return e.llvmType(tt.Elem)
	case *types.Basic:
		if tt.Kind() == types.KindString {
			return "i8"
		}
	}
	return "i64"
}

// compoundToBin maps a compound assignment token to its binary operator.
func compoundToBin(op lexer.TokenType) lexer.TokenType {
	switch op {
	case lexer.TokenPlusAssign:
		return lexer.TokenPlus
	case lexer.TokenMinusAssign:
		return lexer.TokenMinus
	case lexer.TokenStarAssign:
		return lexer.TokenStar
	case lexer.TokenSlashAssign:
		return lexer.TokenSlash
	case lexer.TokenPercentAssign:
		return lexer.TokenPercent
	}
	return lexer.TokenAssign
}

// emitArith emits one scalar arithmetic instruction and returns its register.
func (e *emitter) emitArith(op lexer.TokenType, t, left, right string) string {
	tmp := e.nextTmp()
	f := isFloatType(t)
	switch op {
	case lexer.TokenPlus:
		if f {
			e.emit("  %s = fadd %s %s, %s", tmp, t, left, right)
		} else {
			e.emit("  %s = add %s %s, %s", tmp, t, left, right)
		}
	case lexer.TokenMinus:
		if f {
			e.emit("  %s = fsub %s %s, %s", tmp, t, left, right)
		} else {
			e.emit("  %s = sub %s %s, %s", tmp, t, left, right)
		}
	case lexer.TokenStar:
		if f {
			e.emit("  %s = fmul %s %s, %s", tmp, t, left, right)
		} else {
			e.emit("  %s = mul %s %s, %s", tmp, t, left, right)
		}
	case lexer.TokenSlash:
		if f {
			e.emit("  %s = fdiv %s %s, %s", tmp, t, left, right)
		} else {
			e.emit("  %s = sdiv %s %s, %s", tmp, t, left, right)
		}
	case lexer.TokenPercent:
		if f {
			e.emit("  %s = frem %s %s, %s", tmp, t, left, right)
		} else {
			e.emit("  %s = srem %s %s, %s", tmp, t, left, right)
		}
	default:
		return right
	}
	return tmp
}

// storeToIndex emits a store into an array/slice element: a[i] = v.
func (e *emitter) storeToIndex(ie *ast.IndexExpr, val string) {
	info := e.resolveIndex(ie)
	if info.dataPtr == "" {
		base := e.emitExpr(ie.X)
		idx := e.emitExpr(ie.Indices[0])
		idxType := e.exprType(ie.Indices[0])
		tmp := e.nextTmp()
		e.emit("  %s = getelementptr %s, ptr %s, %s %s", tmp, info.elemType, base, idxType, idx)
		e.emit("  store %s %s, ptr %s", info.elemType, val, tmp)
		return
	}
	if info.lenVal == "" {
		idx := e.emitExpr(ie.Indices[0])
		idxType := e.exprType(ie.Indices[0])
		tmp := e.nextTmp()
		e.emit("  %s = getelementptr %s, ptr %s, %s %s", tmp, info.elemType, info.dataPtr, idxType, idx)
		e.emit("  store %s %s, ptr %s", info.elemType, val, tmp)
		return
	}
	okBlock := e.nextLabel("idx.ok")
	oobBlock := e.nextLabel("idx.oob")
	endBlock := e.nextLabel("idx.end")
	idxType := e.exprType(ie.Indices[0])
	idx := e.emitExpr(ie.Indices[0])
	e.emitBoundsCheck(idx, idxType, info.lenVal, okBlock, oobBlock)

	e.emit("\n%s:", okBlock)
	elemPtr := e.nextTmp()
	e.emit("  %s = getelementptr %s, ptr %s, %s %s", elemPtr, info.elemType, info.dataPtr, idxType, idx)
	e.emit("  store %s %s, ptr %s", info.elemType, val, elemPtr)
	e.emit("  br label %%%s", endBlock)

	e.emit("\n%s:", oobBlock)
	e.emit("  call void @dot_bounds_panic()")
	e.emit("  unreachable")

	e.emit("\n%s:", endBlock)
}

// emitForIterable emits `for v in items` / `for i, v in items` over a slice
// against the collection length.
func (e *emitter) emitForIterable(s *ast.ForStmt) {
	t := e.info.TypeOf(s.Iterable)
	var dataPtr, lenVal, elemType string
	namedAgg := false
	switch tt := types.Underlying(t).(type) {
	case *types.Slice:
		elemType = e.llvmType(tt.Elem)
		namedAgg = isNamedAgg(elemType)
		base := e.emitExpr(s.Iterable)
		f0 := e.nextTmp()
		e.emit("  %s = getelementptr { ptr, i32, i32 }, ptr %s, i32 0, i32 0", f0, base)
		dataPtr = e.nextTmp()
		e.emit("  %s = load ptr, ptr %s", dataPtr, f0)
		f1 := e.nextTmp()
		e.emit("  %s = getelementptr { ptr, i32, i32 }, ptr %s, i32 0, i32 1", f1, base)
		l32 := e.nextTmp()
		e.emit("  %s = load i32, ptr %s", l32, f1)
		lenVal = e.nextTmp()
		e.emit("  %s = sext i32 %s to i64", lenVal, l32)
	case *types.Array:
		elemType = e.llvmType(tt.Elem)
		namedAgg = isNamedAgg(elemType)
		base := e.emitExpr(s.Iterable)
		dataPtr = e.nextTmp()
		e.emit("  %s = getelementptr [%d x %s], ptr %s, i64 0, i64 0", dataPtr, tt.Len, e.llvmType(tt.Elem), base)
		lenVal = fmt.Sprintf("%d", tt.Len)
	default:
		e.emitForCond(s)
		return
	}

	loopVar := ""
	if ident, ok := s.Value.(*ast.Ident); ok {
		loopVar = ident.Name
	} else {
		loopVar = e.nextLabel("for.it")
	}
	if namedAgg {
		e.varTypes[loopVar] = elemType
		e.ptrVars[loopVar] = true
		e.emit("  %%%s = alloca ptr", loopVar)
	} else {
		e.varTypes[loopVar] = elemType
		e.emit("  %%%s = alloca %s", loopVar, elemType)
	}

	idxVar := ""
	if s.Key != nil {
		if ident, ok := s.Key.(*ast.Ident); ok {
			idxVar = ident.Name
		} else {
			idxVar = e.nextLabel("for.idx")
		}
		e.varTypes[idxVar] = "i64"
		e.emit("  %%%s = alloca i64", idxVar)
	} else {
		idxVar = e.nextLabel("for.idx")
		e.emit("  %%%s = alloca i64", idxVar)
	}
	e.emit("  store i64 0, ptr %%%s", idxVar)

	condBlock := e.nextLabel("for.cond")
	bodyBlock := e.nextLabel("for.body")
	stepBlock := e.nextLabel("for.step")
	endBlock := e.nextLabel("for.end")

	e.emit("  br label %%%s", condBlock)

	e.emit("\n%s:", condBlock)
	cur := e.nextTmp()
	e.emit("  %s = load i64, ptr %%%s", cur, idxVar)
	cmpT := e.nextTmp()
	e.emit("  %s = icmp slt i64 %s, %s", cmpT, cur, lenVal)
	e.emit("  br i1 %s, label %%%s, label %%%s", cmpT, bodyBlock, endBlock)

	e.emit("\n%s:", bodyBlock)
	cur2 := e.nextTmp()
	e.emit("  %s = load i64, ptr %%%s", cur2, idxVar)
	elemPtr := e.nextTmp()
	if namedAgg {
		e.emit("  %s = getelementptr ptr, ptr %s, i64 %s", elemPtr, dataPtr, cur2)
		v := e.nextTmp()
		e.emit("  %s = load ptr, ptr %s", v, elemPtr)
		e.emit("  store ptr %s, ptr %%%s", v, loopVar)
	} else {
		e.emit("  %s = getelementptr %s, ptr %s, i64 %s", elemPtr, elemType, dataPtr, cur2)
		v := e.nextTmp()
		e.emit("  %s = load %s, ptr %s", v, elemType, elemPtr)
		e.emit("  store %s %s, ptr %%%s", elemType, v, loopVar)
	}
	if s.Key != nil {
		e.emit("  store i64 %s, ptr %%%s", cur2, idxVar)
	}
	if s.Body != nil {
		for _, bs := range s.Body.Stmts {
			e.emitStmt(bs)
		}
	}
	e.emit("  br label %%%s", stepBlock)

	e.emit("\n%s:", stepBlock)
	cur3 := e.nextTmp()
	e.emit("  %s = load i64, ptr %%%s", cur3, idxVar)
	nextV := e.nextTmp()
	e.emit("  %s = add i64 %s, 1", nextV, cur3)
	e.emit("  store i64 %s, ptr %%%s", nextV, idxVar)
	e.emit("  br label %%%s", condBlock)

	e.emit("\n%s:", endBlock)
}
