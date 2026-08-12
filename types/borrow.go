package types

import (
	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/errors"
)

// Borrow tracks the ownership state of one variable (D55).
type Borrow struct {
	// Name is the variable's name.
	Name string
	// DeclaredAt is the position of the declaring identifier.
	DeclaredAt ast.Position
	// LastUsedAt is the position of the most recent read; zero when never read.
	LastUsedAt ast.Position
	// IsMut is true when the variable is written after declaration.
	IsMut bool
	// MovedAt is the position where the variable's value was moved; zero when
	// it has not been moved.
	MovedAt ast.Position
	// MovedTo describes the move target (e.g. "return", "call argument").
	MovedTo string
}

// moved reports whether the variable has been moved.
func (b *Borrow) moved() bool {
	return b.MovedAt != (ast.Position{})
}

// usedAfterMove reports whether the variable was read after its value was
// moved.
func (b *Borrow) usedAfterMove() bool {
	return b.moved() && b.LastUsedAt != (ast.Position{}) &&
		b.LastUsedAt.Offset > b.MovedAt.Offset
}

// borrowVisitor walks the AST and records the ownership state of every
// variable it sees.
type borrowVisitor struct {
	borrows  map[string]*Borrow
	insideFn bool
	errs     []*errors.Diagnostic
}

// newBorrowVisitor allocates a borrowVisitor.
func newBorrowVisitor() *borrowVisitor {
	return &borrowVisitor{borrows: make(map[string]*Borrow)}
}

// recordDecl records (or looks up) the borrow entry for name at pos.
func (v *borrowVisitor) recordDecl(name string, pos ast.Position) *Borrow {
	if b, ok := v.borrows[name]; ok {
		return b
	}
	b := &Borrow{Name: name, DeclaredAt: pos}
	v.borrows[name] = b
	return b
}

// --- visitor ---------------------------------------------------------------

func (v *borrowVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *ast.FnDecl:
		return &borrowVisitor{
			borrows:  v.borrows,
			insideFn: true,
			errs:     v.errs,
		}
	case *ast.BlockStmt:
		if !v.insideFn {
			return &borrowVisitor{
				borrows: v.borrows,
				errs:    v.errs,
			}
		}
	case *ast.VarDecl:
		v.visitVarDecl(n)
	case *ast.AssignExpr:
		v.visitAssign(n)
	case *ast.ReturnStmt:
		v.visitReturn(n)
	case *ast.Ident:
		v.visitIdent(n)
	}
	return v
}

// visitVarDecl records each declared name and marks reassignments as mut.
func (v *borrowVisitor) visitVarDecl(n *ast.VarDecl) {
	for _, nameExpr := range n.Names {
		if id, ok := nameExpr.(*ast.Ident); ok {
			if b, ok := v.borrows[id.Name]; ok {
				b.IsMut = true
			}
			v.recordDecl(id.Name, id.Pos())
		}
	}
}

// visitIdent records a read of the named variable.
func (v *borrowVisitor) visitIdent(id *ast.Ident) {
	if b, ok := v.borrows[id.Name]; ok {
		b.LastUsedAt = id.Pos()
	}
}

// visitAssign records moves (y = x moves x into y) and mutations.
func (v *borrowVisitor) visitAssign(a *ast.AssignExpr) {
	for i, tgt := range a.Targets {
		if tid, ok := tgt.(*ast.Ident); ok {
			if b, ok := v.borrows[tid.Name]; ok {
				b.IsMut = true
				if i < len(a.Values) {
					b.MovedAt = a.OpPos
					b.MovedTo = "assignment"
				}
			}
		}
	}
	for _, val := range a.Values {
		if vid, ok := val.(*ast.Ident); ok {
			if b, ok := v.borrows[vid.Name]; ok {
				b.MovedAt = a.OpPos
				b.MovedTo = "assignment"
			}
		}
	}
}

// visitReturn records a move of every returned variable.
func (v *borrowVisitor) visitReturn(r *ast.ReturnStmt) {
	for _, val := range r.Values {
		if id, ok := val.(*ast.Ident); ok {
			if b, ok := v.borrows[id.Name]; ok {
				b.MovedAt = r.KwPos
				b.MovedTo = "return"
			}
		}
	}
}

// --- analysis entry point --------------------------------------------------

// AnalyzeBorrows walks the AST and records the ownership state of every
// variable.  When info is non-nil, use-after-move errors are appended to
// info.Diagnostics.
func AnalyzeBorrows(prog *ast.Program, info *Info) []*Borrow {
	if prog == nil {
		return nil
	}
	v := newBorrowVisitor()
	ast.Walk(v, prog)

	out := make([]*Borrow, 0, len(v.borrows))
	for _, b := range v.borrows {
		out = append(out, b)
		if info != nil && b.usedAfterMove() {
			d := errors.NewTypeError(
				b.LastUsedAt.File, b.LastUsedAt.Line, b.LastUsedAt.Column,
				b.LastUsedAt.Offset, len(b.Name),
				"variable "+b.Name+" used after move",
			)
			info.Diagnostics.Add(d)
		}
	}
	return out
}
