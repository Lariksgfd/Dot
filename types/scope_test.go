package types

import "testing"

func TestScope_DeclareLookup(t *testing.T) {
	s := NewScope(nil, ScopeUniverse)
	sym := &Symbol{Kind: SymVar, Name: "x"}
	existing, ok := s.Insert(sym)
	if !ok {
		t.Error("Insert should succeed")
	}
	if existing != nil {
		t.Error("Insert should return nil existing")
	}
	got, ok := s.Lookup("x")
	if !ok || got != sym {
		t.Fatal("Lookup failed after Insert")
	}
}

func TestScope_Underscore(t *testing.T) {
	s := NewScope(nil, ScopeUniverse)
	sym := &Symbol{Kind: SymVar, Name: "_"}
	existing, ok := s.Insert(sym)
	if !ok {
		t.Error("Insert(_) should return ok=true")
	}
	if existing != nil {
		t.Error("Insert(_) should return nil existing")
	}
	_, found := s.Lookup("_")
	if found {
		t.Error("_ should not be stored in scope")
	}
}

func TestScope_Redeclaration(t *testing.T) {
	s := NewScope(nil, ScopeFile)
	s1 := &Symbol{Kind: SymVar, Name: "x"}
	s2 := &Symbol{Kind: SymVar, Name: "x"}
	existing, ok := s.Insert(s1)
	if !ok || existing != nil {
		t.Fatal("first Insert should succeed")
	}
	existing, ok = s.Insert(s2)
	if ok {
		t.Error("second Insert should fail with existing")
	}
	if existing != s1 {
		t.Error("should return first symbol as existing")
	}
}

func TestScope_NestedLookup(t *testing.T) {
	parent := NewScope(nil, ScopeFile)
	child := NewScope(parent, ScopeBlock)

	psym := &Symbol{Kind: SymVar, Name: "p"}
	parent.Insert(psym)

	got, ok := child.Lookup("p")
	if !ok || got != psym {
		t.Error("child should see parent's symbol")
	}
	if !psym.Used {
		t.Error("Lookup should mark symbol as Used")
	}
}

func TestScope_Shadowing(t *testing.T) {
	parent := NewScope(nil, ScopeFile)
	child := NewScope(parent, ScopeBlock)

	psym := &Symbol{Kind: SymVar, Name: "x"}
	csym := &Symbol{Kind: SymVar, Name: "x"}

	parent.Insert(psym)
	child.Insert(csym)

	got, ok := child.Lookup("x")
	if !ok || got != csym {
		t.Error("child should see its own symbol (shadowing)")
	}
	if got == psym {
		t.Error("should not return parent's symbol")
	}
}

func TestScope_LookupLocal(t *testing.T) {
	parent := NewScope(nil, ScopeFile)
	child := NewScope(parent, ScopeBlock)

	psym := &Symbol{Kind: SymVar, Name: "p"}
	parent.Insert(psym)

	_, ok := child.LookupLocal("p")
	if ok {
		t.Error("LookupLocal should not see parent symbols")
	}

	csym := &Symbol{Kind: SymVar, Name: "c"}
	child.Insert(csym)
	got, ok := child.LookupLocal("c")
	if !ok || got != csym {
		t.Error("LookupLocal should see own symbols")
	}
}

func TestScope_Names(t *testing.T) {
	s := NewScope(nil, ScopeFile)
	s.Insert(&Symbol{Kind: SymVar, Name: "a"})
	s.Insert(&Symbol{Kind: SymVar, Name: "b"})
	s.Insert(&Symbol{Kind: SymVar, Name: "c"})

	names := s.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d: %v", len(names), names)
	}
	if names[0] != "a" || names[1] != "b" || names[2] != "c" {
		t.Errorf("wrong insertion order: %v", names)
	}
}

func TestScope_Depth(t *testing.T) {
	file := NewScope(nil, ScopeFile)
	if file.Depth() != 0 {
		t.Errorf("file depth should be 0, got %d", file.Depth())
	}
	universe := NewScope(nil, ScopeUniverse)
	file2 := NewScope(universe, ScopeFile)
	if file2.Depth() != 1 {
		t.Errorf("file depth should be 1, got %d", file2.Depth())
	}
}

func TestScope_Enclosing(t *testing.T) {
	univ := NewScope(nil, ScopeUniverse)
	file := NewScope(univ, ScopeFile)
	fn := NewScope(file, ScopeFunc)
	block := NewScope(fn, ScopeBlock)
	loop := NewScope(block, ScopeLoop)

	if got := block.Enclosing(ScopeFunc); got != fn {
		t.Error("Enclosing(ScopeFunc) should find fn scope")
	}
	if got := block.Enclosing(ScopeFile); got != file {
		t.Error("Enclosing(ScopeFile) should find file scope")
	}
	if got := loop.Enclosing(ScopeLoop); got != nil {
		t.Error("Enclosing(ScopeLoop) on loop scope should not return itself")
	}
	if got := block.Enclosing(ScopeImpl); got != nil {
		t.Error("Enclosing(ScopeImpl) should return nil when none found")
	}
}

func TestScope_Children(t *testing.T) {
	parent := NewScope(nil, ScopeFile)
	c1 := NewScope(parent, ScopeBlock)
	c2 := NewScope(parent, ScopeBlock)

	if len(parent.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(parent.Children))
	}
	if parent.Children[0] != c1 || parent.Children[1] != c2 {
		t.Error("children should be in creation order")
	}
}

func TestScope_InsertNil(t *testing.T) {
	s := NewScope(nil, ScopeFile)
	existing, ok := s.Insert(nil)
	if !ok {
		t.Error("Insert(nil) should return ok=true")
	}
	if existing != nil {
		t.Error("Insert(nil) should return nil existing")
	}
}

func TestSymbolKind_String(t *testing.T) {
	tests := []struct {
		k    SymbolKind
		want string
	}{
		{SymVar, "var"},
		{SymFunc, "func"},
		{SymType, "type"},
		{SymTrait, "trait"},
		{SymVariant, "variant"},
		{SymTypeParam, "typeparam"},
		{SymModule, "module"},
		{SymBuiltin, "builtin"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.k.String(); got != tt.want {
				t.Errorf("SymbolKind.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewScope_NilParent(t *testing.T) {
	s := NewScope(nil, ScopeUniverse)
	if s.Parent != nil {
		t.Error("universe scope should have nil parent")
	}
	if s.Kind != ScopeUniverse {
		t.Error("kind should be ScopeUniverse")
	}
	if s.Symbols == nil {
		t.Error("Symbols map should be initialized")
	}
}

func TestNewScope_WithParent(t *testing.T) {
	parent := NewScope(nil, ScopeFile)
	child := NewScope(parent, ScopeBlock)
	if child.Parent != parent {
		t.Error("child Parent should be set")
	}
}
