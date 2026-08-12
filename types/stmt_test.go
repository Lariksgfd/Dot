package types

import (
	"testing"
)

func TestCheckVarDecl_Infer(t *testing.T) {
	t.Run("int_infer", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42 }")
	})
	t.Run("string_infer", func(t *testing.T) {
		checkDot(t, "fn main() { name = \"Dot\" }")
	})
	t.Run("array_infer", func(t *testing.T) {
		checkDot(t, "fn main() { arr = []int{1, 2, 3} }")
	})
}

func TestCheckVarDecl_Explicit(t *testing.T) {
	t.Run("explicit_int", func(t *testing.T) {
		checkDot(t, "fn main() { count int = 0 }")
	})
	t.Run("explicit_float32", func(t *testing.T) {
		checkDot(t, "fn main() { ratio float32 = 1.5 }")
	})
	t.Run("explicit_no_value", func(t *testing.T) {
		checkDot(t, "fn main() { x int\n x = 1 }")
	})
}

func TestCheckVarDecl_Const(t *testing.T) {
	t.Run("const_infer", func(t *testing.T) {
		checkDot(t, "const PI = 3.14\n fn main() { x = PI }")
	})
	t.Run("const_explicit", func(t *testing.T) {
		checkDot(t, "const MAX_SIZE int = 1024\n fn main() { x = MAX_SIZE }")
	})
	t.Run("const_top_level", func(t *testing.T) {
		checkDot(t, "const VERSION = 1\n fn main() { x = VERSION }")
	})
}

func TestCheckVarDecl_Multiple(t *testing.T) {
	t.Run("multi_declare", func(t *testing.T) {
		checkDot(t, "fn f() -> (int, string) { return (1, \"hi\") }\n fn main() { a, b = f() }")
	})
	t.Run("underscore_discard", func(t *testing.T) {
		checkDot(t, "fn f() -> (int, string) { return (1, \"hi\") }\n fn main() { _, s = f() }")
	})
	t.Run("count_mismatch", func(t *testing.T) {
		checkDotError(t, "fn main() { a, b = 1 }", "assignment count mismatch")
	})
}

func TestCheckVarDecl_Reassign(t *testing.T) {
	t.Run("reassign_same_type", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n x = 43 }")
	})
	t.Run("reassign_different_type", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 42\n x = \"str\" }", "cannot use")
	})
	t.Run("reassign_const", func(t *testing.T) {
		checkDotError(t, "fn main() { const X = 1\n X = 2 }", "constant")
	})
}

func TestCheckVarDecl_Shadow(t *testing.T) {
	t.Run("shadow_new_scope", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n if true { x = \"str\" } }")
	})
}

func TestCheckReturn(t *testing.T) {
	t.Run("return_value", func(t *testing.T) {
		checkDot(t, "fn f() -> int { return 42 }\n fn main() { f() }")
	})
	t.Run("return_wrong_type", func(t *testing.T) {
		checkDotError(t, "fn f() -> int { return \"oops\" }\n fn main() { f() }", "cannot use")
	})
	t.Run("return_in_void_fn", func(t *testing.T) {
		checkDotError(t, "fn f() { return 42 }\n fn main() { f() }", "cannot use")
	})
	t.Run("return_void_fn_ok", func(t *testing.T) {
		checkDot(t, "fn f() { return }\n fn main() { f() }")
	})
	t.Run("missing_return", func(t *testing.T) {
		checkDotError(t, "fn f() -> int { x = 1 }\n fn main() { f() }", "missing return")
	})
	t.Run("multi_return", func(t *testing.T) {
		checkDot(t, "fn f() -> (int, string) { return (1, \"hi\") }\n fn main() { f() }")
	})
	t.Run("multi_return_mismatch", func(t *testing.T) {
		checkDotError(t, "fn f() -> (int, string) { return (1, 2) }\n fn main() { f() }", "cannot use")
	})
}

func TestCheckFor_In(t *testing.T) {
	t.Run("for_range", func(t *testing.T) {
		checkDot(t, "fn main() { for i in 0..10 { print(i) } }")
	})
	t.Run("for_slice", func(t *testing.T) {
		checkDot(t, "fn main() { items = []int{1, 2, 3}\n for item in items { print(item) } }")
	})
	t.Run("for_slice_indexed", func(t *testing.T) {
		checkDot(t, "fn main() { items = []int{1, 2, 3}\n for i, item in items { print(i, item) } }")
	})
	t.Run("for_map", func(t *testing.T) {
		checkDot(t, "fn main() { m = map[string]int{\"a\": 1}\n for k, v in m { print(k, v) } }")
	})
	t.Run("for_string", func(t *testing.T) {
		checkDot(t, "fn main() { s = \"hello\"\n for c in s { print(c) } }")
	})
}

func TestCheckFor_Cond(t *testing.T) {
	t.Run("for_cond", func(t *testing.T) {
		checkDot(t, "fn main() { x = 10\n for x > 0 { x -= 1 } }")
	})
	t.Run("for_cond_not_bool", func(t *testing.T) {
		checkDotError(t, "fn main() { for \"not bool\" { } }", "must be bool")
	})
}

func TestCheckFor_Infinite(t *testing.T) {
	t.Run("for_infinite_break", func(t *testing.T) {
		checkDot(t, "fn main() { for { break } }")
	})
	t.Run("for_infinite_cond_break", func(t *testing.T) {
		checkDot(t, "fn main() { for { if true { break } } }")
	})
}

func TestCheckBreakContinue(t *testing.T) {
	t.Run("break_in_loop", func(t *testing.T) {
		checkDot(t, "fn main() { for { break } }")
	})
	t.Run("continue_in_loop", func(t *testing.T) {
		checkDot(t, "fn main() { for { continue } }")
	})
}

func TestCheckDefer(t *testing.T) {
	t.Run("defer_call", func(t *testing.T) {
		checkDot(t, "fn close() {}\n fn main() { defer close() }")
	})
}
