package types

import (
	"strings"
	"testing"

	"github.com/dotlang/dot/parser"
)

func checkDot(t *testing.T, src string) *Info {
	t.Helper()
	prog, err := parser.ParseFile(src, "<test>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "<test>")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	return info
}

func checkDotError(t *testing.T, src string, wantMsg string) *Info {
	t.Helper()
	prog, err := parser.ParseFile(src, "<test>")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := Check(prog, "<test>")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), wantMsg) {
		t.Fatalf("expected error containing %q, got %q", wantMsg, err.Error())
	}
	return info
}

func TestCheckExpr_Ident(t *testing.T) {
	t.Run("resolved", func(t *testing.T) {
		info := checkDot(t, "fn main() { x = 42\n x + 1 }")
		if info == nil {
			t.Fatal("nil info")
		}
	})
	t.Run("undeclared", func(t *testing.T) {
		checkDotError(t, "fn main() { y + 1 }", "undefined")
	})
	t.Run("type_as_value", func(t *testing.T) {
		checkDotError(t, "fn main() { int }", "type, not a value")
	})
	t.Run("self", func(t *testing.T) {
		checkDot(t, "struct S { x int }\n impl S { fn f(self) -> int { return self.x } }\n fn main() {}")
	})
	t.Run("self_outside_method", func(t *testing.T) {
		checkDotError(t, "fn main() { self }", "self")
	})
}

func TestCheckExpr_Literals(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42 }")
	})
	t.Run("float", func(t *testing.T) {
		checkDot(t, "fn main() { x = 3.14 }")
	})
	t.Run("string", func(t *testing.T) {
		checkDot(t, "fn main() { x = \"hello\" }")
	})
	t.Run("raw_string", func(t *testing.T) {
		checkDot(t, "fn main() { x = `raw` }")
	})
	t.Run("bool_true", func(t *testing.T) {
		checkDot(t, "fn main() { x = true }")
	})
	t.Run("bool_false", func(t *testing.T) {
		checkDot(t, "fn main() { x = false }")
	})
	t.Run("nil", func(t *testing.T) {
		checkDot(t, "fn main() { x = nil }")
	})
	t.Run("string_interpolation", func(t *testing.T) {
		checkDot(t, "fn main() { name = \"Dot\"\n msg = \"Hello {name}\" }")
	})
}

func TestCheckExpr_Unary(t *testing.T) {
	t.Run("negate_int", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n y = -x }")
	})
	t.Run("negate_float", func(t *testing.T) {
		checkDot(t, "fn main() { x = 3.14\n y = -x }")
	})
	t.Run("plus_int", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n y = +x }")
	})
	t.Run("not_bool", func(t *testing.T) {
		checkDot(t, "fn main() { b = true\n c = not b }")
	})
	t.Run("tilde_int", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n y = ~x }")
	})
	t.Run("not_on_int", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 42\n y = not x }", "'not' requires bool")
	})
	t.Run("tilde_on_bool", func(t *testing.T) {
		checkDotError(t, "fn main() { b = true\n y = ~b }", "'~' requires an integer")
	})
	t.Run("negate_bool_error", func(t *testing.T) {
		checkDotError(t, "fn main() { b = true\n y = -b }", "numeric operand")
	})
}

func TestCheckExpr_Binary(t *testing.T) {
	t.Run("add_ints", func(t *testing.T) {
		checkDot(t, "fn main() { x = 1 + 2 }")
	})
	t.Run("add_floats", func(t *testing.T) {
		checkDot(t, "fn main() { x = 1.0 + 2.0 }")
	})
	t.Run("concat_strings", func(t *testing.T) {
		checkDot(t, "fn main() { x = \"a\" + \"b\" }")
	})
	t.Run("sub_ints", func(t *testing.T) {
		checkDot(t, "fn main() { x = 5 - 3 }")
	})
	t.Run("mul_ints", func(t *testing.T) {
		checkDot(t, "fn main() { x = 2 * 3 }")
	})
	t.Run("div_ints", func(t *testing.T) {
		checkDot(t, "fn main() { x = 6 / 2 }")
	})
	t.Run("mod_ints", func(t *testing.T) {
		checkDot(t, "fn main() { x = 7 % 2 }")
	})
	t.Run("pow_ints", func(t *testing.T) {
		checkDot(t, "fn main() { x = 2 ** 3 }")
	})
	t.Run("eq_ints", func(t *testing.T) {
		checkDot(t, "fn main() { x = 1 == 2 }")
	})
	t.Run("neq_ints", func(t *testing.T) {
		checkDot(t, "fn main() { x = 1 != 2 }")
	})
	t.Run("lt_ints", func(t *testing.T) {
		checkDot(t, "fn main() { x = 1 < 2 }")
	})
	t.Run("gt_ints", func(t *testing.T) {
		checkDot(t, "fn main() { x = 1 > 2 }")
	})
	t.Run("lte_ints", func(t *testing.T) {
		checkDot(t, "fn main() { x = 1 <= 2 }")
	})
	t.Run("gte_ints", func(t *testing.T) {
		checkDot(t, "fn main() { x = 1 >= 2 }")
	})
	t.Run("and_bool", func(t *testing.T) {
		checkDot(t, "fn main() { x = true and false }")
	})
	t.Run("or_bool", func(t *testing.T) {
		checkDot(t, "fn main() { x = true or false }")
	})
	t.Run("bitwise_and", func(t *testing.T) {
		checkDot(t, "fn main() { x = 5 & 3 }")
	})
	t.Run("bitwise_or", func(t *testing.T) {
		checkDot(t, "fn main() { x = 5 | 3 }")
	})
	t.Run("bitwise_xor", func(t *testing.T) {
		checkDot(t, "fn main() { x = 5 ^ 3 }")
	})
	t.Run("shift_left", func(t *testing.T) {
		checkDot(t, "fn main() { x = 1 << 2 }")
	})
	t.Run("shift_right", func(t *testing.T) {
		checkDot(t, "fn main() { x = 8 >> 2 }")
	})
	t.Run("type_mismatch", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 1 + \"str\" }", "mismatched types")
	})
	t.Run("mod_non_int", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 3.0 % 1.0 }", "'%' requires integers")
	})
	t.Run("compare_struct_error", func(t *testing.T) {
		checkDotError(t, "struct S {}\n fn main() { a = S {}\n b = S {}\n x = a < b }", "cannot be ordered")
	})
}

func TestCheckExpr_Assign(t *testing.T) {
	t.Run("simple_reassign", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n x = 43 }")
	})
	t.Run("reassign_type_error", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 42\n x = \"str\" }", "cannot assign")
	})
	t.Run("compound_plus", func(t *testing.T) {
		checkDot(t, "fn main() { x = 1\n x += 2 }")
	})
	t.Run("compound_minus", func(t *testing.T) {
		checkDot(t, "fn main() { x = 1\n x -= 2 }")
	})
	t.Run("compound_mul", func(t *testing.T) {
		checkDot(t, "fn main() { x = 2\n x *= 3 }")
	})
	t.Run("compound_div", func(t *testing.T) {
		checkDot(t, "fn main() { x = 6\n x /= 2 }")
	})
	t.Run("compound_concat", func(t *testing.T) {
		checkDot(t, "fn main() { s = \"a\"\n s += \"b\" }")
	})
	t.Run("multi_assign", func(t *testing.T) {
		checkDot(t, "fn f() -> (int, int) { return (1, 2) }\n fn main() { a, b = f() }")
	})
	t.Run("const_reassign_error", func(t *testing.T) {
		checkDotError(t, "fn main() { const X = 1\n X = 2 }", "constant")
	})
	t.Run("assign_to_non_var", func(t *testing.T) {
		checkDotError(t, "fn main() { 1 = 2 }", "cannot assign")
	})
}

func TestCheckExpr_Range(t *testing.T) {
	t.Run("basic_range", func(t *testing.T) {
		checkDot(t, "fn main() { r = 0..10 }")
	})
	t.Run("inclusive_range", func(t *testing.T) {
		checkDot(t, "fn main() { r = 0..=10 }")
	})
	t.Run("range_mismatched_types", func(t *testing.T) {
		checkDotError(t, "fn main() { r = 0..\"end\" }", "different types")
	})
}

func TestCheckExpr_Cast(t *testing.T) {
	t.Run("int_to_float", func(t *testing.T) {
		checkDot(t, "fn main() { x = 5 as float }")
	})
	t.Run("float_to_int", func(t *testing.T) {
		checkDot(t, "fn main() { x = 3.14 as int }")
	})
	t.Run("invalid_cast", func(t *testing.T) {
		checkDotError(t, "fn main() { x = \"hi\" as int }", "cannot convert")
	})
}

func TestCheckExpr_Is(t *testing.T) {
	t.Run("is_int", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n b = x is int }")
	})
}

func TestCheckExpr_Try(t *testing.T) {
	t.Run("try_option_ok", func(t *testing.T) {
		checkDot(t, "fn f() -> Option[int] { return Some(1) }\n fn g() -> Option[int] { v = f()?\n return Some(v) }")
	})
	t.Run("try_result_ok", func(t *testing.T) {
		checkDot(t, "fn f() -> Result[int, Error] { return Ok(1) }\n fn g() -> Result[int, Error] { v = f()?\n return Ok(v) }")
	})
	t.Run("try_outside_fn", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 42?\n }", "'?'")
	})
	t.Run("try_on_plain_int", func(t *testing.T) {
		checkDotError(t, "fn f() -> Option[int] { x = 42?\n return Some(x) }", "Option or Result")
	})
	t.Run("try_result_in_option_fn", func(t *testing.T) {
		checkDotError(t, "fn f() -> Option[int] { r = Ok(1)?\n return Some(r) }", "requires the function to return Result")
	})
}

func TestCheckExpr_Await(t *testing.T) {
	t.Run("await_outside_async", func(t *testing.T) {
		checkDotError(t, "fn main() { x = await 1 }", "async")
	})
	t.Run("await_non_future", func(t *testing.T) {
		checkDotError(t, "async fn main() { x = await 42 }", "cannot await")
	})
}

func TestCheckExpr_Spawn(t *testing.T) {
	t.Run("spawn_block", func(t *testing.T) {
		checkDot(t, "fn main() { h = spawn { print(\"hi\") } }")
	})
}
