package types

import (
	"strings"
	"testing"
)

func TestNewSlice(t *testing.T) {
	tests := []struct {
		name string
		elem Type
		want Type
	}{
		{"int elem", Int, &Slice{Elem: Int}},
		{"string elem", String_, &Slice{Elem: String_}},
		{"invalid elem", Invalid, Invalid},
		{"nil elem", nil, Invalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSlice(tt.elem)
			if tt.want == Invalid {
				if got != Invalid {
					t.Fatalf("NewSlice(%v) = %v, want Invalid", tt.elem, got)
				}
				return
			}
			s := got.(*Slice)
			if s.Elem != tt.want.(*Slice).Elem {
				t.Errorf("Elem = %v, want %v", s.Elem, tt.want.(*Slice).Elem)
			}
		})
	}
}

func TestNewMap(t *testing.T) {
	tests := []struct {
		name string
		k, v Type
		want Type
	}{
		{"string->int", String_, Int, &Map{Key: String_, Value: Int}},
		{"int->bool", Int, Bool, &Map{Key: Int, Value: Bool}},
		{"invalid key", Invalid, Int, Invalid},
		{"nil key", nil, Int, Invalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewMap(tt.k, tt.v)
			if tt.want == Invalid {
				if got != Invalid {
					t.Fatalf("got %v, want Invalid", got)
				}
				return
			}
			m := got.(*Map)
			if m.Key != tt.k || m.Value != tt.v {
				t.Errorf("map[%v]%v, want map[%v]%v", m.Key, m.Value, tt.k, tt.v)
			}
		})
	}
}

func TestNewArray(t *testing.T) {
	tests := []struct {
		name string
		elem Type
		n    int64
		want Type
	}{
		{"[5]int", Int, 5, &Array{Elem: Int, Len: 5}},
		{"invalid elem", Invalid, 3, Invalid},
		{"nil elem", nil, 3, Invalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewArray(tt.elem, tt.n)
			if tt.want == Invalid {
				if got != Invalid {
					t.Fatalf("got %v, want Invalid", got)
				}
				return
			}
			a := got.(*Array)
			if a.Elem != tt.elem || a.Len != tt.n {
				t.Errorf("[%d]%v, want [%d]%v", a.Len, a.Elem, tt.n, tt.elem)
			}
		})
	}
}

func TestNewTuple(t *testing.T) {
	tests := []struct {
		name  string
		elems []Type
		want  Type
	}{
		{"empty", []Type{}, Void},
		{"single int", []Type{Int}, Int},
		{"(int,string)", []Type{Int, String_}, &Tuple{Elems: []Type{Int, String_}}},
		{"with invalid", []Type{Int, Invalid}, Invalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewTuple(tt.elems...)
			switch w := tt.want.(type) {
			case *Tuple:
				g := got.(*Tuple)
				if len(g.Elems) != len(w.Elems) {
					t.Fatalf("len = %d, want %d", len(g.Elems), len(w.Elems))
				}
				for i := range g.Elems {
					if g.Elems[i] != w.Elems[i] {
						t.Errorf("[%d] = %v, want %v", i, g.Elems[i], w.Elems[i])
					}
				}
			default:
				if got != w {
					t.Errorf("= %v, want %v", got, w)
				}
			}
		})
	}
}

func TestNewPointer(t *testing.T) {
	p := NewPointer(Int).(*Pointer)
	if p.Elem != Int {
		t.Errorf("Elem = %v, want Int", p.Elem)
	}
	if got := NewPointer(nil); got != Invalid {
		t.Errorf("NewPointer(nil) = %v, want Invalid", got)
	}
}

func TestNewDyn(t *testing.T) {
	tr := &Trait{Name: "Foo"}
	d := NewDyn(tr).(*Dyn)
	if d.Trait != tr {
		t.Errorf("Trait = %v, want %v", d.Trait, tr)
	}
	if got := NewDyn(nil); got != Invalid {
		t.Errorf("NewDyn(nil) = %v, want Invalid", got)
	}
}

func TestNewChan(t *testing.T) {
	if c := NewChan(Int).(*Chan); c.Elem != Int {
		t.Errorf("Elem = %v, want Int", c.Elem)
	}
	if got := NewChan(nil); got != Invalid {
		t.Errorf("NewChan(nil) = %v, want Invalid", got)
	}
}

func TestNewFuture(t *testing.T) {
	if f := NewFuture(Int).(*Future); f.Result != Int {
		t.Errorf("Result = %v, want Int", f.Result)
	}
	if got := NewFuture(nil); got != Invalid {
		t.Errorf("NewFuture(nil) = %v, want Invalid", got)
	}
}

func TestNewWeak(t *testing.T) {
	if w := NewWeak(Int).(*Weak); w.Elem != Int {
		t.Errorf("Elem = %v, want Int", w.Elem)
	}
	if got := NewWeak(nil); got != Invalid {
		t.Errorf("NewWeak(nil) = %v, want Invalid", got)
	}
}

func TestTypeString(t *testing.T) {
	u := NewUniverse()
	tests := []struct {
		name string
		typ  Type
		want string
	}{
		{"int", Int, "int"},
		{"string", String_, "string"},
		{"void", Void, "void"},
		{"never", Never, "never"},
		{"invalid", Invalid, "invalid type"},
		{"[]int", NewSlice(Int), "[]int"},
		{"map[string]int", NewMap(String_, Int), "map[string]int"},
		{"fn", &Fn{Params: []Param{{Name: "a", Type: Int}}, Result: Bool}, "fn(a int) -> bool"},
		{"Option[int]", u.OptionOf(Int), "Option[int]"},
		{"struct", &Struct{Fields: []Field{{Name: "x", Type: Int}}}, "struct { x int }"},
		{"enum", &Enum{Variants: []Variant{{Name: "Red"}, {Name: "Green"}}}, "enum { Red, Green }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	tests := []struct {
		k    Kind
		want string
	}{
		{KindInt, "int"}, {KindBool, "bool"}, {KindString, "string"},
		{KindSlice, "slice"}, {KindMap, "map"}, {KindFn, "fn"},
		{KindStruct, "struct"}, {KindEnum, "enum"}, {KindNamed, "named"},
		{KindTypeVar, "typevar"}, {KindVoid, "void"}, {KindNever, "never"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.k.String(); got != tt.want {
				t.Errorf("Kind.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBasicKind_AllPrimitives(t *testing.T) {
	tests := []struct {
		name string
		b    *Basic
		kind Kind
	}{
		{"int", Int, KindInt}, {"int8", Int8, KindInt8}, {"int16", Int16, KindInt16},
		{"int32", Int32, KindInt32},
		{"uint", Uint, KindUint}, {"uint8", Uint8, KindUint8}, {"uint16", Uint16, KindUint16},
		{"uint32", Uint32, KindUint32},
		{"float", Float, KindFloat}, {"float32", Float32, KindFloat32},
		{"bool", Bool, KindBool}, {"string", String_, KindString},
		{"rune", Rune, KindRune}, {"byte", Byte, KindUint8},
		{"void", Void, KindVoid}, {"never", Never, KindNever},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.b.Kind() != tt.kind {
				t.Errorf("Kind() = %v, want %v", tt.b.Kind(), tt.kind)
			}
		})
	}
}

func TestByteAlias(t *testing.T) {
	if Byte != Uint8 {
		t.Error("Byte must be the same pointer as Uint8")
	}
}

func TestFnArity(t *testing.T) {
	tests := []struct {
		name             string
		sig              *Fn
		wantReq, wantTot int
	}{
		{"no params", &Fn{}, 0, 0},
		{"one required", &Fn{Params: []Param{{Type: Int}}}, 1, 1},
		{"one with default", &Fn{Params: []Param{{Type: Int, HasDflt: true}}}, 0, 1},
		{"variadic", &Fn{Params: []Param{{Type: Int, Variadic: true}}}, 0, 1},
		{"mixed", &Fn{Params: []Param{{Type: Int}, {Type: String_, HasDflt: true}}}, 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, tot := tt.sig.Arity()
			if req != tt.wantReq || tot != tt.wantTot {
				t.Errorf("Arity() = (%d,%d), want (%d,%d)", req, tot, tt.wantReq, tt.wantTot)
			}
		})
	}
}

func TestIsBasic(t *testing.T) {
	if !IsBasic(Int) {
		t.Error("int should be basic")
	}
	if IsBasic(NewSlice(Int)) {
		t.Error("[]int should not be basic")
	}
}

func TestIsUntyped(t *testing.T) {
	if !IsUntyped(UntypedInt) {
		t.Error("untyped int should be untyped")
	}
	if IsUntyped(Int) {
		t.Error("int should not be untyped")
	}
}

func TestIsInvalid(t *testing.T) {
	if !IsInvalid(Invalid) || IsInvalid(Int) || IsInvalid(nil) {
		t.Error("IsInvalid checks failed")
	}
}

func TestIsNever(t *testing.T) {
	if !IsNever(Never) || IsNever(Int) {
		t.Error("IsNever checks failed")
	}
}

func TestIsVoid(t *testing.T) {
	if !IsVoid(Void) || IsVoid(Int) {
		t.Error("IsVoid checks failed")
	}
}

func TestStructString_Various(t *testing.T) {
	s := &Struct{Fields: []Field{{Name: "name", Type: String_}, {Name: "age", Type: Int}}}
	got := s.String()
	if !strings.Contains(got, "name string") || !strings.Contains(got, "age int") {
		t.Errorf("Struct.String() = %q", got)
	}
}
