package types

// --- slice -----------------------------------------------------------------

var sliceNames = []string{
	"len", "push", "pop", "remove_last", "insert", "remove", "clear",
	"contains", "find", "index_of", "map", "filter", "reduce", "sort",
	"sort_by", "reverse", "first", "last", "slice", "join", "is_empty",
}

func sliceMethod(recv *Slice, name string) (methodSpec, bool) {
	t := recv.Elem
	switch name {
	case "len":
		return methodSpec{Name: "len", Field: true, Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Int}
		}}, true
	case "push":
		return methodSpec{Name: "push", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "value", Type: t}}, Result: Void}
		}}, true
	case "pop":
		return methodSpec{Name: "pop", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: u.OptionOf(t)}
		}}, true
	case "remove_last":
		return methodSpec{Name: "remove_last", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: t}
		}}, true
	case "insert":
		return methodSpec{Name: "insert", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "i", Type: Int}, {Name: "value", Type: t}}, Result: Void}
		}}, true
	case "remove":
		return methodSpec{Name: "remove", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "i", Type: Int}}, Result: t}
		}}, true
	case "clear":
		return methodSpec{Name: "clear", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Void}
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
	case "sort":
		return methodSpec{Name: "sort", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Void}
		}}, true
	case "sort_by":
		return methodSpec{Name: "sort_by", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "cmp", Type: cmpFn(t)}}, Result: Void}
		}}, true
	case "reverse":
		return methodSpec{Name: "reverse", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Void}
		}}, true
	case "first":
		return methodSpec{Name: "first", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: u.OptionOf(t)}
		}}, true
	case "last":
		return methodSpec{Name: "last", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: u.OptionOf(t)}
		}}, true
	case "slice":
		return methodSpec{Name: "slice", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "i", Type: Int}, {Name: "j", Type: Int}}, Result: &Slice{Elem: t}}
		}}, true
	case "join":
		return methodSpec{Name: "join", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "sep", Type: String_}}, Result: String_}
		}}, true
	case "is_empty":
		return methodSpec{Name: "is_empty", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Bool}
		}}, true
	}
	return methodSpec{}, false
}
