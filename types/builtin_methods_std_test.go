package types

import "testing"

func wantParamNames(t *testing.T, fn *Fn, names ...string) {
	t.Helper()
	if len(fn.Params) != len(names) {
		t.Errorf("params = %d, want %d", len(fn.Params), len(names))
		return
	}
	for i, n := range names {
		if fn.Params[i].Name != n {
			t.Errorf("param %d name = %q, want %q", i, fn.Params[i].Name, n)
		}
	}
}

func wantFnType(t *testing.T, typ Type) *Fn {
	t.Helper()
	f, ok := typ.(*Fn)
	if !ok {
		t.Fatalf("type = %T (%v), want *Fn", typ, typ)
	}
	return f
}

func wantResultTypes(t *testing.T, u *Universe, typ, okT, errT Type) {
	t.Helper()
	gotOk, gotErr, isRes := u.IsResult(typ)
	if !isRes {
		t.Errorf("type = %v, want Result", typ)
		return
	}
	if gotOk != okT {
		t.Errorf("result ok type = %v, want %v", gotOk, okT)
	}
	if gotErr != errT {
		t.Errorf("result err type = %v, want %v", gotErr, errT)
	}
}

func oneTypeParam(t *testing.T, fn *Fn, name string) *TypeParam {
	t.Helper()
	if len(fn.TypeParams) != 1 {
		t.Fatalf("type params = %d, want 1", len(fn.TypeParams))
	}
	up := fn.TypeParams[0]
	if up.Name != name {
		t.Errorf("type param name = %q, want %q", up.Name, name)
	}
	if up.Index != 0 {
		t.Errorf("type param index = %d, want 0", up.Index)
	}
	return up
}

func wantMapFnParam(t *testing.T, fn *Fn, targ, up Type) {
	t.Helper()
	if len(fn.Params) != 1 {
		t.Errorf("params = %d, want 1", len(fn.Params))
		return
	}
	mf := wantFnType(t, fn.Params[0].Type)
	wantParams(t, mf, []Type{targ})
	if mf.Result != up {
		t.Errorf("map fn result = %v, want %v", mf.Result, up)
	}
}

type methodTest struct {
	name   string
	recv   Type
	method string
	ok     bool
	field  bool
	check  func(t *testing.T, u *Universe, fn *Fn)
}

func runMethodTests(t *testing.T, u *Universe, tests []methodTest) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, ok := lookupBuiltinMethod(u, tt.recv, tt.method)
			wantOk := tt.ok || tt.check != nil // rows with a checker expect the method to exist
			if ok != wantOk {
				t.Fatalf("lookup ok = %v, want %v", ok, wantOk)
			}
			if !wantOk {
				if fn != nil {
					t.Errorf("fn = %v, want nil", fn)
				}
				return
			}
			f, field, okField := lookupBuiltinMethodField(u, tt.recv, tt.method)
			if !okField {
				t.Fatal("lookupBuiltinMethodField should find the method")
			}
			if field != tt.field {
				t.Errorf("field = %v, want %v", field, tt.field)
			}
			if f == nil {
				t.Fatal("lookupBuiltinMethodField returned nil fn")
			}
			if tt.check != nil {
				tt.check(t, u, f)
			}
		})
	}
}

func TestOptionMethods(t *testing.T) {
	u := NewUniverse()
	opt := u.OptionOf(Int)
	optStr := u.OptionOf(String_)

	tests := []methodTest{
		{
			name: "unwrap", recv: opt, method: "unwrap",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Int)
			},
		},
		{
			name: "unwrap_or", recv: opt, method: "unwrap_or",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "dflt")
				wantParams(t, fn, []Type{Int})
				wantType(t, fn.Result, Int)
			},
		},
		{
			name: "unwrap_or_else", recv: opt, method: "unwrap_or_else",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "f")
				f := wantFnType(t, fn.Params[0].Type)
				wantParams(t, f, []Type{})
				wantType(t, f.Result, Int)
			},
		},
		{
			name: "is_some", recv: opt, method: "is_some",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Bool)
			},
		},
		{
			name: "is_none", recv: opt, method: "is_none",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Bool)
			},
		},
		{
			name: "map", recv: opt, method: "map",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				up := oneTypeParam(t, fn, "U")
				wantParamNames(t, fn, "f")
				wantMapFnParam(t, fn, Int, up)
				wantOptionElem(t, u, fn.Result, up)
			},
		},
		{
			name: "and_then", recv: opt, method: "and_then",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				up := oneTypeParam(t, fn, "U")
				wantParamNames(t, fn, "f")
				f := wantFnType(t, fn.Params[0].Type)
				wantParams(t, f, []Type{Int})
				wantOptionElem(t, u, f.Result, up)
				wantOptionElem(t, u, fn.Result, up)
			},
		},
		{
			name: "ok_or", recv: opt, method: "ok_or",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				ep := oneTypeParam(t, fn, "E")
				wantParamNames(t, fn, "e")
				wantParams(t, fn, []Type{ep})
				wantResultTypes(t, u, fn.Result, Int, ep)
			},
		},
		{
			name: "filter", recv: opt, method: "filter",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "pred")
				wantPredParam(t, fn, Int)
				wantOptionElem(t, u, fn.Result, Int)
			},
		},
		{
			name: "string_elem_unwrap", recv: optStr, method: "unwrap",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, String_)
			},
		},
		{
			name: "string_elem_unwrap_or", recv: optStr, method: "unwrap_or",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{String_})
				wantType(t, fn.Result, String_)
			},
		},
		{
			name: "string_elem_filter", recv: optStr, method: "filter",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantPredParam(t, fn, String_)
				wantOptionElem(t, u, fn.Result, String_)
			},
		},
		{
			name: "unknown", recv: opt, method: "zzz", ok: false,
		},
		{
			name: "slice_method_not_on_option", recv: opt, method: "push", ok: false,
		},
		{
			name: "result_method_not_on_option", recv: opt, method: "unwrap_err", ok: false,
		},
	}
	runMethodTests(t, u, tests)
}

func TestResultMethods(t *testing.T) {
	u := NewUniverse()
	res := u.ResultOf(Int, String_)
	res2 := u.ResultOf(Bool, Int)

	tests := []methodTest{
		{
			name: "unwrap", recv: res, method: "unwrap",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Int)
			},
		},
		{
			name: "unwrap_or", recv: res, method: "unwrap_or",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "dflt")
				wantParams(t, fn, []Type{Int})
				wantType(t, fn.Result, Int)
			},
		},
		{
			name: "unwrap_err", recv: res, method: "unwrap_err",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, String_)
			},
		},
		{
			name: "is_ok", recv: res, method: "is_ok",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Bool)
			},
		},
		{
			name: "is_err", recv: res, method: "is_err",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Bool)
			},
		},
		{
			name: "map", recv: res, method: "map",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				up := oneTypeParam(t, fn, "U")
				wantParamNames(t, fn, "f")
				wantMapFnParam(t, fn, Int, up)
				wantResultTypes(t, u, fn.Result, up, String_)
			},
		},
		{
			name: "map_err", recv: res, method: "map_err",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				ep := oneTypeParam(t, fn, "F")
				wantParamNames(t, fn, "f")
				wantMapFnParam(t, fn, String_, ep)
				wantResultTypes(t, u, fn.Result, Int, ep)
			},
		},
		{
			name: "and_then", recv: res, method: "and_then",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				up := oneTypeParam(t, fn, "U")
				wantParamNames(t, fn, "f")
				f := wantFnType(t, fn.Params[0].Type)
				wantParams(t, f, []Type{Int})
				wantResultTypes(t, u, f.Result, up, String_)
				wantResultTypes(t, u, fn.Result, up, String_)
			},
		},
		{
			name: "ok", recv: res, method: "ok",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantOptionElem(t, u, fn.Result, Int)
			},
		},
		{
			name: "err", recv: res, method: "err",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantOptionElem(t, u, fn.Result, String_)
			},
		},
		{
			name: "bool_int_unwrap", recv: res2, method: "unwrap",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantType(t, fn.Result, Bool)
			},
		},
		{
			name: "bool_int_unwrap_err", recv: res2, method: "unwrap_err",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantType(t, fn.Result, Int)
			},
		},
		{
			name: "bool_int_ok", recv: res2, method: "ok",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantOptionElem(t, u, fn.Result, Bool)
			},
		},
		{
			name: "bool_int_err", recv: res2, method: "err",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantOptionElem(t, u, fn.Result, Int)
			},
		},
		{
			name: "unknown", recv: res, method: "zzz", ok: false,
		},
		{
			name: "option_method_not_on_result", recv: res, method: "filter", ok: false,
		},
		{
			name: "slice_method_not_on_result", recv: res, method: "push", ok: false,
		},
	}
	runMethodTests(t, u, tests)
}

func TestSliceMethods(t *testing.T) {
	u := NewUniverse()
	sl := NewSlice(Int).(*Slice)
	slStr := NewSlice(String_).(*Slice)

	tests := []methodTest{
		{
			name: "len", recv: sl, method: "len", field: true,
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Int)
			},
		},
		{
			name: "push", recv: sl, method: "push",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "value")
				wantParams(t, fn, []Type{Int})
				wantType(t, fn.Result, Void)
			},
		},
		{
			name: "pop", recv: sl, method: "pop",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantOptionElem(t, u, fn.Result, Int)
			},
		},
		{
			name: "remove_last", recv: sl, method: "remove_last",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Int)
			},
		},
		{
			name: "insert", recv: sl, method: "insert",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "i", "value")
				wantParams(t, fn, []Type{Int, Int})
				wantType(t, fn.Result, Void)
			},
		},
		{
			name: "remove", recv: sl, method: "remove",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "i")
				wantParams(t, fn, []Type{Int})
				wantType(t, fn.Result, Int)
			},
		},
		{
			name: "clear", recv: sl, method: "clear",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Void)
			},
		},
		{
			name: "contains", recv: sl, method: "contains",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "value")
				wantParams(t, fn, []Type{Int})
				wantType(t, fn.Result, Bool)
			},
		},
		{
			name: "find", recv: sl, method: "find",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "pred")
				wantPredParam(t, fn, Int)
				wantOptionElem(t, u, fn.Result, Int)
			},
		},
		{
			name: "index_of", recv: sl, method: "index_of",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "value")
				wantParams(t, fn, []Type{Int})
				wantOptionElem(t, u, fn.Result, Int)
			},
		},
		{
			name: "map", recv: sl, method: "map",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				up := oneTypeParam(t, fn, "U")
				wantParamNames(t, fn, "f")
				wantMapFnParam(t, fn, Int, up)
				wantSliceElem(t, fn.Result, up)
			},
		},
		{
			name: "filter", recv: sl, method: "filter",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "pred")
				wantPredParam(t, fn, Int)
				wantSliceElem(t, fn.Result, Int)
			},
		},
		{
			name: "reduce", recv: sl, method: "reduce",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				oneTypeParam(t, fn, "U")
				wantParamNames(t, fn, "init", "f")
				wantReduceParams(t, fn, Int)
				wantReduceResult(t, fn.Result, Int)
			},
		},
		{
			name: "sort", recv: sl, method: "sort",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Void)
			},
		},
		{
			name: "sort_by", recv: sl, method: "sort_by",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "cmp")
				cmp := wantFnType(t, fn.Params[0].Type)
				wantParams(t, cmp, []Type{Int, Int})
				wantType(t, cmp.Result, Int)
				wantType(t, fn.Result, Void)
			},
		},
		{
			name: "reverse", recv: sl, method: "reverse",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Void)
			},
		},
		{
			name: "first", recv: sl, method: "first",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantOptionElem(t, u, fn.Result, Int)
			},
		},
		{
			name: "last", recv: sl, method: "last",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantOptionElem(t, u, fn.Result, Int)
			},
		},
		{
			name: "slice", recv: sl, method: "slice",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "i", "j")
				wantParams(t, fn, []Type{Int, Int})
				wantSliceElem(t, fn.Result, Int)
			},
		},
		{
			name: "join", recv: sl, method: "join",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "sep")
				wantParams(t, fn, []Type{String_})
				wantType(t, fn.Result, String_)
			},
		},
		{
			name: "is_empty", recv: sl, method: "is_empty",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Bool)
			},
		},
		{
			name: "string_elem_push", recv: slStr, method: "push",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{String_})
				wantType(t, fn.Result, Void)
			},
		},
		{
			name: "string_elem_pop", recv: slStr, method: "pop",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantOptionElem(t, u, fn.Result, String_)
			},
		},
		{
			name: "string_elem_join", recv: slStr, method: "join",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{String_})
				wantType(t, fn.Result, String_)
			},
		},
		{
			name: "unknown", recv: sl, method: "zzz", ok: false,
		},
		{
			name: "option_method_not_on_slice", recv: sl, method: "is_some", ok: false,
		},
		{
			name: "string_method_not_on_slice", recv: sl, method: "to_upper", ok: false,
		},
	}
	runMethodTests(t, u, tests)
}

func TestStringMethods(t *testing.T) {
	u := NewUniverse()

	tests := []methodTest{
		{
			name: "len", recv: String_, method: "len", field: true,
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Int)
			},
		},
		{
			name: "contains", recv: String_, method: "contains",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{String_})
				wantType(t, fn.Result, Bool)
			},
		},
		{
			name: "starts_with", recv: String_, method: "starts_with",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{String_})
				wantType(t, fn.Result, Bool)
			},
		},
		{
			name: "ends_with", recv: String_, method: "ends_with",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{String_})
				wantType(t, fn.Result, Bool)
			},
		},
		{
			name: "split", recv: String_, method: "split",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "sep")
				wantParams(t, fn, []Type{String_})
				wantSliceElem(t, fn.Result, String_)
			},
		},
		{
			name: "trim", recv: String_, method: "trim",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, String_)
			},
		},
		{
			name: "to_upper", recv: String_, method: "to_upper",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, String_)
			},
		},
		{
			name: "to_lower", recv: String_, method: "to_lower",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, String_)
			},
		},
		{
			name: "replace", recv: String_, method: "replace",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "old", "new")
				wantParams(t, fn, []Type{String_, String_})
				wantType(t, fn.Result, String_)
			},
		},
		{
			name: "index_of", recv: String_, method: "index_of",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParamNames(t, fn, "sub")
				wantParams(t, fn, []Type{String_})
				wantOptionElem(t, u, fn.Result, Int)
			},
		},
		{
			name: "bytes", recv: String_, method: "bytes",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantSliceElem(t, fn.Result, Byte)
			},
		},
		{
			name: "chars", recv: String_, method: "chars",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantSliceElem(t, fn.Result, Rune)
			},
		},
		{
			name: "parse_int", recv: String_, method: "parse_int",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantResultTypes(t, u, fn.Result, Int, u.Error)
			},
		},
		{
			name: "parse_float", recv: String_, method: "parse_float",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantResultTypes(t, u, fn.Result, Float, u.Error)
			},
		},
		{
			name: "to_string", recv: String_, method: "to_string",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, String_)
			},
		},
		{
			name: "is_empty", recv: String_, method: "is_empty",
			check: func(t *testing.T, u *Universe, fn *Fn) {
				wantParams(t, fn, []Type{})
				wantType(t, fn.Result, Bool)
			},
		},
		{
			name: "unknown", recv: String_, method: "zzz", ok: false,
		},
		{
			name: "slice_method_not_on_string", recv: String_, method: "push", ok: false,
		},
		{
			name: "option_method_not_on_string", recv: String_, method: "unwrap", ok: false,
		},
	}
	runMethodTests(t, u, tests)
}

func TestBuiltinMethodSpecUnknownNames(t *testing.T) {
	tests := []struct {
		name string
		call func() (methodSpec, bool)
	}{
		{"option", func() (methodSpec, bool) { return optionMethod(Int, "zzz") }},
		{"result", func() (methodSpec, bool) { return resultMethod(Int, String_, "zzz") }},
		{"slice", func() (methodSpec, bool) { return sliceMethod(NewSlice(Int).(*Slice), "zzz") }},
		{"string", func() (methodSpec, bool) { return stringMethod("zzz") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, ok := tt.call()
			if ok {
				t.Fatal("ok = true, want false")
			}
			if spec.Name != "" || spec.Build != nil || spec.Field {
				t.Errorf("spec = %+v, want zero value", spec)
			}
		})
	}
}

func TestBuiltinMethodNames_OptionResultAllResolvable(t *testing.T) {
	u := NewUniverse()
	recvs := []struct {
		name  string
		recv  Type
		names []string
	}{
		{"option", u.OptionOf(Int), optionNames},
		{"result", u.ResultOf(Int, String_), resultNames},
	}
	for _, tt := range recvs {
		t.Run(tt.name, func(t *testing.T) {
			for _, n := range tt.names {
				fn, ok := lookupBuiltinMethod(u, tt.recv, n)
				if !ok {
					t.Errorf("method %q not found on %s", n, tt.name)
					continue
				}
				if fn == nil {
					t.Errorf("method %q returned nil fn", n)
				}
			}
		})
	}
}
