package types

import (
	"testing"
)

func TestCheckFnLit_Lambda(t *testing.T) {
	t.Run("lambda_with_types", func(t *testing.T) {
		checkDot(t, "fn main() { f = fn(x int) -> int { return x * 2 }\n y = f(5) }")
	})
	t.Run("lambda_no_return_type", func(t *testing.T) {
		checkDot(t, "fn main() { f = fn(x int) { x * 2 } }")
	})
	t.Run("lambda_short_form", func(t *testing.T) {
		checkDot(t, "fn main() { f = fn(x int) -> int = x * 2 }")
	})
	t.Run("lambda_at_call", func(t *testing.T) {
		checkDot(t, "fn map_int(f fn(int) -> int, arr []int) -> []int { return arr }\n fn main() { doubled = map_int(fn(x int) -> int = x * 2, []int{1, 2, 3}) }")
	})
	t.Run("lambda_capture", func(t *testing.T) {
		checkDot(t, "fn main() { factor = 10\n mul = fn(x int) -> int { return x * factor } }")
	})
}

func TestCheckStructLit(t *testing.T) {
	t.Run("struct_literal", func(t *testing.T) {
		checkDot(t, "struct Point { x float\n y float }\n fn main() { p = Point { x: 1.0, y: 2.0 } }")
	})
	t.Run("struct_literal_wrong_type", func(t *testing.T) {
		checkDotError(t, "struct Point { x float\n y float }\n fn main() { p = Point { x: 1.0, y: \"wrong\" } }", "cannot use")
	})
	t.Run("struct_literal_missing_field", func(t *testing.T) {
		checkDotError(t, "struct Point { x float\n y float }\n fn main() { p = Point { x: 1.0 } }", "missing field")
	})
	t.Run("struct_literal_unknown_field", func(t *testing.T) {
		checkDotError(t, "struct Point { x float\n y float }\n fn main() { p = Point { z: 1.0 } }", "has no field")
	})
	t.Run("struct_literal_default_field", func(t *testing.T) {
		checkDot(t, "struct User { name string\n active bool = true }\n fn main() { u = User { name: \"a\" } }")
	})
}

func TestCheckTupleLit(t *testing.T) {
	t.Run("tuple_literal", func(t *testing.T) {
		checkDot(t, "fn main() { t = (10, 20) }")
	})
	t.Run("named_tuple", func(t *testing.T) {
		checkDot(t, "fn main() { t = (x: 10, y: 20) }")
	})
	t.Run("empty_tuple", func(t *testing.T) {
		checkDot(t, "fn main() { t = () }")
	})
}

func TestCheckArrayLit(t *testing.T) {
	t.Run("typed_array_lit", func(t *testing.T) {
		checkDot(t, "fn main() { arr = []int{1, 2, 3} }")
	})
	t.Run("inferred_array_lit", func(t *testing.T) {
		checkDot(t, "fn main() { arr = [1, 2, 3] }")
	})
	t.Run("fixed_array_lit", func(t *testing.T) {
		checkDot(t, "fn main() { arr = [3]int{1, 2, 3} }")
	})
	t.Run("typed_empty_array", func(t *testing.T) {
		checkDot(t, "fn main() { arr = []int{} }")
	})
}

func TestCheckMapLit(t *testing.T) {
	t.Run("map_literal", func(t *testing.T) {
		checkDot(t, "fn main() { m = map[string]int{\"a\": 1, \"b\": 2} }")
	})
	t.Run("typed_empty_map", func(t *testing.T) {
		checkDot(t, "fn main() { m = map[string]int{} }")
	})
}

func TestCheckIfExpr(t *testing.T) {
	t.Run("if_expr_branches", func(t *testing.T) {
		checkDot(t, "fn main() { x = 5\n y = if x > 0 { 1 } else { -1 } }")
	})
	t.Run("if_branch_mismatch", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 5\n y = if x > 0 { 1 } else { \"neg\" } }", "incompatible types")
	})
	t.Run("if_elseif", func(t *testing.T) {
		checkDot(t, "fn main() { x = 5\n y = if x > 10 { \"big\" } else if x > 0 { \"pos\" } else { \"zero\" } }")
	})
	t.Run("if_let_option", func(t *testing.T) {
		checkDot(t, "fn find(x int) -> Option[int] { return Some(x) }\n fn main() { x = 42\n name = if user = find(x) { user } else { 0 } }")
	})
	t.Run("if_let_not_option", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 42\n if y = x { y } else { 0 } }", "if-let requires")
	})
}

func TestCheckMatchExpr(t *testing.T) {
	t.Run("match_with_wildcard", func(t *testing.T) {
		checkDot(t, "enum Color { Red\n Green\n Blue }\n fn main() { c = Color.Red\n s = match c { Color.Red => \"R\"\n Color.Green => \"G\"\n _ => \"?\" } }")
	})
	t.Run("match_with_guard", func(t *testing.T) {
		checkDot(t, "fn main() { n = 42\n s = match n { x if x > 100 => \"big\"\n _ => \"small\" } }")
	})
	t.Run("match_enum_payload", func(t *testing.T) {
		checkDot(t, "enum Shape { Circle(radius float)\n Rectangle(width float, height float) }\n fn main() { s = Shape.Circle(2.0)\n circ = match s { Shape.Circle(r) => 3.14 * r * r\n Shape.Rectangle(w, h) => w * h } }")
	})
	t.Run("match_on_int", func(t *testing.T) {
		checkDot(t, "fn main() { v = 42\n s = match v { 0 => \"zero\"\n 1 => \"one\"\n _ => \"other\" } }")
	})
}

func TestCheckBlockExpr(t *testing.T) {
	t.Run("if_branch_block_value", func(t *testing.T) {
		checkDot(t, "fn main() { x = if true { y = 1\n y + 2 } else { 0 } }")
	})
}
