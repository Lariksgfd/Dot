package types

import (
	"testing"

	"github.com/dotlang/dot/ast"
)

func newChecker() *Checker {
	u := NewUniverse()
	c := &Checker{
		info:             newInfo(u),
		univ:             u,
		file:             "test.dot",
		instantiateCache: make(map[instantiateCacheKey]*Named),
	}
	globalChecker = c
	c.scope = NewScope(u.Scope, ScopeFile)
	c.info.FileScope = c.scope
	return c
}

func TestResolveType_AllPrimitives(t *testing.T) {
	tests := []struct {
		name string
		ast  ast.Type
		want Type
	}{
		{"int", &ast.NamedType{Name: "int"}, Int},
		{"string", &ast.NamedType{Name: "string"}, String_},
		{"bool", &ast.NamedType{Name: "bool"}, Bool},
		{"float", &ast.NamedType{Name: "float"}, Float},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newChecker()
			got := c.resolveType(tt.ast)
			if got != tt.want {
				t.Errorf("resolveType(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestResolveType_Slice(t *testing.T) {
	c := newChecker()
	st := &ast.SliceType{Elem: &ast.NamedType{Name: "int"}}
	got := c.resolveType(st)

	s, ok := got.(*Slice)
	if !ok {
		t.Fatalf("resolveType([]int) = %T, want *Slice", got)
	}
	if s.Elem != Int {
		t.Errorf("elem = %v, want int", s.Elem)
	}
}

func TestResolveType_Map(t *testing.T) {
	c := newChecker()
	mt := &ast.MapType{
		Key:   &ast.NamedType{Name: "string"},
		Value: &ast.NamedType{Name: "int"},
	}
	got := c.resolveType(mt)

	m, ok := got.(*Map)
	if !ok {
		t.Fatalf("resolveType(map) = %T, want *Map", got)
	}
	if m.Key != String_ || m.Value != Int {
		t.Errorf("map[%v]%v, want map[string]int", m.Key, m.Value)
	}
}

func TestResolveType_Fn(t *testing.T) {
	c := newChecker()
	ft := &ast.FnType{
		Params: []ast.Type{&ast.NamedType{Name: "int"}},
		Result: &ast.NamedType{Name: "bool"},
	}
	got := c.resolveType(ft)

	f, ok := got.(*Fn)
	if !ok {
		t.Fatalf("resolveType(fn) = %T, want *Fn", got)
	}
	if len(f.Params) != 1 || f.Params[0].Type != Int {
		t.Errorf("params = %v", f.Params)
	}
	if f.Result != Bool {
		t.Errorf("result = %v, want bool", f.Result)
	}
}

func TestResolveType_Pointer(t *testing.T) {
	c := newChecker()
	pt := &ast.PointerType{Elem: &ast.NamedType{Name: "int"}}
	got := c.resolveType(pt)

	p, ok := got.(*Pointer)
	if !ok {
		t.Fatalf("resolveType(*int) = %T, want *Pointer", got)
	}
	if p.Elem != Int {
		t.Errorf("elem = %v, want int", p.Elem)
	}
}

func TestResolveType_Generic(t *testing.T) {
	c := newChecker()
	gt := &ast.GenericType{
		Base: &ast.NamedType{Name: "Option"},
		Args: []ast.Type{&ast.NamedType{Name: "int"}},
	}
	got := c.resolveType(gt)

	n, ok := got.(*Named)
	if !ok {
		t.Fatalf("resolveType(Option[int]) = %T, want *Named", got)
	}
	if n.Name != "Option" {
		t.Errorf("name = %q, want Option", n.Name)
	}
	if len(n.TypeArgs) != 1 || n.TypeArgs[0] != Int {
		t.Errorf("type args = %v, want [int]", n.TypeArgs)
	}
}

func TestResolveType_Nil(t *testing.T) {
	c := newChecker()
	got := c.resolveType(nil)
	if got != Invalid {
		t.Errorf("resolveType(nil) = %v, want Invalid", got)
	}
}

func TestResolveType_UndefinedNamed(t *testing.T) {
	c := newChecker()
	got := c.resolveType(&ast.NamedType{Name: "NoSuchType"})
	if got != Invalid {
		t.Errorf("should return Invalid for undefined type, got %v", got)
	}
}

func TestInstantiate(t *testing.T) {
	u := NewUniverse()
	c := &Checker{
		info:             newInfo(u),
		univ:             u,
		file:             "test.dot",
		instantiateCache: make(map[instantiateCacheKey]*Named),
	}
	globalChecker = c

	tp := &TypeParam{Name: "T", Index: 0}
	generic := &Named{
		Name:       "Stack",
		TypeParams: []*TypeParam{tp},
		Underlying: &Slice{Elem: tp},
		Pos:        ast.Position{},
	}

	result := c.instantiate(generic, []Type{Int})
	n, ok := result.(*Named)
	if !ok {
		t.Fatalf("instantiate = %T, want *Named", result)
	}
	if n.Origin != generic {
		t.Error("Origin should point to the generic")
	}
	if n.Name != "Stack" {
		t.Errorf("name = %q, want Stack", n.Name)
	}

	sl, ok := n.Underlying.(*Slice)
	if !ok {
		t.Fatalf("underlying = %T, want *Slice", n.Underlying)
	}
	if sl.Elem != Int {
		t.Errorf("substituted elem = %v, want int", sl.Elem)
	}
}

func TestSubstitute(t *testing.T) {
	tp := &TypeParam{Name: "T", Index: 0}
	subst := map[*TypeParam]Type{tp: Int}

	tests := []struct {
		name string
		typ  Type
		want Type
	}{
		{"type param", tp, Int},
		{"slice", &Slice{Elem: tp}, &Slice{Elem: Int}},
		{"map", &Map{Key: tp, Value: tp}, &Map{Key: Int, Value: Int}},
		{"pointer", &Pointer{Elem: tp}, &Pointer{Elem: Int}},
		{"array", &Array{Elem: tp, Len: 4}, &Array{Elem: Int, Len: 4}},
		{"unnamed", tp, Int},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substitute(tt.typ, subst)
			if !Identical(got, tt.want) {
				t.Errorf("substitute = %s, want %s", got, tt.want)
			}
		})
	}

	if got := substitute(nil, subst); got != nil {
		t.Errorf("substitute(nil) = %v, want nil", got)
	}
	if got := substitute(Int, map[*TypeParam]Type{}); got != Int {
		t.Errorf("substitute with empty map = %v, want int", got)
	}
}

func TestFlattenFields(t *testing.T) {
	inner := &Struct{
		Fields: []Field{{Name: "y", Type: Int, Index: 0}},
	}
	inner.Flat, inner.Ambiguous = flattenFields(inner)

	outer := &Struct{
		Fields: []Field{
			{Name: "x", Type: String_, Index: 0},
			{Name: "", Type: &Named{Name: "Inner", Underlying: inner}, Embedded: true, Index: 1},
		},
	}
	outer.Flat, outer.Ambiguous = flattenFields(outer)

	if _, ok := outer.Flat["x"]; !ok {
		t.Error("direct field 'x' not in flat map")
	}
	if fp, ok := outer.Flat["y"]; !ok {
		t.Error("embedded field 'y' not in flat map")
	} else {
		if len(fp.Path) != 2 {
			t.Errorf("'y' path length = %d, want 2", len(fp.Path))
		}
	}
}

func TestFlattenFields_Ambiguous(t *testing.T) {
	left := &Struct{Fields: []Field{{Name: "z", Type: Int, Index: 0}}}
	left.Flat, left.Ambiguous = flattenFields(left)
	right := &Struct{Fields: []Field{{Name: "z", Type: Int, Index: 0}}}
	right.Flat, right.Ambiguous = flattenFields(right)

	st := &Struct{
		Fields: []Field{
			{Name: "", Type: &Named{Name: "L", Underlying: left}, Embedded: true, Index: 0},
			{Name: "", Type: &Named{Name: "R", Underlying: right}, Embedded: true, Index: 1},
		},
	}
	st.Flat, st.Ambiguous = flattenFields(st)

	if !st.Ambiguous["z"] {
		t.Error("'z' should be ambiguous (two paths at same depth)")
	}
}

func TestInstanceKey(t *testing.T) {
	key := instanceKey("Stack", []Type{Int, String_})
	if key == "" {
		t.Error("instanceKey should not be empty")
	}
	key2 := instanceKey("Stack", []Type{Int, String_})
	if key != key2 {
		t.Error("same args should produce same key")
	}
}

func TestMangleType(t *testing.T) {
	tests := []struct {
		typ  Type
		want string
	}{
		{Int, "int"},
		{String_, "string"},
		{NewSlice(Int), "Slice_int"},
		{NewMap(String_, Int), "Map_string_int"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := mangleType(tt.typ)
			if got != tt.want {
				t.Errorf("mangleType = %q, want %q", got, tt.want)
			}
		})
	}
	if got := mangleType(nil); got != "?" {
		t.Errorf("mangleType(nil) = %q, want ?", got)
	}
}

func TestIsPrivate(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"_private", true},
		{"_x", true},
		{"Public", false},
		{"x", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPrivate(tt.name)
			if got != tt.want {
				t.Errorf("isPrivate(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestPositionNode(t *testing.T) {
	pos := ast.Position{File: "f.dot", Line: 10, Column: 5, Offset: 100}
	pn := positionNode(pos)
	if pn.Pos() != pos {
		t.Error("positionNode.Pos() should return the same position")
	}
	end := pn.End()
	if end.Column != pos.Column+1 || end.Offset != pos.Offset+1 {
		t.Errorf("End() = %v, want offset+1", end)
	}
}
