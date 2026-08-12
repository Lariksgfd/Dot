package ast

import "strings"

// Annotation is `@test`, `@deprecated("...")`, `@inline`, `@extern("C")`.
// Annotations are not statements; they are attached to the declaration that
// follows via its Annotations field (D21). `@perf { }` is a PerfBlock instead.
type Annotation struct {
	BaseNode
	// Name is the annotation name without the '@'.
	Name string
	// NamePos is the position of Name.
	NamePos Position
	// Args are the annotation arguments, empty when written without
	// parentheses.
	Args []Expr
	// AtPos is the position of '@'.
	AtPos Position
}

// TypeParam is one generic parameter: `T`, `T: Comparable`,
// `T: Printable + Hashable`.
type TypeParam struct {
	BaseNode
	// Name is the type parameter name.
	Name string
	// NamePos is the position of Name.
	NamePos Position
	// Bounds are the trait bounds, empty when unconstrained.
	Bounds []Type
}

// Param is a function parameter, and is reused for enum variant fields.
// IsSelf marks the `self` / `mut self` receiver, which the parser stores in
// FnSig.Recv rather than in Params (D22).
type Param struct {
	BaseNode
	// Name is the parameter name.
	Name string
	// NamePos is the position of Name.
	NamePos Position
	// Type is the parameter type, nil for an untyped lambda parameter,
	// e.g. fn(x) { ... }.
	Type Type
	// Default is the default value expression, nil when absent.
	Default Expr
	// Variadic reports whether the parameter was written `nums ...int`.
	Variadic bool
	// Mut reports whether the parameter was written `mut`.
	Mut bool
	// IsSelf marks the `self` / `mut self` receiver.
	IsSelf bool
}

// FnSig is the signature shared by FnDecl, FnLit and trait methods.
// Result is nil for a void function; multi-value returns use a *TupleType.
type FnSig struct {
	BaseNode
	// Recv is the `self` receiver, nil for free functions and lambdas.
	Recv *Param
	// Params are the parameters in source order, excluding the receiver.
	Params []*Param
	// Result is the result type, nil for a void function.
	Result Type
	// LParen is the position of '('.
	LParen Position
	// RParen is the position of ')'.
	RParen Position
	// ArrowPos is the position of `->`; invalid when Result is nil.
	ArrowPos Position
}

// Variadic reports whether the last parameter is variadic.
func (s *FnSig) Variadic() bool {
	if len(s.Params) == 0 {
		return false
	}
	return s.Params[len(s.Params)-1].Variadic
}

// FnDecl is a function, method or trait method.
//
//	Body != nil     -> block body
//	ExprBody != nil -> short form `= expr`
//	both nil        -> declaration only (trait requirement, @extern)
type FnDecl struct {
	BaseNode
	// Annotations are the annotations preceding the declaration (D21).
	Annotations []*Annotation
	// Pub reports whether the function is exported with `pub`.
	Pub bool
	// Async reports whether the function is declared `async`.
	Async bool
	// Name is the function name.
	Name string
	// NamePos is the position of Name.
	NamePos Position
	// TypeParams are the generic parameters, empty for a non-generic
	// function.
	TypeParams []*TypeParam
	// Sig is the function signature.
	Sig *FnSig
	// Body is the block body, nil for the short form and for declarations
	// without a body.
	Body *BlockStmt
	// ExprBody is the body of the short form `= expr`, else nil.
	ExprBody Expr
	// KwPos is the position of the `fn` keyword.
	KwPos Position
}

// FieldDecl is one struct field. For `embed Animal`, Embed is true, Name is
// "" and Type names the embedded struct.
type FieldDecl struct {
	BaseNode
	// Annotations are the annotations preceding the field (D21).
	Annotations []*Annotation
	// Embed reports whether the field is an `embed` of another struct.
	Embed bool
	// Name is the field name, "" for an embedded field.
	Name string
	// NamePos is the position of Name; valid only when Name != "".
	NamePos Position
	// Type is the field type.
	Type Type
	// Default is the default value expression, nil when absent.
	Default Expr
}

// StructDecl is a `struct` declaration.
type StructDecl struct {
	BaseNode
	// Annotations are the annotations preceding the declaration (D21).
	Annotations []*Annotation
	// Pub reports whether the struct is exported with `pub`.
	Pub bool
	// Name is the struct name.
	Name string
	// NamePos is the position of Name.
	NamePos Position
	// TypeParams are the generic parameters, empty for a non-generic struct.
	TypeParams []*TypeParam
	// Fields are the fields in source order.
	Fields []*FieldDecl
	// KwPos is the position of the `struct` keyword.
	KwPos Position
}

// EnumVariant is `Red` or `Circle(radius float)`.
// HasParens distinguishes `Red` from the (legal but empty) `Red()`.
type EnumVariant struct {
	BaseNode
	// Name is the variant name.
	Name string
	// NamePos is the position of Name.
	NamePos Position
	// HasParens reports whether the variant was written with parentheses.
	HasParens bool
	// Fields are the variant's payload fields.
	Fields []*Param
}

// EnumDecl is an `enum` declaration.
type EnumDecl struct {
	BaseNode
	// Annotations are the annotations preceding the declaration (D21).
	Annotations []*Annotation
	// Pub reports whether the enum is exported with `pub`.
	Pub bool
	// Name is the enum name.
	Name string
	// NamePos is the position of Name.
	NamePos Position
	// TypeParams are the generic parameters, empty for a non-generic enum.
	TypeParams []*TypeParam
	// Variants are the variants in source order.
	Variants []*EnumVariant
	// KwPos is the position of the `enum` keyword.
	KwPos Position
}

// TraitDecl holds required methods (FnDecl with no body) and default
// implementations (FnDecl with a body) in one ordered slice.
type TraitDecl struct {
	BaseNode
	// Annotations are the annotations preceding the declaration (D21).
	Annotations []*Annotation
	// Pub reports whether the trait is exported with `pub`.
	Pub bool
	// Name is the trait name.
	Name string
	// NamePos is the position of Name.
	NamePos Position
	// TypeParams are the generic parameters, empty for a non-generic trait.
	TypeParams []*TypeParam
	// Methods are the required and default methods in source order.
	Methods []*FnDecl
	// KwPos is the position of the `trait` keyword.
	KwPos Position
}

// ImplDecl is `impl Point {}`, `impl Printable for Point {}` or
// `impl[T] Stack[T] {}`.
// Trait is nil for an inherent impl.
type ImplDecl struct {
	BaseNode
	// Annotations are the annotations preceding the declaration (D21).
	Annotations []*Annotation
	// TypeParams are the generic parameters, empty for a non-generic impl.
	TypeParams []*TypeParam
	// Trait is the implemented trait, nil for an inherent impl.
	Trait Type
	// Type is the type the impl block targets.
	Type Type
	// Methods are the implemented methods in source order.
	Methods []*FnDecl
	// KwPos is the position of the `impl` keyword.
	KwPos Position
	// ForPos is the position of the `for` keyword; valid only when
	// Trait != nil.
	ForPos Position
}

// ImportName is one name in a `from ... import a, b` list.
type ImportName struct {
	// Name is the imported name.
	Name string
	// NamePos is the position of Name.
	NamePos Position
	// Alias is the local alias, "" when not aliased.
	Alias string
	// AliasPos is the position of Alias; valid only when Alias != "".
	AliasPos Position
}

// ImportDecl covers all three forms:
//
//	import a.b            -> From=false, Path=["a","b"]
//	import a.b as c       -> From=false, Alias="c"
//	from a.b import x, y  -> From=true,  Names=[x, y]
type ImportDecl struct {
	BaseNode
	// From reports whether the declaration uses the `from ... import` form.
	From bool
	// Path is the dotted module path, split into segments.
	Path []string
	// PathPos holds one position per Path segment.
	PathPos []Position
	// Alias is the module alias, "" when not aliased.
	Alias string
	// AliasPos is the position of Alias; valid only when Alias != "".
	AliasPos Position
	// Names are the imported names, non-empty only when From is true.
	Names []ImportName
	// KwPos is the position of the leading `import` or `from` keyword.
	KwPos Position
}

// PathString returns the dotted module path, e.g. "net.http".
func (d *ImportDecl) PathString() string {
	return strings.Join(d.Path, ".")
}

func (*FnDecl) stmtNode()     {}
func (*StructDecl) stmtNode() {}
func (*EnumDecl) stmtNode()   {}
func (*TraitDecl) stmtNode()  {}
func (*ImplDecl) stmtNode()   {}
func (*ImportDecl) stmtNode() {}

func (*FnDecl) declNode()     {}
func (*StructDecl) declNode() {}
func (*EnumDecl) declNode()   {}
func (*TraitDecl) declNode()  {}
func (*ImplDecl) declNode()   {}
func (*ImportDecl) declNode() {}
