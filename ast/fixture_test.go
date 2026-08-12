package ast

import "github.com/dotlang/dot/lexer"

func pos(line, col, off int) Position {
	return Position{File: "test.dot", Line: line, Column: col, Offset: off}
}

func bn() BaseNode {
	return BaseNode{Start: pos(1, 1, 0), Stop: pos(1, 2, 1)}
}

func identNode() *Ident         { return &Ident{BaseNode: bn(), Name: "x"} }
func intLitNode() *IntLit       { return &IntLit{BaseNode: bn(), Value: 1, Base: 10} }
func namedTypeNode() *NamedType { return &NamedType{BaseNode: bn(), Name: "int"} }
func blockStmtNode() *BlockStmt {
	return &BlockStmt{BaseNode: bn(), Stmts: []Stmt{&ExprStmt{BaseNode: bn(), X: identNode()}}}
}
func matchArmNode() *MatchArm {
	return &MatchArm{BaseNode: bn(), Pattern: &IdentPattern{BaseNode: bn(), Name: "x"}, Body: intLitNode()}
}
func fnSigNode() *FnSig {
	return &FnSig{BaseNode: bn(), Params: []*Param{{BaseNode: bn(), Name: "p", Type: namedTypeNode()}}, Result: namedTypeNode()}
}

func allFixtures() map[string]Node {
	p := namedTypeNode()
	i := identNode()
	n := intLitNode()
	b := blockStmtNode()
	ma := matchArmNode()
	sig := fnSigNode()

	return map[string]Node{
		// --- expressions (walk_expr.go) ---
		"ParenExpr":      &ParenExpr{BaseNode: bn(), X: i},
		"UnaryExpr":      &UnaryExpr{BaseNode: bn(), Op: lexer.TokenMinus, X: n},
		"BinaryExpr":     &BinaryExpr{BaseNode: bn(), Op: lexer.TokenPlus, X: n, Y: n},
		"AssignExpr":     &AssignExpr{BaseNode: bn(), Op: lexer.TokenAssign, Targets: []Expr{i}, Values: []Expr{n}},
		"RangeExpr":      &RangeExpr{BaseNode: bn(), Low: n, High: n},
		"CallExpr":       &CallExpr{BaseNode: bn(), Fn: i, Args: []Arg{{Value: n}}},
		"IndexExpr":      &IndexExpr{BaseNode: bn(), X: i, Indices: []Expr{n}},
		"SliceExpr":      &SliceExpr{BaseNode: bn(), X: i, Low: n, High: n},
		"FieldExpr":      &FieldExpr{BaseNode: bn(), X: i, Name: "f"},
		"PipeExpr":       &PipeExpr{BaseNode: bn(), X: n, Fn: i},
		"TryExpr":        &TryExpr{BaseNode: bn(), X: i},
		"AwaitExpr":      &AwaitExpr{BaseNode: bn(), X: i},
		"SpawnExpr":      &SpawnExpr{BaseNode: bn(), Block: b},
		"CastExpr":       &CastExpr{BaseNode: bn(), X: i, Type: p},
		"IsExpr":         &IsExpr{BaseNode: bn(), X: i, Type: p},
		"IfExpr":         &IfExpr{BaseNode: bn(), Cond: i, Then: b},
		"MatchExpr":      &MatchExpr{BaseNode: bn(), Subject: i, Arms: []*MatchArm{ma}},
		"BlockExpr":      &BlockExpr{BaseNode: bn(), Block: b},
		"IntLit":         n,
		"FloatLit":       &FloatLit{BaseNode: bn(), Value: 1.5},
		"RawStringLit":   &RawStringLit{BaseNode: bn(), Value: "raw"},
		"BoolLit":        &BoolLit{BaseNode: bn(), Value: true},
		"NilLit":         &NilLit{BaseNode: bn()},
		"Ident":          i,
		"UnderscoreExpr": &UnderscoreExpr{BaseNode: bn()},
		"SelfExpr":       &SelfExpr{BaseNode: bn()},
		"BadExpr":        &BadExpr{BaseNode: bn()},
		"StringLit":      &StringLit{BaseNode: bn(), Parts: []StringPart{{Kind: PartExpr, Expr: i}}},
		"TupleLit":       &TupleLit{BaseNode: bn(), Elems: []TupleElem{{Value: n}}},
		"ArrayLit":       &ArrayLit{BaseNode: bn(), Elems: []Expr{n}},
		"MapLit":         &MapLit{BaseNode: bn(), Entries: []MapEntry{{Key: i, Value: n}}},
		"StructLit":      &StructLit{BaseNode: bn(), Type: p, Fields: []StructLitField{{Name: "f", Value: n}}},
		"FnLit":          &FnLit{BaseNode: bn(), Sig: sig, Body: b},

		// --- types (walk_type.go walkType) ---
		"NamedType":    p,
		"GenericType":  &GenericType{BaseNode: bn(), Base: p, Args: []Type{p}},
		"SliceType":    &SliceType{BaseNode: bn(), Elem: p},
		"ArrayType":    &ArrayType{BaseNode: bn(), Len: n, Elem: p},
		"MapType":      &MapType{BaseNode: bn(), Key: p, Value: p},
		"FnType":       &FnType{BaseNode: bn(), Params: []Type{p}, Result: p},
		"TupleType":    &TupleType{BaseNode: bn(), Elems: []Type{p}},
		"PointerType":  &PointerType{BaseNode: bn(), Elem: p},
		"DynType":      &DynType{BaseNode: bn(), Trait: p},
		"WeakType":     &WeakType{BaseNode: bn(), Elem: p},
		"OptionalType": &OptionalType{BaseNode: bn(), Elem: p},
		"SelfTypeNode": &SelfTypeNode{BaseNode: bn()},
		"BadType":      &BadType{BaseNode: bn()},

		// --- patterns (walk_type.go walkPattern) ---
		"LiteralPattern":  &LiteralPattern{BaseNode: bn(), Value: n},
		"IdentPattern":    &IdentPattern{BaseNode: bn(), Name: "x"},
		"WildcardPattern": &WildcardPattern{BaseNode: bn()},
		"OrPattern":       &OrPattern{BaseNode: bn(), Alts: []Pattern{&IdentPattern{BaseNode: bn(), Name: "a"}, &WildcardPattern{BaseNode: bn()}}},
		"GuardedPattern":  &GuardedPattern{BaseNode: bn(), Pattern: &IdentPattern{BaseNode: bn(), Name: "n"}, Guard: i},
		"TypePattern":     &TypePattern{BaseNode: bn(), Type: p, Binding: &IdentPattern{BaseNode: bn(), Name: "v"}},
		"EnumPattern":     &EnumPattern{BaseNode: bn(), Enum: "Color", Variant: "Red", HasArgs: true, Args: []Pattern{&IdentPattern{BaseNode: bn(), Name: "r"}}},
		"StructPattern":   &StructPattern{BaseNode: bn(), Type: p, Fields: []StructPatternField{{Name: "x", Pattern: &IdentPattern{BaseNode: bn(), Name: "a"}}}},
		"TuplePattern":    &TuplePattern{BaseNode: bn(), Elems: []Pattern{&IdentPattern{BaseNode: bn(), Name: "a"}}},
		"RangePattern":    &RangePattern{BaseNode: bn(), Low: n, High: n},
		"BadPattern":      &BadPattern{BaseNode: bn()},

		// --- statements / decls / helpers (walk_stmt.go) ---
		"Program":      &Program{BaseNode: bn(), Decls: []Decl{&VarDecl{BaseNode: bn(), Names: []Expr{i}, Values: []Expr{n}}}},
		"BlockStmt":    b,
		"ExprStmt":     &ExprStmt{BaseNode: bn(), X: i},
		"ReturnStmt":   &ReturnStmt{BaseNode: bn(), Values: []Expr{i}},
		"BreakStmt":    &BreakStmt{BaseNode: bn()},
		"ContinueStmt": &ContinueStmt{BaseNode: bn()},
		"BadStmt":      &BadStmt{BaseNode: bn()},
		"DeferStmt":    &DeferStmt{BaseNode: bn(), Call: &CallExpr{BaseNode: bn(), Fn: i, Args: []Arg{{Value: n}}}},
		"PerfBlock":    &PerfBlock{BaseNode: bn(), Block: b},
		"VarDecl":      &VarDecl{BaseNode: bn(), Names: []Expr{i}, Values: []Expr{n}},
		"ForStmt":      &ForStmt{BaseNode: bn(), Kind: ForIn, Value: i, Iterable: &RangeExpr{BaseNode: bn(), Low: n, High: n}, Body: b},
		"FnDecl":       &FnDecl{BaseNode: bn(), Name: "f", Sig: sig, Body: b},
		"StructDecl":   &StructDecl{BaseNode: bn(), Name: "S"},
		"EnumDecl":     &EnumDecl{BaseNode: bn(), Name: "E"},
		"TraitDecl":    &TraitDecl{BaseNode: bn(), Name: "T"},
		"ImplDecl":     &ImplDecl{BaseNode: bn(), Type: p},
		"ImportDecl":   &ImportDecl{BaseNode: bn(), Path: []string{"a", "b"}},
		"MatchArm":     ma,
		"FnSig":        sig,
		"Param":        &Param{BaseNode: bn(), Name: "p", Type: p},
		"FieldDecl":    &FieldDecl{BaseNode: bn(), Name: "f", Type: p},
		"EnumVariant":  &EnumVariant{BaseNode: bn(), Name: "V", Fields: []*Param{{BaseNode: bn(), Name: "x", Type: p}}},
		"Annotation":   &Annotation{BaseNode: bn(), Name: "test", Args: []Expr{n}},
		"TypeParam":    &TypeParam{BaseNode: bn(), Name: "T", Bounds: []Type{p}},
	}
}

func walkCaseCount() int {
	return 81
}
