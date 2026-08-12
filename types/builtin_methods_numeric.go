package types

// --- numeric ---------------------------------------------------------------

var numericNames = []string{
	"to_string", "to_float", "to_int", "abs", "min", "max", "sqrt",
	"floor", "ceil", "round", "pow",
}

func numericMethod(recv Type, name string) (methodSpec, bool) {
	isFloat := IsFloat(recv)
	switch name {
	case "to_string":
		return methodSpec{Name: "to_string", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: String_}
		}}, true
	case "to_float":
		return methodSpec{Name: "to_float", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Float}
		}}, true
	case "to_int":
		return methodSpec{Name: "to_int", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Int}
		}}, true
	case "abs":
		return methodSpec{Name: "abs", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: recv}
		}}, true
	case "min":
		return methodSpec{Name: "min", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "other", Type: recv}}, Result: recv}
		}}, true
	case "max":
		return methodSpec{Name: "max", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "other", Type: recv}}, Result: recv}
		}}, true
	case "sqrt":
		if !isFloat {
			return methodSpec{}, false
		}
		return methodSpec{Name: "sqrt", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Float}
		}}, true
	case "floor", "ceil", "round":
		if !isFloat {
			return methodSpec{}, false
		}
		return methodSpec{Name: name, Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Float}
		}}, true
	case "pow":
		if !isFloat {
			return methodSpec{}, false
		}
		return methodSpec{Name: "pow", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "exp", Type: Float}}, Result: Float}
		}}, true
	}
	return methodSpec{}, false
}

// --- bool ------------------------------------------------------------------

var boolNames = []string{"to_string"}

func boolMethod(name string) (methodSpec, bool) {
	if name == "to_string" {
		return methodSpec{Name: "to_string", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: String_}
		}}, true
	}
	return methodSpec{}, false
}
