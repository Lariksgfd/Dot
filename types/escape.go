package types

import "github.com/dotlang/dot/ast"

// Escape indicates whether an expression is stack or heap allocated.
type Escape int

const (
	Stack Escape = iota
	Heap
)

type escapeVisitor struct {
	info     *Info
	isGlobal bool
	escaping bool
}

func (v *escapeVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	isExpr := false
	switch node.(type) {
	case ast.Expr:
		isExpr = true
	}

	if isExpr {
		if v.escaping {
			v.info.Escapes[node.(ast.Expr)] = Heap
		} else {
			v.info.Escapes[node.(ast.Expr)] = Stack
		}
	}

	switch node.(type) {
	case *ast.FnDecl:
		// Inside a function, we are no longer at global scope.
		return &escapeVisitor{
			info:     v.info,
			isGlobal: false,
			escaping: false,
		}
	case *ast.VarDecl:
		// If it's a global variable declaration, its values escape.
		if v.isGlobal {
			return &escapeVisitor{
				info:     v.info,
				isGlobal: v.isGlobal,
				escaping: true, // assigned to a global
			}
		}
	case *ast.ReturnStmt:
		// Return statements cause expressions to escape.
		return &escapeVisitor{
			info:     v.info,
			isGlobal: v.isGlobal,
			escaping: true,
		}
	}

	// For all other nodes, propagate the current escaping state.
	return v
}

// AnalyzeEscapes walks the AST and populates the Escapes map in Info.
func AnalyzeEscapes(prog *ast.Program, info *Info) {
	if info.Escapes == nil {
		info.Escapes = make(map[ast.Expr]Escape)
	}

	v := &escapeVisitor{
		info:     info,
		isGlobal: true, // we start at the package level
		escaping: false,
	}
	ast.Walk(v, prog)
}
