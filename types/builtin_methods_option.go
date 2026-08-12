package types

// --- option ----------------------------------------------------------------

var optionNames = []string{
	"unwrap", "unwrap_or", "unwrap_or_else", "is_some", "is_none",
	"map", "and_then", "ok_or", "filter",
}

func optionMethod(t Type, name string) (methodSpec, bool) {
	switch name {
	case "unwrap":
		return methodSpec{Name: "unwrap", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: t}
		}}, true
	case "unwrap_or":
		return methodSpec{Name: "unwrap_or", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "dflt", Type: t}}, Result: t}
		}}, true
	case "unwrap_or_else":
		return methodSpec{Name: "unwrap_or_else", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "f", Type: fnOf(t)}}, Result: t}
		}}, true
	case "is_some":
		return methodSpec{Name: "is_some", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Bool}
		}}, true
	case "is_none":
		return methodSpec{Name: "is_none", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Bool}
		}}, true
	case "map":
		return methodSpec{Name: "map", Build: func(u *Universe, r Type) *Fn {
			f, up := mapFn(t)
			return &Fn{Params: []Param{{Name: "f", Type: f}}, Result: u.OptionOf(up), TypeParams: []*TypeParam{up}}
		}}, true
	case "and_then":
		return methodSpec{Name: "and_then", Build: func(u *Universe, r Type) *Fn {
			up := &TypeParam{Name: "U", Index: 0}
			return &Fn{Params: []Param{{Name: "f", Type: fnOf(u.OptionOf(up), t)}}, Result: u.OptionOf(up), TypeParams: []*TypeParam{up}}
		}}, true
	case "ok_or":
		return methodSpec{Name: "ok_or", Build: func(u *Universe, r Type) *Fn {
			ep := &TypeParam{Name: "E", Index: 0}
			return &Fn{Params: []Param{{Name: "e", Type: ep}}, Result: u.ResultOf(t, ep), TypeParams: []*TypeParam{ep}}
		}}, true
	case "filter":
		return methodSpec{Name: "filter", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "pred", Type: predFn(t)}}, Result: u.OptionOf(t)}
		}}, true
	}
	return methodSpec{}, false
}

// --- result ----------------------------------------------------------------

var resultNames = []string{
	"unwrap", "unwrap_or", "unwrap_err", "is_ok", "is_err",
	"map", "map_err", "and_then", "ok", "err",
}

func resultMethod(t Type, e Type, name string) (methodSpec, bool) {
	switch name {
	case "unwrap":
		return methodSpec{Name: "unwrap", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: t}
		}}, true
	case "unwrap_or":
		return methodSpec{Name: "unwrap_or", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Params: []Param{{Name: "dflt", Type: t}}, Result: t}
		}}, true
	case "unwrap_err":
		return methodSpec{Name: "unwrap_err", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: e}
		}}, true
	case "is_ok":
		return methodSpec{Name: "is_ok", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Bool}
		}}, true
	case "is_err":
		return methodSpec{Name: "is_err", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: Bool}
		}}, true
	case "map":
		return methodSpec{Name: "map", Build: func(u *Universe, r Type) *Fn {
			f, up := mapFn(t)
			return &Fn{Params: []Param{{Name: "f", Type: f}}, Result: u.ResultOf(up, e), TypeParams: []*TypeParam{up}}
		}}, true
	case "map_err":
		return methodSpec{Name: "map_err", Build: func(u *Universe, r Type) *Fn {
			ep := &TypeParam{Name: "F", Index: 0}
			return &Fn{Params: []Param{{Name: "f", Type: fnOf(ep, e)}}, Result: u.ResultOf(t, ep), TypeParams: []*TypeParam{ep}}
		}}, true
	case "and_then":
		return methodSpec{Name: "and_then", Build: func(u *Universe, r Type) *Fn {
			up := &TypeParam{Name: "U", Index: 0}
			return &Fn{Params: []Param{{Name: "f", Type: fnOf(u.ResultOf(up, e), t)}}, Result: u.ResultOf(up, e), TypeParams: []*TypeParam{up}}
		}}, true
	case "ok":
		return methodSpec{Name: "ok", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: u.OptionOf(t)}
		}}, true
	case "err":
		return methodSpec{Name: "err", Build: func(u *Universe, r Type) *Fn {
			return &Fn{Result: u.OptionOf(e)}
		}}, true
	}
	return methodSpec{}, false
}
