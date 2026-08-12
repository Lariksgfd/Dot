package ast

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/lexer"
)

func goldenTree() Node {
	innerBlock := &BlockStmt{
		BaseNode: bn(),
		Stmts: []Stmt{
			&ExprStmt{BaseNode: bn(), X: &CallExpr{BaseNode: bn(), Fn: &Ident{BaseNode: bn(), Name: "do_stuff"}, Args: []Arg{{Value: &IntLit{BaseNode: bn(), Value: 1, Base: 10}}}}},
		},
	}
	return &FnDecl{
		BaseNode:   bn(),
		Name:       "transform",
		TypeParams: []*TypeParam{{BaseNode: bn(), Name: "T", Bounds: []Type{&NamedType{BaseNode: bn(), Name: "Comparable"}}}},
		Sig:        &FnSig{BaseNode: bn(), Params: []*Param{{BaseNode: bn(), Name: "items", Type: &SliceType{BaseNode: bn(), Elem: &NamedType{BaseNode: bn(), Name: "int"}}}, {BaseNode: bn(), Name: "threshold", Type: &NamedType{BaseNode: bn(), Name: "int"}, Default: &IntLit{BaseNode: bn(), Value: 10, Base: 10}}}, Result: &NamedType{BaseNode: bn(), Name: "void"}},
		Body: &BlockStmt{
			BaseNode: bn(),
			Stmts: []Stmt{
				&VarDecl{BaseNode: bn(), Names: []Expr{&Ident{BaseNode: bn(), Name: "result"}}, Values: []Expr{&PipeExpr{BaseNode: bn(), X: &Ident{BaseNode: bn(), Name: "items"}, Fn: &CallExpr{BaseNode: bn(), Fn: &Ident{BaseNode: bn(), Name: "process"}, Args: []Arg{}}}}},
				&ExprStmt{BaseNode: bn(), X: &IfExpr{BaseNode: bn(), Cond: &BinaryExpr{BaseNode: bn(), Op: lexer.TokenPlus, X: &Ident{BaseNode: bn(), Name: "x"}, Y: &IntLit{BaseNode: bn(), Value: 1, Base: 10}}, Then: innerBlock, ElseIf: &IfExpr{BaseNode: bn(), Cond: &Ident{BaseNode: bn(), Name: "other"}, Then: innerBlock}}},
				&ExprStmt{BaseNode: bn(), X: &MatchExpr{BaseNode: bn(), Subject: &Ident{BaseNode: bn(), Name: "opt"}, Arms: []*MatchArm{
					{BaseNode: bn(), Pattern: &EnumPattern{BaseNode: bn(), Enum: "Result", Variant: "Ok", HasArgs: true, Args: []Pattern{&IdentPattern{BaseNode: bn(), Name: "val"}}}, Body: &Ident{BaseNode: bn(), Name: "val"}},
					{BaseNode: bn(), Pattern: &EnumPattern{BaseNode: bn(), Enum: "Result", Variant: "Err", HasArgs: false}, Body: &StructLit{BaseNode: bn(), Type: &NamedType{BaseNode: bn(), Name: "Point"}, Fields: []StructLitField{{Name: "x", Value: &IntLit{BaseNode: bn(), Value: 0, Base: 10}}}}},
				}}},
				&ExprStmt{BaseNode: bn(), X: &StringLit{BaseNode: bn(), Raw: "\"hi {name}\"", Parts: []StringPart{{Kind: PartText, Text: "hi "}, {Kind: PartExpr, Expr: &Ident{BaseNode: bn(), Name: "name"}}}}},
				&ForStmt{BaseNode: bn(), Kind: ForIn, Value: &Ident{BaseNode: bn(), Name: "i"}, Iterable: &RangeExpr{BaseNode: bn(), Low: &IntLit{BaseNode: bn(), Value: 0, Base: 10}, High: &IntLit{BaseNode: bn(), Value: 10, Base: 10}}, Body: &BlockStmt{BaseNode: bn()}},
			},
		},
	}
}

func goldenOutput() string {
	return "FnDecl name=transform\n" +
		"  TypeParam name=T\n" +
		"    NamedType name=Comparable\n" +
		"  FnSig\n" +
		"    <nil>\n" +
		"    Param name=items\n" +
		"      SliceType\n" +
		"        NamedType name=int\n" +
		"      <nil>\n" +
		"    Param name=threshold\n" +
		"      NamedType name=int\n" +
		"      IntLit value=10 base=10\n" +
		"    NamedType name=void\n" +
		"  BlockStmt\n" +
		"    VarDecl\n" +
		"      Ident name=result\n" +
		"      <nil>\n" +
		"      PipeExpr\n" +
		"        Ident name=items\n" +
		"        CallExpr\n" +
		"          Ident name=process\n" +
		"    ExprStmt\n" +
		"      IfExpr\n" +
		"        <nil>\n" +
		"        BinaryExpr op=+\n" +
		"          Ident name=x\n" +
		"          IntLit value=1 base=10\n" +
		"        BlockStmt\n" +
		"          ExprStmt\n" +
		"            CallExpr\n" +
		"              Ident name=do_stuff\n" +
		"              Arg\n" +
		"                IntLit value=1 base=10\n" +
		"        <nil>\n" +
		"        IfExpr\n" +
		"          <nil>\n" +
		"          Ident name=other\n" +
		"          BlockStmt\n" +
		"            ExprStmt\n" +
		"              CallExpr\n" +
		"                Ident name=do_stuff\n" +
		"                Arg\n" +
		"                  IntLit value=1 base=10\n" +
		"          <nil>\n" +
		"          <nil>\n" +
		"    ExprStmt\n" +
		"      MatchExpr\n" +
		"        Ident name=opt\n" +
		"        MatchArm\n" +
		"          EnumPattern enum=Result variant=Ok args=true\n" +
		"            IdentPattern name=val\n" +
		"          Ident name=val\n" +
		"        MatchArm\n" +
		"          EnumPattern enum=Result variant=Err\n" +
		"          StructLit\n" +
		"            NamedType name=Point\n" +
		"            StructLitField name=x\n" +
		"              IntLit value=0 base=10\n" +
		"    ExprStmt\n" +
		"      StringLit raw=\"hi {name}\"\n" +
		"        PartText text=hi \n" +
		"        PartExpr\n" +
		"          Ident name=name\n" +
		"    ForStmt kind=ForIn\n" +
		"      <nil>\n" +
		"      Ident name=i\n" +
		"      RangeExpr\n" +
		"        IntLit value=0 base=10\n" +
		"        IntLit value=10 base=10\n" +
		"      <nil>\n" +
		"      BlockStmt\n" +
		"  <nil>\n"
}

func TestPrint_Golden(t *testing.T) {
	tree := goldenTree()
	got := Print(tree)
	want := goldenOutput()
	if got != want {
		t.Errorf("Print output mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrint_Deterministic(t *testing.T) {
	tree := goldenTree()
	first := Print(tree)
	second := Print(tree)
	if first != second {
		t.Error("Print produced different output on second call")
	}
}

func TestPrint_EveryNodeType(t *testing.T) {
	for name, node := range allFixtures() {
		out := Print(node)
		if out == "" {
			t.Errorf("Print(%s) returned empty string", name)
		}
	}
}

func TestPrint_Indentation(t *testing.T) {
	innerBlock := &BlockStmt{
		BaseNode: bn(),
		Stmts:    []Stmt{&ExprStmt{BaseNode: bn(), X: &CallExpr{BaseNode: bn(), Fn: &Ident{BaseNode: bn(), Name: "f"}, Args: []Arg{{Value: &IntLit{BaseNode: bn(), Value: 1, Base: 10}}}}}},
	}
	tree := &FnDecl{
		BaseNode: bn(),
		Name:     "outer",
		Sig:      &FnSig{BaseNode: bn()},
		Body:     innerBlock,
	}
	out := Print(tree)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	for i, line := range lines {
		depth := 0
		for _, l := range lines[:i] {
			if strings.HasPrefix(l, strings.Repeat("  ", depth)) && !strings.HasPrefix(l, strings.Repeat("  ", depth+1)) {
			}
		}
		_ = depth
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		if indent%2 != 0 {
			t.Errorf("line %d has odd indent (%d spaces): %q", i, indent, line)
		}
	}
}

func TestFprint_MatchesPrint(t *testing.T) {
	tree := goldenTree()
	fromPrint := Print(tree)
	var sb strings.Builder
	err := Fprint(&sb, tree)
	if err != nil {
		t.Fatalf("Fprint returned error: %v", err)
	}
	if fromPrint != sb.String() {
		t.Errorf("Print and Fprint output differ:\nPrint:  %q\nFprint: %q", fromPrint, sb.String())
	}
}
