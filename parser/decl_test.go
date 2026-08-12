package parser

import (
	"testing"

	"github.com/dotlang/dot/ast"
)

// fnDecl parses a single top-level fn and returns it.
func fnDecl(t *testing.T, src string) *ast.FnDecl {
	t.Helper()
	prog := mustParse(t, src+"\n")
	if len(prog.Decls) == 0 {
		t.Fatalf("%q: no decls", src)
	}
	d, ok := prog.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("%q: decl is %T, want FnDecl", src, prog.Decls[0])
	}
	return d
}

func TestFnDecl_Simple(t *testing.T) {
	fn := fnDecl(t, "fn add(a int, b int) -> int { return a + b }")
	if fn.Name != "add" {
		t.Errorf("Name = %q, want add", fn.Name)
	}
	if fn.Sig == nil {
		t.Fatalf("Sig is nil")
	}
	if len(fn.Sig.Params) != 2 {
		t.Errorf("Params = %d, want 2", len(fn.Sig.Params))
	}
	if fn.Sig.Result == nil {
		t.Errorf("Result is nil")
	}
	if fn.Body == nil {
		t.Errorf("Body is nil")
	}
}

func TestFnDecl_Void(t *testing.T) {
	fn := fnDecl(t, "fn greet(name string) { }")
	if fn.Sig.Result != nil {
		t.Errorf("Result = %T, want nil (void)", fn.Sig.Result)
	}
}

func TestFnDecl_ShortForm(t *testing.T) {
	fn := fnDecl(t, "fn double(x int) -> int = x * 2")
	if fn.ExprBody == nil {
		t.Errorf("ExprBody is nil")
	}
	if fn.Body != nil {
		t.Errorf("Body should be nil for short form")
	}
}

func TestFnDecl_Async(t *testing.T) {
	fn := fnDecl(t, "async fn f() { }")
	if !fn.Async {
		t.Errorf("Async = false, want true")
	}
}

func TestFnDecl_Pub(t *testing.T) {
	fn := fnDecl(t, "pub fn f() { }")
	if !fn.Pub {
		t.Errorf("Pub = false, want true")
	}
}

func TestFnDecl_Generics(t *testing.T) {
	fn := fnDecl(t, "fn max[T](a T, b T) -> T { }")
	if len(fn.TypeParams) != 1 {
		t.Fatalf("TypeParams = %d, want 1", len(fn.TypeParams))
	}
	if fn.TypeParams[0].Name != "T" {
		t.Errorf("TypeParam name = %q, want T", fn.TypeParams[0].Name)
	}
}

func TestFnDecl_GenericsWithBounds(t *testing.T) {
	fn := fnDecl(t, "fn sort[T: Comparable](items []T) { }")
	if len(fn.TypeParams) != 1 {
		t.Fatalf("TypeParams = %d, want 1", len(fn.TypeParams))
	}
	tp := fn.TypeParams[0]
	if len(tp.Bounds) != 1 {
		t.Errorf("Bounds = %d, want 1", len(tp.Bounds))
	}
	if _, ok := tp.Bounds[0].(*ast.NamedType); !ok {
		t.Errorf("Bound = %T, want NamedType", tp.Bounds[0])
	}
}

func TestFnDecl_GenericsMultipleBounds(t *testing.T) {
	fn := fnDecl(t, "fn f[T: A + B]() { }")
	if len(fn.TypeParams) != 1 {
		t.Fatalf("TypeParams = %d, want 1", len(fn.TypeParams))
	}
	if len(fn.TypeParams[0].Bounds) != 2 {
		t.Errorf("Bounds = %d, want 2", len(fn.TypeParams[0].Bounds))
	}
}

func TestFnDecl_DefaultParam(t *testing.T) {
	fn := fnDecl(t, "fn connect(host string, port int = 8080) { }")
	if len(fn.Sig.Params) != 2 {
		t.Fatalf("Params = %d, want 2", len(fn.Sig.Params))
	}
	p := fn.Sig.Params[1]
	if p.Default == nil {
		t.Errorf("port: Default is nil")
	}
}

func TestFnDecl_Variadic(t *testing.T) {
	fn := fnDecl(t, "fn sum(nums ...int) -> int { }")
	if len(fn.Sig.Params) != 1 {
		t.Fatalf("Params = %d, want 1", len(fn.Sig.Params))
	}
	if !fn.Sig.Params[0].Variadic {
		t.Errorf("nums: Variadic = false, want true")
	}
}

func TestFnDecl_TupleResult(t *testing.T) {
	fn := fnDecl(t, "fn divide(a float, b float) -> (float, Error?) { }")
	if fn.Sig.Result == nil {
		t.Fatalf("Result is nil")
	}
	tt, ok := fn.Sig.Result.(*ast.TupleType)
	if !ok {
		t.Fatalf("Result = %T, want TupleType", fn.Sig.Result)
	}
	if len(tt.Elems) != 2 {
		t.Errorf("TupleType.Elems = %d, want 2", len(tt.Elems))
	}
}

func TestFnDecl_Receiver(t *testing.T) {
	// Method syntax: `fn name(self, other Point)` — self is the first param.
	fn := fnDecl(t, "fn distance(self, other Point) -> float { }")
	if fn.Sig.Recv == nil {
		t.Fatalf("Recv is nil")
	}
	if !fn.Sig.Recv.IsSelf {
		t.Errorf("Recv.IsSelf = false, want true")
	}
	if len(fn.Sig.Params) != 1 {
		t.Errorf("Params = %d, want 1 (receiver excluded)", len(fn.Sig.Params))
	}
}

func TestFnDecl_MutReceiver(t *testing.T) {
	fn := fnDecl(t, "fn translate(mut self, dx float, dy float) { }")
	if fn.Sig.Recv == nil {
		t.Fatalf("Recv is nil")
	}
	if !fn.Sig.Recv.Mut {
		t.Errorf("Recv.Mut = false, want true")
	}
}

func TestFnDecl_Annotation(t *testing.T) {
	fn := fnDecl(t, "@test fn f() { }")
	if len(fn.Annotations) != 1 {
		t.Fatalf("Annotations = %d, want 1", len(fn.Annotations))
	}
	if fn.Annotations[0].Name != "test" {
		t.Errorf("Annotation name = %q, want test", fn.Annotations[0].Name)
	}
}

func TestFnDecl_AnnotationWithArgs(t *testing.T) {
	fn := fnDecl(t, "@deprecated(\"use X\") fn f() { }")
	if len(fn.Annotations) != 1 {
		t.Fatalf("Annotations = %d, want 1", len(fn.Annotations))
	}
	if len(fn.Annotations[0].Args) != 1 {
		t.Errorf("Annotation args = %d, want 1", len(fn.Annotations[0].Args))
	}
}

func TestStructDecl_Simple(t *testing.T) {
	prog := mustParse(t, "struct Point { x float y float }\n")
	sd, ok := prog.Decls[0].(*ast.StructDecl)
	if !ok {
		t.Fatalf("decl = %T, want StructDecl", prog.Decls[0])
	}
	if sd.Name != "Point" {
		t.Errorf("Name = %q, want Point", sd.Name)
	}
	if len(sd.Fields) != 2 {
		t.Errorf("Fields = %d, want 2", len(sd.Fields))
	}
}

func TestStructDecl_Embed(t *testing.T) {
	prog := mustParse(t, "struct Dog { embed Animal breed string }\n")
	sd := prog.Decls[0].(*ast.StructDecl)
	if len(sd.Fields) != 2 {
		t.Fatalf("Fields = %d, want 2", len(sd.Fields))
	}
	if !sd.Fields[0].Embed {
		t.Errorf("field[0].Embed = false, want true")
	}
}

func TestStructDecl_Default(t *testing.T) {
	prog := mustParse(t, "struct Config { retries int = 3 }\n")
	sd := prog.Decls[0].(*ast.StructDecl)
	if sd.Fields[0].Default == nil {
		t.Errorf("Default is nil")
	}
}

func TestStructDecl_Generics(t *testing.T) {
	prog := mustParse(t, "struct Stack[T] { items []T }\n")
	sd := prog.Decls[0].(*ast.StructDecl)
	if len(sd.TypeParams) != 1 {
		t.Fatalf("TypeParams = %d, want 1", len(sd.TypeParams))
	}
	if sd.TypeParams[0].Name != "T" {
		t.Errorf("TypeParam = %q, want T", sd.TypeParams[0].Name)
	}
}

func TestEnumDecl_Plain(t *testing.T) {
	prog := mustParse(t, "enum Color { Red Green Blue }\n")
	ed, ok := prog.Decls[0].(*ast.EnumDecl)
	if !ok {
		t.Fatalf("decl = %T, want EnumDecl", prog.Decls[0])
	}
	if len(ed.Variants) != 3 {
		t.Fatalf("Variants = %d, want 3", len(ed.Variants))
	}
	if ed.Variants[0].Name != "Red" {
		t.Errorf("variant[0] = %q, want Red", ed.Variants[0].Name)
	}
}

func TestEnumDecl_Payload(t *testing.T) {
	prog := mustParse(t, "enum Shape { Circle(radius float) Rectangle(width float, height float) }\n")
	ed := prog.Decls[0].(*ast.EnumDecl)
	if len(ed.Variants) != 2 {
		t.Fatalf("Variants = %d, want 2", len(ed.Variants))
	}
	circle := ed.Variants[0]
	if !circle.HasParens {
		t.Errorf("Circle.HasParens = false, want true")
	}
	if len(circle.Fields) != 1 {
		t.Errorf("Circle.Fields = %d, want 1", len(circle.Fields))
	}
	rect := ed.Variants[1]
	if len(rect.Fields) != 2 {
		t.Errorf("Rectangle.Fields = %d, want 2", len(rect.Fields))
	}
}

func TestTraitDecl_Required(t *testing.T) {
	prog := mustParse(t, "trait Printable { fn to_string(self) -> string }\n")
	td, ok := prog.Decls[0].(*ast.TraitDecl)
	if !ok {
		t.Fatalf("decl = %T, want TraitDecl", prog.Decls[0])
	}
	if len(td.Methods) != 1 {
		t.Fatalf("Methods = %d, want 1", len(td.Methods))
	}
	m := td.Methods[0]
	if m.Body != nil {
		t.Errorf("required method: Body should be nil")
	}
	if m.ExprBody != nil {
		t.Errorf("required method: ExprBody should be nil")
	}
}

func TestTraitDecl_DefaultImpl(t *testing.T) {
	prog := mustParse(t, "trait Comparable { fn less_than(self, other Self) -> bool = self.compare(other) < 0 }\n")
	td := prog.Decls[0].(*ast.TraitDecl)
	if len(td.Methods) != 1 {
		t.Fatalf("Methods = %d, want 1", len(td.Methods))
	}
	m := td.Methods[0]
	if m.ExprBody == nil {
		t.Errorf("default method: ExprBody is nil")
	}
}

func TestImplDecl_Inherent(t *testing.T) {
	prog := mustParse(t, "impl Point { fn origin() -> Point { } }\n")
	id, ok := prog.Decls[0].(*ast.ImplDecl)
	if !ok {
		t.Fatalf("decl = %T, want ImplDecl", prog.Decls[0])
	}
	if id.Trait != nil {
		t.Errorf("Trait = %T, want nil (inherent impl)", id.Trait)
	}
	if id.Type == nil {
		t.Errorf("Type is nil")
	}
	if len(id.Methods) != 1 {
		t.Errorf("Methods = %d, want 1", len(id.Methods))
	}
}

func TestImplDecl_ForTrait(t *testing.T) {
	prog := mustParse(t, "impl Printable for Point { fn to_string(self) -> string = \"\" }\n")
	id := prog.Decls[0].(*ast.ImplDecl)
	if id.Trait == nil {
		t.Fatalf("Trait is nil")
	}
	if _, ok := id.Trait.(*ast.NamedType); !ok {
		t.Errorf("Trait = %T, want NamedType", id.Trait)
	}
	if id.Type == nil {
		t.Errorf("Type is nil")
	}
}

func TestImplDecl_Generics(t *testing.T) {
	prog := mustParse(t, "impl[T] Stack[T] { fn push(mut self, item T) { } }\n")
	id := prog.Decls[0].(*ast.ImplDecl)
	if len(id.TypeParams) != 1 {
		t.Fatalf("TypeParams = %d, want 1", len(id.TypeParams))
	}
	if id.TypeParams[0].Name != "T" {
		t.Errorf("TypeParam = %q, want T", id.TypeParams[0].Name)
	}
}
