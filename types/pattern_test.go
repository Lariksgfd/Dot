package types

import (
	"testing"
)

func TestCheckPattern_Literal(t *testing.T) {
	t.Run("literal_patterns", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n s = match x { 0 => \"zero\"\n 1 => \"one\"\n _ => \"other\" } }")
	})
	t.Run("bool_literal", func(t *testing.T) {
		checkDot(t, "fn main() { b = true\n s = match b { true => \"y\"\n false => \"n\" } }")
	})
	t.Run("string_literal", func(t *testing.T) {
		checkDot(t, "fn main() { s = \"hi\"\n r = match s { \"hi\" => 1\n \"bye\" => 2\n _ => 0 } }")
	})
	t.Run("literal_mismatch", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 42\n s = match x { \"hi\" => \"oops\"\n _ => \"ok\" } }", "cannot match")
	})
}

func TestCheckPattern_Ident(t *testing.T) {
	t.Run("ident_binding", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n s = match x { n => n + 1 } }")
	})
}

func TestCheckPattern_Wildcard(t *testing.T) {
	t.Run("wildcard", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n s = match x { _ => 0 } }")
	})
}

func TestCheckPattern_Or(t *testing.T) {
	t.Run("or_pattern", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n s = match x { 1 | 2 | 3 => \"small\"\n _ => \"other\" } }")
	})
}

func TestCheckPattern_Guard(t *testing.T) {
	t.Run("guarded_pattern", func(t *testing.T) {
		checkDot(t, "fn main() { x = 42\n s = match x { n if n > 100 => \"big\"\n _ => \"small\" } }")
	})
	t.Run("guard_not_bool", func(t *testing.T) {
		checkDotError(t, "fn main() { x = 42\n s = match x { n if n => \"bad\"\n _ => \"ok\" } }", "must be bool")
	})
}

func TestCheckPattern_Enum(t *testing.T) {
	t.Run("unit_variant", func(t *testing.T) {
		checkDot(t, "enum Color { Red\n Green\n Blue }\n fn main() { c = Color.Red\n s = match c { Color.Red => 1\n Color.Green => 2\n Color.Blue => 3 } }")
	})
	t.Run("payload_variant", func(t *testing.T) {
		checkDot(t, "enum Shape { Circle(radius float)\n Rectangle(width float, height float) }\n fn main() { s = Shape.Circle(2.0)\n a = match s { Shape.Circle(r) => r\n Shape.Rectangle(w, h) => w * h } }")
	})
	t.Run("option_variant", func(t *testing.T) {
		checkDot(t, "fn main() { opt = Some(42)\n v = match opt { Some(x) => x\n None => 0 } }")
	})
	t.Run("result_variant", func(t *testing.T) {
		checkDot(t, "fn main() { res = Ok(42)\n v = match res { Ok(x) => x\n Err(e) => 0 } }")
	})
	t.Run("variant_not_found", func(t *testing.T) {
		checkDotError(t, "enum Color { Red\n Green }\n fn main() { c = Color.Red\n s = match c { Color.Unknown => 1 } }", "has no variant")
	})
	t.Run("wrong_payload_count", func(t *testing.T) {
		checkDotError(t, "enum Shape { Circle(radius float) }\n fn main() { s = Shape.Circle(2.0)\n a = match s { Shape.Circle(r, extra) => r } }", "takes 1 value")
	})
}

func TestCheckPattern_Struct(t *testing.T) {
	t.Run("struct_pattern", func(t *testing.T) {
		checkDot(t, "struct Point { x float\n y float }\n fn main() { p = Point { x: 0.0, y: 0.0 }\n s = match p { Point { x: 0.0, y: 0.0 } => \"origin\"\n _ => \"other\" } }")
	})
}

func TestCheckPattern_Tuple(t *testing.T) {
	t.Run("tuple_pattern", func(t *testing.T) {
		checkDot(t, "fn main() { t = (10, 20)\n s = match t { (0, 0) => \"origin\"\n (x, y) => \"other\"\n _ => \"unknown\" } }")
	})
}

func TestCheckPattern_Range(t *testing.T) {
	t.Run("range_pattern", func(t *testing.T) {
		checkDot(t, "fn main() { score = 85\n grade = match score { 0..=59 => \"F\"\n 60..=69 => \"D\"\n _ => \"A\" } }")
	})
}
