package ast

// LiteralPattern is `0`, `"ok"`, `true`, `nil`, `-1`.
// Value is an *IntLit, *FloatLit, *StringLit, *RawStringLit, *BoolLit,
// *NilLit or a *UnaryExpr wrapping one of those.
type LiteralPattern struct {
	BaseNode
	// Value is the literal expression the pattern matches.
	Value Expr
}

// IdentPattern binds a value to a name.
type IdentPattern struct {
	BaseNode
	// Name is the bound name.
	Name string
	// Mut reports whether the binding was written `mut name`.
	Mut bool
}

// WildcardPattern is `_`.
type WildcardPattern struct {
	BaseNode
}

// OrPattern is `1 | 2 | 3`. len(Alts) >= 2.
type OrPattern struct {
	BaseNode
	// Alts are the alternatives in source order.
	Alts []Pattern
}

// GuardedPattern is `n if n > 100`. Only ever appears at the top level of a
// match arm.
type GuardedPattern struct {
	BaseNode
	// Pattern is the guarded pattern.
	Pattern Pattern
	// Guard is the boolean guard expression.
	Guard Expr
	// KwPos is the position of the `if` keyword.
	KwPos Position
}

// TypePattern is `int n`, `string s`, `[]int arr`. Binding may be nil.
type TypePattern struct {
	BaseNode
	// Type is the matched type.
	Type Type
	// Binding is the optional name bound to the value, nil when absent.
	Binding *IdentPattern
}

// EnumPattern is `Shape.Circle(r)`, `Color.Red`, `Ok(n)`, `None`.
// Enum is "" for an unqualified variant. HasArgs distinguishes `Red` from
// `Red()`.
type EnumPattern struct {
	BaseNode
	// Enum is the enum name, "" for an unqualified variant.
	Enum string
	// EnumPos is the position of Enum; valid only when Enum != "".
	EnumPos Position
	// Variant is the variant name.
	Variant string
	// VariantPos is the position of Variant.
	VariantPos Position
	// HasArgs reports whether the variant was written with parentheses.
	HasArgs bool
	// Args are the sub-patterns of the variant's fields.
	Args []Pattern
}

// StructPatternField is one field of a struct pattern. Shorthand is true for
// the `Point{x, y}` form, where Pattern is an *IdentPattern of the same name.
type StructPatternField struct {
	// Name is the field name.
	Name string
	// NamePos is the position of Name.
	NamePos Position
	// Pattern is the sub-pattern matched against the field.
	Pattern Pattern
	// Shorthand reports whether the field was written in the `{x}` form.
	Shorthand bool
}

// StructPattern is `Point { x: 0, y }`.
type StructPattern struct {
	BaseNode
	// Type is the struct type, a *NamedType or *GenericType.
	Type Type
	// Fields are the matched fields in source order.
	Fields []StructPatternField
}

// TuplePattern is `(x, y)`.
type TuplePattern struct {
	BaseNode
	// Elems are the element patterns in source order.
	Elems []Pattern
}

// RangePattern is `1..5` / `1..=5`. Low or High may be nil.
type RangePattern struct {
	BaseNode
	// Low is the lower bound, nil for an open low bound.
	Low Expr
	// High is the upper bound, nil for an open high bound.
	High Expr
	// Inclusive is true for the `..=` form.
	Inclusive bool
}

// BadPattern is a placeholder produced by parser error recovery.
type BadPattern struct {
	BaseNode
}

func (*LiteralPattern) patternNode()  {}
func (*IdentPattern) patternNode()    {}
func (*WildcardPattern) patternNode() {}
func (*OrPattern) patternNode()       {}
func (*GuardedPattern) patternNode()  {}
func (*TypePattern) patternNode()     {}
func (*EnumPattern) patternNode()     {}
func (*StructPattern) patternNode()   {}
func (*TuplePattern) patternNode()    {}
func (*RangePattern) patternNode()    {}
func (*BadPattern) patternNode()      {}
