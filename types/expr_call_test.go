package types

import (
	"testing"
)

func TestCheckCall_Simple(t *testing.T) {
	t.Run("direct_call", func(t *testing.T) {
		checkDot(t, "fn add(a int, b int) -> int { return a + b }\n fn main() { x = add(1, 2) }")
	})
	t.Run("wrong_arg_count", func(t *testing.T) {
		checkDotError(t, "fn add(a int, b int) -> int { return a + b }\n fn main() { x = add(1) }", "missing argument")
	})
	t.Run("wrong_arg_type", func(t *testing.T) {
		checkDotError(t, "fn add(a int, b int) -> int { return a + b }\n fn main() { x = add(\"x\", \"y\") }", "cannot use")
	})
	t.Run("too_many_args", func(t *testing.T) {
		checkDotError(t, "fn add(a int, b int) -> int { return a + b }\n fn main() { x = add(1, 2, 3) }", "too many arguments")
	})
	t.Run("call_non_function", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 42(1) }", "cannot call")
	})
}

func TestCheckCall_DefaultParams(t *testing.T) {
	t.Run("use_default", func(t *testing.T) {
		checkDot(t, "fn f(x int = 42) -> int { return x }\n fn main() { y = f() }")
	})
	t.Run("override_default", func(t *testing.T) {
		checkDot(t, "fn f(x int = 42) -> int { return x }\n fn main() { y = f(10) }")
	})
}

func TestCheckCall_NamedArgs(t *testing.T) {
	t.Run("named_args", func(t *testing.T) {
		checkDot(t, "fn connect(host string, port int = 8080) {}\n fn main() { connect(host: \"localhost\", port: 3000) }")
	})
	t.Run("named_args_reordered", func(t *testing.T) {
		checkDot(t, "fn connect(host string, port int = 8080) {}\n fn main() { connect(port: 3000, host: \"localhost\") }")
	})
	t.Run("unknown_param_name", func(t *testing.T) {
		checkDotError(t, "fn connect(host string, port int) {}\n fn main() { connect(unknown: 42) }", "unknown parameter name")
	})
	t.Run("double_param", func(t *testing.T) {
		checkDotError(t, "fn f(a int, b int) {}\n fn main() { f(a: 1, a: 2) }", "supplied twice")
	})
}

func TestCheckCall_Variadic(t *testing.T) {
	t.Run("variadic_zero", func(t *testing.T) {
		checkDot(t, "fn sum(nums ...int) -> int { return 0 }\n fn main() { x = sum() }")
	})
	t.Run("variadic_three", func(t *testing.T) {
		checkDot(t, "fn sum(nums ...int) -> int { return 0 }\n fn main() { x = sum(1, 2, 3) }")
	})
}

func TestCheckCall_Builtins(t *testing.T) {
	t.Run("print", func(t *testing.T) {
		checkDot(t, "fn main() { print(\"hello\") }")
	})
	t.Run("println", func(t *testing.T) {
		checkDot(t, "fn main() { println(42) }")
	})
	t.Run("eprint", func(t *testing.T) {
		checkDot(t, "fn main() { eprint(\"err\") }")
	})
	t.Run("input", func(t *testing.T) {
		checkDot(t, "fn main() { name = input(\"> \") }")
	})
	t.Run("type_of", func(t *testing.T) {
		checkDot(t, "fn main() { s = type_of(42) }")
	})
	t.Run("assert", func(t *testing.T) {
		checkDot(t, "fn main() { assert(true) }")
	})
	t.Run("assert_with_msg", func(t *testing.T) {
		checkDot(t, "fn main() { assert(true, \"must be ok\") }")
	})
	t.Run("panic", func(t *testing.T) {
		checkDot(t, "fn main() { panic(\"oh no\") }")
	})
}

func TestCheckCall_Method(t *testing.T) {
	t.Run("struct_method", func(t *testing.T) {
		checkDot(t, "struct Point { x float\n y float }\n impl Point { fn dist(self, other Point) -> float { return 0.0 } }\n fn main() { p = Point { x: 1.0, y: 2.0 }\n q = Point { x: 3.0, y: 4.0 }\n d = p.dist(q) }")
	})
	t.Run("static_method", func(t *testing.T) {
		checkDot(t, "struct Point { x float\n y float }\n impl Point { fn origin() -> Point { return Point { x: 0.0, y: 0.0 } } }\n fn main() { p = Point.origin() }")
	})
}

func TestCheckIndex(t *testing.T) {
	t.Run("slice_index", func(t *testing.T) {
		checkDot(t, "fn main() { arr = []int{1, 2, 3}\n x = arr[0] }")
	})
	t.Run("map_index", func(t *testing.T) {
		checkDot(t, "fn main() { m = map[string]int{\"a\": 1}\n v = m[\"a\"] }")
	})
	t.Run("generic_instantiation", func(t *testing.T) {
		checkDot(t, "fn main() { opt = Option[int]\n x = Some(1) }")
	})
	t.Run("string_index", func(t *testing.T) {
		checkDot(t, "fn main() { s = \"hello\"\n c = s[0] }")
	})
	t.Run("non_indexable", func(t *testing.T) {
		checkDotError(t, "fn main() { x = true[0] }", "cannot index")
	})
}

func TestCheckSlice(t *testing.T) {
	t.Run("slice_slice", func(t *testing.T) {
		checkDot(t, "fn main() { arr = []int{1, 2, 3}\n x = arr[1..3] }")
	})
	t.Run("slice_inclusive", func(t *testing.T) {
		checkDot(t, "fn main() { arr = []int{1, 2, 3}\n x = arr[1..=2] }")
	})
	t.Run("string_slice", func(t *testing.T) {
		checkDot(t, "fn main() { s = \"hello\"\n x = s[1..3] }")
	})
	t.Run("cannot_slice_int", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 42[1..3] }", "cannot slice")
	})
}

func TestCheckField(t *testing.T) {
	t.Run("struct_field", func(t *testing.T) {
		checkDot(t, "struct Point { x float\n y float }\n fn main() { p = Point { x: 1.0, y: 2.0 }\n a = p.x }")
	})
	t.Run("tuple_index", func(t *testing.T) {
		checkDot(t, "fn f() -> (int, string) { return (1, \"hi\") }\n fn main() { t = f()\n x = t.0 }")
	})
	t.Run("enum_variant", func(t *testing.T) {
		checkDot(t, "enum Color { Red\n Green\n Blue }\n fn main() { c = Color.Red }")
	})
	t.Run("no_member", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 42\n y = x.nope }", "has no member")
	})
	t.Run("slice_len_prop", func(t *testing.T) {
		checkDot(t, "fn main() { arr = []int{1, 2, 3}\n x = arr.len }")
	})
}

func TestCheckPipe(t *testing.T) {
	t.Run("simple_pipe", func(t *testing.T) {
		checkDot(t, "fn f(x int) -> int { return x * 2 }\n fn main() { y = 5 |> f }")
	})
	t.Run("pipe_non_function", func(t *testing.T) {
		checkDotError(t, "fn main() { y = 5 |> 42 }", "function")
	})
	t.Run("pipe_chained", func(t *testing.T) {
		checkDot(t, "fn f(x int) -> int { return x * 2 }\n fn g(x int) -> int { return x + 1 }\n fn main() { y = 1 |> f |> g }")
	})
}
