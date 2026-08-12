package types

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/errors"
	"github.com/dotlang/dot/parser"
)

// --- collectImpl / resolveTraitRef (via source) ------------------------------

func TestCollectImpl_Inherent(t *testing.T) {
	t.Run("registers_methods", func(t *testing.T) {
		info := checkDot(t, `struct Point {
    x int
}

impl Point {
    fn origin() -> Point { return Point { x: 0 } }
    fn get_x(self) -> int { return self.x }
}

fn main() {}`)
		sym, ok := info.FileScope.Lookup("Point")
		if !ok {
			t.Fatal("Point not declared")
		}
		named := sym.Named
		if named == nil {
			t.Fatal("nil Named")
		}
		origin, ok := named.Methods["origin"]
		if !ok {
			t.Fatal("origin not registered")
		}
		if !origin.Static {
			t.Error("origin should be static")
		}
		if origin.Sig == nil || origin.Sig.Recv != nil {
			t.Error("origin should have no receiver")
		}
		getX, ok := named.Methods["get_x"]
		if !ok {
			t.Fatal("get_x not registered")
		}
		if getX.Static {
			t.Error("get_x should not be static")
		}
		if getX.Sig == nil || getX.Sig.Recv == nil {
			t.Error("get_x should have a receiver")
		}
		if !getX.HasBody {
			t.Error("get_x should have a body")
		}
	})
	t.Run("expression_body", func(t *testing.T) {
		info := checkDot(t, `struct Point {
    x int
}

impl Point {
    fn doubled(self) -> int = self.x * 2
}

fn main() {}`)
		sym, _ := info.FileScope.Lookup("Point")
		doubled, ok := sym.Named.Methods["doubled"]
		if !ok {
			t.Fatal("doubled not registered")
		}
		if !doubled.HasBody {
			t.Error("doubled should report HasBody for an expression body")
		}
	})
}

func TestCollectImpl_TraitImpl(t *testing.T) {
	info := checkDot(t, `trait Printable {
    fn to_string(self) -> string
}

struct Point {
    x int
}

impl Printable for Point {
    fn to_string(self) -> string = "p"
}

fn main() {}`)
	sym, ok := info.FileScope.Lookup("Point")
	if !ok {
		t.Fatal("Point not declared")
	}
	named := sym.Named
	if len(named.Impls) != 1 {
		t.Fatalf("Impls = %d, want 1", len(named.Impls))
	}
	impl := named.Impls[0]
	if impl.Trait == nil || impl.Trait.Name != "Printable" {
		t.Errorf("Trait = %v, want Printable", impl.Trait)
	}
	if impl.Target != named {
		t.Error("Target should be Point's Named")
	}
	if _, ok := impl.Methods["to_string"]; !ok {
		t.Error("to_string missing from impl Methods")
	}
	if _, ok := named.Methods["to_string"]; ok {
		t.Error("trait method should not land in inherent Methods")
	}
}

func TestCollectImpl_DuplicateMethod(t *testing.T) {
	checkDotError(t, `struct P { }

impl P {
    fn f(self) { }
}

impl P {
    fn f(self) { }
}

fn main() {}`, "declared more than once")
}

func TestCollectImpl_NonNamedTarget(t *testing.T) {
	checkDotError(t, `impl int {
    fn f(self) { }
}

fn main() {}`, "cannot implement methods on int")
}

func TestCollectImpl_UndefinedTrait(t *testing.T) {
	checkDotError(t, `struct P { }

impl Missing for P { }

fn main() {}`, "undefined trait")
}

func TestCollectImpl_NotATrait(t *testing.T) {
	checkDotError(t, `struct P { }

impl int for P { }

fn main() {}`, "is not a trait")
}

func TestCollectImpl_NonNameBeforeFor(t *testing.T) {
	checkDotError(t, `struct P { }

impl Option[int] for P { }

fn main() {}`, "expected a trait name before 'for'")
}

func TestCollectImpl_MissingMethod(t *testing.T) {
	checkDotError(t, `trait Printable {
    fn to_string(self) -> string
}

struct P { }

impl Printable for P { }

fn main() {}`, "missing method")
}

func TestCollectImpl_DefaultMethodCovers(t *testing.T) {
	checkDot(t, `trait Printable {
    fn to_string(self) -> string = "?"
}

struct P { }

impl Printable for P { }

fn main() {}`)
}

func TestCollectImpl_ExtraMethod(t *testing.T) {
	checkDotError(t, `trait Printable {
    fn to_string(self) -> string
}

struct P { }

impl Printable for P {
    fn to_string(self) -> string = "p"
    fn extra(self) { }
}

fn main() {}`, "is not a method of trait")
}

func TestCollectImpl_WrongSignature(t *testing.T) {
	checkDotError(t, `trait Printable {
    fn to_string(self) -> string
}

struct P { }

impl Printable for P {
    fn to_string(self) -> int { return 1 }
}

fn main() {}`, "wrong signature")
}

// --- checkConformance (direct) ------------------------------------------------

func TestCheckConformance(t *testing.T) {
	selfParam := &TypeParam{Name: "Self"}
	recv := &Named{Name: "P"}
	newTrait := func(name string, ms []Method) *Trait {
		tr := &Trait{Name: name, Methods: ms}
		tr.byName = make(map[string]*Method, len(ms))
		for i := range tr.Methods {
			tr.byName[tr.Methods[i].Name] = &tr.Methods[i]
		}
		return tr
	}
	traitSig := func() *Fn {
		return &Fn{Recv: selfParam, Params: []Param{{Name: "x", Type: Int}}, Result: Bool}
	}
	match := func() *Method {
		return &Method{Name: "m", Decl: &ast.FnDecl{}, Sig: &Fn{Recv: recv, Params: []Param{{Name: "x", Type: Int}}, Result: Bool}}
	}
	wrongSig := func() *Method {
		return &Method{Name: "m", Decl: &ast.FnDecl{}, Sig: &Fn{Recv: recv, Params: []Param{{Name: "x", Type: String_}}, Result: Bool}}
	}

	tests := []struct {
		name    string
		trait   *Trait
		methods map[string]*Method
		wantN   int
		wantMsg string
	}{
		{"nil_trait", nil, map[string]*Method{}, 0, ""},
		{"matching", newTrait("Tr", []Method{{Name: "m", Sig: traitSig()}}), map[string]*Method{"m": match()}, 0, ""},
		{"missing_method", newTrait("Tr", []Method{{Name: "m", Sig: traitSig()}}), map[string]*Method{}, 1, "missing method"},
		{"default_method_covers", newTrait("Tr", []Method{{Name: "m", Sig: traitSig(), HasBody: true}}), map[string]*Method{}, 0, ""},
		{"wrong_signature", newTrait("Tr", []Method{{Name: "m", Sig: traitSig()}}), map[string]*Method{"m": wrongSig()}, 1, "wrong signature"},
		{"extra_method", newTrait("Tr", []Method{{Name: "m", Sig: traitSig()}}), map[string]*Method{"m": match(), "n": match()}, 1, "is not a method of trait"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newChecker()
			c.checkConformance(&ast.ImplDecl{}, &Named{Name: "P"}, tt.trait, tt.methods)
			if n := c.info.Diagnostics.Len(); n != tt.wantN {
				t.Errorf("diagnostics = %d, want %d (%s)", n, tt.wantN, c.info.Diagnostics.Error())
			}
			if tt.wantMsg != "" {
				d := c.info.Diagnostics.Errors[0].(*errors.Diagnostic)
				if !strings.Contains(d.Message, tt.wantMsg) {
					t.Errorf("message = %q, want containing %q", d.Message, tt.wantMsg)
				}
			}
		})
	}
}

// --- signaturesMatch (direct) --------------------------------------------------

func TestSignaturesMatch(t *testing.T) {
	self := &TypeParam{Name: "Self"}
	named := &Named{Name: "P"}
	base := func() *Fn {
		return &Fn{Recv: named, Params: []Param{{Name: "x", Type: Int}}, Result: Bool}
	}

	tests := []struct {
		name string
		want *Fn
		got  *Fn
		ok   bool
	}{
		{"both_nil", nil, nil, true},
		{"want_nil", nil, base(), false},
		{"got_nil", base(), nil, false},
		{"identical", base(), base(), true},
		{"want_no_recv", &Fn{Params: []Param{{Type: Int}}, Result: Bool}, base(), false},
		{"got_no_recv", base(), &Fn{Params: []Param{{Type: Int}}, Result: Bool}, false},
		{"recv_types_ignored", &Fn{Recv: self, Params: []Param{{Type: Int}}, Result: Bool}, base(), true},
		{"param_count", &Fn{Recv: named, Params: []Param{{Type: Int}, {Type: Int}}, Result: Bool}, base(), false},
		{"param_type", &Fn{Recv: named, Params: []Param{{Type: String_}}, Result: Bool}, base(), false},
		{"want_self_param", &Fn{Recv: named, Params: []Param{{Type: self}}, Result: Bool}, base(), true},
		{"want_self_result", &Fn{Recv: named, Params: []Param{{Type: Int}}, Result: self}, base(), true},
		{"got_self_param", &Fn{Recv: named, Params: []Param{{Type: Int}}, Result: Bool}, &Fn{Recv: named, Params: []Param{{Type: self}}, Result: Bool}, false},
		{"result_mismatch", &Fn{Recv: named, Params: []Param{{Type: Int}}, Result: String_}, base(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := signaturesMatch(tt.want, tt.got); got != tt.ok {
				t.Errorf("signaturesMatch = %v, want %v", got, tt.ok)
			}
		})
	}
}

// --- compatibleParam (direct) ---------------------------------------------------

func TestCompatibleParam(t *testing.T) {
	self := &TypeParam{Name: "Self"}
	tests := []struct {
		name string
		want Type
		got  Type
		ok   bool
	}{
		{"both_nil", nil, nil, true},
		{"want_nil", nil, Int, false},
		{"got_nil", Int, nil, false},
		{"self_wildcard", self, Int, true},
		{"self_wildcard_struct", self, &Struct{}, true},
		{"identical", Int, Int, true},
		{"different", Int, Float, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compatibleParam(tt.want, tt.got); got != tt.ok {
				t.Errorf("compatibleParam(%v, %v) = %v, want %v", tt.want, tt.got, got, tt.ok)
			}
		})
	}
}

// --- AnalyzeEscapes --------------------------------------------------------------

type exprCollector struct {
	exprs []ast.Expr
}

func (v *exprCollector) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}
	if e, ok := node.(ast.Expr); ok {
		v.exprs = append(v.exprs, e)
	}
	return v
}

func escapesOf(t *testing.T, src string) []Escape {
	t.Helper()
	prog, err := parser.ParseFile(src, "<escape>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := &Info{}
	AnalyzeEscapes(prog, info)
	collect := &exprCollector{}
	ast.Walk(collect, prog)
	out := make([]Escape, len(collect.exprs))
	for i, e := range collect.exprs {
		esc, ok := info.Escapes[e]
		if !ok {
			t.Fatalf("expr %d (%s) has no escape info", i, ast.NodeName(e))
		}
		out[i] = esc
	}
	return out
}

func TestAnalyzeEscapes(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []Escape
	}{
		{"global_var_escapes", `s = "hello"
fn main() {}`, []Escape{Heap, Heap}},
		{"global_nested_expr", `g = 1 + 2
fn main() {}`, []Escape{Heap, Heap, Heap, Heap}},
		{"local_vars_stack", `fn main() { x = 1 }`, []Escape{Stack, Stack}},
		{"return_escapes", `fn main() {
    x = 1
    return x + 1
}`, []Escape{Stack, Stack, Heap, Heap, Heap}},
		{"escape_resets_between_fns", `fn a() { y = 1 }
fn b() { return 2 }`, []Escape{Stack, Stack, Heap}},
		{"nested_return", `fn main() { if true { return 1 } }`, []Escape{Stack, Stack, Heap}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapesOf(t, tt.src)
			if len(got) != len(tt.want) {
				t.Fatalf("escape sequence = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("expr %d: escape = %v, want %v (full: %v vs %v)",
						i, got[i], tt.want[i], got, tt.want)
				}
			}
		})
	}
}

func TestAnalyzeEscapes_NilProgram(t *testing.T) {
	info := &Info{}
	AnalyzeEscapes(nil, info)
	if info.Escapes == nil {
		t.Error("Escapes map should be initialised even for a nil program")
	}
	if len(info.Escapes) != 0 {
		t.Errorf("Escapes = %d entries, want 0", len(info.Escapes))
	}
}

// --- checkFor / checkForIn -------------------------------------------------------

func TestCheckFor_Extra(t *testing.T) {
	t.Run("iterate_int_value", func(t *testing.T) {
		checkDot(t, "fn main() { for i in 5 { print(i) } }")
	})
	t.Run("iterate_float_error", func(t *testing.T) {
		checkDotError(t, "fn main() { for i in 1.5 { } }", "cannot iterate over")
	})
	t.Run("iterate_struct_error", func(t *testing.T) {
		checkDotError(t, "struct P { x int }\n fn main() { p = P { x: 1 }\n for i in p { } }", "cannot iterate over")
	})
	t.Run("single_var_map_yields_keys", func(t *testing.T) {
		checkDot(t, "fn main() { m = map[string]int{\"a\": 1}\n for k in m { s string = k } }")
	})
	t.Run("single_var_map_wrong_key_type", func(t *testing.T) {
		checkDotError(t, "fn main() { m = map[string]int{\"a\": 1}\n for k in m { n int = k } }", "cannot use")
	})
	t.Run("two_var_map", func(t *testing.T) {
		checkDot(t, "fn main() { m = map[string]int{\"a\": 1}\n for k, v in m { s string = k\n i int = v } }")
	})
}

func TestIterationTypes(t *testing.T) {
	c := &Checker{}
	namedSlice := &Named{Name: "List", Underlying: &Slice{Elem: String_}}
	tests := []struct {
		name         string
		typ          Type
		wantK, wantV Type
		wantOK       bool
	}{
		{"slice", &Slice{Elem: Int}, Int, Int, true},
		{"named_slice", namedSlice, Int, String_, true},
		{"array", &Array{Elem: Bool, Len: 3}, Int, Bool, true},
		{"map", &Map{Key: String_, Value: Int}, String_, Int, true},
		{"chan", &Chan{Elem: Float}, Int, Float, true},
		{"string", String_, Int, Rune, true},
		{"integer", Int, Int, Int, true},
		{"int8", Int8, Int, Int8, true},
		{"float", Float, Invalid, Invalid, false},
		{"bool", Bool, Invalid, Invalid, false},
		{"struct", &Struct{}, Invalid, Invalid, false},
		{"invalid", Invalid, Invalid, Invalid, true},
		{"nil", nil, Invalid, Invalid, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, v, ok := c.iterationTypes(tt.typ)
			if ok != tt.wantOK || k != tt.wantK || v != tt.wantV {
				t.Errorf("iterationTypes(%v) = (%v, %v, %v), want (%v, %v, %v)",
					tt.typ, k, v, ok, tt.wantK, tt.wantV, tt.wantOK)
			}
		})
	}
}

// --- bindLoopVar ------------------------------------------------------------------

func TestBindLoopVar(t *testing.T) {
	t.Run("ident", func(t *testing.T) {
		c := newChecker()
		id := &ast.Ident{Name: "i"}
		c.bindLoopVar(id, Int)
		sym, ok := c.scope.Lookup("i")
		if !ok {
			t.Fatal("loop var not declared")
		}
		if sym.Kind != SymVar {
			t.Errorf("kind = %s, want SymVar", sym.Kind)
		}
		if !sym.Mutable {
			t.Error("loop var should be mutable")
		}
		if sym.Type != Int {
			t.Errorf("type = %v, want int", sym.Type)
		}
		if got := c.info.TypeOf(id); got != Int {
			t.Errorf("TypeOf = %v, want int", got)
		}
	})
	t.Run("private", func(t *testing.T) {
		c := newChecker()
		c.bindLoopVar(&ast.Ident{Name: "_i"}, Int)
		sym, ok := c.scope.Lookup("_i")
		if !ok {
			t.Fatal("loop var not declared")
		}
		if !sym.Private {
			t.Error("loop var starting with '_' should be private")
		}
	})
	t.Run("non_ident_ignored", func(t *testing.T) {
		c := newChecker()
		c.bindLoopVar(&ast.IntLit{}, Int)
		if n := len(c.scope.Names()); n != 0 {
			t.Errorf("scope has %d names, want 0", n)
		}
	})
	t.Run("nil", func(t *testing.T) {
		c := newChecker()
		c.bindLoopVar(nil, Int)
		if n := len(c.scope.Names()); n != 0 {
			t.Errorf("scope has %d names, want 0", n)
		}
	})
}

// --- blockTerminates ----------------------------------------------------------------

func TestBlockTerminates(t *testing.T) {
	c := newChecker()
	terminating := &ast.ReturnStmt{}
	c.info.Terminates[terminating] = true
	lit := &ast.IntLit{}
	esInt := &ast.ExprStmt{X: lit}
	c.info.Types[lit] = Int
	voidLit := &ast.StringLit{}
	esVoid := &ast.ExprStmt{X: voidLit}
	c.info.Types[voidLit] = Void
	invLit := &ast.IntLit{}
	esInvalid := &ast.ExprStmt{X: invLit}
	c.info.Types[invLit] = Invalid
	plain := &ast.BreakStmt{}

	tests := []struct {
		name  string
		block *ast.BlockStmt
		want  bool
	}{
		{"nil", nil, false},
		{"empty", &ast.BlockStmt{}, false},
		{"terminates", &ast.BlockStmt{Stmts: []ast.Stmt{terminating}}, true},
		{"plain_stmt", &ast.BlockStmt{Stmts: []ast.Stmt{plain}}, false},
		{"trailing_expr", &ast.BlockStmt{Stmts: []ast.Stmt{esInt}}, true},
		{"trailing_void_expr", &ast.BlockStmt{Stmts: []ast.Stmt{esVoid}}, false},
		{"trailing_invalid_expr", &ast.BlockStmt{Stmts: []ast.Stmt{esInvalid}}, false},
		{"only_last_counts", &ast.BlockStmt{Stmts: []ast.Stmt{terminating, plain}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.blockTerminates(tt.block); got != tt.want {
				t.Errorf("blockTerminates = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- requireBool ----------------------------------------------------------------------

func TestRequireBool(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want int
	}{
		{"bool", Bool, 0},
		{"named_bool", &Named{Name: "Flag", Underlying: Bool}, 0},
		{"int", Int, 1},
		{"string", String_, 1},
		{"invalid", Invalid, 0},
		{"never", Never, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newChecker()
			c.requireBool(&ast.BoolLit{}, tt.typ, "condition")
			if n := c.info.Diagnostics.Len(); n != tt.want {
				t.Errorf("diagnostics = %d, want %d (%s)", n, tt.want, c.info.Diagnostics.Error())
			}
		})
	}
}

// --- assignCompatible -------------------------------------------------------------------

func TestAssignCompatible(t *testing.T) {
	t.Run("same_type", func(t *testing.T) {
		c := newChecker()
		c.assignCompatible(&ast.Ident{}, Int, Int, "assignment")
		if n := c.info.Diagnostics.Len(); n != 0 {
			t.Errorf("diagnostics = %d, want 0", n)
		}
	})
	t.Run("mismatch", func(t *testing.T) {
		c := newChecker()
		c.assignCompatible(&ast.Ident{}, Int, String_, "assignment")
		if n := c.info.Diagnostics.Len(); n != 1 {
			t.Fatalf("diagnostics = %d, want 1", n)
		}
		d := c.info.Diagnostics.Errors[0].(*errors.Diagnostic)
		if !strings.Contains(d.Message, "cannot use int as string") {
			t.Errorf("message = %q", d.Message)
		}
	})
	t.Run("numeric_hint", func(t *testing.T) {
		c := newChecker()
		c.assignCompatible(&ast.Ident{}, Int, Float, "assignment")
		d := c.info.Diagnostics.Errors[0].(*errors.Diagnostic)
		if !strings.Contains(d.Hint, "as float") {
			t.Errorf("hint = %q, want to contain \"as float\"", d.Hint)
		}
	})
	t.Run("nil_got", func(t *testing.T) {
		c := newChecker()
		c.assignCompatible(&ast.Ident{}, nil, Int, "assignment")
		if n := c.info.Diagnostics.Len(); n != 0 {
			t.Errorf("diagnostics = %d, want 0", n)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		c := newChecker()
		c.assignCompatible(&ast.Ident{}, Invalid, Int, "assignment")
		if n := c.info.Diagnostics.Len(); n != 0 {
			t.Errorf("diagnostics = %d, want 0", n)
		}
	})
	t.Run("never", func(t *testing.T) {
		c := newChecker()
		c.assignCompatible(&ast.Ident{}, Never, Int, "assignment")
		if n := c.info.Diagnostics.Len(); n != 0 {
			t.Errorf("diagnostics = %d, want 0", n)
		}
	})
	t.Run("unbound_var_got", func(t *testing.T) {
		c := newChecker()
		tv := &TypeVar{ID: 1}
		c.assignCompatible(&ast.Ident{}, tv, Int, "assignment")
		if n := c.info.Diagnostics.Len(); n != 0 {
			t.Errorf("diagnostics = %d, want 0", n)
		}
		if tv.Bound != Int {
			t.Errorf("inference var should bind to int, got %v", tv.Bound)
		}
	})
	t.Run("bound_var_mismatch", func(t *testing.T) {
		c := newChecker()
		tv := &TypeVar{ID: 2, Bound: String_}
		c.assignCompatible(&ast.Ident{}, tv, Int, "assignment")
		if n := c.info.Diagnostics.Len(); n != 1 {
			t.Errorf("diagnostics = %d, want 1", n)
		}
	})
	t.Run("unbound_var_want", func(t *testing.T) {
		c := newChecker()
		tv := &TypeVar{ID: 3}
		c.assignCompatible(&ast.Ident{}, Int, tv, "assignment")
		if n := c.info.Diagnostics.Len(); n != 0 {
			t.Errorf("diagnostics = %d, want 0", n)
		}
		if tv.Bound != Int {
			t.Errorf("inference var should bind to int, got %v", tv.Bound)
		}
	})
}
