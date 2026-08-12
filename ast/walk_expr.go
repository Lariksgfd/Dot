package ast

// walkExpr visits the children of an expression node in source order.
// It returns true when n was an expression and its children were handled;
// returning false signals Walk to try the next node category.
func walkExpr(v Visitor, n Node) bool {
	switch e := n.(type) {
	case *ParenExpr:
		Walk(v, e.X)
	case *UnaryExpr:
		Walk(v, e.X)
	case *BinaryExpr:
		Walk(v, e.X)
		Walk(v, e.Y)
	case *AssignExpr:
		walkExprList(v, e.Targets)
		walkExprList(v, e.Values)
	case *RangeExpr:
		walkNode(v, e.Low)
		walkNode(v, e.High)
	case *CallExpr:
		Walk(v, e.Fn)
		for i := range e.Args {
			Walk(v, e.Args[i].Value)
		}
	case *IndexExpr:
		Walk(v, e.X)
		walkExprList(v, e.Indices)
	case *SliceExpr:
		Walk(v, e.X)
		walkNode(v, e.Low)
		walkNode(v, e.High)
	case *FieldExpr:
		Walk(v, e.X)
	case *PipeExpr:
		Walk(v, e.X)
		Walk(v, e.Fn)
	case *TryExpr:
		Walk(v, e.X)
	case *AwaitExpr:
		Walk(v, e.X)
	case *SpawnExpr:
		walkNode(v, e.Block)
	case *CastExpr:
		Walk(v, e.X)
		walkNode(v, e.Type)
	case *IsExpr:
		Walk(v, e.X)
		walkNode(v, e.Type)
	case *IfExpr:
		walkPattern(v, e.Bind)
		walkNode(v, e.Cond)
		walkNode(v, e.Then)
		walkNode(v, e.Else)
		walkNode(v, e.ElseIf)
	case *MatchExpr:
		Walk(v, e.Subject)
		for i := range e.Arms {
			Walk(v, e.Arms[i])
		}
	case *BlockExpr:
		walkNode(v, e.Block)
	case *IntLit, *FloatLit, *RawStringLit, *BoolLit, *NilLit, *Ident,
		*UnderscoreExpr, *SelfExpr, *BadExpr:
		// leaf expressions — no children
	case *StringLit:
		for i := range e.Parts {
			if e.Parts[i].Kind == PartExpr {
				Walk(v, e.Parts[i].Expr)
			}
		}
	case *TupleLit:
		for i := range e.Elems {
			Walk(v, e.Elems[i].Value)
		}
	case *ArrayLit:
		walkNode(v, e.Type)
		walkExprList(v, e.Elems)
	case *MapLit:
		walkNode(v, e.Type)
		for i := range e.Entries {
			Walk(v, e.Entries[i].Key)
			Walk(v, e.Entries[i].Value)
		}
	case *StructLit:
		walkNode(v, e.Type)
		for i := range e.Fields {
			Walk(v, e.Fields[i].Value)
		}
	case *FnLit:
		walkNode(v, e.Sig)
		walkNode(v, e.Body)
		walkNode(v, e.ExprBody)
	default:
		return false
	}
	return true
}
