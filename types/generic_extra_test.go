package types

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/errors"
	"github.com/dotlang/dot/parser"
)

// --- getOrAddCache ----------------------------------------------------------------

func TestGetOrAddCache(t *testing.T) {
	tp := &TypeParam{Name: "T", Index: 0}
	generic := &Named{
		Name:       "Box",
		TypeParams: []*TypeParam{tp},
		Underlying: &Struct{Fields: []Field{{Name: "v", Type: tp, Index: 0}}},
	}
	c := newChecker()

	t.Run("caches_same_args", func(t *testing.T) {
		a := c.getOrAddCache(generic, []Type{Int})
		b := c.getOrAddCache(generic, []Type{Int})
		if a == nil {
			t.Fatal("nil instantiation")
		}
		if a != b {
			t.Error("same args should return the same instance pointer")
		}
		if a.Origin != generic {
			t.Error("Origin should point to the generic")
		}
	})
	t.Run("different_args", func(t *testing.T) {
		a := c.getOrAddCache(generic, []Type{Int})
		b := c.getOrAddCache(generic, []Type{String_})
		if a == nil || b == nil {
			t.Fatal("nil instantiation")
		}
		if a == b {
			t.Error("different args should return different instances")
		}
	})
	t.Run("nil_and_empty_args_share_key", func(t *testing.T) {
		a := c.getOrAddCache(generic, nil)
		b := c.getOrAddCache(generic, []Type{})
		if a == nil || b == nil {
			t.Fatal("nil instantiation")
		}
		if a != b {
			t.Error("nil and empty args produce the same cache key")
		}
	})
	t.Run("different_origin", func(t *testing.T) {
		other := &Named{
			Name:       "Box",
			TypeParams: []*TypeParam{{Name: "T", Index: 0}},
			Underlying: &Struct{},
		}
		a := c.getOrAddCache(generic, []Type{Int})
		b := c.getOrAddCache(other, []Type{Int})
		if a == nil || b == nil {
			t.Fatal("nil instantiation")
		}
		if a == b {
			t.Error("different origin pointers must not collide")
		}
	})
	t.Run("lazy_cache_init", func(t *testing.T) {
		c := newChecker()
		c.instantiateCache = nil
		a := c.getOrAddCache(generic, []Type{Int})
		if a == nil || a.Origin != generic {
			t.Error("nil cache should be initialised lazily")
		}
	})
}

// --- typeListString ----------------------------------------------------------------

func TestTypeListString(t *testing.T) {
	u := NewUniverse()
	tests := []struct {
		name string
		ts   []Type
		want string
	}{
		{"nil", nil, ""},
		{"empty", []Type{}, ""},
		{"single", []Type{Int}, "int"},
		{"multiple", []Type{Int, String_}, "int,string"},
		{"nil_element", []Type{nil}, "<nil>"},
		{"mixed", []Type{Int, nil, Bool}, "int,<nil>,bool"},
		{"named", []Type{u.OptionOf(Int)}, "Option[int]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := typeListString(tt.ts); got != tt.want {
				t.Errorf("typeListString = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- checkBounds -------------------------------------------------------------------

func TestCheckBounds(t *testing.T) {
	pos := ast.Position{File: "t.dot", Line: 1, Offset: 1}
	tr := &Trait{Name: "Printable"}
	trWithPos := &Trait{Name: "Located", Methods: []Method{
		{Name: "m", Pos: ast.Position{File: "lib.dot", Line: 3, Offset: 7}},
	}}

	tests := []struct {
		name      string
		tp        *TypeParam
		arg       Type
		wantOK    bool
		wantDiags int
		wantMsg   string
		wantHint  string
	}{
		{"no_bounds", &TypeParam{Name: "T"}, Int, true, 0, "", ""},
		{"impl_via_named", &TypeParam{Name: "T", Bounds: []*Trait{tr}},
			&Named{Name: "S", Impls: []*TraitImpl{{Trait: tr}}}, true, 0, "", ""},
		{"impl_via_typeparam_bound", &TypeParam{Name: "T", Bounds: []*Trait{tr}},
			&TypeParam{Name: "U", Bounds: []*Trait{tr}}, true, 0, "", ""},
		{"trait_itself", &TypeParam{Name: "T", Bounds: []*Trait{tr}}, tr, true, 0, "", ""},
		{"violation", &TypeParam{Name: "T", Bounds: []*Trait{tr}}, Int,
			false, 1, "does not implement Printable", ""},
		{"violation_with_hint", &TypeParam{Name: "T", Bounds: []*Trait{trWithPos}}, Int,
			false, 1, "does not implement Located", "declared at lib.dot:3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newChecker()
			ok := checkBounds(tt.tp, tt.arg, pos)
			if ok != tt.wantOK {
				t.Errorf("checkBounds = %v, want %v", ok, tt.wantOK)
			}
			if n := c.info.Diagnostics.Len(); n != tt.wantDiags {
				t.Fatalf("diagnostics = %d, want %d", n, tt.wantDiags)
			}
			if tt.wantMsg != "" {
				d := c.info.Diagnostics.Errors[0].(*errors.Diagnostic)
				if !strings.Contains(d.Message, tt.wantMsg) {
					t.Errorf("message = %q, want containing %q", d.Message, tt.wantMsg)
				}
				if tt.wantHint != "" && !strings.Contains(d.Hint, tt.wantHint) {
					t.Errorf("hint = %q, want containing %q", d.Hint, tt.wantHint)
				}
			}
		})
	}
}

// --- checkObjectSafe ----------------------------------------------------------------

func TestCheckObjectSafe(t *testing.T) {
	pos := ast.Position{File: "t.dot"}
	recv := &Named{Name: "S"}

	tests := []struct {
		name      string
		tr        *Trait
		wantOK    bool
		wantDiags int
		wantMsg   string
	}{
		{"empty_trait", &Trait{Name: "Empty"}, true, 0, ""},
		{"object_safe", &Trait{Name: "Safe", Methods: []Method{
			{Name: "f", Sig: &Fn{Recv: recv, Params: []Param{{Type: Int}}, Result: Int}},
		}}, true, 0, ""},
		{"static_method", &Trait{Name: "Static", Methods: []Method{
			{Name: "make", Static: true, Sig: &Fn{Result: Int}},
		}}, false, 1, "static method"},
		{"self_param", &Trait{Name: "TakesSelf", Methods: []Method{
			{Name: "clone", Sig: &Fn{Recv: recv, Params: []Param{{Type: &TypeParam{Name: "Self"}}}, Result: Void}},
		}}, false, 1, "takes a 'Self' parameter"},
		{"generic_method", &Trait{Name: "Generic", Methods: []Method{
			{Name: "id", Sig: &Fn{Recv: recv, Result: Void, TypeParams: []*TypeParam{{Name: "U"}}}},
		}}, false, 1, "is generic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newChecker()
			ok := checkObjectSafe(tt.tr, pos)
			if ok != tt.wantOK {
				t.Errorf("checkObjectSafe = %v, want %v", ok, tt.wantOK)
			}
			if n := c.info.Diagnostics.Len(); n != tt.wantDiags {
				t.Fatalf("diagnostics = %d, want %d", n, tt.wantDiags)
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

// --- mentionsSelf --------------------------------------------------------------------

func TestMentionsSelf(t *testing.T) {
	self := &TypeParam{Name: "Self"}
	other := &TypeParam{Name: "T"}
	tests := []struct {
		name string
		sig  *Fn
		want bool
	}{
		{"nil", nil, false},
		{"no_self", &Fn{Params: []Param{{Type: Int}}, Result: Bool}, false},
		{"param_self", &Fn{Params: []Param{{Type: self}}, Result: Void}, true},
		{"result_self", &Fn{Params: []Param{{Type: Int}}, Result: self}, true},
		{"recv_only_self", &Fn{Recv: self, Result: Void}, false},
		{"nested_param", &Fn{Params: []Param{{Type: &Slice{Elem: self}}}, Result: Void}, true},
		{"other_param", &Fn{Params: []Param{{Type: other}}, Result: Void}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mentionsSelf(tt.sig, self); got != tt.want {
				t.Errorf("mentionsSelf = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- typeContainsParam ----------------------------------------------------------------

func TestTypeContainsParam(t *testing.T) {
	p := &TypeParam{Name: "Self"}
	q := &TypeParam{Name: "T"}
	tests := []struct {
		name string
		typ  Type
		want bool
	}{
		{"nil", nil, false},
		{"same_param", p, true},
		{"different_param", q, false},
		{"basic", Int, false},
		{"slice", &Slice{Elem: p}, true},
		{"array", &Array{Elem: p, Len: 2}, true},
		{"map_key", &Map{Key: p, Value: Int}, true},
		{"map_value", &Map{Key: Int, Value: p}, true},
		{"pointer", &Pointer{Elem: p}, true},
		{"weak", &Weak{Elem: p}, true},
		{"chan", &Chan{Elem: p}, true},
		{"future", &Future{Result: p}, true},
		{"tuple", &Tuple{Elems: []Type{Int, p}}, true},
		{"fn_param", &Fn{Params: []Param{{Type: p}}, Result: Void}, true},
		{"fn_result", &Fn{Params: []Param{{Type: Int}}, Result: p}, true},
		{"struct_field", &Struct{Fields: []Field{{Name: "x", Type: p}}}, true},
		{"enum_variant", &Enum{Variants: []Variant{{Name: "V", Fields: []Param{{Type: p}}}}}, true},
		{"named_args", &Named{Name: "Box", TypeArgs: []Type{p}}, true},
		{"plain_named", &Named{Name: "Box"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := typeContainsParam(tt.typ, p); got != tt.want {
				t.Errorf("typeContainsParam(%v) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}

// --- recordInstance -------------------------------------------------------------------

func TestRecordInstance(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		c := newChecker()
		if got := c.recordInstance(nil); got != nil {
			t.Error("nil instance should return nil")
		}
		if len(c.info.InstanceList) != 0 {
			t.Error("list should stay empty")
		}
	})
	t.Run("first", func(t *testing.T) {
		c := newChecker()
		inst := &Instance{Mangled: "m1", Result: Int}
		if got := c.recordInstance(inst); got != inst {
			t.Error("first record should return the instance itself")
		}
		if c.info.instanceByMangled == nil || c.info.instanceByMangled["m1"] != inst {
			t.Error("instance not indexed by mangled name")
		}
		if len(c.info.InstanceList) != 1 {
			t.Errorf("InstanceList = %d, want 1", len(c.info.InstanceList))
		}
	})
	t.Run("duplicate_mangled", func(t *testing.T) {
		c := newChecker()
		first := c.recordInstance(&Instance{Mangled: "m1", Result: Int})
		got := c.recordInstance(&Instance{Mangled: "m1", Result: String_})
		if got != first {
			t.Error("duplicate mangled name should return the first instance")
		}
		if len(c.info.InstanceList) != 1 {
			t.Errorf("InstanceList = %d, want 1", len(c.info.InstanceList))
		}
	})
	t.Run("distinct_mangled", func(t *testing.T) {
		c := newChecker()
		first := c.recordInstance(&Instance{Mangled: "m1", Result: Int})
		second := c.recordInstance(&Instance{Mangled: "m2", Result: String_})
		if first == second {
			t.Error("distinct mangled names should produce distinct entries")
		}
		if len(c.info.InstanceList) != 2 {
			t.Errorf("InstanceList = %d, want 2", len(c.info.InstanceList))
		}
	})
}

// --- substituteSelf ---------------------------------------------------------------------

func TestSubstituteSelf(t *testing.T) {
	self := &TypeParam{Name: "Self"}
	concrete := Int

	tests := []struct {
		name string
		typ  Type
		want Type
	}{
		{"nil", nil, nil},
		{"self_param", self, Int},
		{"other_param", &TypeParam{Name: "T"}, &TypeParam{Name: "T"}},
		{"basic", Int, Int},
		{"slice", &Slice{Elem: self}, &Slice{Elem: Int}},
		{"array", &Array{Elem: self, Len: 4}, &Array{Elem: Int, Len: 4}},
		{"map", &Map{Key: self, Value: String_}, &Map{Key: Int, Value: String_}},
		{"pointer", &Pointer{Elem: self}, &Pointer{Elem: Int}},
		{"weak", &Weak{Elem: self}, &Weak{Elem: Int}},
		{"chan", &Chan{Elem: self}, &Chan{Elem: Int}},
		{"future", &Future{Result: self}, &Future{Result: Int}},
		{"tuple", &Tuple{Elems: []Type{self, String_}}, &Tuple{Elems: []Type{Int, String_}}},
		{"fn", &Fn{Params: []Param{{Name: "x", Type: self}}, Result: Bool},
			&Fn{Params: []Param{{Name: "x", Type: Int}}, Result: Bool}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteSelf(tt.typ, self, concrete)
			if tt.want == nil {
				if got != nil {
					t.Errorf("substituteSelf = %v, want nil", got)
				}
				return
			}
			if !Identical(got, tt.want) {
				t.Errorf("substituteSelf = %s, want %s", got, tt.want)
			}
		})
	}

	t.Run("fn_preserves_flags", func(t *testing.T) {
		in := &Fn{Params: []Param{{Name: "x", Type: self}}, Result: self, Recv: self,
			RecvMut: true, Async: true, Variadic: true}
		got := substituteSelf(in, self, concrete).(*Fn)
		if got.Params[0].Type != Int || got.Result != Int || got.Recv != Int {
			t.Error("fn types not substituted")
		}
		if !got.RecvMut || !got.Async || !got.Variadic {
			t.Error("fn flags not preserved")
		}
	})
	t.Run("struct_rebuilds_flat", func(t *testing.T) {
		st := &Struct{Fields: []Field{{Name: "x", Type: self, Index: 0}}}
		got := substituteSelf(st, self, concrete).(*Struct)
		if got.Fields[0].Type != Int {
			t.Error("field not substituted")
		}
		if _, ok := got.Flat["x"]; !ok {
			t.Error("Flat map not rebuilt")
		}
	})
	t.Run("enum_rebuilds_byname", func(t *testing.T) {
		en := &Enum{Variants: []Variant{{Name: "V", Fields: []Param{{Type: self}}}}}
		en.byName = variantByName(en)
		got := substituteSelf(en, self, concrete).(*Enum)
		if got.Variants[0].Fields[0].Type != Int {
			t.Error("variant not substituted")
		}
		if _, ok := got.Variant("V"); !ok {
			t.Error("byName not rebuilt")
		}
	})
	t.Run("named_preserves_identity_fields", func(t *testing.T) {
		sym := &Symbol{Name: "Box"}
		origin := &Named{Name: "Box"}
		methods := map[string]*Method{"m": {Name: "m"}}
		n := &Named{Name: "Box", Sym: sym, Origin: origin, TypeArgs: []Type{self},
			Underlying: &Slice{Elem: self}, Methods: methods, Pos: ast.Position{Line: 9}}
		got := substituteSelf(n, self, concrete)
		gn, ok := got.(*Named)
		if !ok {
			t.Fatalf("substituteSelf = %T, want *Named", got)
		}
		if gn.Sym != sym || gn.Origin != origin || gn.Methods["m"] != methods["m"] {
			t.Error("identity fields not preserved")
		}
		if len(gn.TypeArgs) != 1 || gn.TypeArgs[0] != Int {
			t.Errorf("TypeArgs = %v, want [int]", gn.TypeArgs)
		}
		sl, ok := gn.Underlying.(*Slice)
		if !ok || sl.Elem != Int {
			t.Errorf("underlying = %v, want []int", gn.Underlying)
		}
		if gn.Pos.Line != 9 {
			t.Error("position not preserved")
		}
	})
	t.Run("composites_rebuilt_even_without_self", func(t *testing.T) {
		orig := &Slice{Elem: Int}
		got := substituteSelf(orig, self, concrete)
		if got == orig {
			t.Error("composites should be rebuilt even without Self")
		}
		if !Identical(got, orig) {
			t.Error("rebuilt composite should stay identical")
		}
	})
}

// --- mangleType / instanceKey extras ---------------------------------------------------------

func TestMangleType_Extra(t *testing.T) {
	stack := &Named{Name: "Stack"}
	tests := []struct {
		name string
		typ  Type
		want string
	}{
		{"array", NewArray(Int, 5), "Array_5_int"},
		{"pointer", NewPointer(Int), "Ptr_int"},
		{"weak", NewWeak(Int), "Weak_int"},
		{"chan", NewChan(Int), "Chan_int"},
		{"future", NewFuture(Int), "Fut_int"},
		{"tuple", &Tuple{Elems: []Type{Int, String_}}, "Tup_int_string"},
		{"fn", &Fn{Params: []Param{{Type: Int}}, Result: Bool}, "Fn_int_bool"},
		{"named_plain", stack, "Stack"},
		{"named_args_no_origin", &Named{Name: "Stack", TypeArgs: []Type{Int}}, "DotStack__int"},
		{"named_origin", &Named{Name: "Stack", Origin: stack, TypeArgs: []Type{Int}}, "DotStack__int"},
		{"type_param", &TypeParam{Name: "T"}, "T"},
		{"trait", &Trait{Name: "Printable"}, "Printable"},
		{"dyn", &Dyn{Trait: &Trait{Name: "Printable"}}, "dyn_Printable"},
		{"struct", &Struct{}, "struct"},
		{"enum", &Enum{}, "enum"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mangleType(tt.typ); got != tt.want {
				t.Errorf("mangleType = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstanceKey_Extra(t *testing.T) {
	tests := []struct {
		name string
		base string
		args []Type
		want string
	}{
		{"no_args", "F", nil, "DotF__"},
		{"two_args", "F", []Type{Int, String_}, "DotF__int_string"},
		{"named_arg", "F", []Type{&Named{Name: "User"}}, "DotF__User"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := instanceKey(tt.base, tt.args)
			if got != tt.want {
				t.Errorf("instanceKey = %q, want %q", got, tt.want)
			}
			if again := instanceKey(tt.base, tt.args); again != got {
				t.Error("instanceKey must be deterministic")
			}
		})
	}
}

// --- checkCompoundAssign ----------------------------------------------------------------

func TestCheckCompoundAssign(t *testing.T) {
	t.Run("mod_int", func(t *testing.T) {
		checkDot(t, "fn main() { x = 6\n x %= 4 }")
	})
	t.Run("and_int", func(t *testing.T) {
		checkDot(t, "fn main() { x = 5\n x &= 3 }")
	})
	t.Run("or_int", func(t *testing.T) {
		checkDot(t, "fn main() { x = 5\n x |= 2 }")
	})
	t.Run("xor_int", func(t *testing.T) {
		checkDot(t, "fn main() { x = 5\n x ^= 3 }")
	})
	t.Run("shl_int", func(t *testing.T) {
		checkDot(t, "fn main() { x = 1\n x <<= 2 }")
	})
	t.Run("shr_int", func(t *testing.T) {
		checkDot(t, "fn main() { x = 8\n x >>= 2 }")
	})
	t.Run("float_minus", func(t *testing.T) {
		checkDot(t, "fn main() { f = 1.5\n f -= 0.5 }")
	})
	t.Run("bool_plus_error", func(t *testing.T) {
		checkDotError(t, "fn main() { b = true\n b += true }", "requires a numeric target")
	})
	t.Run("string_minus_error", func(t *testing.T) {
		checkDotError(t, "fn main() { s = \"a\"\n s -= \"b\" }", "requires a numeric target")
	})
	t.Run("float_mod_error", func(t *testing.T) {
		checkDotError(t, "fn main() { f = 1.5\n f %= 0.5 }", "requires an integer target")
	})
	t.Run("float_bitand_error", func(t *testing.T) {
		checkDotError(t, "fn main() { f = 1.5\n f &= 2 }", "requires an integer target")
	})
	t.Run("int_plus_string_error", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 1\n x += \"s\" }", "cannot use")
	})
	t.Run("float_plus_string_error", func(t *testing.T) {
		checkDotError(t, "fn main() { f = 1.5\n f += \"s\" }", "cannot use")
	})
	t.Run("string_plus_int_error", func(t *testing.T) {
		checkDotError(t, "fn main() { s = \"a\"\n s += 1 }", "cannot use")
	})
}

// --- checkRange extras ----------------------------------------------------------------------

func TestCheckRange_Extra(t *testing.T) {
	t.Run("open_high", func(t *testing.T) {
		checkDot(t, "fn main() { r = 1.. }")
	})
	t.Run("float_bounds_error", func(t *testing.T) {
		checkDotError(t, "fn main() { r = 1.5..2.5 }", "range bounds must be integers")
	})
	t.Run("string_bounds_error", func(t *testing.T) {
		checkDotError(t, "fn main() { r = \"a\"..\"b\" }", "range bounds must be integers")
	})
}

// --- checkCast extras -------------------------------------------------------------------------

func TestCheckCast_Extra(t *testing.T) {
	t.Run("int_to_bool_error", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 1 as bool }", "cannot convert")
	})
	t.Run("string_to_bytes", func(t *testing.T) {
		checkDot(t, "fn main() { s = \"hi\"\n b = s as []byte }")
	})
	t.Run("bytes_to_string", func(t *testing.T) {
		checkDot(t, "fn main() { b = []byte{104}\n s = b as string }")
	})
}

// --- checkTry extras -----------------------------------------------------------------------------

func TestCheckTry_Extra(t *testing.T) {
	t.Run("outside_function", func(t *testing.T) {
		checkDotError(t, "x = None?\n fn main() {}", "'?' is only valid inside a function")
	})
	t.Run("option_in_non_option_fn", func(t *testing.T) {
		checkDotError(t, "fn f() -> Option[int] { return Some(1) }\n fn g() -> int { v = f()?\n return v }\n fn main() {}", "requires the function to return Option")
	})
	t.Run("error_type_mismatch", func(t *testing.T) {
		checkDotError(t, "fn f() -> Result[int, Error] { return Ok(1) }\n fn g() -> Result[int, string] { v = f()?\n return Ok(v) }\n fn main() {}", "propagates")
	})
}

// --- checkAwait extras ------------------------------------------------------------------------------

func TestCheckAwait_Extra(t *testing.T) {
	t.Run("await_future_ok", func(t *testing.T) {
		checkDot(t, "async fn slow() -> int { return 42 }\n async fn main() { v = await slow() }")
	})
	t.Run("await_in_sync_fn", func(t *testing.T) {
		checkDotError(t, "async fn slow() -> int { return 42 }\n fn main() { v = await slow() }", "async")
	})
}

// --- checkSpawn / spawnResult -------------------------------------------------------------------------

func parseCheckDot(t *testing.T, src string) (*ast.Program, *Info) {
	t.Helper()
	prog, err := parser.ParseFile(src, "<test>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "<test>")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	return prog, info
}

func findSpawnExpr(t *testing.T, prog *ast.Program) *ast.SpawnExpr {
	t.Helper()
	var found *ast.SpawnExpr
	ast.Inspect(prog, func(n ast.Node) bool {
		if se, ok := n.(*ast.SpawnExpr); ok {
			found = se
			return false
		}
		return true
	})
	if found == nil {
		t.Fatal("no spawn expression found")
	}
	return found
}

func TestCheckSpawn(t *testing.T) {
	t.Run("future_of_void", func(t *testing.T) {
		prog, info := parseCheckDot(t, "fn main() { h = spawn { print(\"hi\") } }")
		se := findSpawnExpr(t, prog)
		f, ok := info.TypeOf(se).(*Future)
		if !ok {
			t.Fatalf("spawn type = %v, want *Future", info.TypeOf(se))
		}
		if !IsVoid(f.Result) {
			t.Errorf("spawn result = %v, want void", f.Result)
		}
	})
	t.Run("bare_return", func(t *testing.T) {
		prog, info := parseCheckDot(t, "fn main() { h = spawn { return } }")
		se := findSpawnExpr(t, prog)
		f, ok := info.TypeOf(se).(*Future)
		if !ok || !IsVoid(f.Result) {
			t.Errorf("spawn type = %v, want Future[void]", info.TypeOf(se))
		}
	})
	t.Run("return_value_inside_spawn", func(t *testing.T) {
		prog, info := parseCheckDot(t, "fn main() { h = spawn { return 42 } }")
		se := findSpawnExpr(t, prog)
		f, ok := info.TypeOf(se).(*Future)
		if !ok {
			t.Fatalf("spawn type = %v, want *Future", info.TypeOf(se))
		}
		if f.Result != Int {
			t.Errorf("spawn result = %v, want int", f.Result)
		}
	})
}

func TestSpawnResult(t *testing.T) {
	lit := &ast.IntLit{Value: 42}
	tests := []struct {
		name  string
		block *ast.BlockStmt
		want  Type
	}{
		{"nil_block", nil, Void},
		{"empty", &ast.BlockStmt{}, Void},
		{"no_return", &ast.BlockStmt{Stmts: []ast.Stmt{&ast.ExprStmt{X: &ast.IntLit{}}}}, Void},
		{"bare_return", &ast.BlockStmt{Stmts: []ast.Stmt{&ast.ReturnStmt{}}}, Void},
		{"two_values", &ast.BlockStmt{Stmts: []ast.Stmt{&ast.ReturnStmt{Values: []ast.Expr{lit, lit}}}}, Void},
		{"single_return", &ast.BlockStmt{Stmts: []ast.Stmt{&ast.ReturnStmt{Values: []ast.Expr{lit}}}}, Int},
		{"nested_return_ignored", &ast.BlockStmt{Stmts: []ast.Stmt{
			&ast.ExprStmt{X: &ast.IfExpr{Then: &ast.BlockStmt{Stmts: []ast.Stmt{&ast.ReturnStmt{Values: []ast.Expr{lit}}}}}},
		}}, Void},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newChecker()
			c.info.Types[lit] = Int
			got := c.spawnResult(&ast.SpawnExpr{Block: tt.block})
			if got != tt.want {
				t.Errorf("spawnResult = %v, want %v", got, tt.want)
			}
		})
	}
}
