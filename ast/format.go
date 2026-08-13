package ast

import (
	"strings"
	"fmt"
)

// Format prints the AST back into source code.
func Format(n Node) string {
	var sb strings.Builder
	p := &formatter{w: &sb}
	p.formatNode(n)
	return sb.String()
}

type formatter struct {
	w       *strings.Builder
	indent  int
	lineBegan bool
}

func (p *formatter) write(s string) {
	if !p.lineBegan {
		p.w.WriteString(strings.Repeat("    ", p.indent))
		p.lineBegan = true
	}
	p.w.WriteString(s)
}

func (p *formatter) writeLn(s string) {
	p.write(s)
	p.w.WriteString("\n")
	p.lineBegan = false
}

func (p *formatter) formatNode(n Node) {
	if isNilNode(n) {
		return
	}

	// Docs
	if docs := n.GetDoc(); len(docs) > 0 {
		for _, doc := range docs {
			p.writeLn(doc.Lexeme)
		}
	}

	switch n := n.(type) {
	case *Annotation:
		// p.write("/* Annotation */")
		p.write(n.Name)
		for i, c := range n.Args {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *TypeParam:
		// p.write("/* TypeParam */")
		p.write(n.Name)
		for i, c := range n.Bounds {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *Param:
		// p.write("/* Param */")
		p.write(n.Name)
		p.formatNode(n.Type)
		p.formatNode(n.Default)
	case *FnSig:
		// p.write("/* FnSig */")
		p.formatNode(n.Recv)
		for i, c := range n.Params {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		p.formatNode(n.Result)
	case *FnDecl:
		// p.write("/* FnDecl */")
		for i, c := range n.Annotations {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		if n.Pub { p.write("pub ") }
		if n.Async { p.write("async ") }
		p.write(n.Name)
		for i, c := range n.TypeParams {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		p.formatNode(n.Sig)
		p.formatNode(n.Body)
		p.formatNode(n.ExprBody)
	case *FieldDecl:
		// p.write("/* FieldDecl */")
		for i, c := range n.Annotations {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		p.write(n.Name)
		p.formatNode(n.Type)
		p.formatNode(n.Default)
	case *StructDecl:
		// p.write("/* StructDecl */")
		for i, c := range n.Annotations {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		if n.Pub { p.write("pub ") }
		p.write(n.Name)
		for i, c := range n.TypeParams {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		for i, c := range n.Fields {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *EnumVariant:
		// p.write("/* EnumVariant */")
		p.write(n.Name)
		for i, c := range n.Fields {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *EnumDecl:
		// p.write("/* EnumDecl */")
		for i, c := range n.Annotations {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		if n.Pub { p.write("pub ") }
		p.write(n.Name)
		for i, c := range n.TypeParams {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		for i, c := range n.Variants {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *TraitDecl:
		// p.write("/* TraitDecl */")
		for i, c := range n.Annotations {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		if n.Pub { p.write("pub ") }
		p.write(n.Name)
		for i, c := range n.TypeParams {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		for i, c := range n.Methods {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *ImplDecl:
		// p.write("/* ImplDecl */")
		for i, c := range n.Annotations {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		for i, c := range n.TypeParams {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		p.formatNode(n.Trait)
		p.formatNode(n.Type)
		for i, c := range n.Methods {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *ImportDecl:
		// p.write("/* ImportDecl */")
		p.write(n.Alias)
	case *ParenExpr:
		// p.write("/* ParenExpr */")
		p.formatNode(n.X)
	case *UnaryExpr:
		// p.write("/* UnaryExpr */")
		p.formatNode(n.X)
	case *BinaryExpr:
		// p.write("/* BinaryExpr */")
		p.formatNode(n.X)
		p.formatNode(n.Y)
	case *AssignExpr:
		// p.write("/* AssignExpr */")
		for i, c := range n.Targets {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		for i, c := range n.Values {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *RangeExpr:
		// p.write("/* RangeExpr */")
		p.formatNode(n.Low)
		p.formatNode(n.High)
	case *CallExpr:
		// p.write("/* CallExpr */")
		p.formatNode(n.Fn)
	case *IndexExpr:
		// p.write("/* IndexExpr */")
		p.formatNode(n.X)
		for i, c := range n.Indices {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *SliceExpr:
		// p.write("/* SliceExpr */")
		p.formatNode(n.X)
		p.formatNode(n.Low)
		p.formatNode(n.High)
	case *FieldExpr:
		// p.write("/* FieldExpr */")
		p.formatNode(n.X)
		p.write(n.Name)
	case *PipeExpr:
		// p.write("/* PipeExpr */")
		p.formatNode(n.X)
		p.formatNode(n.Fn)
	case *TryExpr:
		// p.write("/* TryExpr */")
		p.formatNode(n.X)
	case *AwaitExpr:
		// p.write("/* AwaitExpr */")
		p.formatNode(n.X)
	case *SpawnExpr:
		// p.write("/* SpawnExpr */")
		p.formatNode(n.Block)
	case *CastExpr:
		// p.write("/* CastExpr */")
		p.formatNode(n.X)
		p.formatNode(n.Type)
	case *IsExpr:
		// p.write("/* IsExpr */")
		p.formatNode(n.X)
		p.formatNode(n.Type)
	case *IfExpr:
		// p.write("/* IfExpr */")
		p.formatNode(n.Bind)
		p.formatNode(n.Cond)
		p.formatNode(n.Then)
		p.formatNode(n.Else)
		p.formatNode(n.ElseIf)
	case *MatchArm:
		// p.write("/* MatchArm */")
		p.formatNode(n.Pattern)
		p.formatNode(n.Body)
	case *MatchExpr:
		// p.write("/* MatchExpr */")
		p.formatNode(n.Subject)
		for i, c := range n.Arms {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *BlockExpr:
		p.write("{")
		p.writeLn("")
		p.indent++
		for _, stmt := range n.Block.Stmts {
			p.formatNode(stmt)
			p.writeLn("")
		}
		p.indent--
		p.write("}")
	case *BadExpr:
		// p.write("/* BadExpr */")
	case *IntLit:
		// p.write("/* IntLit */")
		p.write(n.Suffix)
		p.write(n.Raw)
	case *FloatLit:
		// p.write("/* FloatLit */")
		p.write(n.Suffix)
		p.write(n.Raw)
	case *StringLit:
		// p.write("/* StringLit */")
		p.write(n.Raw)
	case *RawStringLit:
		// p.write("/* RawStringLit */")
		p.write(n.Value)
	case *BoolLit:
		// p.write("/* BoolLit */")
	case *NilLit:
		// p.write("/* NilLit */")
	case *Ident:
		// p.write("/* Ident */")
		p.write(n.Name)
	case *UnderscoreExpr:
		// p.write("/* UnderscoreExpr */")
	case *SelfExpr:
		// p.write("/* SelfExpr */")
	case *TupleLit:
		// p.write("/* TupleLit */")
	case *ArrayLit:
		// p.write("/* ArrayLit */")
		p.formatNode(n.Type)
		for i, c := range n.Elems {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *MapLit:
		// p.write("/* MapLit */")
		p.formatNode(n.Type)
	case *StructLit:
		// p.write("/* StructLit */")
		p.formatNode(n.Type)
	case *FnLit:
		// p.write("/* FnLit */")
		p.formatNode(n.Sig)
		p.formatNode(n.Body)
		p.formatNode(n.ExprBody)
	case *Program:
		for i, decl := range n.Decls {
			if i > 0 { p.writeLn("") }
			p.formatNode(decl)
			p.writeLn("")
		}
		for _, comment := range n.LooseComments {
			p.writeLn(comment.Lexeme)
		}
	case *BlockStmt:
		p.write("{")
		p.writeLn("")
		p.indent++
		for _, stmt := range n.Stmts {
			p.formatNode(stmt)
			p.writeLn("")
		}
		p.indent--
		p.write("}")
	case *ExprStmt:
		// p.write("/* ExprStmt */")
		p.formatNode(n.X)
	case *VarDecl:
		// p.write("/* VarDecl */")
		for i, c := range n.Annotations {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		if n.Pub { p.write("pub ") }
		for i, c := range n.Names {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		p.formatNode(n.Type)
		for i, c := range n.Values {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *ReturnStmt:
		// p.write("/* ReturnStmt */")
		for i, c := range n.Values {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *ForStmt:
		// p.write("/* ForStmt */")
		p.write(n.Label)
		p.formatNode(n.Key)
		p.formatNode(n.Value)
		p.formatNode(n.Iterable)
		p.formatNode(n.Cond)
		p.formatNode(n.Body)
	case *BreakStmt:
		// p.write("/* BreakStmt */")
		p.write(n.Label)
	case *ContinueStmt:
		// p.write("/* ContinueStmt */")
		p.write(n.Label)
	case *DeferStmt:
		// p.write("/* DeferStmt */")
		p.formatNode(n.Call)
	case *PerfBlock:
		// p.write("/* PerfBlock */")
		p.formatNode(n.Block)
	case *BadStmt:
		// p.write("/* BadStmt */")
	case *NamedType:
		// p.write("/* NamedType */")
		p.write(n.Pkg)
		p.write(n.Name)
	case *GenericType:
		// p.write("/* GenericType */")
		p.formatNode(n.Base)
		for i, c := range n.Args {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *SliceType:
		// p.write("/* SliceType */")
		p.formatNode(n.Elem)
	case *ArrayType:
		// p.write("/* ArrayType */")
		p.formatNode(n.Len)
		p.formatNode(n.Elem)
	case *MapType:
		// p.write("/* MapType */")
		p.formatNode(n.Key)
		p.formatNode(n.Value)
	case *FnType:
		// p.write("/* FnType */")
		for i, c := range n.Params {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
		p.formatNode(n.Result)
	case *TupleType:
		// p.write("/* TupleType */")
		for i, c := range n.Elems {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *PointerType:
		// p.write("/* PointerType */")
		p.formatNode(n.Elem)
	case *DynType:
		// p.write("/* DynType */")
		p.formatNode(n.Trait)
	case *WeakType:
		// p.write("/* WeakType */")
		p.formatNode(n.Elem)
	case *OptionalType:
		// p.write("/* OptionalType */")
		p.formatNode(n.Elem)
	case *SelfTypeNode:
		// p.write("/* SelfTypeNode */")
	case *BadType:
		// p.write("/* BadType */")
	case *LiteralPattern:
		// p.write("/* LiteralPattern */")
		p.formatNode(n.Value)
	case *IdentPattern:
		// p.write("/* IdentPattern */")
		p.write(n.Name)
	case *WildcardPattern:
		// p.write("/* WildcardPattern */")
	case *OrPattern:
		// p.write("/* OrPattern */")
		for i, c := range n.Alts {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *GuardedPattern:
		// p.write("/* GuardedPattern */")
		p.formatNode(n.Pattern)
		p.formatNode(n.Guard)
	case *TypePattern:
		// p.write("/* TypePattern */")
		p.formatNode(n.Type)
		p.formatNode(n.Binding)
	case *EnumPattern:
		// p.write("/* EnumPattern */")
		p.write(n.Enum)
		p.write(n.Variant)
		for i, c := range n.Args {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *StructPattern:
		// p.write("/* StructPattern */")
		p.formatNode(n.Type)
	case *TuplePattern:
		// p.write("/* TuplePattern */")
		for i, c := range n.Elems {
			if i > 0 { p.write(", ") }
			p.formatNode(c)
		}
	case *RangePattern:
		// p.write("/* RangePattern */")
		p.formatNode(n.Low)
		p.formatNode(n.High)
	case *BadPattern:
		// p.write("/* BadPattern */")

	default:
		p.write(fmt.Sprintf("/* UNIMPLEMENTED: %T */", n))
	}

	// Comments
	if comments := n.GetComment(); len(comments) > 0 {
		for _, comment := range comments {
			p.write(" ")
			p.write(comment.Lexeme)
		}
	}
}
