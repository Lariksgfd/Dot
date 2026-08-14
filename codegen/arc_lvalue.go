package codegen

import "github.com/dotlang/dot/ast"

func isLValue(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return true
	case *ast.FieldExpr:
		return true
	case *ast.IndexExpr:
		return true
	case *ast.ParenExpr:
		return isLValue(x.X)
	}
	return false
}