package ast

import (
	"testing"

	"github.com/dotlang/dot/lexer"
)

var (
	_ Node = (*Program)(nil)
	_ Node = (*MatchArm)(nil)
	_ Node = (*FnSig)(nil)
	_ Node = (*Param)(nil)
	_ Node = (*FieldDecl)(nil)
	_ Node = (*EnumVariant)(nil)
	_ Node = (*Annotation)(nil)
	_ Node = (*TypeParam)(nil)

	_ Expr = (*ParenExpr)(nil)
	_ Expr = (*UnaryExpr)(nil)
	_ Expr = (*BinaryExpr)(nil)
	_ Expr = (*AssignExpr)(nil)
	_ Expr = (*RangeExpr)(nil)
	_ Expr = (*CallExpr)(nil)
	_ Expr = (*IndexExpr)(nil)
	_ Expr = (*SliceExpr)(nil)
	_ Expr = (*FieldExpr)(nil)
	_ Expr = (*PipeExpr)(nil)
	_ Expr = (*TryExpr)(nil)
	_ Expr = (*AwaitExpr)(nil)
	_ Expr = (*SpawnExpr)(nil)
	_ Expr = (*CastExpr)(nil)
	_ Expr = (*IsExpr)(nil)
	_ Expr = (*IfExpr)(nil)
	_ Expr = (*MatchExpr)(nil)
	_ Expr = (*BlockExpr)(nil)
	_ Expr = (*IntLit)(nil)
	_ Expr = (*FloatLit)(nil)
	_ Expr = (*StringLit)(nil)
	_ Expr = (*RawStringLit)(nil)
	_ Expr = (*BoolLit)(nil)
	_ Expr = (*NilLit)(nil)
	_ Expr = (*Ident)(nil)
	_ Expr = (*UnderscoreExpr)(nil)
	_ Expr = (*SelfExpr)(nil)
	_ Expr = (*TupleLit)(nil)
	_ Expr = (*ArrayLit)(nil)
	_ Expr = (*MapLit)(nil)
	_ Expr = (*StructLit)(nil)
	_ Expr = (*FnLit)(nil)
	_ Expr = (*BadExpr)(nil)

	_ Type = (*NamedType)(nil)
	_ Type = (*GenericType)(nil)
	_ Type = (*SliceType)(nil)
	_ Type = (*ArrayType)(nil)
	_ Type = (*MapType)(nil)
	_ Type = (*FnType)(nil)
	_ Type = (*TupleType)(nil)
	_ Type = (*PointerType)(nil)
	_ Type = (*DynType)(nil)
	_ Type = (*WeakType)(nil)
	_ Type = (*OptionalType)(nil)
	_ Type = (*SelfTypeNode)(nil)
	_ Type = (*BadType)(nil)

	_ Pattern = (*LiteralPattern)(nil)
	_ Pattern = (*IdentPattern)(nil)
	_ Pattern = (*WildcardPattern)(nil)
	_ Pattern = (*OrPattern)(nil)
	_ Pattern = (*GuardedPattern)(nil)
	_ Pattern = (*TypePattern)(nil)
	_ Pattern = (*EnumPattern)(nil)
	_ Pattern = (*StructPattern)(nil)
	_ Pattern = (*TuplePattern)(nil)
	_ Pattern = (*RangePattern)(nil)
	_ Pattern = (*BadPattern)(nil)

	_ Stmt = (*BlockStmt)(nil)
	_ Stmt = (*ExprStmt)(nil)
	_ Stmt = (*ReturnStmt)(nil)
	_ Stmt = (*BreakStmt)(nil)
	_ Stmt = (*ContinueStmt)(nil)
	_ Stmt = (*DeferStmt)(nil)
	_ Stmt = (*PerfBlock)(nil)
	_ Stmt = (*ForStmt)(nil)
	_ Stmt = (*BadStmt)(nil)

	_ Decl = (*VarDecl)(nil)
	_ Decl = (*FnDecl)(nil)
	_ Decl = (*StructDecl)(nil)
	_ Decl = (*EnumDecl)(nil)
	_ Decl = (*TraitDecl)(nil)
	_ Decl = (*ImplDecl)(nil)
	_ Decl = (*ImportDecl)(nil)
)

func TestPosEndRoundTrip(t *testing.T) {
	var n BaseNode
	start := pos(1, 1, 0)
	stop := pos(2, 5, 10)
	n.SetSpan(start, stop)
	if n.Pos() != start {
		t.Errorf("Pos() = %v, want %v", n.Pos(), start)
	}
	if n.End() != stop {
		t.Errorf("End() = %v, want %v", n.End(), stop)
	}
}

func TestSpan(t *testing.T) {
	start := pos(1, 1, 0)
	stop := pos(2, 5, 10)
	n := Span(start, stop)
	if n.Start != start || n.Stop != stop {
		t.Errorf("Span() = {%v %v}, want {%v %v}", n.Start, n.Stop, start, stop)
	}
}

func TestSpanOf(t *testing.T) {
	if got := SpanOf(nil, nil); got.Start != (Position{}) || got.Stop != (Position{}) {
		t.Errorf("SpanOf(nil,nil) = %v, want zero BaseNode", got)
	}

	a := &Ident{BaseNode: bn()}
	a.SetSpan(pos(1, 1, 0), pos(1, 5, 4))
	b := &Ident{BaseNode: bn()}
	b.SetSpan(pos(2, 1, 5), pos(2, 3, 7))

	if got := SpanOf(a, b); got.Start != a.Pos() || got.Stop != b.End() {
		t.Errorf("SpanOf(a,b) = {%v %v}, want {%v %v}", got.Start, got.Stop, a.Pos(), b.End())
	}
	if got := SpanOf(nil, b); got.Start != b.Pos() || got.Stop != b.End() {
		t.Errorf("SpanOf(nil,b) = {%v %v}, want {%v %v}", got.Start, got.Stop, b.Pos(), b.End())
	}
	if got := SpanOf(a, nil); got.Start != a.Pos() || got.Stop != a.End() {
		t.Errorf("SpanOf(a,nil) = {%v %v}, want {%v %v}", got.Start, got.Stop, a.Pos(), a.End())
	}
}

func TestSpanTok(t *testing.T) {
	tok := lexer.Token{Pos: pos(1, 1, 0), End: pos(1, 2, 1), Type: lexer.TokenInt, Lexeme: "1"}
	n := SpanTok(tok)
	if n.Start != tok.Pos || n.Stop != tok.End {
		t.Errorf("SpanTok() = {%v %v}, want {%v %v}", n.Start, n.Stop, tok.Pos, tok.End)
	}
}

func TestNodeName(t *testing.T) {
	cases := []struct {
		node Node
		want string
	}{
		{identNode(), "Ident"},
		{intLitNode(), "IntLit"},
		{namedTypeNode(), "NamedType"},
		{blockStmtNode(), "BlockStmt"},
		{&FnDecl{BaseNode: bn()}, "FnDecl"},
		{&EnumPattern{BaseNode: bn()}, "EnumPattern"},
		{&StructDecl{BaseNode: bn()}, "StructDecl"},
		{&TraitDecl{BaseNode: bn()}, "TraitDecl"},
		{&ImplDecl{BaseNode: bn()}, "ImplDecl"},
		{&ImportDecl{BaseNode: bn()}, "ImportDecl"},
		{&Program{BaseNode: bn()}, "Program"},
		{&MatchArm{BaseNode: bn()}, "MatchArm"},
	}
	for _, c := range cases {
		if got := NodeName(c.node); got != c.want {
			t.Errorf("NodeName(%T) = %q, want %q", c.node, got, c.want)
		}
	}
	if got := NodeName(nil); got != "nil" {
		t.Errorf("NodeName(nil) = %q, want %q", got, "nil")
	}
}
