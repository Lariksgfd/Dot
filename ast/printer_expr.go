package ast

import "strconv"

// printExprTypePattern handles the expression, type, and pattern cases of
// the printer. It is called from print for any node the statement/decl
// switch did not handle.
func (p *printer) printExprTypePattern(n Node) {
	switch v := n.(type) {
	// --- expressions ---
	case *IntLit:
		p.leaf("IntLit", joinAttrs(
			"value="+strconv.FormatUint(v.Value, 10),
			attr("raw", v.Raw),
			attrBase("base", v.Base),
		))
	case *FloatLit:
		p.leaf("FloatLit", joinAttrs(
			"value="+strconv.FormatFloat(v.Value, 'g', -1, 64),
			attr("raw", v.Raw),
		))
	case *StringLit:
		p.nodeHead("StringLit", attr("raw", v.Raw))
		for i := range v.Parts {
			part := v.Parts[i]
			if part.Kind == PartExpr {
				p.nodeHead("PartExpr", "")
				p.child(part.Expr)
				p.nodeTail()
			} else {
				p.leaf("PartText", attr("text", part.Text))
			}
		}
		p.nodeTail()
	case *RawStringLit:
		p.leaf("RawStringLit", attr("value", v.Value))
	case *BoolLit:
		p.leaf("BoolLit", attr("value", strconv.FormatBool(v.Value)))
	case *NilLit:
		p.leaf("NilLit", "")
	case *Ident:
		p.leaf("Ident", attr("name", v.Name))
	case *UnderscoreExpr:
		p.leaf("UnderscoreExpr", "")
	case *SelfExpr:
		p.leaf("SelfExpr", "")
	case *BadExpr:
		p.leaf("BadExpr", "")
	case *ParenExpr:
		p.nodeHead("ParenExpr", "")
		p.child(v.X)
		p.nodeTail()
	case *UnaryExpr:
		p.nodeHead("UnaryExpr", attr("op", v.OpString()))
		p.child(v.X)
		p.nodeTail()
	case *BinaryExpr:
		p.nodeHead("BinaryExpr", attr("op", v.OpString()))
		p.child(v.X)
		p.child(v.Y)
		p.nodeTail()
	case *AssignExpr:
		p.nodeHead("AssignExpr", attr("op", v.OpString()))
		walkFunc(p, v.Targets)
		walkFunc(p, v.Values)
		p.nodeTail()
	case *RangeExpr:
		p.nodeHead("RangeExpr", attrBool("inclusive", v.Inclusive))
		p.child(v.Low)
		p.child(v.High)
		p.nodeTail()
	case *CallExpr:
		p.nodeHead("CallExpr", "")
		p.child(v.Fn)
		for i := range v.Args {
			a := v.Args[i]
			p.nodeHead("Arg", argAttrs(&a))
			p.child(a.Value)
			p.nodeTail()
		}
		p.nodeTail()
	case *IndexExpr:
		p.nodeHead("IndexExpr", "")
		p.child(v.X)
		walkFunc(p, v.Indices)
		p.nodeTail()
	case *SliceExpr:
		p.nodeHead("SliceExpr", attrBool("inclusive", v.Inclusive))
		p.child(v.X)
		p.child(v.Low)
		p.child(v.High)
		p.nodeTail()
	case *FieldExpr:
		p.nodeHead("FieldExpr", joinAttrs(attr("name", v.Name), attrBool("tuple", v.IsTupleIndex)))
		p.child(v.X)
		p.nodeTail()
	case *PipeExpr:
		p.nodeHead("PipeExpr", "")
		p.child(v.X)
		p.child(v.Fn)
		p.nodeTail()
	case *TryExpr:
		p.nodeHead("TryExpr", "")
		p.child(v.X)
		p.nodeTail()
	case *AwaitExpr:
		p.nodeHead("AwaitExpr", "")
		p.child(v.X)
		p.nodeTail()
	case *SpawnExpr:
		p.nodeHead("SpawnExpr", "")
		p.child(v.Block)
		p.nodeTail()
	case *CastExpr:
		p.nodeHead("CastExpr", "")
		p.child(v.X)
		p.child(v.Type)
		p.nodeTail()
	case *IsExpr:
		p.nodeHead("IsExpr", "")
		p.child(v.X)
		p.child(v.Type)
		p.nodeTail()
	case *IfExpr:
		p.nodeHead("IfExpr", "")
		p.child(v.Bind)
		p.child(v.Cond)
		p.child(v.Then)
		p.child(v.Else)
		p.child(v.ElseIf)
		p.nodeTail()
	case *MatchExpr:
		p.nodeHead("MatchExpr", "")
		p.child(v.Subject)
		for i := range v.Arms {
			p.child(v.Arms[i])
		}
		p.nodeTail()
	case *BlockExpr:
		p.nodeHead("BlockExpr", "")
		p.child(v.Block)
		p.nodeTail()
	case *TupleLit:
		p.nodeHead("TupleLit", "")
		for i := range v.Elems {
			e := v.Elems[i]
			p.nodeHead("TupleElem", attr("name", e.Name))
			p.child(e.Value)
			p.nodeTail()
		}
		p.nodeTail()
	case *ArrayLit:
		p.nodeHead("ArrayLit", "")
		p.child(v.Type)
		walkFunc(p, v.Elems)
		p.nodeTail()
	case *MapLit:
		p.nodeHead("MapLit", "")
		p.child(v.Type)
		for i := range v.Entries {
			p.nodeHead("MapEntry", "")
			p.child(v.Entries[i].Key)
			p.child(v.Entries[i].Value)
			p.nodeTail()
		}
		p.nodeTail()
	case *StructLit:
		p.nodeHead("StructLit", "")
		p.child(v.Type)
		for i := range v.Fields {
			f := v.Fields[i]
			p.nodeHead("StructLitField", joinAttrs(attr("name", f.Name), attrBool("shorthand", f.Shorthand)))
			p.child(f.Value)
			p.nodeTail()
		}
		p.nodeTail()
	case *FnLit:
		p.nodeHead("FnLit", "")
		p.child(v.Sig)
		p.child(v.Body)
		p.child(v.ExprBody)
		p.nodeTail()

	// --- types ---
	case *NamedType:
		p.leaf("NamedType", joinAttrs(attr("name", v.Name), attr("pkg", v.Pkg)))
	case *GenericType:
		p.nodeHead("GenericType", "")
		p.child(v.Base)
		walkFunc(p, v.Args)
		p.nodeTail()
	case *SliceType:
		p.nodeHead("SliceType", "")
		p.child(v.Elem)
		p.nodeTail()
	case *ArrayType:
		p.nodeHead("ArrayType", "")
		p.child(v.Len)
		p.child(v.Elem)
		p.nodeTail()
	case *MapType:
		p.nodeHead("MapType", "")
		p.child(v.Key)
		p.child(v.Value)
		p.nodeTail()
	case *FnType:
		p.nodeHead("FnType", attrBool("variadic", v.Variadic))
		walkFunc(p, v.Params)
		p.child(v.Result)
		p.nodeTail()
	case *TupleType:
		p.nodeHead("TupleType", "")
		walkFunc(p, v.Elems)
		p.nodeTail()
	case *PointerType:
		p.nodeHead("PointerType", "")
		p.child(v.Elem)
		p.nodeTail()
	case *DynType:
		p.nodeHead("DynType", "")
		p.child(v.Trait)
		p.nodeTail()
	case *WeakType:
		p.nodeHead("WeakType", "")
		p.child(v.Elem)
		p.nodeTail()
	case *OptionalType:
		p.nodeHead("OptionalType", "")
		p.child(v.Elem)
		p.nodeTail()
	case *SelfTypeNode:
		p.leaf("SelfTypeNode", "")
	case *BadType:
		p.leaf("BadType", "")

	// --- patterns ---
	case *LiteralPattern:
		p.nodeHead("LiteralPattern", "")
		p.child(v.Value)
		p.nodeTail()
	case *IdentPattern:
		p.leaf("IdentPattern", joinAttrs(attr("name", v.Name), attrBool("mut", v.Mut)))
	case *WildcardPattern:
		p.leaf("WildcardPattern", "")
	case *OrPattern:
		p.nodeHead("OrPattern", "")
		walkFunc(p, v.Alts)
		p.nodeTail()
	case *GuardedPattern:
		p.nodeHead("GuardedPattern", "")
		p.child(v.Pattern)
		p.child(v.Guard)
		p.nodeTail()
	case *TypePattern:
		p.nodeHead("TypePattern", "")
		p.child(v.Type)
		p.child(v.Binding)
		p.nodeTail()
	case *EnumPattern:
		p.nodeHead("EnumPattern", joinAttrs(attr("enum", v.Enum), attr("variant", v.Variant), attrBool("args", v.HasArgs)))
		walkFunc(p, v.Args)
		p.nodeTail()
	case *StructPattern:
		p.nodeHead("StructPattern", "")
		p.child(v.Type)
		for i := range v.Fields {
			f := v.Fields[i]
			p.nodeHead("StructPatternField", joinAttrs(attr("name", f.Name), attrBool("shorthand", f.Shorthand)))
			p.child(f.Pattern)
			p.nodeTail()
		}
		p.nodeTail()
	case *TuplePattern:
		p.nodeHead("TuplePattern", "")
		walkFunc(p, v.Elems)
		p.nodeTail()
	case *RangePattern:
		p.nodeHead("RangePattern", attrBool("inclusive", v.Inclusive))
		p.child(v.Low)
		p.child(v.High)
		p.nodeTail()
	case *BadPattern:
		p.leaf("BadPattern", "")
	}
}

// argAttrs returns the attribute string for an Arg.
func argAttrs(a *Arg) string {
	return joinAttrs(attr("name", a.Name), attrBool("spread", a.Spread))
}
