package llvm

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/parser"
	"github.com/dotlang/dot/types"
)

func generateLLVM(t *testing.T, source string) string {
	t.Helper()
	prog, err := parser.ParseFile(source, "test.dot")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	c, err := Generate(nil, prog)
	if err != nil {
		t.Fatalf("llvm generation error: %v", err)
	}
	return c
}

func generateLLVMWithTypes(t *testing.T, source string) string {
	t.Helper()
	prog, err := parser.ParseFile(source, "test.dot")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	info, err := types.Check(prog, "test.dot")
	if err != nil {
		t.Fatalf("type check error: %v", err)
	}

	c, err := Generate(info, prog)
	if err != nil {
		t.Fatalf("llvm generation error: %v", err)
	}
	return c
}

func TestGenerate_Struct(t *testing.T) {
	source := `
struct Point {
	x int
	y int
}
struct User {
	age int
}
fn main() {
	p = Point{x: 10, y: 20}
	z = p.x
	u = User{age: 30}
	w = u.age
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "%Point = type { i32, i32 }") {
		t.Errorf("Missing Point struct definition, got: %s", c)
	}
	if !strings.Contains(c, "%User = type { i32 }") {
		t.Errorf("Missing User struct definition, got: %s", c)
	}
	if !strings.Contains(c, "alloca %Point") {
		t.Errorf("Missing Point struct allocation, got: %s", c)
	}
	if !strings.Contains(c, "alloca %User") {
		t.Errorf("Missing User struct allocation, got: %s", c)
	}
	if !strings.Contains(c, "getelementptr %Point") {
		t.Errorf("Missing Point getelementptr, got: %s", c)
	}
	if !strings.Contains(c, "getelementptr %User") {
		t.Errorf("Missing User getelementptr, got: %s", c)
	}
}

func TestGenerate_Enum(t *testing.T) {
	source := `
enum Color {
	Red
	Green
	Blue
}
enum Shape {
	Circle(radius int)
	Rectangle(width int, height int)
}
enum Option {
	None
	Some(value int)
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "%Color = type { i32 }") {
		t.Errorf("Missing Color enum definition, got: %s", c)
	}
	if !strings.Contains(c, "%Shape = type { i32, i32, i32 }") {
		t.Errorf("Missing Shape enum definition (tag + 2 payload fields), got: %s", c)
	}
	if !strings.Contains(c, "%Option = type { i32, i32 }") {
		t.Errorf("Missing Option enum definition (tag + 1 payload field), got: %s", c)
	}
}

func TestGenerate_MatchEnum(t *testing.T) {
	source := `
enum Color {
	Red
	Green
	Blue
}
fn main() {
	x = 1
	y = match x {
		0 => 10,
		1 => 20,
		_ => 0
	}
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "%Color = type { i32 }") {
		t.Errorf("Missing Color enum definition, got: %s", c)
	}
	if !strings.Contains(c, "match.") {
		t.Errorf("Missing match labels in output, got: %s", c)
	}
	if !strings.Contains(c, "icmp eq") {
		t.Errorf("Missing icmp eq comparison for match arms, got: %s", c)
	}
}

func TestGenerate_MatchWithEnumPattern(t *testing.T) {
	source := `
enum Color {
	Red
	Green
	Blue
}
fn main() {
	c = Color.Red
	y = match c {
		Color.Red => 1,
		Color.Green => 2,
		Color.Blue => 3,
		_ => 0
	}
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "%Color = type { i32 }") {
		t.Errorf("Missing Color enum definition, got: %s", c)
	}
	if !strings.Contains(c, "icmp eq i32") {
		t.Errorf("Missing icmp eq comparison for enum match, got: %s", c)
	}
}

func TestGenerate_MatchWithGuard(t *testing.T) {
	source := `
fn main() {
	x = 5
	y = match x {
		n if n > 0 => 1,
		_ => 0
	}
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "match.") {
		t.Errorf("Missing match labels in output, got: %s", c)
	}
	if !strings.Contains(c, "icmp sgt") {
		t.Errorf("Missing 'icmp sgt' for guard condition, got: %s", c)
	}
}

func TestGenerate_MatchWithLiteralGuard(t *testing.T) {
	source := `
fn main() {
	x = 5
	y = match x {
		0 if x > 100 => 1,
		0 => 2,
		_ => 0
	}
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "match.") {
		t.Errorf("Missing match labels in output, got: %s", c)
	}
	if !strings.Contains(c, "and i1") {
		t.Errorf("Missing 'and i1' for combined pattern+guard condition, got: %s", c)
	}
}

func TestGenerate_AddressOf(t *testing.T) {
	source := `
fn main() {
	x = 42
	p = &x
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "%x = alloca i64") {
		t.Errorf("Missing alloca for x, got: %s", c)
	}
	if !strings.Contains(c, "%p = alloca ptr") {
		t.Errorf("Missing ptr alloca for p, got: %s", c)
	}
	if !strings.Contains(c, "store ptr %x, ptr %p") {
		t.Errorf("Missing store of &x (ptr) into p, got: %s", c)
	}
}

func TestGenerate_Dereference(t *testing.T) {
	source := `
fn main() {
	x = 42
	p = &x
	v = *p
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "%p = alloca ptr") {
		t.Errorf("Missing ptr alloca for p, got: %s", c)
	}
	if !strings.Contains(c, "store ptr %x, ptr %p") {
		t.Errorf("Missing store of &x into p, got: %s", c)
	}
	if !strings.Contains(c, "load ptr, ptr %p") {
		t.Errorf("Missing load ptr from p, got: %s", c)
	}
	if !strings.Contains(c, "load i64, ptr %") {
		t.Errorf("Missing load i32 through pointer for *p, got: %s", c)
	}
}

func TestGenerate_AddrOfStructField(t *testing.T) {
	source := `
struct Point {
	x int
	y int
}
fn main() {
	pt = Point{x: 10, y: 20}
	px = &pt.x
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "getelementptr %Point") {
		t.Errorf("Missing getelementptr for struct field address, got: %s", c)
	}
	if !strings.Contains(c, "store ptr") {
		t.Errorf("Missing store of ptr for &pt.x, got: %s", c)
	}
}

func TestGenerate_PtrAssignment(t *testing.T) {
	source := `
fn main() {
	x = 42
	p = &x
	q = p
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "%p = alloca ptr") {
		t.Errorf("Missing ptr alloca for p, got: %s", c)
	}
	if !strings.Contains(c, "%q = alloca ptr") {
		t.Errorf("Missing ptr alloca for q, got: %s", c)
	}
	if !strings.Contains(c, "load ptr, ptr %p") {
		t.Errorf("Missing load ptr from p for assignment to q, got: %s", c)
	}
}

func TestGenerate_DerefStoredPtr(t *testing.T) {
	source := `
fn main() {
	x = 42
	p = &x
	q = p
	v = *q
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "load ptr, ptr %q") {
		t.Errorf("Missing load ptr from q, got: %s", c)
	}
	if !strings.Contains(c, "load i64, ptr %") {
		t.Errorf("Missing load i32 through pointer for *q, got: %s", c)
	}
}

func TestGenerate_ArrayLit(t *testing.T) {
	source := `
fn main() {
	a = [5]int{1, 2, 3, 4, 5}
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "[5 x i64]") {
		t.Errorf("Missing [5 x i64] array type, got: %s", c)
	}
	if !strings.Contains(c, "alloca [5 x i64]") {
		t.Errorf("Missing alloca for array, got: %s", c)
	}
	if !strings.Contains(c, "getelementptr [5 x i64]") {
		t.Errorf("Missing getelementptr for array element, got: %s", c)
	}
}

func TestGenerate_SliceLit(t *testing.T) {
	source := `
fn main() {
	s = []int{10, 20, 30}
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "{ ptr, i32, i32 }") {
		t.Errorf("Missing slice struct type, got: %s", c)
	}
	if !strings.Contains(c, "alloca { ptr, i32, i32 }") {
		t.Errorf("Missing alloca for slice, got: %s", c)
	}
	if !strings.Contains(c, "getelementptr { ptr, i32, i32 }") {
		t.Errorf("Missing getelementptr for slice fields, got: %s", c)
	}
	if !strings.Contains(c, "store i32 3,") {
		t.Errorf("Missing length store (3 elements), got: %s", c)
	}
}

func TestGenerate_ArrayIndex(t *testing.T) {
	source := `
fn main() {
	a = [5]int{1, 2, 3, 4, 5}
	x = a[2]
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "getelementptr i64, ptr") {
		t.Errorf("Missing getelementptr for array indexing, got: %s", c)
	}
	if !strings.Contains(c, "load i64, ptr") {
		t.Errorf("Missing load for indexed element, got: %s", c)
	}
}

func TestGenerate_StringLit(t *testing.T) {
	source := `
fn main() {
	s = "hello world"
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "@.str.0 = private unnamed_addr constant [12 x i8] c\"hello world\\00\"") {
		t.Errorf("Missing string literal constant, got: %s", c)
	}
	if !strings.Contains(c, "alloca { ptr, i32 }") {
		t.Errorf("Missing alloca for string struct, got: %s", c)
	}
	if !strings.Contains(c, "getelementptr [12 x i8], ptr @.str.0") {
		t.Errorf("Missing getelementptr for string data, got: %s", c)
	}
	if !strings.Contains(c, "store i32 11,") {
		t.Errorf("Missing length store (11 elements) for string, got: %s", c)
	}
}

func TestGenerate_Generics(t *testing.T) {
	source := `
fn id[T](x T) -> T {
	return x
}
fn main() {
	a = id(42)
}
`
	prog, err := parser.ParseFile(source, "test.dot")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Forge types.Info since the type checker's generic function call support is incomplete.
	info := &types.Info{
		Instances: make(map[ast.Expr]*types.Instance),
	}

	var fnDecl *ast.FnDecl
	var callExpr *ast.CallExpr

	for _, decl := range prog.Decls {
		if fn, ok := decl.(*ast.FnDecl); ok && fn.Name == "id" {
			fnDecl = fn
		}
		if fn, ok := decl.(*ast.FnDecl); ok && fn.Name == "main" {
			for _, stmt := range fn.Body.Stmts {
				if varDecl, ok := stmt.(*ast.VarDecl); ok {
					if call, ok := varDecl.Values[0].(*ast.CallExpr); ok {
						callExpr = call
					}
				}
			}
		}
	}

	if fnDecl == nil || callExpr == nil {
		t.Fatalf("failed to find fnDecl or callExpr")
	}

	inst := &types.Instance{
		Generic: &types.Symbol{
			Kind: types.SymFunc,
			Decl: fnDecl,
		},
		Mangled: "id__int",
	}
	info.InstanceList = append(info.InstanceList, inst)
	info.Instances[callExpr] = inst

	c, err := Generate(info, prog)
	if err != nil {
		t.Fatalf("llvm generation error: %v", err)
	}

	if !strings.Contains(c, "define i64 @id__int(i64") {
		t.Errorf("Missing instantiated generic function, got: %s", c)
	}
	if !strings.Contains(c, "call i64 @id__int(i64") {
		t.Errorf("Missing call to instantiated generic function, got: %s", c)
	}
	if strings.Contains(c, "define i64 @id(i64") {
		t.Errorf("Should not emit uninstantiated generic function, got: %s", c)
	}
}

func TestGenerate_ForLoop(t *testing.T) {
	source := `
fn main() {
	sum = 0
	for i in 0..10 {
		sum = sum + i
	}
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "for.cond") {
		t.Errorf("Missing for.cond block, got: %s", c)
	}
	if !strings.Contains(c, "for.body") {
		t.Errorf("Missing for.body block, got: %s", c)
	}
	if !strings.Contains(c, "for.step") {
		t.Errorf("Missing for.step block, got: %s", c)
	}
	if !strings.Contains(c, "for.end") {
		t.Errorf("Missing for.end block, got: %s", c)
	}
	if !strings.Contains(c, "icmp slt i64") {
		t.Errorf("Missing icmp slt comparison for range loop, got: %s", c)
	}
	if !strings.Contains(c, "add i64") {
		t.Errorf("Missing increment (add i64) for loop variable, got: %s", c)
	}
}

func TestGenerate_WhileLoop(t *testing.T) {
	source := `
fn main() {
	x = 10
	for x > 0 {
		x = x - 1
	}
}
`
	c := generateLLVM(t, source)

	if !strings.Contains(c, "for.cond") {
		t.Errorf("Missing for.cond block, got: %s", c)
	}
	if !strings.Contains(c, "for.body") {
		t.Errorf("Missing for.body block, got: %s", c)
	}
	if !strings.Contains(c, "for.end") {
		t.Errorf("Missing for.end block, got: %s", c)
	}
	if !strings.Contains(c, "icmp sgt i64") {
		t.Errorf("Missing icmp sgt comparison for while loop, got: %s", c)
	}
	if !strings.Contains(c, "br i1") {
		t.Errorf("Missing conditional branch, got: %s", c)
	}
}
