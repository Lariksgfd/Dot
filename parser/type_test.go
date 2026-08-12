package parser

import (
	"testing"

	"github.com/dotlang/dot/ast"
)

// parseType parses src as a type expression by wrapping it in a struct
// field declaration, which accepts any type form (including those starting
// with `(`, `*`, `[`, `fn`).
func parseType(t *testing.T, src string) ast.Type {
	t.Helper()
	prog := mustParse(t, "struct S { field "+src+" }\n")
	sd := prog.Decls[0].(*ast.StructDecl)
	if len(sd.Fields) == 0 {
		t.Fatalf("struct S { field %s }: no fields", src)
	}
	if sd.Fields[0].Type == nil {
		t.Fatalf("struct S { field %s }: Type is nil", src)
	}
	return sd.Fields[0].Type
}

func TestType_Named(t *testing.T) {
	ty := parseType(t, "int")
	if _, ok := ty.(*ast.NamedType); !ok {
		t.Errorf("int: Type = %T, want NamedType", ty)
	}
}

func TestType_Generic(t *testing.T) {
	ty := parseType(t, "Option[int]")
	gty, ok := ty.(*ast.GenericType)
	if !ok {
		t.Fatalf("Option[int]: Type = %T, want GenericType", ty)
	}
	if len(gty.Args) != 1 {
		t.Errorf("Option[int]: Args = %d, want 1", len(gty.Args))
	}
}

func TestType_GenericTwoArgs(t *testing.T) {
	ty := parseType(t, "Result[int, Error]")
	gty, ok := ty.(*ast.GenericType)
	if !ok {
		t.Fatalf("Result[int, Error]: Type = %T, want GenericType", ty)
	}
	if len(gty.Args) != 2 {
		t.Errorf("Result[int, Error]: Args = %d, want 2", len(gty.Args))
	}
}

func TestType_Slice(t *testing.T) {
	ty := parseType(t, "[]int")
	sty, ok := ty.(*ast.SliceType)
	if !ok {
		t.Fatalf("[]int: Type = %T, want SliceType", ty)
	}
	if _, ok := sty.Elem.(*ast.NamedType); !ok {
		t.Errorf("[]int: Elem = %T, want NamedType", sty.Elem)
	}
}

func TestType_Array(t *testing.T) {
	ty := parseType(t, "[5]int")
	aty, ok := ty.(*ast.ArrayType)
	if !ok {
		t.Fatalf("[5]int: Type = %T, want ArrayType", ty)
	}
	if aty.Len == nil {
		t.Errorf("[5]int: Len is nil")
	}
	if _, ok := aty.Elem.(*ast.NamedType); !ok {
		t.Errorf("[5]int: Elem = %T, want NamedType", aty.Elem)
	}
}

func TestType_Map(t *testing.T) {
	ty := parseType(t, "map[string]int")
	mty, ok := ty.(*ast.MapType)
	if !ok {
		t.Fatalf("map[string]int: Type = %T, want MapType", ty)
	}
	if _, ok := mty.Key.(*ast.NamedType); !ok {
		t.Errorf("map[string]int: Key = %T, want NamedType", mty.Key)
	}
	if _, ok := mty.Value.(*ast.NamedType); !ok {
		t.Errorf("map[string]int: Value = %T, want NamedType", mty.Value)
	}
}

func TestType_Fn(t *testing.T) {
	ty := parseType(t, "fn(int, int) -> int")
	fty, ok := ty.(*ast.FnType)
	if !ok {
		t.Fatalf("fn(int,int)->int: Type = %T, want FnType", ty)
	}
	if len(fty.Params) != 2 {
		t.Errorf("fn(int,int)->int: Params = %d, want 2", len(fty.Params))
	}
	if fty.Result == nil {
		t.Errorf("fn(int,int)->int: Result is nil")
	}
}

func TestType_FnVoid(t *testing.T) {
	ty := parseType(t, "fn(int)")
	fty, ok := ty.(*ast.FnType)
	if !ok {
		t.Fatalf("fn(int): Type = %T, want FnType", ty)
	}
	if fty.Result != nil {
		t.Errorf("fn(int): Result = %T, want nil (void)", fty.Result)
	}
}

func TestType_FnVariadic(t *testing.T) {
	ty := parseType(t, "fn(...int)")
	fty, ok := ty.(*ast.FnType)
	if !ok {
		t.Fatalf("fn(...int): Type = %T, want FnType", ty)
	}
	if !fty.Variadic {
		t.Errorf("fn(...int): Variadic = false, want true")
	}
}

func TestType_Tuple(t *testing.T) {
	ty := parseType(t, "(int, string)")
	tty, ok := ty.(*ast.TupleType)
	if !ok {
		t.Fatalf("(int, string): Type = %T, want TupleType", ty)
	}
	if len(tty.Elems) != 2 {
		t.Errorf("(int, string): Elems = %d, want 2", len(tty.Elems))
	}
}

func TestType_Pointer(t *testing.T) {
	ty := parseType(t, "*int")
	pty, ok := ty.(*ast.PointerType)
	if !ok {
		t.Fatalf("*int: Type = %T, want PointerType", ty)
	}
	if _, ok := pty.Elem.(*ast.NamedType); !ok {
		t.Errorf("*int: Elem = %T, want NamedType", pty.Elem)
	}
}

func TestType_Dyn(t *testing.T) {
	ty := parseType(t, "dyn Printable")
	dty, ok := ty.(*ast.DynType)
	if !ok {
		t.Fatalf("dyn Printable: Type = %T, want DynType", ty)
	}
	if _, ok := dty.Trait.(*ast.NamedType); !ok {
		t.Errorf("dyn Printable: Trait = %T, want NamedType", dty.Trait)
	}
}

func TestType_Weak(t *testing.T) {
	ty := parseType(t, "weak Option[Node]")
	wty, ok := ty.(*ast.WeakType)
	if !ok {
		t.Fatalf("weak Option[Node]: Type = %T, want WeakType", ty)
	}
	if wty.Elem == nil {
		t.Errorf("weak Option[Node]: Elem is nil")
	}
}

func TestType_Optional(t *testing.T) {
	ty := parseType(t, "int?")
	oty, ok := ty.(*ast.OptionalType)
	if !ok {
		t.Fatalf("int?: Type = %T, want OptionalType", ty)
	}
	if _, ok := oty.Elem.(*ast.NamedType); !ok {
		t.Errorf("int?: Elem = %T, want NamedType", oty.Elem)
	}
}

func TestType_OptionalChain(t *testing.T) {
	// int??  — only one `?` postfix is consumed; the second `?` would be a
	// lex error. So test the one-? case which yields OptionalType. A chain
	// like `int?` is the single level; for deeper, the parser does NOT
	// collapse. Verify a single OptionalType.
	ty := parseType(t, "int?")
	if _, ok := ty.(*ast.OptionalType); !ok {
		t.Errorf("int?: Type = %T, want OptionalType", ty)
	}
}

func TestType_SelfNode(t *testing.T) {
	// `Self` as a type appears in trait/impl bodies. Parse it via a fn trait.
	prog := mustParse(t, "trait T { fn m(self) -> Self }\n")
	td := prog.Decls[0].(*ast.TraitDecl)
	m := td.Methods[0]
	if m.Sig.Result == nil {
		t.Fatalf("Self: Result is nil")
	}
	if _, ok := m.Sig.Result.(*ast.SelfTypeNode); !ok {
		t.Errorf("Self: Result = %T, want SelfTypeNode", m.Sig.Result)
	}
}

func TestType_Qualified(t *testing.T) {
	ty := parseType(t, "net.http.Client")
	nty, ok := ty.(*ast.NamedType)
	if !ok {
		t.Fatalf("net.http.Client: Type = %T, want NamedType", ty)
	}
	if nty.Pkg != "net.http" {
		t.Errorf("net.http.Client: Pkg = %q, want net.http", nty.Pkg)
	}
	if nty.Name != "Client" {
		t.Errorf("net.http.Client: Name = %q, want Client", nty.Name)
	}
}

func TestType_DeepNesting(t *testing.T) {
	// map[string][]Option[int]
	ty := parseType(t, "map[string][]Option[int]")
	mty, ok := ty.(*ast.MapType)
	if !ok {
		t.Fatalf("map[string][]Option[int]: Type = %T, want MapType", ty)
	}
	if _, ok := mty.Key.(*ast.NamedType); !ok {
		t.Errorf("map[string][]Option[int]: Key = %T, want NamedType", mty.Key)
	}
	// Value should be []Option[int] — a SliceType whose Elem is GenericType.
	sty, ok := mty.Value.(*ast.SliceType)
	if !ok {
		t.Errorf("map[string][]Option[int]: Value = %T, want SliceType", mty.Value)
	} else {
		gty, ok := sty.Elem.(*ast.GenericType)
		if !ok {
			t.Errorf("map[string][]Option[int]: Elem = %T, want GenericType", sty.Elem)
		} else if len(gty.Args) != 1 {
			t.Errorf("map[string][]Option[int]: Args = %d, want 1", len(gty.Args))
		}
	}
}

func TestType_ParenSingle(t *testing.T) {
	// (A) collapses to A — there is no ParenType node (D42/D44).
	ty := parseType(t, "(int)")
	if _, ok := ty.(*ast.NamedType); !ok {
		t.Errorf("(int): Type = %T, want NamedType (collapsed)", ty)
	}
}

func TestType_ParenEmpty(t *testing.T) {
	// () is the empty tuple type.
	ty := parseType(t, "()")
	if _, ok := ty.(*ast.TupleType); !ok {
		t.Errorf("(): Type = %T, want TupleType (empty)", ty)
	}
}

func TestType_FnResult(t *testing.T) {
	// fn() -> (int, string) — tuple result.
	ty := parseType(t, "fn() -> (int, string)")
	fty, ok := ty.(*ast.FnType)
	if !ok {
		t.Fatalf("fn()->(int,string): Type = %T, want FnType", ty)
	}
	tty, ok := fty.Result.(*ast.TupleType)
	if !ok {
		t.Errorf("fn()->(int,string): Result = %T, want TupleType", fty.Result)
	} else if len(tty.Elems) != 2 {
		t.Errorf("fn()->(int,string): Elems = %d, want 2", len(tty.Elems))
	}
}

func TestType_AllRoundTrip(t *testing.T) {
	// Every type form parses without producing a BadType.
	types := []string{
		"int", "Option[int]", "Result[int, Error]", "[]int", "[5]int",
		"map[string]int", "fn(int, int) -> int", "fn(int)", "(int, string)",
		"*int", "dyn Printable", "weak int", "int?", "net.http.Client",
		"map[string][]Option[int]",
	}
	for _, src := range types {
		ty := parseType(t, src)
		if _, ok := ty.(*ast.BadType); ok {
			t.Errorf("type %q parsed as BadType", src)
		}
	}
}
