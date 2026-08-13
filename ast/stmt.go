package ast

import "github.com/dotlang/dot/lexer"

// Program is one parsed .dot file (D19: top level holds Decls, and every
// legal top-level construct — including `x = 1` — implements Decl).
type Program struct {
	BaseNode
	// File is the source file name.
	File string
	// Decls are the top-level declarations in source order.
	Decls []Decl
	// LooseComments contains comments at the end of the file not attached to any decl.
	LooseComments []lexer.Token
}

// Imports returns every *ImportDecl in Decls, in source order.
func (p *Program) Imports() []*ImportDecl {
	var out []*ImportDecl
	for _, d := range p.Decls {
		if imp, ok := d.(*ImportDecl); ok {
			out = append(out, imp)
		}
	}
	return out
}

// BlockStmt is a brace-delimited statement list.
type BlockStmt struct {
	BaseNode
	// Stmts are the statements in source order.
	Stmts []Stmt
	// LooseComments contains comments inside the block not attached to any stmt.
	LooseComments []lexer.Token
	// LBrace is the position of '{'.
	LBrace Position
	// RBrace is the position of '}'.
	RBrace Position
}

// ExprStmt wraps an expression used as a statement. Statement-position `if`,
// `match` and `spawn` are ExprStmt-wrapped (D14).
type ExprStmt struct {
	BaseNode
	// X is the wrapped expression.
	X Expr
}

// VarDecl is `x = 42`, `count int = 0`, `const PI float = 3.14`,
// `a, b = 1, 2`, `_, err = f()`.
// Type is nil when inferred. len(Values) is 0 (declaration without value),
// 1 (single value or a multi-value call) or len(Names).
// Whether a VarDecl introduces a binding or reassigns one is decided in
// Phase 4, not by the parser (D11).
type VarDecl struct {
	BaseNode
	// Annotations are the annotations preceding the declaration (D21).
	Annotations []*Annotation
	// Const reports whether the declaration is a `const`.
	Const bool
	// Pub reports whether the declaration is exported with `pub`.
	Pub bool
	// Names are the declared names: *Ident or *UnderscoreExpr.
	Names []Expr
	// Type is the declared type, nil when inferred.
	Type Type
	// Values are the initialisers.
	Values []Expr
	// AssignPos is the position of '='; invalid when there is no initialiser.
	AssignPos Position
}

// ReturnStmt is `return`, `return x`, `return a / b, nil`.
type ReturnStmt struct {
	BaseNode
	// Values are the returned expressions, empty for a bare `return`.
	Values []Expr
	// KwPos is the position of the `return` keyword.
	KwPos Position
}

// ForKind discriminates the loop forms of ForStmt (D20).
type ForKind int

const (
	// ForInfinite is `for { }`.
	ForInfinite ForKind = iota
	// ForCond is `for x > 0 { }`.
	ForCond
	// ForIn is `for i in 0..10 { }` / `for k, v in m { }`.
	ForIn
)

// String returns the name of the kind, e.g. "ForIn".
func (k ForKind) String() string {
	switch k {
	case ForInfinite:
		return "ForInfinite"
	case ForCond:
		return "ForCond"
	case ForIn:
		return "ForIn"
	default:
		return "ForKind(?)"
	}
}

// ForStmt is the single loop node covering all five source forms (D20).
// Label is the `@outer` label text without the '@' ("" when unlabelled).
// For ForIn: Value is always set, Key is set only for the two-variable form.
// Whether Key is an index or a map key is decided in Phase 4.
type ForStmt struct {
	BaseNode
	// Label is the loop label without the '@', "" when unlabelled.
	Label string
	// LabelPos is the position of the label; valid only when Label != "".
	LabelPos Position
	// Kind selects which of the optional fields are meaningful.
	Kind ForKind
	// Key is the first loop variable: *Ident or *UnderscoreExpr,
	// nil unless the two-variable form is used.
	Key Expr
	// Value is the loop variable: *Ident or *UnderscoreExpr,
	// nil unless Kind == ForIn.
	Value Expr
	// Iterable is the iterated expression, nil unless Kind == ForIn.
	Iterable Expr
	// Cond is the loop condition, nil unless Kind == ForCond.
	Cond Expr
	// Body is the loop body.
	Body *BlockStmt
	// KwPos is the position of the `for` keyword.
	KwPos Position
}

// BreakStmt is `break` or `break @outer`.
type BreakStmt struct {
	BaseNode
	// Label is the target loop label without the '@', "" when unlabelled.
	Label string
	// LabelPos is the position of the label; valid only when Label != "".
	LabelPos Position
	// KwPos is the position of the `break` keyword.
	KwPos Position
}

// ContinueStmt is `continue` or `continue @outer`.
type ContinueStmt struct {
	BaseNode
	// Label is the target loop label without the '@', "" when unlabelled.
	Label string
	// LabelPos is the position of the label; valid only when Label != "".
	LabelPos Position
	// KwPos is the position of the `continue` keyword.
	KwPos Position
}

// DeferStmt is `defer file.close()`. Call must be a *CallExpr; the parser
// reports anything else.
type DeferStmt struct {
	BaseNode
	// Call is the deferred call expression.
	Call Expr
	// KwPos is the position of the `defer` keyword.
	KwPos Position
}

// PerfBlock is `@perf { ... }` — an arena-allocated block (SPEC §13).
type PerfBlock struct {
	BaseNode
	// Block is the arena-allocated block.
	Block *BlockStmt
	// AtPos is the position of '@'.
	AtPos Position
}

// BadStmt is a placeholder produced by parser error recovery.
type BadStmt struct {
	BaseNode
}

func (*BlockStmt) stmtNode()    {}
func (*ExprStmt) stmtNode()     {}
func (*VarDecl) stmtNode()      {}
func (*ReturnStmt) stmtNode()   {}
func (*ForStmt) stmtNode()      {}
func (*BreakStmt) stmtNode()    {}
func (*ContinueStmt) stmtNode() {}
func (*DeferStmt) stmtNode()    {}
func (*PerfBlock) stmtNode()    {}
func (*BadStmt) stmtNode()      {}

func (*VarDecl) declNode() {}
