// Package ast defines the abstract syntax tree produced by the Dot parser.
//
// Every node is used as a pointer (*IntLit, *FnDecl, ...) because BaseNode's
// Pos/End methods have pointer receivers. The package imports only lexer
// (for Position and TokenType) and the standard library; lexer never imports
// ast, so there is no cycle. See Design Decision D8.
package ast

import (
	"reflect"

	"github.com/dotlang/dot/lexer"
)

// Position is an alias (not a defined type) so ast and lexer positions are
// interchangeable without conversion. See Design Decision D9.
type Position = lexer.Position

// Node is the interface implemented by every AST node.
type Node interface {
	// Pos returns the position of the first character of the node.
	Pos() Position
	// End returns the position of the first character after the node.
	End() Position
	// SetDoc attaches preceding doc comments.
	SetDoc([]lexer.Token)
	// SetComment attaches end-of-line comments.
	SetComment([]lexer.Token)
	// GetDoc returns attached doc comments.
	GetDoc() []lexer.Token
	// GetComment returns attached end-of-line comments.
	GetComment() []lexer.Token
}

// Expr is a node that produces a value. Anything value-producing is an Expr;
// statement usage always goes through ExprStmt (D14).
type Expr interface {
	Node
	exprNode()
}

// Stmt is a node that may appear in a statement list.
type Stmt interface {
	Node
	stmtNode()
}

// Decl is a top-level declaration. Every Decl is also a Stmt so declarations
// may appear inside blocks (nested const/var today, nested fn later).
type Decl interface {
	Stmt
	declNode()
}

// Type is a node that denotes a type in type position.
type Type interface {
	Node
	typeNode()
}

// Pattern is a node that may appear in match arms and destructuring binds.
type Pattern interface {
	Node
	patternNode()
}

// BaseNode is embedded by every node and supplies Pos/End.
type BaseNode struct {
	// Start is the position of the first character of the node.
	Start Position
	// Stop is the position of the first character after the node.
	Stop Position
	// Doc contains comments appearing before the node (only used on statements/declarations).
	Doc []lexer.Token
	// Comment contains line comments appearing at the end of the node's line.
	Comment []lexer.Token
}

// Pos returns the position of the first character of the node.
func (b *BaseNode) Pos() Position { return b.Start }

// End returns the position of the first character after the node.
func (b *BaseNode) End() Position { return b.Stop }

// SetSpan overwrites the node's start and end positions. The parser uses it
// to widen a node's span once its children have been parsed.
func (b *BaseNode) SetSpan(start, stop Position) { b.Start, b.Stop = start, stop }

// SetDoc attaches preceding doc comments.
func (b *BaseNode) SetDoc(doc []lexer.Token) { b.Doc = doc }

// SetComment attaches end-of-line comments.
func (b *BaseNode) SetComment(comment []lexer.Token) { b.Comment = comment }

// GetDoc returns attached doc comments.
func (b *BaseNode) GetDoc() []lexer.Token { return b.Doc }

// GetComment returns attached end-of-line comments.
func (b *BaseNode) GetComment() []lexer.Token { return b.Comment }

// Span builds a BaseNode from two positions.
func Span(start, stop Position) BaseNode {
	return BaseNode{Start: start, Stop: stop}
}

// SpanOf builds a BaseNode covering from.Pos() .. to.End().
// Either argument may be nil; a nil argument contributes nothing, so
// SpanOf(nil, n) spans only n and SpanOf(nil, nil) is the zero BaseNode.
func SpanOf(from, to Node) BaseNode {
	var b BaseNode
	if !isNilNode(from) {
		b.Start = from.Pos()
	}
	if !isNilNode(to) {
		b.Stop = to.End()
		if isNilNode(from) {
			b.Start = to.Pos()
		}
	} else if !isNilNode(from) {
		b.Stop = from.End()
	}
	return b
}

// SpanTok builds a BaseNode from a single token's Pos/End.
func SpanTok(tok lexer.Token) BaseNode {
	return BaseNode{Start: tok.Pos, Stop: tok.End}
}

// NodeName returns the Go type name of n without the package qualifier or
// leading '*' ("BinaryExpr", "FnDecl"). It returns "nil" for a nil node.
// Used by the printer and by error messages.
func NodeName(n Node) string {
	if isNilNode(n) {
		return "nil"
	}
	t := reflect.TypeOf(n)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// isNilNode reports whether n is nil, including a typed nil pointer stored in
// a non-nil interface value (for example Expr(( *Ident)(nil))).
func isNilNode(n Node) bool {
	if n == nil {
		return true
	}
	v := reflect.ValueOf(n)
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func:
		return v.IsNil()
	}
	return false
}
