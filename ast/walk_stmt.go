package ast

// walkStmt visits the children of a statement, declaration, or top-level
// helper node in source order. It returns true when n was handled; false
// signals Walk to try the next node category.
func walkStmt(v Visitor, n Node) bool {
	switch s := n.(type) {
	case *Program:
		for i := range s.Decls {
			Walk(v, s.Decls[i])
		}
	case *BlockStmt:
		walkStmtList(v, s.Stmts)
	case *ExprStmt:
		Walk(v, s.X)
	case *ReturnStmt:
		walkExprList(v, s.Values)
	case *BreakStmt, *ContinueStmt, *BadStmt:
		// leaf statements — no children
	case *DeferStmt:
		Walk(v, s.Call)
	case *PerfBlock:
		walkNode(v, s.Block)
	case *VarDecl:
		walkAnnotations(v, s.Annotations)
		walkExprList(v, s.Names)
		walkNode(v, s.Type)
		walkExprList(v, s.Values)
	case *ForStmt:
		walkNode(v, s.Key)
		walkNode(v, s.Value)
		walkNode(v, s.Iterable)
		walkNode(v, s.Cond)
		walkNode(v, s.Body)
	case *FnDecl:
		walkAnnotations(v, s.Annotations)
		walkTypeParams(v, s.TypeParams)
		walkNode(v, s.Sig)
		walkNode(v, s.Body)
		walkNode(v, s.ExprBody)
	case *StructDecl:
		walkAnnotations(v, s.Annotations)
		walkTypeParams(v, s.TypeParams)
		for i := range s.Fields {
			Walk(v, s.Fields[i])
		}
	case *EnumDecl:
		walkAnnotations(v, s.Annotations)
		walkTypeParams(v, s.TypeParams)
		for i := range s.Variants {
			Walk(v, s.Variants[i])
		}
	case *TraitDecl:
		walkAnnotations(v, s.Annotations)
		walkTypeParams(v, s.TypeParams)
		for i := range s.Methods {
			Walk(v, s.Methods[i])
		}
	case *ImplDecl:
		walkAnnotations(v, s.Annotations)
		walkTypeParams(v, s.TypeParams)
		walkNode(v, s.Trait)
		walkNode(v, s.Type)
		for i := range s.Methods {
			Walk(v, s.Methods[i])
		}
	case *ImportDecl:
		// ImportName fields are scalars (Name, Alias strings); no nodes to walk
	case *MatchArm:
		walkPattern(v, s.Pattern)
		Walk(v, s.Body)
	case *FnSig:
		walkNode(v, s.Recv)
		for i := range s.Params {
			Walk(v, s.Params[i])
		}
		walkNode(v, s.Result)
	case *Param:
		walkNode(v, s.Type)
		walkNode(v, s.Default)
	case *FieldDecl:
		walkAnnotations(v, s.Annotations)
		walkNode(v, s.Type)
		walkNode(v, s.Default)
	case *EnumVariant:
		for i := range s.Fields {
			Walk(v, s.Fields[i])
		}
	case *Annotation:
		walkExprList(v, s.Args)
	case *TypeParam:
		walkTypeList(v, s.Bounds)
	default:
		return false
	}
	return true
}

// walkAnnotations visits each annotation's argument expressions.
func walkAnnotations(v Visitor, annotations []*Annotation) {
	for i := range annotations {
		Walk(v, annotations[i])
	}
}

// walkTypeParams visits each type parameter's bounds.
func walkTypeParams(v Visitor, params []*TypeParam) {
	for i := range params {
		Walk(v, params[i])
	}
}
