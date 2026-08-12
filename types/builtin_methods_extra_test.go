package types

import (
	"reflect"
	"slices"
	"sort"
	"testing"
)

func mustLookup(t *testing.T, u *Universe, recv Type, name string) *Fn {
	t.Helper()
	fn, ok := lookupBuiltinMethod(u, recv, name)
	if !ok {
		t.Fatalf("method %q not found on %s", name, recv)
	}
	if fn == nil {
		t.Fatalf("method %q found but signature is nil", name)
	}
	return fn
}

func wantNotFound(t *testing.T, u *Universe, recv Type, name string) {
	t.Helper()
	fn, ok := lookupBuiltinMethod(u, recv, name)
	if ok {
		t.Errorf("method %q on %s = %v, want not found", name, recv, fn)
	}
	if fn != nil {
		t.Errorf("method %q on %s returned non-nil fn, want nil", name, recv)
	}
}

// --- map methods ------------------------------------------------------------

func TestMapMethod_Signatures(t *testing.T) {
	u := NewUniverse()
	m := NewMap(String_, Int)
	tests := []struct {
		name       string
		wantField  bool
		wantParams []Type
		wantResult func(*testing.T, Type)
	}{
		{
			"len", true, []Type{},
			func(t *testing.T, r Type) { wantType(t, r, Int) },
		},
		{
			"get", false, []Type{String_},
			func(t *testing.T, r Type) { wantOptionElem(t, u, r, Int) },
		},
		{
			"set", false, []Type{String_, Int},
			func(t *testing.T, r Type) { wantType(t, r, Void) },
		},
		{
			"has", false, []Type{String_},
			func(t *testing.T, r Type) { wantType(t, r, Bool) },
		},
		{
			"remove", false, []Type{String_},
			func(t *testing.T, r Type) { wantType(t, r, Bool) },
		},
		{
			"keys", false, []Type{},
			func(t *testing.T, r Type) { wantSliceElem(t, r, String_) },
		},
		{
			"values", false, []Type{},
			func(t *testing.T, r Type) { wantSliceElem(t, r, Int) },
		},
		{
			"clear", false, []Type{},
			func(t *testing.T, r Type) { wantType(t, r, Void) },
		},
		{
			"is_empty", false, []Type{},
			func(t *testing.T, r Type) { wantType(t, r, Bool) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, isField, ok := lookupBuiltinMethodField(u, m, tt.name)
			if !ok {
				t.Fatalf("map.%s not found", tt.name)
			}
			if isField != tt.wantField {
				t.Errorf("field = %v, want %v", isField, tt.wantField)
			}
			wantParams(t, fn, tt.wantParams)
			tt.wantResult(t, fn.Result)
		})
	}
}

func TestMapMethod_Unknown(t *testing.T) {
	u := NewUniverse()
	m := NewMap(String_, Int)
	for _, name := range []string{"push", "pop", "first", "get_or", "", "Get", "LEN"} {
		t.Run("name_"+name, func(t *testing.T) {
			wantNotFound(t, u, m, name)
		})
	}
}

func TestMapMethod_KeyValuePropagation(t *testing.T) {
	u := NewUniverse()
	boolSlice := NewSlice(Bool)
	m := NewMap(Int, boolSlice)

	fn := mustLookup(t, u, m, "get")
	wantParams(t, fn, []Type{Int})
	wantOptionElem(t, u, fn.Result, boolSlice)

	fn = mustLookup(t, u, m, "set")
	wantParams(t, fn, []Type{Int, boolSlice})

	fn = mustLookup(t, u, m, "keys")
	wantSliceElem(t, fn.Result, Int)

	fn = mustLookup(t, u, m, "values")
	wantSliceElem(t, fn.Result, boolSlice)
}

// --- array methods ----------------------------------------------------------

func TestArrayMethod_Signatures(t *testing.T) {
	u := NewUniverse()
	a := NewArray(Int, 3)
	tests := []struct {
		name       string
		wantField  bool
		wantParams []Type
		wantResult func(*testing.T, *Universe, Type)
	}{
		{"len", true, []Type{}, func(t *testing.T, u *Universe, r Type) { wantType(t, r, Int) }},
		{"contains", false, []Type{Int}, func(t *testing.T, u *Universe, r Type) { wantType(t, r, Bool) }},
		{"find", false, nil, func(t *testing.T, u *Universe, r Type) { wantOptionElem(t, u, r, Int) }},
		{"index_of", false, []Type{Int}, func(t *testing.T, u *Universe, r Type) { wantOptionElem(t, u, r, Int) }},
		{"first", false, []Type{}, func(t *testing.T, u *Universe, r Type) { wantOptionElem(t, u, r, Int) }},
		{"last", false, []Type{}, func(t *testing.T, u *Universe, r Type) { wantOptionElem(t, u, r, Int) }},
		{"is_empty", false, []Type{}, func(t *testing.T, u *Universe, r Type) { wantType(t, r, Bool) }},
		{"map", false, nil, func(t *testing.T, u *Universe, r Type) { wantTypeParamSlice(t, r) }},
		{"filter", false, nil, func(t *testing.T, u *Universe, r Type) { wantSliceElem(t, r, Int) }},
		{"reduce", false, nil, func(t *testing.T, u *Universe, r Type) { wantReduceResult(t, r, Int) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, isField, ok := lookupBuiltinMethodField(u, a, tt.name)
			if !ok {
				t.Fatalf("array.%s not found", tt.name)
			}
			if isField != tt.wantField {
				t.Errorf("field = %v, want %v", isField, tt.wantField)
			}
			wantParams(t, fn, tt.wantParams)
			if tt.name == "find" {
				wantPredParam(t, fn, Int)
			}
			if tt.name == "filter" {
				wantPredParam(t, fn, Int)
			}
			if tt.name == "map" {
				wantMapParam(t, fn, Int)
			}
			if tt.name == "reduce" {
				wantReduceParams(t, fn, Int)
			}
			tt.wantResult(t, u, fn.Result)
		})
	}
}

func TestArrayMethod_Unknown(t *testing.T) {
	u := NewUniverse()
	a := NewArray(Int, 3)
	for _, name := range []string{"push", "sort", "clear", "pop", "reduce_or"} {
		t.Run("name_"+name, func(t *testing.T) {
			wantNotFound(t, u, a, name)
		})
	}
}

// --- channel methods --------------------------------------------------------

func TestChanMethod_Signatures(t *testing.T) {
	u := NewUniverse()
	c := NewChan(Int)
	tests := []struct {
		name       string
		wantParams []Type
		wantResult func(*testing.T, *Universe, Type)
	}{
		{"send", []Type{Int}, func(t *testing.T, u *Universe, r Type) { wantType(t, r, Void) }},
		{"recv", []Type{}, func(t *testing.T, u *Universe, r Type) { wantOptionElem(t, u, r, Int) }},
		{"close", []Type{}, func(t *testing.T, u *Universe, r Type) { wantType(t, r, Void) }},
		{"is_closed", []Type{}, func(t *testing.T, u *Universe, r Type) { wantType(t, r, Bool) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, isField, ok := lookupBuiltinMethodField(u, c, tt.name)
			if !ok {
				t.Fatalf("chan.%s not found", tt.name)
			}
			if isField {
				t.Errorf("chan.%s field = true, want false", tt.name)
			}
			wantParams(t, fn, tt.wantParams)
			tt.wantResult(t, u, fn.Result)
		})
	}
}

func TestChanMethod_Unknown(t *testing.T) {
	u := NewUniverse()
	c := NewChan(Int)
	for _, name := range []string{"len", "push", "await", "cancel"} {
		t.Run("name_"+name, func(t *testing.T) {
			wantNotFound(t, u, c, name)
		})
	}
}

// --- future methods ---------------------------------------------------------

func TestFutureMethod_Signatures(t *testing.T) {
	u := NewUniverse()
	f := NewFuture(Int)
	tests := []struct {
		name       string
		wantParams []Type
		wantResult Type
	}{
		{"is_done", []Type{}, Bool},
		{"cancel", []Type{}, Void},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, isField, ok := lookupBuiltinMethodField(u, f, tt.name)
			if !ok {
				t.Fatalf("future.%s not found", tt.name)
			}
			if isField {
				t.Errorf("future.%s field = true, want false", tt.name)
			}
			wantParams(t, fn, tt.wantParams)
			wantType(t, fn.Result, tt.wantResult)
		})
	}
}

func TestFutureMethod_Unknown(t *testing.T) {
	u := NewUniverse()
	f := NewFuture(Int)
	for _, name := range []string{"await", "len", "recv"} {
		t.Run("name_"+name, func(t *testing.T) {
			wantNotFound(t, u, f, name)
		})
	}
}

// --- numeric methods --------------------------------------------------------

func TestNumericMethod_Signatures(t *testing.T) {
	u := NewUniverse()
	tests := []struct {
		name       string
		recv       Type
		wantParams []Type
		wantResult Type
	}{
		{"to_string", Int, []Type{}, String_},
		{"to_string", Float, []Type{}, String_},
		{"to_string", Rune, []Type{}, String_},
		{"to_float", Int, []Type{}, Float},
		{"to_float", Int8, []Type{}, Float},
		{"to_float", Uint32, []Type{}, Float},
		{"to_int", Float, []Type{}, Int},
		{"to_int", Float32, []Type{}, Int},
		{"abs", Int, []Type{}, Int},
		{"abs", Int8, []Type{}, Int8},
		{"abs", Float, []Type{}, Float},
		{"abs", Float32, []Type{}, Float32},
		{"min", Int, []Type{Int}, Int},
		{"min", Float, []Type{Float}, Float},
		{"max", Int, []Type{Int}, Int},
		{"max", Uint, []Type{Uint}, Uint},
		{"sqrt", Float, []Type{}, Float},
		{"sqrt", Float32, []Type{}, Float},
		{"floor", Float, []Type{}, Float},
		{"ceil", Float, []Type{}, Float},
		{"round", Float, []Type{}, Float},
		{"pow", Float, []Type{Float}, Float},
	}
	for _, tt := range tests {
		t.Run(tt.recv.String()+"_"+tt.name, func(t *testing.T) {
			fn, isField, ok := lookupBuiltinMethodField(u, tt.recv, tt.name)
			if !ok {
				t.Fatalf("%s.%s not found", tt.recv, tt.name)
			}
			if isField {
				t.Errorf("%s.%s field = true, want false", tt.recv, tt.name)
			}
			wantParams(t, fn, tt.wantParams)
			wantType(t, fn.Result, tt.wantResult)
		})
	}
}

func TestNumericMethod_FloatOnlyRejectsInts(t *testing.T) {
	u := NewUniverse()
	methods := []string{"sqrt", "floor", "ceil", "round", "pow"}
	recvs := []Type{Int, Int8, Int16, Int32, Uint, Uint8, Uint16, Uint32, Rune}
	for _, name := range methods {
		for _, recv := range recvs {
			t.Run(recv.String()+"_"+name, func(t *testing.T) {
				wantNotFound(t, u, recv, name)
			})
		}
	}
}

func TestNumericMethod_Unknown(t *testing.T) {
	u := NewUniverse()
	names := []string{"foo", "len", "to_upper", "sqrt2", "ToInt", ""}
	for _, recv := range []Type{Int, Float} {
		for _, name := range names {
			t.Run(recv.String()+"_"+name, func(t *testing.T) {
				wantNotFound(t, u, recv, name)
			})
		}
	}
}

func TestNumericMethod_AllNumericKinds(t *testing.T) {
	u := NewUniverse()
	for _, recv := range []Type{
		Int, Int8, Int16, Int32, Uint, Uint8, Uint16, Uint32,
		Byte, Rune, Float, Float32, UntypedInt, UntypedFloat,
	} {
		t.Run(recv.String(), func(t *testing.T) {
			fn := mustLookup(t, u, recv, "to_string")
			wantType(t, fn.Result, String_)
			fn = mustLookup(t, u, recv, "to_float")
			wantType(t, fn.Result, Float)
			fn = mustLookup(t, u, recv, "to_int")
			wantType(t, fn.Result, Int)
			fn = mustLookup(t, u, recv, "abs")
			wantType(t, fn.Result, recv)
		})
	}
}

// --- bool methods -----------------------------------------------------------

func TestBoolMethod_Signatures(t *testing.T) {
	u := NewUniverse()
	fn, isField, ok := lookupBuiltinMethodField(u, Bool, "to_string")
	if !ok {
		t.Fatal("bool.to_string not found")
	}
	if isField {
		t.Error("bool.to_string field = true, want false")
	}
	wantParams(t, fn, []Type{})
	wantType(t, fn.Result, String_)
}

func TestBoolMethod_Unknown(t *testing.T) {
	u := NewUniverse()
	for _, name := range []string{"len", "to_int", "sqrt", "abs"} {
		t.Run("name_"+name, func(t *testing.T) {
			wantNotFound(t, u, Bool, name)
		})
	}
}

// --- dispatch over receiver shapes ------------------------------------------

func TestBuiltinMethodSpec_NonBuiltinReceivers(t *testing.T) {
	u := NewUniverse()
	recvs := map[string]Type{
		"struct":        &Struct{},
		"named":         &Named{Name: "User"},
		"pointer":       NewPointer(Int),
		"weak":          NewWeak(Int),
		"tuple":         &Tuple{Elems: []Type{Int, Int}},
		"dyn":           NewDyn(&Trait{Name: "T"}),
		"void":          Void,
		"never":         Never,
		"untypedString": UntypedString,
		"invalid":       Invalid,
	}
	for shape, recv := range recvs {
		t.Run(shape+"_len", func(t *testing.T) { wantNotFound(t, u, recv, "len") })
		t.Run(shape+"_to_string", func(t *testing.T) { wantNotFound(t, u, recv, "to_string") })
	}
}

func TestBuiltinMethodSpec_UnrelatedMethodOnShape(t *testing.T) {
	u := NewUniverse()
	cases := []struct {
		name string
		recv Type
		meth string
	}{
		{"slice_get", NewSlice(Int), "get"},
		{"map_push", NewMap(String_, Int), "push"},
		{"array_recv", NewArray(Int, 2), "recv"},
		{"string_sqrt", String_, "sqrt"},
		{"bool_abs", Bool, "abs"},
		{"int_len", Int, "len"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			wantNotFound(t, u, tt.recv, tt.meth)
		})
	}
}

func TestBuiltinMethodSpec_OptionResultDispatch(t *testing.T) {
	u := NewUniverse()

	opt := u.OptionOf(String_)
	fn := mustLookup(t, u, opt, "is_some")
	wantType(t, fn.Result, Bool)

	res := u.ResultOf(Int, String_)
	fn = mustLookup(t, u, res, "is_ok")
	wantType(t, fn.Result, Bool)
	fn = mustLookup(t, u, res, "map_err")
	if fn == nil {
		t.Fatal("result.map_err not found")
	}

	plain := &Named{Name: "Foo", TypeArgs: []Type{Int}, Underlying: &Struct{}}
	wantNotFound(t, u, plain, "is_some")
}

func TestLookupBuiltinMethodField_UnknownReturnsZeros(t *testing.T) {
	u := NewUniverse()
	fn, isField, ok := lookupBuiltinMethodField(u, NewSlice(Int), "nope")
	if ok || isField || fn != nil {
		t.Errorf("unknown member: ok=%v field=%v fn=%v, want false/false/nil", ok, isField, fn)
	}
}

func TestLookupBuiltinMethod_NilReceiver(t *testing.T) {
	u := NewUniverse()
	wantNotFound(t, u, nil, "len")
	wantNotFound(t, u, nil, "to_string")
}

// --- method names -----------------------------------------------------------

func TestMethodNames_Shapes(t *testing.T) {
	tests := []struct {
		name string
		recv Type
		want []string
	}{
		{"slice", NewSlice(Int), sliceNames},
		{"map", NewMap(String_, Int), mapNames},
		{"array", NewArray(Int, 2), arrayNames},
		{"chan", NewChan(Int), chanNames},
		{"future", NewFuture(Int), futureNames},
		{"string", String_, stringNames},
		{"bool", Bool, boolNames},
		{"int", Int, numericNames},
		{"float", Float, numericNames},
		{"rune", Rune, numericNames},
		{"named", &Named{Name: "User"}, nil},
		{"struct", &Struct{}, nil},
		{"pointer", NewPointer(Int), nil},
		{"void", Void, nil},
		{"untypedString", UntypedString, nil},
		{"optionNamed", nil, nil}, // filled below
	}
	u := NewUniverse()
	tests[len(tests)-1].recv = u.OptionOf(Int)
	tests[len(tests)-1].name = "optionNamed"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := methodNames(tt.recv)
			if tt.want == nil && got != nil {
				t.Errorf("methodNames(%s) = %v, want nil", tt.recv, got)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("methodNames(%s) = %v, want %v", tt.recv, got, tt.want)
			}
		})
	}
}

func TestBuiltinMethodNames_OptionResult(t *testing.T) {
	u := NewUniverse()

	got := builtinMethodNames(u.OptionOf(Int))
	wantSorted(t, got, optionNames)

	got = builtinMethodNames(u.ResultOf(Int, String_))
	wantSorted(t, got, resultNames)

	got = builtinMethodNames(NewSlice(Int))
	wantSorted(t, got, sliceNames)

	got = builtinMethodNames(&Named{Name: "User"})
	if got != nil {
		t.Errorf("plain named = %v, want nil", got)
	}
}

func TestBuiltinMethodNames_NonOptionNamedBug(t *testing.T) {
	got := builtinMethodNames(&Named{Name: "Foo", TypeArgs: []Type{Int}})
	if got != nil {
		t.Errorf("non-Option Named with 1 TypeArg = %v, want nil", got)
	}
}

func TestBuiltinMethodNames_Sorted(t *testing.T) {
	u := NewUniverse()
	recvs := []Type{u.OptionOf(Int), u.ResultOf(Int, String_), NewSlice(Int), NewMap(String_, Int), NewArray(Int, 2), NewChan(Int), NewFuture(Int), String_, Bool, Int, Float}
	for _, recv := range recvs {
		t.Run(recv.String(), func(t *testing.T) {
			got := builtinMethodNames(recv)
			if !slices.IsSorted(got) {
				t.Errorf("builtinMethodNames(%s) = %v, not sorted", recv, got)
			}
		})
	}
}

// TestBuiltinMethodNames_Consistency checks that every advertised name
// resolves and that the resolver never advertises a name outside the list.
func TestBuiltinMethodNames_Consistency(t *testing.T) {
	u := NewUniverse()
	shapes := []struct {
		recv  Type
		names []string
	}{
		{NewSlice(Int), sliceNames},
		{NewMap(String_, Int), mapNames},
		{NewArray(Int, 2), arrayNames},
		{NewChan(Int), chanNames},
		{NewFuture(Int), futureNames},
		{String_, stringNames},
		{Bool, boolNames},
		{Float, numericNames},
	}
	for _, sh := range shapes {
		for _, name := range sh.names {
			t.Run(sh.recv.String()+"_"+name, func(t *testing.T) {
				fn, _, ok := lookupBuiltinMethodField(u, sh.recv, name)
				if !ok {
					t.Fatalf("%q advertised for %s but not found", name, sh.recv)
				}
				if fn == nil {
					t.Fatalf("%q found on %s but signature is nil", name, sh.recv)
				}
			})
		}
	}
}

// --- fn helpers -------------------------------------------------------------

func TestFnOf(t *testing.T) {
	tests := []struct {
		name   string
		params []Type
	}{
		{"zero", nil},
		{"one", []Type{Int}},
		{"three", []Type{Int, String_, Bool}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := fnOf(Void, tt.params...)
			if fn == nil {
				t.Fatal("fnOf returned nil")
			}
			if len(fn.Params) != len(tt.params) {
				t.Fatalf("params = %d, want %d", len(fn.Params), len(tt.params))
			}
			for i, p := range fn.Params {
				if p.Type != tt.params[i] {
					t.Errorf("param %d = %v, want %v", i, p.Type, tt.params[i])
				}
			}
			if fn.Result != Void {
				t.Errorf("result = %v, want Void", fn.Result)
			}
		})
	}
}

func TestPredFn(t *testing.T) {
	fn := predFn(String_)
	if fn == nil || len(fn.Params) != 1 {
		t.Fatalf("predFn params = %v, want 1", fn)
	}
	if fn.Params[0].Type != String_ {
		t.Errorf("param = %v, want String_", fn.Params[0].Type)
	}
	if fn.Result != Bool {
		t.Errorf("result = %v, want Bool", fn.Result)
	}
}

func TestCmpFn(t *testing.T) {
	fn := cmpFn(Int)
	if fn == nil || len(fn.Params) != 2 {
		t.Fatalf("cmpFn params = %v, want 2", fn)
	}
	if fn.Params[0].Type != Int || fn.Params[1].Type != Int {
		t.Errorf("params = %v, want [int, int]", fn.Params)
	}
	if fn.Result != Int {
		t.Errorf("result = %v, want Int", fn.Result)
	}
}

func TestMapFn(t *testing.T) {
	fn, up := mapFn(Int)
	if fn == nil || up == nil {
		t.Fatal("mapFn returned nil")
	}
	if up.Name != "U" {
		t.Errorf("type param name = %q, want U", up.Name)
	}
	if up.Index != 0 {
		t.Errorf("type param index = %d, want 0", up.Index)
	}
	if fn.Result != up {
		t.Errorf("result = %v, want type param %v", fn.Result, up)
	}
	if len(fn.Params) != 1 || fn.Params[0].Type != Int {
		t.Errorf("params = %v, want [int]", fn.Params)
	}
}

func TestReduceFn(t *testing.T) {
	fn, up := reduceFn(Int)
	if fn == nil || up == nil {
		t.Fatal("reduceFn returned nil")
	}
	if fn.Result != up {
		t.Errorf("result = %v, want type param %v", fn.Result, up)
	}
	if len(fn.Params) != 2 {
		t.Fatalf("params = %v, want 2", fn.Params)
	}
	if fn.Params[0].Type != up {
		t.Errorf("param 0 = %v, want type param %v", fn.Params[0].Type, up)
	}
	if fn.Params[1].Type != Int {
		t.Errorf("param 1 = %v, want Int", fn.Params[1].Type)
	}
}

func TestMkString(t *testing.T) {
	sig := &Fn{Result: String_}
	spec := mkString("foo", sig)
	if spec.Name != "foo" {
		t.Errorf("spec name = %q, want foo", spec.Name)
	}
	got := spec.Build(NewUniverse(), String_)
	if got != sig {
		t.Errorf("Build returned %v, want the exact sig pointer %v", got, sig)
	}
}

// --- field flags ------------------------------------------------------------

func TestLookupBuiltinMethodField_FieldFlags(t *testing.T) {
	u := NewUniverse()
	tests := []struct {
		recv  Type
		name  string
		field bool
	}{
		{NewSlice(Int), "len", true},
		{NewMap(String_, Int), "len", true},
		{NewArray(Int, 2), "len", true},
		{String_, "len", true},
		{NewSlice(Int), "push", false},
		{NewMap(String_, Int), "get", false},
		{NewArray(Int, 2), "contains", false},
		{Bool, "to_string", false},
		{Int, "abs", false},
	}
	for _, tt := range tests {
		t.Run(tt.recv.String()+"_"+tt.name, func(t *testing.T) {
			_, isField, ok := lookupBuiltinMethodField(u, tt.recv, tt.name)
			if !ok {
				t.Fatalf("%s.%s not found", tt.recv, tt.name)
			}
			if isField != tt.field {
				t.Errorf("field = %v, want %v", isField, tt.field)
			}
		})
	}
}

// --- bug documentation ------------------------------------------------------

func TestLookupBuiltinMethod_NilBasicPanics(t *testing.T) {
	u := NewUniverse()
	var b *Basic
	wantNotFound(t, u, b, "len")
}

// --- assertion helpers ------------------------------------------------------

func wantType(t *testing.T, got, want Type) {
	t.Helper()
	if got != want {
		t.Errorf("type = %v, want %v", got, want)
	}
}

func wantParams(t *testing.T, fn *Fn, want []Type) {
	t.Helper()
	if want == nil {
		return
	}
	if len(fn.Params) != len(want) {
		t.Errorf("params = %d (%v), want %d", len(fn.Params), fn.Params, len(want))
		return
	}
	for i, p := range fn.Params {
		if p.Type != want[i] {
			t.Errorf("param %d = %v, want %v", i, p.Type, want[i])
		}
	}
}

func wantOptionElem(t *testing.T, u *Universe, got Type, elem Type) {
	t.Helper()
	e, ok := u.IsOption(got)
	if !ok {
		t.Errorf("%v is not an Option", got)
		return
	}
	if e != elem {
		t.Errorf("option elem = %v, want %v", e, elem)
	}
}

func wantSliceElem(t *testing.T, got Type, elem Type) {
	t.Helper()
	sl, ok := got.(*Slice)
	if !ok {
		t.Errorf("%v is not a slice", got)
		return
	}
	if sl.Elem != elem {
		t.Errorf("slice elem = %v, want %v", sl.Elem, elem)
	}
}

func wantPredParam(t *testing.T, fn *Fn, targ Type) {
	t.Helper()
	if len(fn.Params) != 1 {
		t.Errorf("pred params = %d, want 1", len(fn.Params))
		return
	}
	pf, ok := fn.Params[0].Type.(*Fn)
	if !ok {
		t.Errorf("pred param type = %T, want *Fn", fn.Params[0].Type)
		return
	}
	if len(pf.Params) != 1 || pf.Params[0].Type != targ {
		t.Errorf("pred arg = %v, want [%v]", pf.Params, targ)
	}
	if pf.Result != Bool {
		t.Errorf("pred result = %v, want Bool", pf.Result)
	}
}

func wantMapParam(t *testing.T, fn *Fn, targ Type) {
	t.Helper()
	if len(fn.Params) != 1 {
		t.Errorf("map params = %d, want 1", len(fn.Params))
		return
	}
	mf, ok := fn.Params[0].Type.(*Fn)
	if !ok {
		t.Errorf("map param type = %T, want *Fn", fn.Params[0].Type)
		return
	}
	if len(mf.Params) != 1 || mf.Params[0].Type != targ {
		t.Errorf("map fn arg = %v, want [%v]", mf.Params, targ)
	}
	if mf.Result != fn.Result.(*Slice).Elem {
		t.Errorf("map fn result = %v, want slice elem %v", mf.Result, fn.Result)
	}
}

func wantTypeParamSlice(t *testing.T, r Type) {
	t.Helper()
	sl, ok := r.(*Slice)
	if !ok {
		t.Errorf("%v is not a slice", r)
		return
	}
	if _, ok := sl.Elem.(*TypeParam); !ok {
		t.Errorf("map elem = %T, want *TypeParam", sl.Elem)
	}
}

func wantReduceResult(t *testing.T, r Type, targ Type) {
	t.Helper()
	up, ok := r.(*TypeParam)
	if !ok {
		t.Errorf("reduce result = %T, want *TypeParam", r)
		return
	}
	if up.Name != "U" {
		t.Errorf("reduce type param name = %q, want U", up.Name)
	}
	_ = targ
}

func wantReduceParams(t *testing.T, fn *Fn, targ Type) {
	t.Helper()
	if len(fn.Params) != 2 {
		t.Errorf("reduce params = %d, want 2", len(fn.Params))
		return
	}
	if fn.Params[0].Type != fn.Result {
		t.Errorf("reduce init = %v, want result %v", fn.Params[0].Type, fn.Result)
	}
	rf, ok := fn.Params[1].Type.(*Fn)
	if !ok {
		t.Errorf("reduce fn param = %T, want *Fn", fn.Params[1].Type)
		return
	}
	if len(rf.Params) != 2 {
		t.Errorf("reduce fn args = %d, want 2", len(rf.Params))
		return
	}
	if rf.Params[0].Type != fn.Result || rf.Params[1].Type != targ {
		t.Errorf("reduce fn args = [%v, %v], want [result, %v]", rf.Params[0].Type, rf.Params[1].Type, targ)
	}
	if rf.Result != fn.Result {
		t.Errorf("reduce fn result = %v, want %v", rf.Result, fn.Result)
	}
}

func wantSorted(t *testing.T, got, want []string) {
	t.Helper()
	sorted := append([]string(nil), want...)
	sort.Strings(sorted)
	if !reflect.DeepEqual(got, sorted) {
		t.Errorf("names = %v, want sorted %v", got, sorted)
	}
}
