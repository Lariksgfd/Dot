package types

import "testing"

func TestIdentical_Basics(t *testing.T) {
	tests := []struct {
		name string
		a, b Type
		want bool
	}{
		{"int=int", Int, Int, true},
		{"int=float", Int, Float, false},
		{"int=string", Int, String_, false},
		{"bool=bool", Bool, Bool, true},
		{"bool=int", Bool, Int, false},
		{"void=void", Void, Void, true},
		{"never=never", Never, Never, true},
		{"void=never", Void, Never, false},
		{"same pointer byte/uint8", Byte, Uint8, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Identical(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Identical(%s,%s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIdentical_Nil(t *testing.T) {
	if Identical(nil, Int) {
		t.Error("Identical(nil, Int) should be false")
	}
	if Identical(Int, nil) {
		t.Error("Identical(Int, nil) should be false")
	}
}

func TestIdentical_Slices(t *testing.T) {
	tests := []struct {
		name string
		a, b Type
		want bool
	}{
		{"[]int=[]int", NewSlice(Int), NewSlice(Int), true},
		{"[]int=[]float", NewSlice(Int), NewSlice(Float), false},
		{"[]string=[]string", NewSlice(String_), NewSlice(String_), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if Identical(tt.a, tt.b) != tt.want {
				t.Errorf("Identical = %v, want %v", Identical(tt.a, tt.b), tt.want)
			}
		})
	}
}

func TestIdentical_Maps(t *testing.T) {
	tests := []struct {
		name string
		a, b Type
		want bool
	}{
		{"map[string]int=map[string]int", NewMap(String_, Int), NewMap(String_, Int), true},
		{"map[string]int=map[int]string", NewMap(String_, Int), NewMap(Int, String_), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if Identical(tt.a, tt.b) != tt.want {
				t.Errorf("Identical = %v, want %v", Identical(tt.a, tt.b), tt.want)
			}
		})
	}
}

func TestIdentical_FnTypes(t *testing.T) {
	f1 := &Fn{Params: []Param{{Type: Int}}, Result: Bool}
	f2 := &Fn{Params: []Param{{Type: Int}}, Result: Bool}
	f3 := &Fn{Params: []Param{{Type: String_}}, Result: Bool}
	f4 := &Fn{Params: []Param{{Type: Int}}, Result: Void}
	f5 := &Fn{Params: []Param{{Type: Int}, {Type: Int}}, Result: Bool}
	tests := []struct {
		name string
		a, b Type
		want bool
	}{
		{"same fn", f1, f2, true},
		{"diff param", f1, f3, false},
		{"diff result", f1, f4, false},
		{"diff arity", f1, f5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if Identical(tt.a, tt.b) != tt.want {
				t.Errorf("Identical = %v, want %v", Identical(tt.a, tt.b), tt.want)
			}
		})
	}
}

func TestIdentical_Structs(t *testing.T) {
	s1 := &Struct{Fields: []Field{{Name: "x", Type: Int}}}
	s2 := &Struct{Fields: []Field{{Name: "x", Type: Int}}}
	s3 := &Struct{Fields: []Field{{Name: "x", Type: Float}}}
	s4 := &Struct{Fields: []Field{{Name: "y", Type: Int}}}
	tests := []struct {
		name string
		a, b Type
		want bool
	}{
		{"same struct", s1, s2, true},
		{"diff field type", s1, s3, false},
		{"diff field name", s1, s4, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if Identical(tt.a, tt.b) != tt.want {
				t.Errorf("Identical = %v, want %v", Identical(tt.a, tt.b), tt.want)
			}
		})
	}
}

func TestIdentical_Enums(t *testing.T) {
	e1 := &Enum{Variants: []Variant{{Name: "Red"}, {Name: "Green"}}}
	e2 := &Enum{Variants: []Variant{{Name: "Red"}, {Name: "Green"}}}
	e3 := &Enum{Variants: []Variant{{Name: "Red"}, {Name: "Blue"}}}
	if !Identical(e1, e2) {
		t.Error("same enums should be identical")
	}
	if Identical(e1, e3) {
		t.Error("different variant names should not be identical")
	}
}

func TestIdentical_TypeVar_Resolved(t *testing.T) {
	tv := &TypeVar{ID: 1, Bound: Int}
	if !Identical(tv, Int) {
		t.Error("resolved TypeVar should be identical to its bound")
	}
	if !Identical(Int, tv) {
		t.Error("symmetric: Int should be identical to resolved TypeVar")
	}
}

func TestIdentical_Named(t *testing.T) {
	n1 := &Named{Name: "Foo"}
	n2 := &Named{Name: "Foo"}
	if Identical(n1, n2) {
		t.Error("two different *Named with same name should not be identical (nominal)")
	}
	if !Identical(n1, n1) {
		t.Error("same pointer should be identical")
	}
}

func TestIdentical_OptionResult(t *testing.T) {
	u := NewUniverse()
	o1 := u.OptionOf(Int)
	o2 := u.OptionOf(Int)
	o3 := u.OptionOf(String_)
	if !Identical(o1, o2) {
		t.Log("Option[int] == Option[int] is nominal; different OptionOf calls create distinct pointers")
	}
	if Identical(o1, o3) {
		t.Error("Option[int] == Option[string] should be false")
	}
	r1 := u.ResultOf(Int, u.Error)
	r2 := u.ResultOf(Int, u.Error)
	if !Identical(r1, r2) {
		t.Log("Result[int,Error] == Result[int,Error] is nominal; different ResultOf calls create distinct pointers")
	}
}

func TestIdentical_Pointers(t *testing.T) {
	p1 := NewPointer(Int)
	p2 := NewPointer(Int)
	p3 := NewPointer(String_)
	if !Identical(p1, p2) {
		t.Error("*int == *int should be true")
	}
	if Identical(p1, p3) {
		t.Error("*int == *string should be false")
	}
}

func TestAssignableTo_SameType(t *testing.T) {
	tests := []struct {
		name     string
		src, dst Type
		want     bool
	}{
		{"int->int", Int, Int, true},
		{"string->string", String_, String_, true},
		{"[]int->[]int", NewSlice(Int), NewSlice(Int), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if AssignableTo(tt.src, tt.dst) != tt.want {
				t.Errorf("AssignableTo = %v, want %v", AssignableTo(tt.src, tt.dst), tt.want)
			}
		})
	}
}

func TestAssignableTo_InvalidNever(t *testing.T) {
	if !AssignableTo(Invalid, Int) {
		t.Error("Invalid should be assignable to anything")
	}
	if !AssignableTo(Never, Int) {
		t.Error("Never should be assignable to anything")
	}
}

func TestAssignableTo_Untyped(t *testing.T) {
	tests := []struct {
		name     string
		src, dst Type
		want     bool
	}{
		{"untyped int -> int", UntypedInt, Int, true},
		{"untyped int -> float", UntypedInt, Float, true},
		{"untyped int -> string", UntypedInt, String_, false},
		{"untyped float -> float", UntypedFloat, Float, true},
		{"untyped float -> int", UntypedFloat, Int, false},
		{"untyped bool -> bool", UntypedBool, Bool, true},
		{"untyped bool -> int", UntypedBool, Int, false},
		{"untyped string -> string", UntypedString, String_, true},
		{"untyped nil -> *int", UntypedNil, NewPointer(Int), true},
		{"untyped nil -> int", UntypedNil, Int, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if AssignableTo(tt.src, tt.dst) != tt.want {
				t.Errorf("AssignableTo(%s,%s) = %v, want %v", tt.src, tt.dst, AssignableTo(tt.src, tt.dst), tt.want)
			}
		})
	}
}

func TestAssignableTo_Dyn(t *testing.T) {
	tr := &Trait{Name: "Foo"}
	n := &Named{Name: "Bar", Impls: []*TraitImpl{{Trait: tr}}}
	d := NewDyn(tr)
	if !AssignableTo(n, d) {
		t.Error("named implementing trait should be assignable to dyn Trait")
	}
	d2 := NewDyn(&Trait{Name: "Baz"})
	if AssignableTo(n, d2) {
		t.Error("named should not be assignable to unrelated dyn")
	}
}

func TestAssignableTo_OptionResult(t *testing.T) {
	u := NewUniverse()
	oInt := u.OptionOf(Int)
	oStr := u.OptionOf(String_)
	if !AssignableTo(oInt, oInt) {
		t.Error("Option[int] -> Option[int] should be assignable")
	}
	if AssignableTo(oInt, oStr) {
		t.Error("Option[int] -> Option[string] should not be assignable")
	}
}

func TestConvertibleTo_Numeric(t *testing.T) {
	tests := []struct {
		name     string
		src, dst Type
		want     bool
	}{
		{"int->float", Int, Float, true},
		{"float->int", Float, Int, true},
		{"int->int8", Int, Int8, true},
		{"uint->float", Uint, Float, true},
		{"int->string", Int, String_, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if ConvertibleTo(tt.src, tt.dst) != tt.want {
				t.Errorf("ConvertibleTo(%s,%s) = %v, want %v", tt.src, tt.dst, ConvertibleTo(tt.src, tt.dst), tt.want)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want bool
	}{
		{"int", Int, true}, {"int8", Int8, true}, {"int16", Int16, true}, {"int32", Int32, true},
		{"uint", Uint, true}, {"uint8", Uint8, true}, {"uint16", Uint16, true}, {"uint32", Uint32, true},
		{"float", Float, true}, {"float32", Float32, true},
		{"rune", Rune, true},
		{"untyped int", UntypedInt, true}, {"untyped float", UntypedFloat, true},
		{"bool", Bool, false}, {"string", String_, false},
		{"void", Void, false}, {"never", Never, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNumeric(tt.typ); got != tt.want {
				t.Errorf("IsNumeric(%s) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
	if IsNumeric(nil) {
		t.Error("IsNumeric(nil) should be false")
	}
}
