package ast

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/lexer"
)

type collector struct {
	visited map[string]bool
}

func newCollector() *collector {
	return &collector{visited: make(map[string]bool)}
}

func (c *collector) Visit(n Node) Visitor {
	if n != nil {
		c.visited[NodeName(n)] = true
	}
	return c
}

// expectedWalkCases is the exact set of Go type names handled by the walk
// switches in walk_expr.go (33), walk_stmt.go (24), walk_type.go walkType
// (13), and walk_type.go walkPattern (11) — 81 in total.
func expectedWalkCases() []string {
	return []string{
		// walk_expr.go (33)
		"ParenExpr", "UnaryExpr", "BinaryExpr", "AssignExpr", "RangeExpr",
		"CallExpr", "IndexExpr", "SliceExpr", "FieldExpr", "PipeExpr",
		"TryExpr", "AwaitExpr", "SpawnExpr", "CastExpr", "IsExpr",
		"IfExpr", "MatchExpr", "BlockExpr", "IntLit", "FloatLit",
		"RawStringLit", "BoolLit", "NilLit", "Ident", "UnderscoreExpr",
		"SelfExpr", "BadExpr", "StringLit", "TupleLit", "ArrayLit",
		"MapLit", "StructLit", "FnLit",
		// walk_stmt.go (24)
		"Program", "BlockStmt", "ExprStmt", "ReturnStmt", "BreakStmt",
		"ContinueStmt", "BadStmt", "DeferStmt", "PerfBlock", "VarDecl",
		"ForStmt", "FnDecl", "StructDecl", "EnumDecl", "TraitDecl",
		"ImplDecl", "ImportDecl", "MatchArm", "FnSig", "Param",
		"FieldDecl", "EnumVariant", "Annotation", "TypeParam",
		// walk_type.go walkType (13)
		"NamedType", "GenericType", "SliceType", "ArrayType", "MapType",
		"FnType", "TupleType", "PointerType", "DynType", "WeakType",
		"OptionalType", "SelfTypeNode", "BadType",
		// walk_type.go walkPattern (11)
		"LiteralPattern", "IdentPattern", "WildcardPattern", "OrPattern",
		"GuardedPattern", "TypePattern", "EnumPattern", "StructPattern",
		"TuplePattern", "RangePattern", "BadPattern",
	}
}

func TestWalk_CoversEveryNodeType(t *testing.T) {
	fixtures := allFixtures()
	c := newCollector()

	for name, node := range fixtures {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Walk panicked on fixture %s: %v", name, r)
				}
			}()
			Walk(c, node)
		}()
	}

	expected := expectedWalkCases()
	if len(expected) != walkCaseCount() {
		t.Errorf("expectedWalkCases has %d entries, walkCaseCount=%d", len(expected), walkCaseCount())
	}
	for _, name := range expected {
		if !c.visited[name] {
			t.Errorf("node type %q was never visited during Walk", name)
		}
	}
}

type dummyExprNode struct {
	BaseNode
}

func (*dummyExprNode) exprNode() {}

func TestWalk_PanicsOnUnknownNode(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unknown node type, got nil")
		}
		msg, ok := r.(string)
		if !ok {
			msg = lexer.TokenIdent.Literal()
		}
		if !strings.Contains(msg, "unhandled") {
			t.Fatalf("expected panic message to contain \"unhandled\", got %q", r)
		}
	}()
	Walk(newCollector(), &dummyExprNode{BaseNode: bn()})
}

func TestInspect_StopsDescent(t *testing.T) {
	inner := &Ident{BaseNode: bn(), Name: "inner"}
	root := &ParenExpr{BaseNode: bn(), X: inner}

	var visited []string
	Inspect(root, func(n Node) bool {
		if n == nil {
			return true
		}
		visited = append(visited, NodeName(n))
		return false
	})

	if len(visited) != 1 || visited[0] != "ParenExpr" {
		t.Errorf("expected only ParenExpr visited, got %v", visited)
	}
}

type nilVisitor struct{}

func (nilVisitor) Visit(n Node) Visitor { return nil }

func TestWalk_VisitorNilStopsDescent(t *testing.T) {
	root := &ParenExpr{BaseNode: bn(), X: &Ident{BaseNode: bn(), Name: "child"}}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Walk panicked: %v", r)
		}
	}()
	Walk(nilVisitor{}, root)
}

func TestWalk_NilChildren(t *testing.T) {
	cases := map[string]Node{
		"IfExpr (nil Else/ElseIf)": &IfExpr{BaseNode: bn(), Then: blockStmtNode()},
		"ReturnStmt (no values)":   &ReturnStmt{BaseNode: bn()},
		"FnSig (nil Recv/Result)":  &FnSig{BaseNode: bn()},
		"RangeExpr (nil bounds)":   &RangeExpr{BaseNode: bn()},
		"VarDecl (no names/vals)":  &VarDecl{BaseNode: bn()},
		"ForStmt (nil fields)":     &ForStmt{BaseNode: bn()},
		"FnDecl (no sig/body)":     &FnDecl{BaseNode: bn()},
		"Param (nil Type/Default)": &Param{BaseNode: bn()},
		"EnumVariant (no fields)":  &EnumVariant{BaseNode: bn()},
		"TypeParam (no bounds)":    &TypeParam{BaseNode: bn()},
		"FnLit (nil Sig/Body)":     &FnLit{BaseNode: bn()},
		"Annotation (no args)":     &Annotation{BaseNode: bn()},
		"MatchExpr (no arms)":      &MatchExpr{BaseNode: bn()},
		"MatchArm (nil pattern)":   &MatchArm{BaseNode: bn()},
		"EnumPattern (no args)":    &EnumPattern{BaseNode: bn()},
	}

	for label, node := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: Walk panicked: %v", label, r)
				}
			}()
			Walk(newCollector(), node)
		}()
	}
}
