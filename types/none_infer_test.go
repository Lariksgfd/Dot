package types

import (
	"testing"

	"github.com/dotlang/dot/ast"
)

// --- helpers ---------------------------------------------------------------------------------

// findIdent returns the first identifier named name in prog.
func findIdent(prog *ast.Program, name string) *ast.Ident {
	var found *ast.Ident
	ast.Inspect(prog, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = id
			return false
		}
		return true
	})
	return found
}

// findIdentIn returns the first identifier named name inside node's subtree.
func findIdentIn(node ast.Node, name string) *ast.Ident {
	var found *ast.Ident
	ast.Inspect(node, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = id
			return false
		}
		return true
	})
	return found
}

// findCall returns the first call whose callee is the identifier fn.
func findCall(prog *ast.Program, fn string) *ast.CallExpr {
	var found *ast.CallExpr
	ast.Inspect(prog, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fn.(*ast.Ident); ok && id.Name == fn {
				found = call
				return false
			}
		}
		return true
	})
	return found
}

// findFnLit returns the first lambda literal in prog.
func findFnLit(prog *ast.Program) *ast.FnLit {
	var found *ast.FnLit
	ast.Inspect(prog, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		if fn, ok := n.(*ast.FnLit); ok {
			found = fn
			return false
		}
		return true
	})
	return found
}

// optionElem returns the resolved element type of an Option[...] type.
func optionElem(t Type) (Type, bool) {
	named, ok := resolve(t).(*Named)
	if !ok || named.Name != "Option" || len(named.TypeArgs) != 1 {
		return nil, false
	}
	return resolve(named.TypeArgs[0]), true
}

// identType returns the resolved type recorded for identifier name in info.
func identType(t *testing.T, prog *ast.Program, info *Info, name string) Type {
	t.Helper()
	id := findIdent(prog, name)
	if id == nil {
		t.Fatalf("identifier %q not found", name)
	}
	ty := info.TypeOf(id)
	if IsInvalid(ty) {
		if sym, ok := info.Uses[id]; ok && sym != nil {
			ty = sym.Type
		}
	}
	return resolve(ty)
}

// --- TY-03: bidirectional None inference -------------------------------------------------------

func TestNoneInfer_MapBodies(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantElem Type // element of y: Option.map wraps the lambda RESULT
		wantX    Type // element of x: the receiver's ?T is pinned by the operand
	}{
		{"add_int", "v + 1", Int, Int},
		{"add_string", "v + \"s\"", String_, String_},
		{"eq_int", "v == 5", Bool, Int},
		{"lt_int", "v < 5", Bool, Int},
		{"bitor_int", "v | 1", Int, Int},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "fn main() {\n x = None\n y = x.map(fn(v) { " + tt.body + " })\n}"
			prog, info := parseCheckDot(t, src)
			got := identType(t, prog, info, "y")
			elem, ok := optionElem(got)
			if !ok {
				t.Fatalf("y type = %v (%T), want Option[...]", got, got)
			}
			if elem != tt.wantElem {
				t.Errorf("y element type = %v, want %v", elem, tt.wantElem)
			}
			// The receiver must have learned its element type too.
			x := identType(t, prog, info, "x")
			if xe, ok := optionElem(x); !ok || xe != tt.wantX {
				t.Errorf("x type = %v, want Option[%v]", x, tt.wantX)
			}
		})
	}
}

func TestNoneInfer_NoMismatchedDiagnostic(t *testing.T) {
	src := "fn main() {\n x = None\n y = x.map(fn(v) { v + 1 })\n}"
	info := checkDot(t, src)
	for _, d := range info.Diagnostics.Errors {
		t.Errorf("unexpected diagnostic: %s", d.Error())
	}
}

func TestNoneInfer_AssignSome(t *testing.T) {
	src := "fn main() {\n x = None\n x = Some(5)\n}"
	prog, info := parseCheckDot(t, src)
	got := identType(t, prog, info, "x")
	elem, ok := optionElem(got)
	if !ok {
		t.Fatalf("x type = %v (%T), want Option[...]", got, got)
	}
	if elem != Int {
		t.Errorf("x element type = %v, want int", elem)
	}
}

func TestNoneInfer_UnboundWithoutHint(t *testing.T) {
	// An inference variable with no further hints must not be an error.
	checkDot(t, "fn main() {\n x = None\n}")
}

// TestNoneInfer_MapUnwrapChain documents the residual TY-03 bug: the map
// method's result type parameter U never binds to the lambda's inferred
// result, so unwrap() hands back U and arithmetic on it misreports.
func TestNoneInfer_MapUnwrapChain(t *testing.T) {
	info := checkDot(t, "fn main() {\n x = None\n y = x.map(fn(v) { v + 1 })\n z = y.unwrap()\n w = z + 1\n}")
	for _, d := range info.Diagnostics.Errors {
		t.Errorf("unexpected diagnostic: %s", d.Error())
	}
}

// TestNoneInfer_MapExprBody documents a second residual bug: the short-form
// lambda `fn(v) = expr` is checked against the unbound result TypeParam U and
// produces a false positive.
func TestNoneInfer_MapExprBody(t *testing.T) {
	info := checkDot(t, "fn main() {\n x = None\n y = x.map(fn(v) = v + 1)\n}")
	for _, d := range info.Diagnostics.Errors {
		t.Errorf("unexpected diagnostic: %s", d.Error())
	}
}

func TestNoneInfer_MismatchStillReported(t *testing.T) {
	checkDotError(t, "fn main() { y = 1 + \"str\" }", "mismatched types")
}

func TestNoneInfer_BinaryTypeVarNoError(t *testing.T) {
	// Every TypeVar-pinning binary op must infer without diagnostics.
	bodies := []string{
		"v == 5",
		"v < 5",
		"v | 1",
		"v + \"s\"",
	}
	for _, body := range bodies {
		src := "fn main() {\n x = None\n y = x.map(fn(v) { " + body + " })\n}"
		info := checkDot(t, src)
		if info.Diagnostics.Len() != 0 {
			t.Errorf("body %q produced %d diagnostic(s)", body, info.Diagnostics.Len())
		}
	}
}

// --- TY-02: Info.Instances under both callee and CallExpr ----------------------------------------

func TestEnumInstance_RecordedUnderBothKeys(t *testing.T) {
	tests := []struct {
		name    string
		variant string
		src     string
	}{
		{"option_some", "Some", "fn main() { x = Some(42) }"},
		{"result_ok", "Ok", "fn main() { r = Ok(42) }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, info := parseCheckDot(t, tt.src)
			call := findCall(prog, tt.variant)
			if call == nil {
				t.Fatalf("call to %s not found", tt.variant)
			}
			calleeInst, calleeOK := info.Instances[call.Fn]
			if !calleeOK || calleeInst == nil {
				t.Fatalf("no instance recorded under the callee node %s", tt.variant)
			}
			callInst, callOK := info.Instances[call]
			if !callOK || callInst == nil {
				t.Fatalf("no instance recorded under the CallExpr of %s", tt.variant)
			}
			if calleeInst != callInst {
				t.Errorf("callee and CallExpr must share one *Instance, got %p and %p",
					calleeInst, callInst)
			}
			if calleeInst.Result == nil {
				t.Error("instance Result is nil")
			}
			if _, ok := optionOrResult(calleeInst.Result); !ok {
				t.Errorf("instance Result = %v, want Option/Result", calleeInst.Result)
			}
		})
	}
}

// optionOrResult reports whether t is an Option or Result instantiation.
func optionOrResult(t Type) (Type, bool) {
	if t == nil {
		return nil, false
	}
	named, ok := resolve(t).(*Named)
	if !ok {
		return nil, false
	}
	switch named.Name {
	case "Option", "Result":
		return named, true
	}
	return nil, false
}

// --- TY-04: Info.Heap on captures ---------------------------------------------------------------

func TestHeapCapture_Closure(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "captured_string_is_heap",
			src:  "fn main() {\n s = \"hi\"\n f = fn() { print(s) }\n}",
			want: true,
		},
		{
			name: "captured_int_is_not_heap",
			src:  "fn main() {\n n = 5\n f = fn() { print(n) }\n}",
			want: false,
		},
		{
			name: "captured_slice_is_heap",
			src:  "fn main() {\n a = []int{1, 2}\n f = fn() { print(a) }\n}",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, info := parseCheckDot(t, tt.src)
			fn := findFnLit(prog)
			if fn == nil {
				t.Fatal("no lambda found")
			}
			var capturedName string
			for _, name := range []string{"s", "n", "a"} {
				if findIdentIn(fn, name) != nil {
					capturedName = name
					break
				}
			}
			id := findIdentIn(fn, capturedName)
			if id == nil {
				t.Fatalf("captured ident %q not found inside the lambda", capturedName)
			}
			got, marked := info.Heap[id]
			if !marked {
				t.Fatalf("Info.Heap has no entry for captured %q", capturedName)
			}
			if got != tt.want {
				t.Errorf("Info.Heap[%s] = %v, want %v", capturedName, got, tt.want)
			}
		})
	}
}

func TestHeapCapture_Spawn(t *testing.T) {
	prog, info := parseCheckDot(t, "fn main() {\n s = \"hi\"\n h = spawn { print(s) }\n}")
	var spawn *ast.SpawnExpr
	ast.Inspect(prog, func(n ast.Node) bool {
		if spawn != nil {
			return false
		}
		if se, ok := n.(*ast.SpawnExpr); ok {
			spawn = se
			return false
		}
		return true
	})
	if spawn == nil {
		t.Fatal("no spawn expression found")
	}
	id := findIdentIn(spawn, "s")
	if id == nil {
		t.Fatal("captured ident \"s\" not found inside spawn")
	}
	got, marked := info.Heap[id]
	if !marked {
		t.Fatal("Info.Heap has no entry for the spawn capture")
	}
	if !got {
		t.Error("Info.Heap[spawn capture of string] = false, want true")
	}
}
