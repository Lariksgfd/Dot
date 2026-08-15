package codegen

import (
	"fmt"
	"strings"
	"testing"
)

// TestEmitEnumEq_TagComparisonNoRawPointerEq checks that == and != on enum
// operands (CG-38) are emitted as tag-based, NULL-safe comparisons, not as
// raw pointer comparisons: two separately allocated values of the same
// variant must compare equal.
func TestEmitEnumEq_TagComparisonNoRawPointerEq(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   string
		raw  string
	}{
		{"eq", "a == b", "(a == b)"},
		{"neq", "a != b", "(a != b)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := generateFromSource(t, fmt.Sprintf(`enum Color {
    Red
    Green
}

fn main() {
    a = Color.Red
    b = Color.Green
    print(%s)
}`, tc.op))

			if !strings.Contains(c, "->tag == ") {
				t.Errorf("missing tag comparison in enum emission:\n%s", c)
			}
			if !strings.Contains(c, "!= NULL") {
				t.Errorf("missing NULL-safety in enum emission:\n%s", c)
			}
			if strings.Contains(c, tc.raw) {
				t.Errorf("raw pointer comparison on operands, want tag-based:\n%s", c)
			}
			if !strings.Contains(c, "_dot_eq_a_") || !strings.Contains(c, "_dot_eq_b_") {
				t.Errorf("operands not materialised into temps:\n%s", c)
			}
		})
	}
}

// TestEmitEnumEq_PayloadVariantUsesSameForm checks the payload enum case:
// comparisons on payload-carrying variants get the same tag-based emission.
func TestEmitEnumEq_PayloadVariantUsesSameForm(t *testing.T) {
	c := generateFromSource(t, `enum Shape {
    Circle(radius float)
    Square(side float)
}

fn main() {
    s1 = Shape.Circle(1.0)
    s2 = Shape.Circle(2.0)
    print(s1 == s2)
    print(s1 != s2)
}`)

	if !strings.Contains(c, "->tag == ") {
		t.Errorf("missing tag comparison in payload-enum emission:\n%s", c)
	}
	if strings.Contains(c, "(s1 == s2)") {
		t.Errorf("raw pointer comparison on payload-enum operands, want tag-based:\n%s", c)
	}
}

// TestEmitEnumEq_OperandsEvaluatedOnce checks that call operands are
// materialised into temps so each appears exactly once in the emitted
// comparison: a duplicated statement-expression operand would run its side
// effects twice.
func TestEmitEnumEq_OperandsEvaluatedOnce(t *testing.T) {
	c := generateFromSource(t, `enum Color {
    Red
    Green
}

fn make() -> Color {
    return Color.Red
}

fn main() {
    print(make() == make())
    print(make() != make())
}`)

	if got := strings.Count(c, "(Dot_make())"); got != 4 {
		t.Errorf("make() call sites = %d, want 4 (two per comparison):\n%s", got, c)
	}
	if got := strings.Count(c, "DotColor* _dot_eq_a_"); got != 2 {
		t.Errorf("lhs temp declarations = %d, want 2:\n%s", got, c)
	}
	if got := strings.Count(c, "DotColor* _dot_eq_b_"); got != 2 {
		t.Errorf("rhs temp declarations = %d, want 2:\n%s", got, c)
	}
}

// TestEmitBinary_NonEnumOperandsUsePlainComparisons is a regression guard:
// int and string comparisons keep their plain (pre-CG-38) emission paths.
func TestEmitBinary_NonEnumOperandsUsePlainComparisons(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		c := generateFromSource(t, `fn main() {
    a = 1
    b = 2
    print(a == b)
    print(a != b)
}`)
		if !strings.Contains(c, "(a == b)") {
			t.Errorf("missing plain int == emission:\n%s", c)
		}
		if !strings.Contains(c, "(a != b)") {
			t.Errorf("missing plain int != emission:\n%s", c)
		}
		if strings.Contains(c, "_dot_eq_a_") {
			t.Errorf("enum temp machinery leaked into int comparison:\n%s", c)
		}
	})
	t.Run("string", func(t *testing.T) {
		c := generateFromSource(t, `fn main() {
    a = "x"
    b = "y"
    print(a == b)
    print(a != b)
}`)
		if !strings.Contains(c, "dot_string_eq(a, b)") {
			t.Errorf("missing string == helper call:\n%s", c)
		}
		if !strings.Contains(c, "!dot_string_eq(a, b)") {
			t.Errorf("missing string != helper call:\n%s", c)
		}
	})
}
