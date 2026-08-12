package types

// --- map -------------------------------------------------------------------

var mapNames = []string{
	"len", "get", "set", "has", "remove", "keys", "values", "clear", "is_empty",
}

func mapMethod(recv *Map, name string) (methodSpec, bool) {
	k, v := recv.Key, recv.Value
	switch name {
	case "len":
		return methodSpec{Name: "len", Field: true, Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Int}
		}}, true
	case "get":
		return methodSpec{Name: "get", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "key", Type: k}}, Result: u.OptionOf(v)}
		}}, true
	case "set":
		return methodSpec{Name: "set", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "key", Type: k}, {Name: "value", Type: v}}, Result: Void}
		}}, true
	case "has":
		return methodSpec{Name: "has", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "key", Type: k}}, Result: Bool}
		}}, true
	case "remove":
		return methodSpec{Name: "remove", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "key", Type: k}}, Result: Bool}
		}}, true
	case "keys":
		return methodSpec{Name: "keys", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: &Slice{Elem: k}}
		}}, true
	case "values":
		return methodSpec{Name: "values", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: &Slice{Elem: v}}
		}}, true
	case "clear":
		return methodSpec{Name: "clear", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Void}
		}}, true
	case "is_empty":
		return methodSpec{Name: "is_empty", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Bool}
		}}, true
	}
	return methodSpec{}, false
}

// --- array -----------------------------------------------------------------

var arrayNames = []string{
	"len", "contains", "find", "index_of", "map", "filter", "reduce",
	"first", "last", "is_empty",
}

func arrayMethod(recv *Array, name string) (methodSpec, bool) {
	t := recv.Elem
	switch name {
	case "len":
		return methodSpec{Name: "len", Field: true, Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Int}
		}}, true
	case "contains":
		return methodSpec{Name: "contains", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "value", Type: t}}, Result: Bool}
		}}, true
	case "find":
		return methodSpec{Name: "find", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "pred", Type: predFn(t)}}, Result: u.OptionOf(t)}
		}}, true
	case "index_of":
		return methodSpec{Name: "index_of", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "value", Type: t}}, Result: u.OptionOf(Int)}
		}}, true
	case "map":
		return methodSpec{Name: "map", Build: func(u *Universe, r Type) *Fn {
			f, up := mapFn(t)
			return &Fn{Params: []Param{{Name: "f", Type: f}}, Result: &Slice{Elem: up}, TypeParams: []*TypeParam{up}}
		}}, true
	case "filter":
		return methodSpec{Name: "filter", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "pred", Type: predFn(t)}}, Result: &Slice{Elem: t}}
		}}, true
	case "reduce":
		return methodSpec{Name: "reduce", Build: func(u *Universe, r Type) *Fn {
			f, up := reduceFn(t)
			return &Fn{Params: []Param{{Name: "init", Type: up}, {Name: "f", Type: f}}, Result: up, TypeParams: []*TypeParam{up}}
		}}, true
	case "first":
		return methodSpec{Name: "first", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: u.OptionOf(t)}
		}}, true
	case "last":
		return methodSpec{Name: "last", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: u.OptionOf(t)}
		}}, true
	case "is_empty":
		return methodSpec{Name: "is_empty", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Bool}
		}}, true
	}
	return methodSpec{}, false
}

// --- channel ---------------------------------------------------------------

var chanNames = []string{"send", "recv", "close", "is_closed"}

func chanMethod(recv *Chan, name string) (methodSpec, bool) {
	t := recv.Elem
	switch name {
	case "send":
		return methodSpec{Name: "send", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "value", Type: t}}, Result: Void}
		}}, true
	case "recv":
		return methodSpec{Name: "recv", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: u.OptionOf(t)}
		}}, true
	case "close":
		return methodSpec{Name: "close", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Void}
		}}, true
	case "is_closed":
		return methodSpec{Name: "is_closed", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Bool}
		}}, true
	}
	return methodSpec{}, false
}

// --- future ----------------------------------------------------------------

var futureNames = []string{"is_done", "cancel"}

func futureMethod(recv *Future, name string) (methodSpec, bool) {
	switch name {
	case "is_done":
		return methodSpec{Name: "is_done", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Bool}
		}}, true
	case "cancel":
		return methodSpec{Name: "cancel", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Void}
		}}, true
	}
	return methodSpec{}, false
}
