package types

import (
	"fmt"
	"strings"

	"github.com/dotlang/dot/ast"
)

// Slice is `[]T`.
type Slice struct{ Elem Type }

// Kind returns KindSlice.
func (t *Slice) Kind() Kind { return KindSlice }

// String renders the slice as "[]T".
func (t *Slice) String() string {
	if t.Elem == nil {
		return "[]?"
	}
	return "[]" + t.Elem.String()
}

// Array is `[N]T`. Len is the constant-folded length.
type Array struct {
	Elem Type
	Len  int64
}

// Kind returns KindArray.
func (t *Array) Kind() Kind { return KindArray }

// String renders the array as "[N]T".
func (t *Array) String() string {
	if t.Elem == nil {
		return fmt.Sprintf("[%d]?", t.Len)
	}
	return fmt.Sprintf("[%d]%s", t.Len, t.Elem.String())
}

// Map is `map[K]V`.
type Map struct{ Key, Value Type }

// Kind returns KindMap.
func (t *Map) Kind() Kind { return KindMap }

// String renders the map as "map[K]V".
func (t *Map) String() string {
	k, v := "?", "?"
	if t.Key != nil {
		k = t.Key.String()
	}
	if t.Value != nil {
		v = t.Value.String()
	}
	return "map[" + k + "]" + v
}

// Tuple is `(A, B)`; also the result type of a multi-value function (D15).
// A Tuple never has exactly one element: `(A)` collapses to A (D44).
type Tuple struct{ Elems []Type }

// Kind returns KindTuple.
func (t *Tuple) Kind() Kind { return KindTuple }

// Len returns the number of elements.
func (t *Tuple) Len() int { return len(t.Elems) }

// String renders the tuple as "(A, B)".
func (t *Tuple) String() string {
	parts := make([]string, len(t.Elems))
	for i, e := range t.Elems {
		if e == nil {
			parts[i] = "?"
		} else {
			parts[i] = e.String()
		}
	}
	return "(" + joinStrings(parts) + ")"
}

// Param is one parameter of a Fn type.
type Param struct {
	Name     string
	Type     Type
	HasDflt  bool     // a default expression was supplied
	Default  ast.Expr // the un-evaluated default, for codegen; nil when none
	Variadic bool
	Mut      bool
}

// Fn is a function type. Result is Void for a void function and a *Tuple for
// a multi-value return. Recv is non-nil for a method value.
type Fn struct {
	Params     []Param
	Result     Type
	Recv       Type // nil for free functions
	RecvMut    bool // the receiver was declared `mut self`
	Async      bool
	Variadic   bool         // == last param's Variadic
	TypeParams []*TypeParam // non-empty for a generic function (uninstantiated)
}

// Kind returns KindFn.
func (f *Fn) Kind() Kind { return KindFn }

// Arity returns (required, total) parameter counts, ignoring the receiver.
func (f *Fn) Arity() (required, total int) {
	total = len(f.Params)
	required = total
	for _, p := range f.Params {
		if p.HasDflt || p.Variadic {
			required--
		}
	}
	if required < 0 {
		required = 0
	}
	return required, total
}

// String renders the fn type as "fn(A, B) -> R" or "fn(A, B)" when void.
func (f *Fn) String() string {
	parts := make([]string, len(f.Params))
	for i, p := range f.Params {
		s := p.Name + " "
		if p.Type == nil {
			s += "?"
		} else {
			s += p.Type.String()
		}
		if p.Variadic {
			s = "..." + s
		}
		parts[i] = s
	}
	var b strings.Builder
	b.WriteString("fn(")
	b.WriteString(joinStrings(parts))
	b.WriteString(")")
	if f.Result != nil && f.Result.Kind() != KindVoid {
		b.WriteString(" -> ")
		b.WriteString(f.Result.String())
	}
	return b.String()
}

// Pointer is `*T` (only legal inside @perf, D68).
type Pointer struct{ Elem Type }

// Kind returns KindPointer.
func (t *Pointer) Kind() Kind { return KindPointer }

// String renders the pointer as "*T".
func (t *Pointer) String() string {
	if t.Elem == nil {
		return "*?"
	}
	return "*" + t.Elem.String()
}

// Dyn is `dyn Trait`.
type Dyn struct{ Trait *Trait }

// Kind returns KindDyn.
func (t *Dyn) Kind() Kind { return KindDyn }

// String renders the dyn as "dyn TraitName" or "dyn <trait>".
func (t *Dyn) String() string {
	if t.Trait == nil {
		return "dyn ?"
	}
	return "dyn " + t.Trait.Name
}

// Weak is `weak T`.
type Weak struct{ Elem Type }

// Kind returns KindWeak.
func (t *Weak) Kind() Kind { return KindWeak }

// String renders the weak as "weak T".
func (t *Weak) String() string {
	if t.Elem == nil {
		return "weak ?"
	}
	return "weak " + t.Elem.String()
}

// Chan is `Channel[T]`, produced by instantiating the builtin Channel.
type Chan struct{ Elem Type }

// Kind returns KindChan.
func (t *Chan) Kind() Kind { return KindChan }

// String renders the chan as "Channel[T]".
func (t *Chan) String() string {
	if t.Elem == nil {
		return "Channel[?]"
	}
	return "Channel[" + t.Elem.String() + "]"
}

// Future is the type of an async call result and of a spawn handle (D69).
type Future struct{ Result Type }

// Kind returns KindFuture.
func (t *Future) Kind() Kind { return KindFuture }

// String renders the future as "Future[R]".
func (t *Future) String() string {
	if t.Result == nil {
		return "Future[?]"
	}
	return "Future[" + t.Result.String() + "]"
}

// FieldPath is the chain of field indices needed to reach an embedded field.
// len(Path) == 1 for a direct field.
type FieldPath struct {
	Path  []int
	Field *Field
	Depth int
}

// Field is one struct field.
type Field struct {
	Name     string
	Type     Type
	Embedded bool // written `embed T`
	HasDflt  bool
	Default  ast.Expr
	Private  bool // name starts with '_'
	Pos      ast.Position
	Index    int // position in Struct.Fields
}

// Struct is the underlying type of a named struct declaration.
type Struct struct {
	Fields []Field
	// Flat is the flattened field lookup built from Fields plus every
	// embedded struct's Flat, keyed by field name (D59).
	Flat map[string]FieldPath
	// Ambiguous holds names reachable through more than one embedding path;
	// looking one up is an error, not a silent pick.
	Ambiguous map[string]bool
}

// Kind returns KindStruct.
func (t *Struct) Kind() Kind { return KindStruct }

// String renders the struct as its field list.
func (t *Struct) String() string {
	parts := make([]string, len(t.Fields))
	for i, f := range t.Fields {
		s := f.Name
		if f.Type != nil {
			s += " " + f.Type.String()
		}
		parts[i] = s
	}
	return "struct { " + joinStrings(parts) + " }"
}

// Variant is one enum variant.
type Variant struct {
	Name      string
	Fields    []Param // empty for a unit variant
	Tag       int     // 0-based discriminant, used by codegen
	Pos       ast.Position
	HasParens bool
}

// Enum is the underlying type of a named enum declaration.
type Enum struct {
	Variants []Variant
	byName   map[string]*Variant
}

// Kind returns KindEnum.
func (e *Enum) Kind() Kind { return KindEnum }

// Variant looks up a variant by name.
func (e *Enum) Variant(name string) (*Variant, bool) {
	if e.byName == nil {
		return nil, false
	}
	v, ok := e.byName[name]
	return v, ok
}

// String renders the enum as "EnumName[V1, V2]".
func (e *Enum) String() string {
	parts := make([]string, len(e.Variants))
	for i, v := range e.Variants {
		parts[i] = v.Name
	}
	return "enum { " + joinStrings(parts) + " }"
}

// Method is one trait method (required or defaulted).
type Method struct {
	Name    string
	Sig     *Fn
	HasBody bool // a default implementation exists
	Static  bool // no self receiver
	Decl    *ast.FnDecl
	Pos     ast.Position
}

// Trait is the underlying type of a trait declaration.
type Trait struct {
	Name       string
	Methods    []Method
	TypeParams []*TypeParam
	byName     map[string]*Method
}

// Kind returns KindTrait.
func (t *Trait) Kind() Kind { return KindTrait }

// Method looks up a method by name.
func (t *Trait) Method(name string) (*Method, bool) {
	if t.byName == nil {
		return nil, false
	}
	m, ok := t.byName[name]
	return m, ok
}

// String renders the trait as "trait TraitName".
func (t *Trait) String() string {
	return "trait " + t.Name
}

// ObjectSafe reports whether the trait may be used as `dyn Trait`, and why not
// when it may not (D64). v0.1: a trait is object-safe iff every method has a
// self receiver, never mentions Self except as the receiver, and is not
// generic. The returned string is empty when safe, otherwise a human-readable
// reason.
func (t *Trait) ObjectSafe() (ok bool, reason string) {
	for _, m := range t.SigMethods() {
		if m.Static {
			return false, fmt.Sprintf(
				"%s cannot be used as 'dyn' because '%s' is a static method",
				t.Name, m.Name)
		}
	}
	return true, ""
}

// SigMethods returns the methods slice (named to avoid clashing with the
// exported Method lookup method).
func (t *Trait) SigMethods() []Method { return t.Methods }
