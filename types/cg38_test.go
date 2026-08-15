package types

import "testing"

// TestCheckEnumEq_CG38 guards that the checker accepts == and != on enum
// values, for unit and payload variants. The CG-38 C-backend fix relies on
// these expressions typechecking.
func TestCheckEnumEq_CG38(t *testing.T) {
	checkDot(t, `enum Color {
    Red
    Green
}

fn main() {
    a = Color.Red
    b = Color.Green
    x = a == b
    y = a != b
}`)

	checkDot(t, `enum Shape {
    Circle(radius float)
    Square(side float)
}

fn main() {
    s1 = Shape.Circle(1.0)
    s2 = Shape.Square(2.0)
    x = s1 == s2
    y = s1 != s2
}`)
}

// TestCheckEnumEq_CG38_MixedWithOtherTypes verifies the result is bool and
// that cross-type comparisons (enum vs int) are still rejected.
func TestCheckEnumEq_CG38_MixedWithOtherTypes(t *testing.T) {
	info := checkDot(t, `enum Color {
    Red
    Green
}

fn main() {
    a = Color.Red
    x = a == a
}`)
	if info.Diagnostics.Len() != 0 {
		t.Errorf("expected no diagnostics, got: %v", info.Diagnostics)
	}

	checkDotError(t, `enum Color {
    Red
    Green
}

fn main() {
    a = Color.Red
    x = a == 42
}`, "mismatched types")
}
