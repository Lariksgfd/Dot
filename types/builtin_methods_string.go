package types

// --- string ----------------------------------------------------------------

var stringNames = []string{
	"len", "contains", "starts_with", "ends_with", "split", "trim",
	"to_upper", "to_lower", "replace", "index_of", "bytes", "chars",
	"parse_int", "parse_float", "to_string", "is_empty",
}

func stringMethod(name string) (methodSpec, bool) {
	switch name {
	case "len":
		return methodSpec{Name: "len", Field: true, Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Int}
		}}, true
	case "contains":
		return mkString("contains", fnOf(Bool, String_)), true
	case "starts_with":
		return mkString("starts_with", fnOf(Bool, String_)), true
	case "ends_with":
		return mkString("ends_with", fnOf(Bool, String_)), true
	case "split":
		return mkString("split", &Fn{Params: []Param{{Name: "sep", Type: String_}}, Result: &Slice{Elem: String_}}), true
	case "trim":
		return mkString("trim", &Fn{Result: String_}), true
	case "to_upper":
		return mkString("to_upper", &Fn{Result: String_}), true
	case "to_lower":
		return mkString("to_lower", &Fn{Result: String_}), true
	case "replace":
		return mkString("replace", &Fn{Params: []Param{{Name: "old", Type: String_}, {Name: "new", Type: String_}}, Result: String_}), true
	case "index_of":
		return methodSpec{Name: "index_of", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "sub", Type: String_}}, Result: u.OptionOf(Int)}
		}}, true
	case "bytes":
		return mkString("bytes", &Fn{Result: &Slice{Elem: Byte}}), true
	case "chars":
		return mkString("chars", &Fn{Result: &Slice{Elem: Rune}}), true
	case "parse_int":
		return methodSpec{Name: "parse_int", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: u.ResultOf(Int, u.Error)}
		}}, true
	case "parse_float":
		return methodSpec{Name: "parse_float", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: u.ResultOf(Float, u.Error)}
		}}, true
	case "to_string":
		return mkString("to_string", &Fn{Result: String_}), true
	case "is_empty":
		return mkString("is_empty", &Fn{Result: Bool}), true
	}
	return methodSpec{}, false
}
