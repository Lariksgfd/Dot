package types

import (
	"testing"
)

func TestExhaustive_EnumExact(t *testing.T) {
	t.Run("exact_three", func(t *testing.T) {
		checkDot(t, "enum Color { Red\n Green\n Blue }\n fn main() { c = Color.Red\n s = match c { Color.Red => 1\n Color.Green => 2\n Color.Blue => 3 } }")
	})
	t.Run("missing_variant", func(t *testing.T) {
		checkDotError(t, "enum Color { Red\n Green\n Blue }\n fn main() { c = Color.Red\n s = match c { Color.Red => 1\n Color.Green => 2 } }", "not exhaustive")
	})
}

func TestExhaustive_Bool(t *testing.T) {
	t.Run("bool_both", func(t *testing.T) {
		checkDot(t, "fn main() { b = true\n s = match b { true => 1\n false => 0 } }")
	})
	t.Run("bool_missing", func(t *testing.T) {
		checkDotError(t, "fn main() { b = true\n s = match b { true => 1 } }", "not exhaustive")
	})
	t.Run("bool_missing_false", func(t *testing.T) {
		checkDotError(t, "fn main() { b = true\n s = match b { false => 1 } }", "not exhaustive")
	})
}

func TestExhaustive_Wildcard(t *testing.T) {
	t.Run("wildcard_covers", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n s = match x { 1 => \"one\"\n 2 => \"two\"\n _ => \"other\" } }")
	})
	t.Run("ident_covers", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n s = match x { n => n } }")
	})
}

func TestExhaustive_Option(t *testing.T) {
	t.Run("option_both", func(t *testing.T) {
		checkDot(t, "fn main() { opt = Some(42)\n v = match opt { Some(x) => x\n None => 0 } }")
	})
	t.Run("option_missing", func(t *testing.T) {
		checkDotError(t, "fn main() { opt = Some(42)\n v = match opt { Some(x) => x } }", "not exhaustive")
	})
}

func TestExhaustive_Result(t *testing.T) {
	t.Run("result_both", func(t *testing.T) {
		checkDot(t, "fn main() { res = Ok(42)\n v = match res { Ok(x) => x\n Err(e) => 0 } }")
	})
	t.Run("result_missing", func(t *testing.T) {
		checkDotError(t, "fn main() { res = Ok(42)\n v = match res { Ok(x) => x } }", "not exhaustive")
	})
}

func TestExhaustive_Guarded(t *testing.T) {
	t.Run("guarded_not_cover", func(t *testing.T) {
		checkDotError(t, "enum Color { Red\n Green }\n fn main() { c = Color.Red\n s = match c { Color.Red => 1\n Color.Green if true => 2 } }", "not exhaustive")
	})
}
