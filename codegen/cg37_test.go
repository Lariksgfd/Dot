package codegen

import (
	"strings"
	"testing"
)

// TestEmitVarDecl_LoopReassignNoRedeclaration checks that a loop-body
// reassignment of an enclosing variable (CG-37) is emitted as a plain C
// assignment, not as a second declaration. `s = s + i` must never become
// `int64_t s = (s + i);`, which would shadow the accumulator and read the
// uninitialised new binding.
func TestEmitVarDecl_LoopReassignNoRedeclaration(t *testing.T) {
	c := generateFromSource(t, `fn sum_to(n int) -> int {
    s = 0
    for i in 1..n {
        s = s + i
    }
    return s
}

fn main() {}`)

	if got := strings.Count(c, "int64_t s = "); got != 1 {
		t.Errorf("expected exactly one declaration of s, found %d:\n%s", got, c)
	}
	if !strings.Contains(c, "s = (s + i);") {
		t.Error("missing plain reassignment `s = (s + i);` in the loop body")
	}
	if strings.Contains(c, "int64_t s = (s + i);") {
		t.Error("loop-body reassignment was emitted as a declaration, want a plain assignment")
	}
}

// TestEmitVarDecl_MatchArmReassignNoRedeclaration checks the same plain
// assignment for a loop nested in a block match arm that reassigns the
// arm-local accumulator.
func TestEmitVarDecl_MatchArmReassignNoRedeclaration(t *testing.T) {
	c := generateFromSource(t, `fn sum_to(n int) -> int {
    match n {
        0 => 0
        _ => {
            s = 0
            for i in 1..n {
                s = s + i
            }
            s
        }
    }
}

fn main() {}`)

	if got := strings.Count(c, "int64_t s = "); got != 1 {
		t.Errorf("expected exactly one declaration of s, found %d:\n%s", got, c)
	}
	if !strings.Contains(c, "s = (s + i);") {
		t.Error("missing plain reassignment `s = (s + i);` in the arm loop body")
	}
	if strings.Contains(c, "int64_t s = (s + i);") {
		t.Error("arm-loop reassignment was emitted as a declaration, want a plain assignment")
	}
}

// TestEmitMatch_NeverArmNoResultAssign checks that a return-ending arm body
// (type Never, CG-37) is emitted as a plain statement expression, not as an
// assignment to _match_res, which would be invalid C.
func TestEmitMatch_NeverArmNoResultAssign(t *testing.T) {
	c := generateFromSource(t, `fn f(x int) -> int {
    match x {
        0 => 0
        _ => {
            return 42
        }
    }
}

fn main() {}`)

	if strings.Contains(c, "_match_res = ({") {
		t.Errorf("Never arm assigned to _match_res, want a bare statement expression:\n%s", c)
	}
	if !strings.Contains(c, "_match_res = 0;") {
		t.Error("value arm did not assign to _match_res")
	}
}

// TestEmitBody_NeverTailNotMaterialised checks that a trailing Never
// expression in a function body is emitted as a statement, without a
// return-temporary (which would produce `return ({ ... return ... })`).
func TestEmitBody_NeverTailNotMaterialised(t *testing.T) {
	c := generateFromSource(t, `fn f(x int) -> int {
    match x {
        _ => {
            return 42
        }
    }
}

fn main() {}`)

	if strings.Contains(c, "_dot_ret_tail_") {
		t.Errorf("Never-typed tail was materialised into a return temporary:\n%s", c)
	}
	if !strings.Contains(c, "return _dot_ret_") {
		t.Error("the diverging arm's return was not emitted")
	}
}
