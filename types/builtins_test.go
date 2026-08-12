package types

import "testing"

func TestUniverse_Primitives(t *testing.T) {
	u := NewUniverse()

	primitives := []struct {
		name string
		kind Kind
	}{
		{"bool", KindBool},
		{"int", KindInt},
		{"int8", KindInt8},
		{"int16", KindInt16},
		{"int32", KindInt32},
		{"uint", KindUint},
		{"uint8", KindUint8},
		{"uint16", KindUint16},
		{"uint32", KindUint32},
		{"byte", KindUint8},
		{"float", KindFloat},
		{"float32", KindFloat32},
		{"rune", KindRune},
		{"string", KindString},
	}

	for _, p := range primitives {
		t.Run(p.name, func(t *testing.T) {
			sym, ok := u.Scope.Lookup(p.name)
			if !ok {
				t.Fatalf("%q not found in universe", p.name)
			}
			if sym.Kind != SymType {
				t.Errorf("%q kind = %s, want SymType", p.name, sym.Kind)
			}
			if sym.Type == nil {
				t.Errorf("%q type is nil", p.name)
			}
			if sym.Type.Kind() != p.kind {
				t.Errorf("%q type.Kind() = %s, want %s", p.name, sym.Type.Kind(), p.kind)
			}
		})
	}
}

func TestUniverse_Builtins(t *testing.T) {
	u := NewUniverse()

	builtins := []struct {
		name string
		id   BuiltinID
	}{
		{"print", BuiltinPrint},
		{"println", BuiltinPrintln},
		{"eprint", BuiltinEprint},
		{"input", BuiltinInput},
		{"len", BuiltinLen},
		{"type_of", BuiltinTypeOf},
		{"assert", BuiltinAssert},
		{"panic", BuiltinPanic},
	}

	for _, b := range builtins {
		t.Run(b.name, func(t *testing.T) {
			sym, ok := u.Scope.Lookup(b.name)
			if !ok {
				t.Fatalf("%q not found in universe", b.name)
			}
			if sym.Kind != SymBuiltin {
				t.Errorf("%q kind = %s, want SymBuiltin", b.name, sym.Kind)
			}
			if sym.Builtin != b.id {
				t.Errorf("%q Builtin = %d, want %d", b.name, sym.Builtin, b.id)
			}
		})
	}
}

func TestUniverse_NamedTypes(t *testing.T) {
	u := NewUniverse()

	namedTypes := []string{"Option", "Result", "Error", "Channel", "Arena"}
	for _, name := range namedTypes {
		t.Run(name, func(t *testing.T) {
			sym, ok := u.Scope.Lookup(name)
			if !ok {
				t.Fatalf("%q not found in universe", name)
			}
			if sym.Kind != SymType {
				t.Errorf("%q kind = %s, want SymType", name, sym.Kind)
			}
			if sym.Named == nil {
				t.Errorf("%q Named is nil", name)
			}
		})
	}
}

func TestUniverse_VariantSymbols(t *testing.T) {
	u := NewUniverse()

	tests := []struct {
		name  string
		owner string
	}{
		{"Some", "Option"},
		{"None", "Option"},
		{"Ok", "Result"},
		{"Err", "Result"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sym, ok := u.Scope.Lookup(tt.name)
			if !ok {
				t.Fatalf("variant %q not found", tt.name)
			}
			if sym.Kind != SymVariant {
				t.Errorf("%q kind = %s, want SymVariant", tt.name, sym.Kind)
			}
			if sym.Variant == nil {
				t.Errorf("%q Variant is nil", tt.name)
			}
			if sym.Variant.Name != tt.name {
				t.Errorf("%q Variant.Name = %q, want %q", tt.name, sym.Variant.Name, tt.name)
			}
		})
	}
}

func TestOptionOf(t *testing.T) {
	u := NewUniverse()
	o := u.OptionOf(Int)

	named, ok := o.(*Named)
	if !ok {
		t.Fatalf("OptionOf = %T, want *Named", o)
	}
	if named.Name != "Option" {
		t.Errorf("name = %q, want Option", named.Name)
	}
	if named.Origin != u.Option {
		t.Error("origin should point to universe Option")
	}
	if len(named.TypeArgs) != 1 || named.TypeArgs[0] != Int {
		t.Errorf("type args = %v, want [int]", named.TypeArgs)
	}

	enum, ok := named.Underlying.(*Enum)
	if !ok {
		t.Fatalf("underlying = %T, want *Enum", named.Underlying)
	}
	if len(enum.Variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(enum.Variants))
	}
}

func TestResultOf(t *testing.T) {
	u := NewUniverse()
	r := u.ResultOf(Int, String_)

	named, ok := r.(*Named)
	if !ok {
		t.Fatalf("ResultOf = %T, want *Named", r)
	}
	if named.Name != "Result" {
		t.Errorf("name = %q, want Result", named.Name)
	}
	if named.Origin != u.Result {
		t.Error("origin should point to universe Result")
	}
	if len(named.TypeArgs) != 2 {
		t.Errorf("expected 2 type args, got %d", len(named.TypeArgs))
	}
}

func TestIsOption(t *testing.T) {
	u := NewUniverse()
	o := u.OptionOf(Int)

	elem, ok := u.IsOption(o)
	if !ok {
		t.Fatal("IsOption should return true")
	}
	if elem != Int {
		t.Errorf("IsOption elem = %v, want Int", elem)
	}

	_, ok = u.IsOption(u.ResultOf(Int, String_))
	if ok {
		t.Error("IsOption on Result should return false")
	}

	_, ok = u.IsOption(Int)
	if ok {
		t.Error("IsOption on int should return false")
	}
}

func TestIsResult(t *testing.T) {
	u := NewUniverse()
	r := u.ResultOf(Int, String_)

	ok_, err_, isOk := u.IsResult(r)
	if !isOk {
		t.Fatal("IsResult should return true")
	}
	if ok_ != Int {
		t.Errorf("IsResult ok_ = %v, want Int", ok_)
	}
	if err_ != String_ {
		t.Errorf("IsResult err_ = %v, want String_", err_)
	}

	_, _, _ = ok_, err_, isOk

	_, _, isOk = u.IsResult(u.OptionOf(Int))
	if isOk {
		t.Error("IsResult on Option should return false")
	}
}

func TestLookupBuiltinMethod_Slice(t *testing.T) {
	u := NewUniverse()
	sl := NewSlice(Int).(*Slice)

	fn, ok := lookupBuiltinMethod(u, sl, "len")
	if !ok {
		t.Fatal("slice.len not found")
	}
	if fn.Result != Int {
		t.Errorf("len result = %v, want Int", fn.Result)
	}

	fn, ok = lookupBuiltinMethod(u, sl, "push")
	if !ok {
		t.Fatal("slice.push not found")
	}
	if fn.Result.Kind() != KindVoid {
		t.Errorf("push result = %v, want void", fn.Result)
	}

	_, ok = lookupBuiltinMethod(u, sl, "nonexistent")
	if ok {
		t.Error("nonexistent method should not be found")
	}
}

func TestLookupBuiltinMethod_String(t *testing.T) {
	u := NewUniverse()

	fn, ok := lookupBuiltinMethod(u, String_, "len")
	if !ok {
		t.Fatal("string.len not found")
	}
	if fn.Result != Int {
		t.Errorf("len result = %v, want Int", fn.Result)
	}

	fn, ok = lookupBuiltinMethod(u, String_, "to_upper")
	if !ok {
		t.Fatal("string.to_upper not found")
	}
	if fn.Result != String_ {
		t.Errorf("to_upper result = %v, want string", fn.Result)
	}
}

func TestLookupBuiltinMethod_Numeric(t *testing.T) {
	u := NewUniverse()

	fn, ok := lookupBuiltinMethod(u, Int, "to_string")
	if !ok {
		t.Fatal("int.to_string not found")
	}
	if fn.Result != String_ {
		t.Errorf("to_string result = %v, want string", fn.Result)
	}

	fn, ok = lookupBuiltinMethod(u, Float, "sqrt")
	if !ok {
		t.Fatal("float.sqrt not found")
	}

	_, ok = lookupBuiltinMethod(u, Int, "sqrt")
	if ok {
		t.Error("int.sqrt should not exist")
	}
}

func TestLookupBuiltinMethod_Option(t *testing.T) {
	u := NewUniverse()
	opt := u.OptionOf(Int)

	fn, ok := lookupBuiltinMethod(u, opt, "is_some")
	if !ok {
		t.Fatal("Option.is_some not found")
	}
	if fn.Result != Bool {
		t.Errorf("is_some result = %v, want bool", fn.Result)
	}

	fn, ok = lookupBuiltinMethod(u, opt, "unwrap")
	if !ok {
		t.Fatal("Option.unwrap not found")
	}
}

func TestLookupBuiltinMethod_Result(t *testing.T) {
	u := NewUniverse()
	res := u.ResultOf(Int, String_)

	fn, ok := lookupBuiltinMethod(u, res, "is_ok")
	if !ok {
		t.Fatal("Result.is_ok not found")
	}
	if fn.Result != Bool {
		t.Errorf("is_ok result = %v, want bool", fn.Result)
	}

	fn, ok = lookupBuiltinMethod(u, res, "unwrap")
	if !ok {
		t.Fatal("Result.unwrap not found")
	}

	fn, ok = lookupBuiltinMethod(u, res, "map")
	if !ok {
		t.Fatal("Result.map not found")
	}
}

func TestNewUniverse_Idempotent(t *testing.T) {
	u1 := NewUniverse()
	u2 := NewUniverse()

	if u1 == u2 {
		t.Error("NewUniverse should return different instances")
	}

	sym1, _ := u1.Scope.Lookup("int")
	sym2, _ := u2.Scope.Lookup("int")
	if sym1.Type != sym2.Type {
		t.Error("both universes should share the same canonical int pointer")
	}
}
