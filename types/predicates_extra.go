package types

import "github.com/dotlang/dot/ast"

// IsCopy is the negation used by the checker for move/copy diagnostics.
func IsCopy(t Type) bool {
	return !IsHeap(t)
}

// Deref returns the element type of a pointer or the type itself otherwise.
// It is the type-system face of pointer dereference.
func Deref(t Type) Type {
	if t == nil {
		return Invalid
	}
	switch x := t.(type) {
	case *Pointer:
		return x.Elem
	case *Weak:
		return x.Elem
	default:
		return t
	}
}

// structuralKey is a helper the checker may use to compare two structurally
// identical types that do not share a canonical pointer. It is a best-effort
// string key; two types with the same key are very likely identical.
func structuralKey(t Type) string {
	if t == nil {
		return ""
	}
	return t.String()
}

// ensure ast is referenced — the composite types above use ast.Position and
// ast.Expr through their field types, so this import is live. This var guards
// against the import being dropped by a future edit that removes the last
// direct ast reference.
var _ ast.Node = nil
