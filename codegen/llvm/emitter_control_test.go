package llvm

import (
	"strings"
	"testing"
)

func TestGenerate_IfStmt(t *testing.T) {
	source := `
fn main() {
	x = 10
	if x > 5 {
		x = 20
	}
}
`
	c := generateLLVM(t, source)
	if !strings.Contains(c, "if.then") {
		t.Errorf("Missing if.then block")
	}
}

func TestGenerate_BlockExpr_Match(t *testing.T) {
	source := `
fn main() {
	x = 1
	y = match x {
		0 => {
			z = 10
			z + 5
		},
		_ => 0
	}
}
`
	c := generateLLVM(t, source)
	if !strings.Contains(c, "add i32") {
		t.Errorf("Missing add in block expression inside match")
	}
}

func TestGenerate_NestedIfExpr(t *testing.T) {
	source := `
fn main() {
	x = 10
	y = if x > 5 {
		if x > 8 {
			1
		} else {
			2
		}
	} else {
		3
	}
}
`
	c := generateLLVM(t, source)
	if !strings.Contains(c, "if.then") || !strings.Contains(c, "if.else") {
		t.Errorf("Missing branch labels")
	}
}

func TestGenerate_ForCond(t *testing.T) {
	source := `
fn main() {
	i = 0
	for i < 5 {
		i = i + 1
	}
}
`
	c := generateLLVM(t, source)
	if !strings.Contains(c, "for.cond") || !strings.Contains(c, "for.body") || !strings.Contains(c, "for.end") {
		t.Errorf("Missing for loop blocks:\n%s", c)
	}
}

func TestGenerate_ForIn(t *testing.T) {
	source := `
fn main() {
	sum = 0
	for i in 0..10 {
		sum = sum + i
	}
}
`
	c := generateLLVM(t, source)
	if !strings.Contains(c, "for.cond") || !strings.Contains(c, "for.body") || !strings.Contains(c, "for.step") {
		t.Errorf("Missing for-in loop blocks:\n%s", c)
	}
}

func TestGenerate_ForInInclusive(t *testing.T) {
	source := `
fn main() {
	for i in 0..=5 {
		x = i
	}
}
`
	c := generateLLVM(t, source)
	if !strings.Contains(c, "sle i32") { // sle means signed less than or equal
		t.Errorf("Missing inclusive comparison (sle) in for-in loop:\n%s", c)
	}
}

func TestGenerate_EmptyBlockExpr(t *testing.T) {
	source := `
fn main() {
	if true {}
}
`
	c := generateLLVM(t, source)
	if len(c) == 0 {
		t.Errorf("Empty output")
	}
}

func TestGenerate_ForEmptyBody(t *testing.T) {
	source := `
fn main() {
	for false {}
	for i in 1..2 {}
}
`
	c := generateLLVM(t, source)
	if len(c) == 0 {
		t.Errorf("Empty output")
	}
}
