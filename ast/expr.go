package ast

import "github.com/dotlang/dot/lexer"

// ParenExpr is a parenthesised expression `(x)`.
type ParenExpr struct {
	BaseNode
	// X is the parenthesised expression.
	X Expr
	// LParen is the position of '('.
	LParen Position
	// RParen is the position of ')'.
	RParen Position
}

// UnaryExpr is `-x`, `not x`, `~x`.
type UnaryExpr struct {
	BaseNode
	// Op is TokenMinus, TokenNot, TokenTilde or TokenPlus.
	Op lexer.TokenType
	// OpPos is the position of the operator.
	OpPos Position
	// X is the operand.
	X Expr
}

// OpString returns the source spelling of the operator.
func (u *UnaryExpr) OpString() string { return u.Op.Literal() }

// BinaryExpr covers arithmetic, comparison, bitwise and and/or.
type BinaryExpr struct {
	BaseNode
	// Op is the operator token type.
	Op lexer.TokenType
	// OpPos is the position of the operator.
	OpPos Position
	// X is the left operand.
	X Expr
	// Y is the right operand.
	Y Expr
}

// OpString returns the source spelling of the operator.
func (b *BinaryExpr) OpString() string { return b.Op.Literal() }

// AssignExpr covers `x = 1`, `x += 1` and `a, b = 1, 2` (D11).
// Op is TokenAssign or one of the compound assignment tokens; compound ops
// always have len(Targets) == len(Values) == 1.
type AssignExpr struct {
	BaseNode
	// Op is TokenAssign or a compound assignment token.
	Op lexer.TokenType
	// OpPos is the position of the operator.
	OpPos Position
	// Targets are the assignment targets: *Ident, *UnderscoreExpr,
	// *FieldExpr or *IndexExpr.
	Targets []Expr
	// Values are the assigned values.
	Values []Expr
}

// OpString returns the source spelling of the operator.
func (a *AssignExpr) OpString() string { return a.Op.Literal() }

// RangeExpr is `0..10` / `0..=10`. Low or High may be nil (`..n`, `n..`).
type RangeExpr struct {
	BaseNode
	// Low is the lower bound, nil for an open low bound.
	Low Expr
	// High is the upper bound, nil for an open high bound.
	High Expr
	// Inclusive is true for the `..=` form.
	Inclusive bool
	// OpPos is the position of `..` or `..=`.
	OpPos Position
}

// Arg is one call argument. Name != "" for `port: 3000`.
// Spread is true for `f(xs...)`.
type Arg struct {
	// Name is the argument label, "" for a positional argument.
	Name string
	// NamePos is the position of Name; valid only when Name != "".
	NamePos Position
	// Value is the argument expression.
	Value Expr
	// Spread reports whether the argument was written `xs...`.
	Spread bool
}

// CallExpr is `f(1, 2)`. Method calls are a CallExpr whose Fn is a
// *FieldExpr (D12). Generic instantiation is an *IndexExpr on Fn (D13).
type CallExpr struct {
	BaseNode
	// Fn is the callee expression.
	Fn Expr
	// Args are the arguments in source order.
	Args []Arg
	// LParen is the position of '('.
	LParen Position
	// RParen is the position of ')'.
	RParen Position
}

// IndexExpr is `a[i]` and also generic instantiation `Stack[int]`,
// `Result[int, Error]` (D13) — hence a slice of indices.
type IndexExpr struct {
	BaseNode
	// X is the indexed expression.
	X Expr
	// Indices are the index expressions, at least one.
	Indices []Expr
	// LBracket is the position of '['.
	LBracket Position
	// RBracket is the position of ']'.
	RBracket Position
}

// SliceExpr is `items[1..3]`, `items[..3]`, `items[1..]`, `items[1..=3]`.
type SliceExpr struct {
	BaseNode
	// X is the sliced expression.
	X Expr
	// Low is the lower bound, nil for an open low bound.
	Low Expr
	// High is the upper bound, nil for an open high bound.
	High Expr
	// Inclusive is true for the `..=` form.
	Inclusive bool
	// LBracket is the position of '['.
	LBracket Position
	// RBracket is the position of ']'.
	RBracket Position
}

// FieldExpr is `a.b` and tuple indexing `t.0`.
// For a tuple index, Name is the digit text and Index holds its value.
type FieldExpr struct {
	BaseNode
	// X is the receiver expression.
	X Expr
	// Name is the field name, or the digit text for a tuple index.
	Name string
	// NamePos is the position of Name.
	NamePos Position
	// Index is the tuple index value; meaningful when IsTupleIndex is true.
	Index int
	// IsTupleIndex reports whether the selector is a tuple index.
	IsTupleIndex bool
}

// PipeExpr is `x |> f(y)`; Phase 4 rewrites it to f(x, y) (D17).
type PipeExpr struct {
	BaseNode
	// X is the piped value.
	X Expr
	// Fn is the right operand, whose call gains X as its first argument.
	Fn Expr
	// OpPos is the position of `|>`.
	OpPos Position
}

// TryExpr is `expr?`.
type TryExpr struct {
	BaseNode
	// X is the operand.
	X Expr
	// OpPos is the position of '?'.
	OpPos Position
}

// AwaitExpr is `await expr`.
type AwaitExpr struct {
	BaseNode
	// X is the awaited expression.
	X Expr
	// KwPos is the position of the `await` keyword.
	KwPos Position
}

// SpawnExpr is `spawn { ... }` or `spawn thread { ... }`; its value is a task handle.
type SpawnExpr struct {
	BaseNode
	// Block is the spawned block.
	Block *BlockStmt
	// KwPos is the position of the `spawn` keyword.
	KwPos Position
	// IsThread is true for OS thread spawn (`spawn thread { ... }`).
	IsThread bool
}

// CastExpr is `x as int`.
type CastExpr struct {
	BaseNode
	// X is the value being cast.
	X Expr
	// Type is the target type.
	Type Type
	// KwPos is the position of the `as` keyword.
	KwPos Position
}

// IsExpr is `x is Printable`.
type IsExpr struct {
	BaseNode
	// X is the value being tested.
	X Expr
	// Type is the tested type.
	Type Type
	// KwPos is the position of the `is` keyword.
	KwPos Position
}

// IfExpr models every `if`: expression, statement, `else if` chains and the
// if-let form `if user = find(42) {` (Bind != nil). See D14.
// Exactly one of Else / ElseIf is non-nil, or both are nil.
type IfExpr struct {
	BaseNode
	// Bind is nil for a plain condition; set for the if-let form.
	Bind Pattern
	// Cond is the condition, or the scrutinee when Bind != nil.
	Cond Expr
	// Then is the consequent block.
	Then *BlockStmt
	// Else is the `else` block, nil when absent or when ElseIf is set.
	Else *BlockStmt
	// ElseIf is the `else if` continuation, nil when absent.
	ElseIf *IfExpr
}

// MatchArm is `pattern => body`. Body is a *BlockExpr for a braced arm.
type MatchArm struct {
	BaseNode
	// Pattern is the arm's pattern.
	Pattern Pattern
	// Body is the arm's body expression.
	Body Expr
	// ArrowPos is the position of `=>`.
	ArrowPos Position
}

// MatchExpr is `match subject { arm, ... }`.
type MatchExpr struct {
	BaseNode
	// Subject is the matched expression.
	Subject Expr
	// Arms are the match arms in source order.
	Arms []*MatchArm
	// KwPos is the position of the `match` keyword.
	KwPos Position
	// LBrace is the position of '{'.
	LBrace Position
	// RBrace is the position of '}'.
	RBrace Position
}

// BlockExpr adapts a block to expression position (match arms, lambda
// bodies, spawn results). Its value is the value of the last ExprStmt (D18).
type BlockExpr struct {
	BaseNode
	// Block is the wrapped block.
	Block *BlockStmt
}

// BadExpr is a placeholder produced by parser error recovery.
type BadExpr struct {
	BaseNode
}

func (*ParenExpr) exprNode()  {}
func (*UnaryExpr) exprNode()  {}
func (*BinaryExpr) exprNode() {}
func (*AssignExpr) exprNode() {}
func (*RangeExpr) exprNode()  {}
func (*CallExpr) exprNode()   {}
func (*IndexExpr) exprNode()  {}
func (*SliceExpr) exprNode()  {}
func (*FieldExpr) exprNode()  {}
func (*PipeExpr) exprNode()   {}
func (*TryExpr) exprNode()    {}
func (*AwaitExpr) exprNode()  {}
func (*SpawnExpr) exprNode()  {}
func (*CastExpr) exprNode()   {}
func (*IsExpr) exprNode()     {}
func (*IfExpr) exprNode()     {}
func (*MatchExpr) exprNode()  {}
func (*BlockExpr) exprNode()  {}
func (*BadExpr) exprNode()    {}
