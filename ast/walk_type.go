package ast

// walkType visits the children of a type node in source order. It returns
// true when n was a type and its children were handled; false signals Walk
// to try the next node category.
func walkType(v Visitor, n Node) bool {
	switch t := n.(type) {
	case *NamedType:
		// leaf — Pkg/Name are scalar strings
	case *GenericType:
		walkNode(v, t.Base)
		walkTypeList(v, t.Args)
	case *SliceType:
		walkNode(v, t.Elem)
	case *ArrayType:
		walkNode(v, t.Len)
		walkNode(v, t.Elem)
	case *MapType:
		walkNode(v, t.Key)
		walkNode(v, t.Value)
	case *FnType:
		walkTypeList(v, t.Params)
		walkNode(v, t.Result)
	case *TupleType:
		walkTypeList(v, t.Elems)
	case *PointerType:
		walkNode(v, t.Elem)
	case *DynType:
		walkNode(v, t.Trait)
	case *WeakType:
		walkNode(v, t.Elem)
	case *OptionalType:
		walkNode(v, t.Elem)
	case *SelfTypeNode:
		// leaf
	case *BadType:
		// leaf
	default:
		return false
	}
	return true
}

// walkPattern visits the children of a pattern node in source order. It
// returns true when n was a pattern and its children were handled; false
// signals Walk to try the next node category.
func walkPattern(v Visitor, n Node) bool {
	switch p := n.(type) {
	case *LiteralPattern:
		Walk(v, p.Value)
	case *IdentPattern:
		// leaf — Name/Mut are scalar
	case *WildcardPattern:
		// leaf
	case *OrPattern:
		walkPatternList(v, p.Alts)
	case *GuardedPattern:
		walkPattern(v, p.Pattern)
		walkNode(v, p.Guard)
	case *TypePattern:
		walkNode(v, p.Type)
		walkPattern(v, p.Binding)
	case *EnumPattern:
		walkPatternList(v, p.Args)
	case *StructPattern:
		walkNode(v, p.Type)
		for i := range p.Fields {
			walkPattern(v, p.Fields[i].Pattern)
		}
	case *TuplePattern:
		walkPatternList(v, p.Elems)
	case *RangePattern:
		walkNode(v, p.Low)
		walkNode(v, p.High)
	case *BadPattern:
		// leaf
	default:
		return false
	}
	return true
}
