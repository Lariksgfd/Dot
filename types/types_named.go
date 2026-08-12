package types

import (
	"fmt"

	"github.com/dotlang/dot/ast"
)

// Named is a user-declared nominal type: struct, enum or trait target.
// Identity is by pointer (D48).
type Named struct {
	Name       string
	Sym        *Symbol            // the type's own symbol, set during collection
	Origin     *Named             // the uninstantiated generic, nil when this IS it
	Underlying Type               // *Struct, *Enum or *Trait
	TypeParams []*TypeParam       // declared parameters (generic definition)
	TypeArgs   []Type             // non-nil for an instantiated generic
	Methods    map[string]*Method // inherent methods from `impl T { }`
	Impls      []*TraitImpl       // trait impls for this type
	Pos        ast.Position
}

// Kind returns KindNamed.
func (n *Named) Kind() Kind { return KindNamed }

// String renders the named type, e.g. "Stack[int]".
func (n *Named) String() string {
	if n.Name == "" {
		return "<anonymous named>"
	}
	if len(n.TypeArgs) == 0 {
		return n.Name
	}
	parts := make([]string, len(n.TypeArgs))
	for i, a := range n.TypeArgs {
		if a == nil {
			parts[i] = "?"
		} else {
			parts[i] = a.String()
		}
	}
	return n.Name + "[" + joinStrings(parts) + "]"
}

// TraitImpl records `impl Trait for T`.
type TraitImpl struct {
	Trait   *Trait
	Target  *Named
	Methods map[string]*Method
	Decl    *ast.ImplDecl
}

// TypeParam is a declared generic parameter `T: Printable + Hashable`.
// Identity is by pointer.
type TypeParam struct {
	Name   string
	Index  int
	Bounds []*Trait
	Pos    ast.Position
}

// Kind returns KindTypeParam.
func (t *TypeParam) Kind() Kind { return KindTypeParam }

// String renders the type parameter as its name.
func (t *TypeParam) String() string { return t.Name }

// TypeVar is an inference variable created during checking. Bound is nil until
// unification resolves it (D52).
type TypeVar struct {
	ID    int
	Bound Type
	Pos   ast.Position
}

// Kind returns KindTypeVar.
func (t *TypeVar) Kind() Kind { return KindTypeVar }

// String renders the inference variable as a "?T<n>" placeholder.
func (t *TypeVar) String() string {
	return fmt.Sprintf("?T%d", t.ID)
}

// Constructors — all types are always built through these so that
// canonicalisation (e.g. Slice-of-Invalid folding to Invalid) happens in one
// place.

// NewSlice returns `[]elem`. If elem is Invalid the result is Invalid.
func NewSlice(elem Type) Type {
	if elem == nil || IsInvalid(elem) {
		return Invalid
	}
	return &Slice{Elem: elem}
}

// NewArray returns `[n]elem`. If elem is Invalid the result is Invalid.
func NewArray(elem Type, n int64) Type {
	if elem == nil || IsInvalid(elem) {
		return Invalid
	}
	return &Array{Elem: elem, Len: n}
}

// NewMap returns `map[k]v`. If either is Invalid the result is Invalid.
func NewMap(k, v Type) Type {
	if k == nil || v == nil || IsInvalid(k) || IsInvalid(v) {
		return Invalid
	}
	return &Map{Key: k, Value: v}
}

// NewTuple returns a Tuple over elems. Zero elems yields Void; one elem yields
// that elem directly (D44 collapses `(A)` to A).
func NewTuple(elems ...Type) Type {
	switch len(elems) {
	case 0:
		return Void
	case 1:
		return elems[0]
	}
	// If any element is Invalid the whole tuple is Invalid.
	for _, e := range elems {
		if e == nil || IsInvalid(e) {
			return Invalid
		}
	}
	return &Tuple{Elems: elems}
}

// NewPointer returns `*elem`.
func NewPointer(elem Type) Type {
	if elem == nil {
		return Invalid
	}
	return &Pointer{Elem: elem}
}

// NewWeak returns `weak elem`.
func NewWeak(elem Type) Type {
	if elem == nil {
		return Invalid
	}
	return &Weak{Elem: elem}
}

// NewDyn returns `dyn tr`.
func NewDyn(tr *Trait) Type {
	if tr == nil {
		return Invalid
	}
	return &Dyn{Trait: tr}
}

// NewChan returns `Channel[elem]`.
func NewChan(elem Type) Type {
	if elem == nil {
		return Invalid
	}
	return &Chan{Elem: elem}
}

// NewFuture returns `Future[res]`.
func NewFuture(res Type) Type {
	if res == nil {
		return Invalid
	}
	return &Future{Result: res}
}
