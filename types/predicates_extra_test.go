package types

import "testing"

func TestIsInteger(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want bool
	}{
		{"int", Int, true}, {"int8", Int8, true}, {"int16", Int16, true}, {"int32", Int32, true},
		{"uint", Uint, true}, {"uint8", Uint8, true}, {"uint16", Uint16, true}, {"uint32", Uint32, true},
		{"rune", Rune, true}, {"untyped int", UntypedInt, true},
		{"float", Float, false}, {"float32", Float32, false},
		{"bool", Bool, false}, {"string", String_, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInteger(tt.typ); got != tt.want {
				t.Errorf("IsInteger(%s) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
	if IsInteger(nil) {
		t.Error("IsInteger(nil) should be false")
	}
}

func TestIsFloat(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want bool
	}{
		{"float", Float, true}, {"float32", Float32, true}, {"untyped float", UntypedFloat, true},
		{"int", Int, false}, {"bool", Bool, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsFloat(tt.typ); got != tt.want {
				t.Errorf("IsFloat(%s) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
	if IsFloat(nil) {
		t.Error("IsFloat(nil) should be false")
	}
}

func TestIsSigned(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want bool
	}{
		{"int", Int, true}, {"int8", Int8, true}, {"int16", Int16, true}, {"int32", Int32, true},
		{"rune", Rune, true}, {"untyped int", UntypedInt, true},
		{"uint", Uint, false}, {"uint8", Uint8, false}, {"byte", Byte, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSigned(tt.typ); got != tt.want {
				t.Errorf("IsSigned(%s) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}

func TestIsUnsigned(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want bool
	}{
		{"uint", Uint, true}, {"uint8", Uint8, true}, {"uint16", Uint16, true}, {"uint32", Uint32, true},
		{"int", Int, false}, {"untyped int", UntypedInt, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnsigned(tt.typ); got != tt.want {
				t.Errorf("IsUnsigned(%s) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}

func TestIsBooleanString(t *testing.T) {
	if !IsBoolean(Bool) {
		t.Error("Bool should be boolean")
	}
	if IsBoolean(Int) {
		t.Error("Int should not be boolean")
	}
	if !IsStringType(String_) {
		t.Error("String_ should be string type")
	}
	if IsStringType(Int) {
		t.Error("Int should not be string type")
	}
}

func TestComparable_Ordered(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want bool
	}{
		{"int", Int, true}, {"float", Float, true}, {"string", String_, true},
		{"bool", Bool, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Ordered(tt.typ); got != tt.want {
				t.Errorf("Ordered(%s) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}

func TestComparable(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want bool
	}{
		{"int", Int, true}, {"string", String_, true}, {"bool", Bool, true},
		{"void", Void, false}, {"never", Never, false},
		{"*int", NewPointer(Int), true},
		{"struct", &Struct{}, true},
		{"tuple", &Tuple{Elems: []Type{Int, Int}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Comparable(tt.typ); got != tt.want {
				t.Errorf("Comparable(%s) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}

func TestUnderlying(t *testing.T) {
	n := &Named{Name: "Foo", Underlying: Int}
	if got := Underlying(n); got != Int {
		t.Errorf("Underlying(Named{Int}) = %v, want Int", got)
	}
	if got := Underlying(Int); got != Int {
		t.Errorf("Underlying(int) = %v, want int", got)
	}
	nested := &Named{Name: "Bar", Underlying: n}
	if got := Underlying(nested); got != Int {
		t.Errorf("Underlying(chain) = %v, want Int", got)
	}
	tv := &TypeVar{ID: 1, Bound: Int}
	if got := Underlying(tv); got != Int {
		t.Errorf("Underlying(TypeVar->Int) = %v, want Int", got)
	}
}

func TestDefault(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want Type
	}{
		{"untyped int", UntypedInt, Int},
		{"untyped float", UntypedFloat, Float},
		{"untyped bool", UntypedBool, Bool},
		{"untyped string", UntypedString, String_},
		{"untyped nil", UntypedNil, Invalid},
		{"int", Int, Int},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Default(tt.typ); got != tt.want {
				t.Errorf("Default(%s) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
	if got := Default(nil); got != Invalid {
		t.Errorf("Default(nil) = %v, want Invalid", got)
	}
}

func TestIsHeap(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want bool
	}{
		{"int", Int, false},
		{"string", String_, true},
		{"[]int", NewSlice(Int), true},
		{"map", NewMap(String_, Int), true},
		{"struct", &Struct{}, false},
		{"dyn", NewDyn(&Trait{Name: "T"}), true},
		{"chan", NewChan(Int), true},
		{"future", NewFuture(Int), true},
		{"named struct", &Named{Name: "Foo", Underlying: &Struct{}}, false},
		{"named int", &Named{Name: "Bar", Underlying: Int}, false},
		{"enum without payload", &Enum{Variants: []Variant{{Name: "A"}, {Name: "B"}}}, false},
		{"enum with payload", &Enum{Variants: []Variant{{Name: "A", Fields: []Param{{Type: Int}}}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHeap(tt.typ)
			if got != tt.want {
				t.Errorf("IsHeap(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
	if IsHeap(nil) {
		t.Error("IsHeap(nil) should be false")
	}
}

func TestIsCopy(t *testing.T) {
	if IsCopy(Int) != !IsHeap(Int) {
		t.Error("IsCopy should be !IsHeap")
	}
}

func TestDeref(t *testing.T) {
	if got := Deref(NewPointer(Int)); got != Int {
		t.Errorf("Deref(*int) = %v, want Int", got)
	}
	if got := Deref(NewWeak(Int)); got != Int {
		t.Errorf("Deref(weak int) = %v, want Int", got)
	}
	if got := Deref(Int); got != Int {
		t.Errorf("Deref(int) = %v, want Int", got)
	}
	if got := Deref(nil); got != Invalid {
		t.Errorf("Deref(nil) = %v, want Invalid", got)
	}
}
