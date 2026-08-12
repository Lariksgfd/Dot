// Package codegen translates a typed Dot AST into C source code.
//
// This file maps every Dot type to its C representation, mangles field names
// to avoid C namespace conflicts, produces monomorphisation suffixes for
// generic instances, and emits C literals from Dot literal AST nodes.
package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/types"
)

// cType returns the C type string for a Dot type. It dispatches on the type's
// Kind and maps every primitive, composite and named type to its C
// representation per PLAN.md section 2.
func cType(g *generator, t types.Type) string {
	if t == nil {
		return "void"
	}
	switch x := t.(type) {
	case *types.Basic:
		return cBasic(x)
	case *types.Slice:
		return "DotSlice*"
	case *types.Array:
		// Fixed-size array becomes a struct with a data field and length.
		elem := cType(g, x.Elem)
		return fmt.Sprintf("struct { %s data[%d]; int64_t len; }", elem, x.Len)
	case *types.Map:
		return "DotMap*"
	case *types.Tuple:
		return cTupleName(g, x)
	case *types.Fn:
		return cFnPtr(g, x)
	case *types.Pointer:
		return cType(g, x.Elem) + "*"
	case *types.Dyn:
		return "DotDyn*"
	case *types.Weak:
		return "DotWeak*"
	case *types.Chan:
		return "DotChannel*"
	case *types.Future:
		return "DotFuture*"
	case *types.Named:
		return cNamed(g, x)
	case *types.Struct:
		return cAnonStruct(g, x)
	case *types.Enum:
		return cAnonEnum(g, x)
	case *types.Trait:
		return "struct Dot" + x.Name + "_vtable*"
	case *types.TypeParam:
		return "void*"
	case *types.TypeVar:
		if x.Bound != nil {
			return cType(g, x.Bound)
		}
		return "void*"
	}
	return "void*"
}

// cBasic maps a primitive Dot type to its C equivalent.
func cBasic(b *types.Basic) string {
	switch b.Kind() {
	case types.KindBool:
		return "bool"
	case types.KindInt:
		return "int64_t"
	case types.KindInt8:
		return "int8_t"
	case types.KindInt16:
		return "int16_t"
	case types.KindInt32:
		return "int32_t"
	case types.KindUint:
		return "uint64_t"
	case types.KindUint8:
		return "uint8_t"
	case types.KindUint16:
		return "uint16_t"
	case types.KindUint32:
		return "uint32_t"
	case types.KindFloat:
		return "double"
	case types.KindFloat32:
		return "float"
	case types.KindRune:
		return "int32_t"
	case types.KindString:
		return "DotString*"
	case types.KindVoid:
		return "void"
	case types.KindNever:
		return "void"
	case types.KindUntypedInt:
		return "int64_t"
	case types.KindUntypedFloat:
		return "double"
	case types.KindUntypedBool:
		return "bool"
	case types.KindUntypedString:
		return "DotString*"
	case types.KindUntypedNil:
		return "void*"
	}
	return "void*"
}

// cTupleStruct emits an anonymous tuple as a positional C struct.
func cTupleStruct(g *generator, t *types.Tuple) string {
	var b strings.Builder
	b.WriteString("struct { ")
	for i, e := range t.Elems {
		b.WriteString(cType(g, e))
		b.WriteString(fmt.Sprintf(" _%d; ", i))
	}
	b.WriteString("}")
	return b.String()
}

// tupleInfo records one positional tuple struct typedef.
type tupleInfo struct {
	Name   string
	Fields []string
}

// cTupleName returns a stable typedef name for a tuple shape, registering the
// typedef for emission in the header. Identical shapes share one typedef so
// that declarations, definitions and returns all reference the same C type.
func cTupleName(g *generator, t *types.Tuple) string {
	fields := make([]string, len(t.Elems))
	for i, e := range t.Elems {
		fields[i] = cType(g, e)
	}
	key := strings.Join(fields, ",")
	if g.tupleDefs == nil {
		g.tupleDefs = make(map[string]*tupleInfo)
	}
	if info, ok := g.tupleDefs[key]; ok {
		return info.Name
	}
	info := &tupleInfo{
		Name:   fmt.Sprintf("DotTuple_%d", len(g.tupleList)),
		Fields: fields,
	}
	g.tupleDefs[key] = info
	g.tupleList = append(g.tupleList, info)
	return info.Name
}

// cFnPtr emits a function pointer type: R (*)(A, B).
func cFnPtr(g *generator, f *types.Fn) string {
	result := cType(g, f.Result)
	var params []string
	for _, p := range f.Params {
		params = append(params, cType(g, p.Type))
	}
	if len(params) == 0 {
		return fmt.Sprintf("%s (*)(void)", result)
	}
	return fmt.Sprintf("%s (*)(%s)", result, strings.Join(params, ", "))
}

// cNamed handles a user-declared nominal type. Generic instances use
// info.Instances to find the mangled name.
func cNamed(g *generator, n *types.Named) string {
	name := "Dot" + n.Name
	if len(n.TypeArgs) == 0 {
		return name
	}
	if g != nil && g.info != nil {
		for _, inst := range g.info.InstanceList {
			if inst.Result == n && inst.Mangled != "" {
				return inst.Mangled
			}
		}
	}
	return name + "_" + mangleArgs(n.TypeArgs)
}

// mangleArgs concatenates mangled type arguments separated by underscores.
func mangleArgs(args []types.Type) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = mangleType(a)
	}
	return strings.Join(parts, "_")
}

// cAnonStruct emits an anonymous (non-named) struct inline.
func cAnonStruct(g *generator, s *types.Struct) string {
	var b strings.Builder
	b.WriteString("struct { ")
	for _, f := range s.Fields {
		b.WriteString(cType(g, f.Type))
		b.WriteString(" ")
		b.WriteString(cFieldName(f.Name))
		b.WriteString("; ")
	}
	b.WriteString("}")
	return b.String()
}

// cAnonEnum emits an anonymous enum as a tagged union struct.
func cAnonEnum(g *generator, e *types.Enum) string {
	var b strings.Builder
	b.WriteString("struct { int32_t tag; union { ")
	for _, v := range e.Variants {
		for _, f := range v.Fields {
			b.WriteString(cType(g, f.Type))
			b.WriteString(" ")
			b.WriteString(cFieldName(v.Name + "_" + f.Name))
			b.WriteString("; ")
		}
	}
	b.WriteString("}; }")
	return b.String()
}

// cFieldName prefixes a Dot field name to avoid conflicts with C keywords
// and Dot builtins (e.g. `len`, `tag`).
func cFieldName(name string) string {
	return "dot_" + name
}

// mangleType produces a type's mangled suffix for monomorphisation.
// Primitives use their kind name; composites encode structure.
func mangleType(t types.Type) string {
	if t == nil {
		return "void"
	}
	switch x := t.(type) {
	case *types.Basic:
		return mangleBasic(x)
	case *types.Slice:
		return "Slice_" + mangleType(x.Elem)
	case *types.Array:
		return fmt.Sprintf("Array_%d_%s", x.Len, mangleType(x.Elem))
	case *types.Map:
		return "Map_" + mangleType(x.Key) + "_" + mangleType(x.Value)
	case *types.Tuple:
		parts := make([]string, len(x.Elems))
		for i, e := range x.Elems {
			parts[i] = mangleType(e)
		}
		return "Tuple_" + strings.Join(parts, "_")
	case *types.Fn:
		params := make([]string, len(x.Params))
		for i, p := range x.Params {
			params[i] = mangleType(p.Type)
		}
		return "Fn_" + strings.Join(params, "_") + "_" + mangleType(x.Result)
	case *types.Pointer:
		return mangleType(x.Elem) + "_star"
	case *types.Dyn:
		if x.Trait != nil {
			return "Dyn_" + x.Trait.Name
		}
		return "Dyn"
	case *types.Weak:
		return "Weak_" + mangleType(x.Elem)
	case *types.Chan:
		return "Channel_" + mangleType(x.Elem)
	case *types.Future:
		return "Future_" + mangleType(x.Result)
	case *types.Named:
		name := x.Name
		if len(x.TypeArgs) > 0 {
			name += "_" + mangleArgs(x.TypeArgs)
		}
		return name
	case *types.Struct:
		parts := make([]string, len(x.Fields))
		for i, f := range x.Fields {
			parts[i] = mangleType(f.Type)
		}
		return "Struct_" + strings.Join(parts, "_")
	case *types.Enum:
		parts := make([]string, len(x.Variants))
		for i, v := range x.Variants {
			parts[i] = v.Name
		}
		return "Enum_" + strings.Join(parts, "_")
	case *types.Trait:
		return "Trait_" + x.Name
	case *types.TypeParam:
		return "T_" + t.String()
	case *types.TypeVar:
		if x.Bound != nil {
			return mangleType(x.Bound)
		}
		return fmt.Sprintf("TV%d", x.ID)
	}
	return t.String()
}

// mangleBasic returns the monomorphisation suffix for a primitive type.
func mangleBasic(b *types.Basic) string {
	switch b.Kind() {
	case types.KindInt:
		return "int"
	case types.KindInt8:
		return "int8"
	case types.KindInt16:
		return "int16"
	case types.KindInt32:
		return "int32"
	case types.KindUint:
		return "uint"
	case types.KindUint8:
		return "uint8"
	case types.KindUint16:
		return "uint16"
	case types.KindUint32:
		return "uint32"
	case types.KindFloat:
		return "float"
	case types.KindFloat32:
		return "float32"
	case types.KindBool:
		return "bool"
	case types.KindRune:
		return "rune"
	case types.KindString:
		return "DotString_star"
	case types.KindVoid:
		return "void"
	case types.KindNever:
		return "never"
	}
	return b.Name()
}

// cLiteral returns a C literal string from a Dot literal AST node and its
// type. Int, float, bool and nil emit directly; strings become a runtime
// call that allocates a DotString*.
func cLiteral(g *generator, lit ast.Expr, t types.Type) string {
	switch l := lit.(type) {
	case *ast.IntLit:
		return strconv.FormatUint(l.Value, 10)
	case *ast.FloatLit:
		if t != nil && t.Kind() == types.KindFloat32 {
			return strconv.FormatFloat(l.Value, 'f', -1, 32) + "f"
		}
		return strconv.FormatFloat(l.Value, 'f', -1, 64)
	case *ast.BoolLit:
		if l.Value {
			return "true"
		}
		return "false"
	case *ast.StringLit:
		s := stringify(l)
		return fmt.Sprintf("dot_string_from_lit(%s, %d)", strconv.Quote(s), len(s))
	case *ast.RawStringLit:
		return fmt.Sprintf("dot_string_from_lit(%s, %d)", strconv.Quote(l.Value), len(l.Value))
	case *ast.NilLit:
		return "NULL"
	}
	return "0"
}

// stringify extracts the full text of a StringLit, interpolations included,
// as the raw source span. For non-interpolated literals this is just the text.
func stringify(s *ast.StringLit) string {
	if len(s.Parts) == 0 {
		return ""
	}
	if len(s.Parts) == 1 && s.Parts[0].Kind == ast.PartText {
		return s.Parts[0].Text
	}
	return s.Raw
}
