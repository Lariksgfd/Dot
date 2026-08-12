package types

import "testing"

func TestUnify_Primitives(t *testing.T) {
	tests := []struct {
		name string
		a, b Type
		want bool
	}{
		{"int=int", Int, Int, true},
		{"int=float", Int, Float, false},
		{"invalid=invalid", Invalid, Invalid, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unify(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("unify(%s,%s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestUnify_TypeVar(t *testing.T) {
	tv := &TypeVar{ID: 1}
	if !unify(tv, Int) {
		t.Error("unify(?T, int) should succeed")
	}
	if tv.Bound != Int {
		t.Errorf("?T should be bound to int, got %v", tv.Bound)
	}

	tv2 := &TypeVar{ID: 2}
	tv3 := &TypeVar{ID: 3}
	if !unify(tv2, tv3) {
		t.Error("unify(?T, ?U) without bounds should succeed")
	}
	if tv2.Bound != tv3 {
		t.Errorf("?T should be bound to ?U, got %v", tv2.Bound)
	}
}

func TestUnify_AlreadyBound(t *testing.T) {
	tv := &TypeVar{ID: 1, Bound: Int}
	if !unify(tv, Int) {
		t.Error("unify(bound ?T to int, int) should succeed")
	}
	if unify(tv, Float) {
		t.Error("unify(bound ?T to int, float) should fail")
	}
}

func TestUnify_Composites(t *testing.T) {
	tv := &TypeVar{ID: 1}
	if !unify(&Slice{Elem: tv}, NewSlice(Int)) {
		t.Error("unify([]?T, []int) should succeed")
	}
	if tv.Bound != Int {
		t.Errorf("?T should be bound to int, got %v", tv.Bound)
	}
}

func TestUnify_Maps(t *testing.T) {
	tk := &TypeVar{ID: 1}
	tv := &TypeVar{ID: 2}
	if !unify(&Map{Key: tk, Value: tv}, NewMap(String_, Int)) {
		t.Error("unify(map[?K]?V, map[string]int) should succeed")
	}
	if tk.Bound != String_ {
		t.Errorf("?K should be bound to string, got %v", tk.Bound)
	}
	if tv.Bound != Int {
		t.Errorf("?V should be bound to int, got %v", tv.Bound)
	}
}

func TestUnify_FnTypes(t *testing.T) {
	tv := &TypeVar{ID: 1}
	f1 := &Fn{Params: []Param{{Type: tv}}, Result: Bool}
	f2 := &Fn{Params: []Param{{Type: Int}}, Result: Bool}
	if !unify(f1, f2) {
		t.Error("unify(fn(?T)->bool, fn(int)->bool) should succeed")
	}
	if tv.Bound != Int {
		t.Errorf("?T should be bound to int, got %v", tv.Bound)
	}
}

func TestUnify_Named(t *testing.T) {
	u := NewUniverse()
	optInt := u.OptionOf(Int)
	optStr := u.OptionOf(String_)

	if !unify(optInt, optInt) {
		t.Error("unify(Option[int], Option[int]) should succeed")
	}
	if unify(optInt, optStr) {
		t.Error("unify(Option[int], Option[string]) should fail")
	}
}

func TestBindVar(t *testing.T) {
	tv := &TypeVar{ID: 1}
	if !bindVar(tv, Int) {
		t.Error("bindVar should succeed")
	}
	if tv.Bound != Int {
		t.Errorf("bound should be int, got %v", tv.Bound)
	}
}

func TestBindVar_OccursCheck(t *testing.T) {
	tv := &TypeVar{ID: 1}
	selfRef := &Slice{Elem: tv}
	if bindVar(tv, selfRef) {
		t.Error("bindVar(?T, []?T) should fail due to occurs check")
	}
}

func TestBindVar_SelfBind(t *testing.T) {
	tv := &TypeVar{ID: 1}
	if !bindVar(tv, tv) {
		t.Error("bindVar(?T, ?T) should succeed (same pointer)")
	}
}

func TestBindVar_AlreadyBound(t *testing.T) {
	tv := &TypeVar{ID: 1, Bound: Int}
	if !bindVar(tv, Int) {
		t.Error("re-binding bound var to same type should succeed")
	}
	if bindVar(tv, Float) {
		t.Error("re-binding bound var to different type should fail")
	}
}

func TestOccurs(t *testing.T) {
	tv := &TypeVar{ID: 1}

	tests := []struct {
		name string
		typ  Type
		want bool
	}{
		{"in slice", &Slice{Elem: tv}, true},
		{"in array", &Array{Elem: tv, Len: 5}, true},
		{"in map key", &Map{Key: tv, Value: Int}, true},
		{"in map val", &Map{Key: Int, Value: tv}, true},
		{"in pointer", &Pointer{Elem: tv}, true},
		{"in weak", &Weak{Elem: tv}, true},
		{"in chan", &Chan{Elem: tv}, true},
		{"in future", &Future{Result: tv}, true},
		{"in tuple", &Tuple{Elems: []Type{Int, tv}}, true},
		{"in fn param", &Fn{Params: []Param{{Type: tv}}, Result: Void}, true},
		{"in fn result", &Fn{Params: []Param{{Type: Int}}, Result: tv}, true},
		{"in named args", &Named{Name: "Foo", TypeArgs: []Type{tv}}, true},
		{"not in int", Int, false},
		{"not in string", String_, false},
		{"same var", tv, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := occurs(tv, tt.typ)
			if got != tt.want {
				t.Errorf("occurs(%s, %s) = %v, want %v", tv, tt.typ, got, tt.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	tv := &TypeVar{ID: 1, Bound: Int}
	if got := resolve(tv); got != Int {
		t.Errorf("resolve(?T->int) = %v, want Int", got)
	}

	chained := &TypeVar{ID: 2, Bound: tv}
	if got := resolve(chained); got != Int {
		t.Errorf("resolve(?U->?T->int) = %v, want Int", got)
	}

	if got := resolve(Int); got != Int {
		t.Errorf("resolve(int) = %v, want Int", got)
	}
}

func TestSameOrigin(t *testing.T) {
	n1 := &Named{Name: "Foo"}
	n2 := &Named{Name: "Foo", Origin: n1}
	n3 := &Named{Name: "Foo"}

	if !sameOrigin(n1, n2) {
		t.Error("sameOrigin(nil origin, has origin) should be true")
	}
	if sameOrigin(n1, n3) {
		t.Error("sameOrigin(different pointers) should be false")
	}
}

func TestHasUnresolvedVars(t *testing.T) {
	tv := &TypeVar{ID: 1}
	bound := &TypeVar{ID: 2, Bound: Int}

	if !hasUnresolvedVars(tv) {
		t.Error("unbound TypeVar should have unresolved vars")
	}
	if hasUnresolvedVars(bound) {
		t.Error("bound TypeVar should not have unresolved vars")
	}
	if !hasUnresolvedVars(&Slice{Elem: tv}) {
		t.Error("nested unbound should have unresolved vars")
	}
	if hasUnresolvedVars(&Slice{Elem: bound}) {
		t.Error("nested bound should not have unresolved vars")
	}
	if hasUnresolvedVars(Int) {
		t.Error("primitive should not have unresolved vars")
	}
}

func TestUnify_Untyped(t *testing.T) {
	if !unify(UntypedInt, Int) {
		t.Error("unify(untyped int, int) should succeed (assignment falls through)")
	}
	if unify(UntypedInt, String_) {
		t.Error("unify(untyped int, string) should fail")
	}
}

func TestUnify_Nil(t *testing.T) {
	if unify(nil, Int) {
		t.Error("unify(nil, int) should fail")
	}
	if unify(Int, nil) {
		t.Error("unify(int, nil) should fail")
	}
}

func TestUnify_MismatchedKinds(t *testing.T) {
	if unify(NewSlice(Int), Int) {
		t.Error("unify([]int, int) should fail")
	}
	if unify(NewMap(String_, Int), NewSlice(Int)) {
		t.Error("unify(map, slice) should fail")
	}
}

func TestBindVar_AlreadyBoundCompatible(t *testing.T) {
	_ = &TypeVar{ID: 1, Bound: Int}
	tv2 := &TypeVar{ID: 2}
	if !bindVar(tv2, Int) {
		t.Error("first bind should succeed")
	}
	if !bindVar(tv2, Int) {
		t.Error("re-bind same type should succeed")
	}
}
