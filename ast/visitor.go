package ast

import "fmt"

// Visitor is the interface for AST visitors (D23).
//
// Visit is called for every node during a Walk. Returning a non-nil Visitor
// continues descent into that node's children; returning nil skips them.
// After a node's children are visited, Walk calls v.Visit(nil) once so the
// visitor can detect "leave" events.
type Visitor interface {
	Visit(node Node) Visitor
}

// inspector adapts a closure to the Visitor interface for Inspect.
type inspector func(Node) bool

func (f inspector) Visit(n Node) Visitor {
	if f(n) {
		return f
	}
	return nil
}

// Inspect traverses n in depth-first order, calling f for each node.
// When f returns false the children of that node are skipped.
func Inspect(n Node, f func(Node) bool) {
	Walk(inspector(f), n)
}

// Walk traverses n in depth-first source order, calling v.Visit for each node.
// It panics on an unknown node type so an unhandled case can never be silently
// skipped (D23).
func Walk(v Visitor, n Node) {
	if isNilNode(n) {
		return
	}
	if v = v.Visit(n); v == nil {
		return
	}
	if walkExpr(v, n) {
		return
	}
	if walkStmt(v, n) {
		return
	}
	if walkType(v, n) {
		return
	}
	if walkPattern(v, n) {
		return
	}
	panic(fmt.Sprintf("ast.Walk: unhandled node type %T", n))
}

// walkNode visits a single child node if it is non-nil.
func walkNode(v Visitor, n Node) {
	if !isNilNode(n) {
		Walk(v, n)
	}
}

// walkExprList visits each expression in list, in source order.
func walkExprList(v Visitor, list []Expr) {
	for _, e := range list {
		Walk(v, e)
	}
}

// walkStmtList visits each statement in list, in source order.
func walkStmtList(v Visitor, list []Stmt) {
	for _, s := range list {
		Walk(v, s)
	}
}

// walkTypeList visits each type in list, in source order.
func walkTypeList(v Visitor, list []Type) {
	for _, t := range list {
		Walk(v, t)
	}
}

// walkPatternList visits each pattern in list, in source order.
func walkPatternList(v Visitor, list []Pattern) {
	for _, p := range list {
		Walk(v, p)
	}
}
