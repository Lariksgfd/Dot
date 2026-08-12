package llvm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/types"
)

// llvmType maps a checked Dot type to its LLVM IR spelling. Scalars map to
// i64/double/float/i1 per PLAN, void maps to void, strings keep the
// { ptr, i32 } fat-pointer stub. Unknown types fall back to i64.
func (e *emitter) llvmType(t types.Type) string {
	if t == nil {
		return "i64"
	}
	if b, ok := t.(*types.Basic); ok {
		switch b.Kind() {
		case types.KindBool, types.KindUntypedBool:
			return "i1"
		case types.KindInt, types.KindUint, types.KindUntypedInt:
			return "i64"
		case types.KindInt8, types.KindUint8:
			return "i8"
		case types.KindInt16, types.KindUint16:
			return "i16"
		case types.KindInt32, types.KindUint32, types.KindRune:
			return "i32"
		case types.KindFloat, types.KindUntypedFloat:
			return "double"
		case types.KindFloat32:
			return "float"
		case types.KindVoid, types.KindNever:
			return "void"
		case types.KindString:
			return "{ ptr, i32 }"
		}
	}
	if n, ok := t.(*types.Named); ok {
		return "%" + n.Name
	}
	if a, ok := t.(*types.Array); ok {
		return fmt.Sprintf("[%d x %s]", a.Len, e.llvmType(a.Elem))
	}
	if _, ok := t.(*types.Slice); ok {
		return "{ ptr, i32, i32 }"
	}
	if _, ok := t.(*types.Pointer); ok {
		return "ptr"
	}
	return "i64"
}

// isTypeParam reports whether name is one of fn's generic type parameters.
func (e *emitter) isTypeParam(fn *ast.FnDecl, name string) bool {
	for _, tp := range fn.TypeParams {
		if tp.Name == name {
			return true
		}
	}
	return false
}

// instantiatedType returns the LLVM type for a generic type parameter in the
// current instantiation, defaulting to i64 when unresolved.
func (e *emitter) instantiatedType(name string) string {
	if e.genericArgs != nil {
		if ta, ok := e.genericArgs[name]; ok {
			return e.llvmType(ta)
		}
	}
	return "i64"
}

// shapeType returns the LLVM shape of a pointer-returning literal expression
// (array, slice, struct, string), or "" when unknown.
func (e *emitter) shapeType(expr ast.Expr) string {
	switch ex := expr.(type) {
	case *ast.ArrayLit:
		switch ex.Type.(type) {
		case *ast.SliceType:
			return "{ ptr, i32, i32 }"
		case *ast.ArrayType:
			at := ex.Type.(*ast.ArrayType)
			elem := e.resolveType(at.Elem)
			n := len(ex.Elems)
			if il, ok := at.Len.(*ast.IntLit); ok {
				n = int(il.Value)
			}
			return fmt.Sprintf("[%d x %s]", n, elem)
		}
	case *ast.StructLit:
		if named, ok := ex.Type.(*ast.NamedType); ok {
			return "%" + named.Name
		}
	case *ast.StringLit:
		return "{ ptr, i32 }"
	}
	return ""
}

// exprType resolves the LLVM type of an expression. It prefers the recorded
// checker type, then falls back to literal shape and known variable types so
// the emitter still works without a types.Info (unit tests).
func (e *emitter) exprType(expr ast.Expr) string {
	if expr == nil {
		return "i64"
	}
	if e.info != nil {
		if t := e.info.TypeOf(expr); t != nil && t.Kind() != types.KindInvalid {
			return e.llvmType(t)
		}
	}
	switch x := expr.(type) {
	case *ast.FloatLit:
		return "double"
	case *ast.BoolLit:
		return "i1"
	case *ast.StringLit:
		return "{ ptr, i32 }"
	case *ast.Ident:
		if vt, ok := e.varTypes[x.Name]; ok {
			return vt
		}
	}
	return "i64"
}

// isFloatType reports whether an LLVM type string is a floating-point type.
func isFloatType(t string) bool {
	return t == "double" || t == "float"
}

// zeroConst returns a zero LLVM constant of the given type.
func zeroConst(t string) string {
	if isFloatType(t) {
		return "0.0"
	}
	if t == "ptr" {
		return "null"
	}
	if strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") {
		return "zeroinitializer"
	}
	return "0"
}

// llvmEscape escapes a string for embedding in an LLVM c"..." constant.
// Newlines and other control characters must be written as hex escapes so
// the emitted module survives CRLF translation and IR pretty-printing.
func llvmEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\n':
			b.WriteString(`\0A`)
		case '\r':
			b.WriteString(`\0D`)
		case '\t':
			b.WriteString(`\09`)
		case '\\':
			b.WriteString(`\5C`)
		case '"':
			b.WriteString(`\22`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// floatConst formats a float64 as an LLVM IR floating-point constant.
// LLVM requires a decimal point or exponent, so integer-shaped values get
// a trailing ".0" and exponent forms get one before the "e".
func floatConst(v float64) string {
	if v < 0 {
		return "-" + floatConst(-v)
	}
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		if !strings.Contains(s[:i], ".") {
			s = s[:i] + ".0" + s[i:]
		}
	} else if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}
