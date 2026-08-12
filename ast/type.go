package ast

// NamedType is `int`, `User`, `http.Client`.
type NamedType struct {
	BaseNode
	// Pkg is the qualifying module name, "" when unqualified.
	Pkg string
	// PkgPos is the position of Pkg; valid only when Pkg != "".
	PkgPos Position
	// Name is the type name.
	Name string
	// NamePos is the position of Name.
	NamePos Position
}

// GenericType is `Option[int]`, `Result[int, Error]`, `Stack[T]`.
type GenericType struct {
	BaseNode
	// Base is the instantiated type, normally a *NamedType.
	Base Type
	// Args are the type arguments, at least one.
	Args []Type
	// LBracket is the position of '['.
	LBracket Position
	// RBracket is the position of ']'.
	RBracket Position
}

// SliceType is `[]int`.
type SliceType struct {
	BaseNode
	// Elem is the element type.
	Elem Type
}

// ArrayType is `[5]int`. Len is a constant expression, evaluated in Phase 4.
type ArrayType struct {
	BaseNode
	// Len is the constant length expression.
	Len Expr
	// Elem is the element type.
	Elem Type
}

// MapType is `map[string]int`.
type MapType struct {
	BaseNode
	// Key is the key type.
	Key Type
	// Value is the value type.
	Value Type
}

// FnType is `fn(int, int) -> int`. Result is nil for a void function type.
type FnType struct {
	BaseNode
	// Params are the parameter types in source order.
	Params []Type
	// Variadic reports whether the last parameter is written `...T`.
	Variadic bool
	// Result is the result type, nil for a void function type.
	Result Type
}

// TupleType is `(int, string)`. Also used for multi-value returns (D15).
type TupleType struct {
	BaseNode
	// Elems are the element types in source order.
	Elems []Type
}

// PointerType is `*int` (only legal inside @perf blocks; enforced in Phase 4).
type PointerType struct {
	BaseNode
	// Elem is the pointee type.
	Elem Type
}

// DynType is `dyn Printable`.
type DynType struct {
	BaseNode
	// Trait is the trait being made dynamic.
	Trait Type
}

// WeakType is `weak Option[Node]`.
type WeakType struct {
	BaseNode
	// Elem is the referenced type.
	Elem Type
}

// OptionalType is the `T?` shorthand for Option[T] (SPEC §6). Desugared in
// Phase 4, kept distinct here so `dot fmt` round-trips the source (D16).
type OptionalType struct {
	BaseNode
	// Elem is the wrapped type.
	Elem Type
}

// SelfTypeNode is the `Self` type inside a trait or impl body.
// Named with the -Node suffix to avoid colliding with the SelfExpr value.
type SelfTypeNode struct {
	BaseNode
}

// BadType is a placeholder produced by parser error recovery.
type BadType struct {
	BaseNode
}

func (*NamedType) typeNode()    {}
func (*GenericType) typeNode()  {}
func (*SliceType) typeNode()    {}
func (*ArrayType) typeNode()    {}
func (*MapType) typeNode()      {}
func (*FnType) typeNode()       {}
func (*TupleType) typeNode()    {}
func (*PointerType) typeNode()  {}
func (*DynType) typeNode()      {}
func (*WeakType) typeNode()     {}
func (*OptionalType) typeNode() {}
func (*SelfTypeNode) typeNode() {}
func (*BadType) typeNode()      {}
