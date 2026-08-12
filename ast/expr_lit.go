package ast

// IntLit holds the unsigned magnitude; a leading `-` is a UnaryExpr.
type IntLit struct {
	BaseNode
	// Value is the magnitude of the literal.
	Value uint64
	// Base is the numeric base of the source spelling: 10, 16, 2 or 8.
	Base int
	// Suffix is the unit suffix, e.g. "mb" in 1_mb ("" if none).
	Suffix string
	// Raw is the original lexeme, separators included.
	Raw string
}

// FloatLit is a floating point literal such as `3.14`, `1e-9` or `2.5s`.
type FloatLit struct {
	BaseNode
	// Value is the decoded value of the literal.
	Value float64
	// Suffix is the unit suffix ("" if none).
	Suffix string
	// Raw is the original lexeme, separators included.
	Raw string
}

// StringPartKind distinguishes the two kinds of chunk in a string literal.
type StringPartKind int

const (
	// PartText is a literal chunk whose escapes have already been decoded.
	PartText StringPartKind = iota
	// PartExpr is a parsed interpolation expression.
	PartExpr
)

// String returns the name of the kind ("PartText" or "PartExpr").
func (k StringPartKind) String() string {
	switch k {
	case PartText:
		return "PartText"
	case PartExpr:
		return "PartExpr"
	default:
		return "StringPartKind(?)"
	}
}

// StringPart is one chunk of a string literal. Exactly one of Text/Expr is
// meaningful, selected by Kind. See D10.
type StringPart struct {
	// Kind selects which of Text and Expr is meaningful.
	Kind StringPartKind
	// Text is the decoded literal text, valid when Kind == PartText.
	Text string
	// Expr is the parsed interpolation, valid and never nil when
	// Kind == PartExpr.
	Expr Expr
	// Pos is the absolute position of this chunk in the source file.
	Pos Position
}

// StringLit is `"hello {name}"`. A literal with no interpolation has exactly
// one PartText part (or zero parts when empty).
type StringLit struct {
	BaseNode
	// Parts are the chunks of the literal in source order.
	Parts []StringPart
	// Raw is the original lexeme including the quotes.
	Raw string
}

// RawStringLit is a backtick string: no escapes, no interpolation.
type RawStringLit struct {
	BaseNode
	// Value is the literal text between the backticks.
	Value string
}

// BoolLit is `true` or `false`.
type BoolLit struct {
	BaseNode
	// Value is the boolean value of the literal.
	Value bool
}

// NilLit is the `nil` literal.
type NilLit struct {
	BaseNode
}

// Ident is an identifier used in expression position.
type Ident struct {
	BaseNode
	// Name is the identifier text.
	Name string
}

// UnderscoreExpr is `_` used as a discard target.
type UnderscoreExpr struct {
	BaseNode
}

// SelfExpr is the `self` receiver value.
type SelfExpr struct {
	BaseNode
}

// TupleElem is one element of a tuple literal. Name != "" for named tuples.
type TupleElem struct {
	// Name is the element label, "" for a positional element.
	Name string
	// NamePos is the position of Name; valid only when Name != "".
	NamePos Position
	// Value is the element expression.
	Value Expr
}

// TupleLit is `(10, 20)` or `(x: 10, y: 20)`. A one-element parenthesised
// expression is a ParenExpr, never a TupleLit.
type TupleLit struct {
	BaseNode
	// Elems are the elements in source order.
	Elems []TupleElem
	// LParen is the position of '('.
	LParen Position
	// RParen is the position of ')'.
	RParen Position
}

// ArrayLit is `[]int{1, 2, 3}`, `[1, 2, 3]` or `[5]int{...}`.
// Type is nil when the literal is written without one.
type ArrayLit struct {
	BaseNode
	// Type is a *SliceType or *ArrayType, or nil when the literal is bare.
	Type Type
	// Elems are the elements in source order.
	Elems []Expr
}

// MapEntry is one `key: value` pair of a map literal.
type MapEntry struct {
	// Key is the key expression.
	Key Expr
	// Value is the value expression.
	Value Expr
}

// MapLit is `map[string]int{"a": 1}`.
type MapLit struct {
	BaseNode
	// Type is a *MapType, or nil for a bare `{}` literal.
	Type Type
	// Entries are the entries in source order.
	Entries []MapEntry
}

// StructLitField is one field of a struct literal. Shorthand is true for the
// `Point{x, y}` form, where Value is the *Ident with the same name.
type StructLitField struct {
	// Name is the field name.
	Name string
	// NamePos is the position of Name.
	NamePos Position
	// Value is the field value.
	Value Expr
	// Shorthand reports whether the field was written in the `{x}` form.
	Shorthand bool
}

// StructLit is `Point { x: 1.0, y: 2.0 }`.
type StructLit struct {
	BaseNode
	// Type is the struct type, a *NamedType or *GenericType.
	Type Type
	// Fields are the fields in source order.
	Fields []StructLitField
}

// FnLit is a lambda: `fn(x int) -> int { x * 2 }`.
// Exactly one of Body / ExprBody is non-nil.
type FnLit struct {
	BaseNode
	// Sig is the lambda's signature.
	Sig *FnSig
	// Body is the block body, nil for the short form.
	Body *BlockStmt
	// ExprBody is the body of the short form `fn(x) = x * 2`, else nil.
	ExprBody Expr
}

func (*IntLit) exprNode()         {}
func (*FloatLit) exprNode()       {}
func (*StringLit) exprNode()      {}
func (*RawStringLit) exprNode()   {}
func (*BoolLit) exprNode()        {}
func (*NilLit) exprNode()         {}
func (*Ident) exprNode()          {}
func (*UnderscoreExpr) exprNode() {}
func (*SelfExpr) exprNode()       {}
func (*TupleLit) exprNode()       {}
func (*ArrayLit) exprNode()       {}
func (*MapLit) exprNode()         {}
func (*StructLit) exprNode()      {}
func (*FnLit) exprNode()          {}
