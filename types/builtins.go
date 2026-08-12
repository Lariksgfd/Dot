package types

// BuiltinID identifies a builtin function that needs special checking rules
// (variadic any, a type argument, a diverging result, ...).
type BuiltinID int

const (
	BuiltinNone BuiltinID = iota
	BuiltinPrint
	BuiltinPrintln
	BuiltinEprint
	BuiltinInput
	BuiltinLen
	BuiltinTypeOf
	BuiltinAssert
	BuiltinPanic // result type is Never
)

// Universe holds the predeclared scope plus the builtin named types the
// checker needs by direct reference.
type Universe struct {
	Scope   *Scope
	Option  *Named // generic Option[T]
	Result  *Named // generic Result[T, E]
	Error   *Named // the builtin Error struct
	Channel *Named // generic Channel[T]
	Arena   *Named
}

// NewUniverse builds a fresh universe scope. It is called once per Check so
// that no state leaks between compilations.
func NewUniverse() *Universe {
	u := &Universe{}
	u.Scope = NewScope(nil, ScopeUniverse)

	// Primitive type symbols (D49): one SymType per canonical *Basic.
	for _, t := range []struct {
		name string
		typ  Type
	}{
		{"bool", Bool},
		{"int", Int},
		{"int8", Int8},
		{"int16", Int16},
		{"int32", Int32},
		{"uint", Uint},
		{"uint8", Uint8},
		{"uint16", Uint16},
		{"uint32", Uint32},
		{"byte", Byte},
		{"float", Float},
		{"float32", Float32},
		{"rune", Rune},
		{"string", String_},
	} {
		u.insertType(t.name, t.typ)
	}

	// Generic Option[T] (Some/None) as an ordinary generic enum (D55).
	optionT := &TypeParam{Name: "T", Index: 0}
	u.Option = &Named{
		Name:       "Option",
		TypeParams: []*TypeParam{optionT},
		Underlying: &Enum{
			Variants: []Variant{
				{Name: "Some", Fields: []Param{{Name: "value", Type: optionT}}, HasParens: true},
				{Name: "None"},
			},
		},
	}
	u.Option.Underlying.(*Enum).byName = variantByName(u.Option.Underlying.(*Enum))
	u.insertNamed(u.Option)

	// Generic Result[T, E] (Ok/Err) as an ordinary generic enum (D55).
	resultT := &TypeParam{Name: "T", Index: 0}
	resultE := &TypeParam{Name: "E", Index: 1}
	u.Result = &Named{
		Name:       "Result",
		TypeParams: []*TypeParam{resultT, resultE},
		Underlying: &Enum{
			Variants: []Variant{
				{Name: "Ok", Fields: []Param{{Name: "value", Type: resultT}}, HasParens: true},
				{Name: "Err", Fields: []Param{{Name: "error", Type: resultE}}, HasParens: true},
			},
		},
	}
	u.Result.Underlying.(*Enum).byName = variantByName(u.Result.Underlying.(*Enum))
	u.insertNamed(u.Result)

	// Error: a builtin struct with a single `message string` field (D55).
	errorStruct := &Struct{
		Fields: []Field{{Name: "message", Type: String_, Index: 0}},
	}
	errorStruct.Flat, errorStruct.Ambiguous = flattenFields(errorStruct)
	u.Error = &Named{Name: "Error", Underlying: errorStruct}
	u.insertNamed(u.Error)

	// Generic Channel[T].
	channelT := &TypeParam{Name: "T", Index: 0}
	u.Channel = &Named{
		Name:       "Channel",
		TypeParams: []*TypeParam{channelT},
	}
	u.insertNamed(u.Channel)

	// Arena: an opaque builtin type (D68).
	u.Arena = &Named{Name: "Arena"}
	u.insertNamed(u.Arena)

	// Builtin functions (SPEC §17).
	u.insertBuiltin("print", &Fn{
		Params:   []Param{{Name: "args", Type: Void, Variadic: true}},
		Result:   Void,
		Variadic: true,
	}, BuiltinPrint)
	u.insertBuiltin("println", &Fn{
		Params:   []Param{{Name: "args", Type: Void, Variadic: true}},
		Result:   Void,
		Variadic: true,
	}, BuiltinPrintln)
	u.insertBuiltin("eprint", &Fn{
		Params:   []Param{{Name: "args", Type: Void, Variadic: true}},
		Result:   Void,
		Variadic: true,
	}, BuiltinEprint)
	u.insertBuiltin("input", &Fn{
		Params: []Param{{Name: "prompt", Type: String_}},
		Result: String_,
	}, BuiltinInput)
	u.insertBuiltin("len", &Fn{
		Params: []Param{{Name: "collection", Type: Void}},
		Result: Int,
	}, BuiltinLen)
	u.insertBuiltin("type_of", &Fn{
		Params: []Param{{Name: "value", Type: Void}},
		Result: String_,
	}, BuiltinTypeOf)
	u.insertBuiltin("assert", &Fn{
		Params: []Param{
			{Name: "condition", Type: Bool},
			{Name: "msg", Type: String_, HasDflt: true},
		},
		Result: Void,
	}, BuiltinAssert)
	u.insertBuiltin("panic", &Fn{
		Params: []Param{{Name: "msg", Type: String_}},
		Result: Never,
	}, BuiltinPanic)

	return u
}

// insertType registers a primitive type symbol in the universe scope.
func (u *Universe) insertType(name string, typ Type) {
	sym := &Symbol{Kind: SymType, Name: name, Type: typ}
	u.Scope.Insert(sym)
}

// insertNamed registers a named type and its variant constructors (if any) in
// the universe scope.
func (u *Universe) insertNamed(n *Named) {
	sym := &Symbol{
		Kind:  SymType,
		Name:  n.Name,
		Type:  n,
		Named: n,
	}
	u.Scope.Insert(sym)
	n.Sym = sym
	if e, ok := n.Underlying.(*Enum); ok {
		for i := range e.Variants {
			v := &e.Variants[i]
			v.Tag = i
			u.Scope.Insert(&Symbol{
				Kind:    SymVariant,
				Name:    v.Name,
				Type:    n,
				Variant: v,
				Owner:   n,
			})
		}
	}
}

// insertBuiltin registers a builtin function symbol.
func (u *Universe) insertBuiltin(name string, sig *Fn, id BuiltinID) {
	sym := &Symbol{
		Kind:    SymBuiltin,
		Name:    name,
		Type:    sig,
		Fn:      sig,
		Builtin: id,
	}
	u.Scope.Insert(sym)
}

// variantByName builds the byName lookup table for an enum.
func variantByName(e *Enum) map[string]*Variant {
	m := make(map[string]*Variant, len(e.Variants))
	for i := range e.Variants {
		m[e.Variants[i].Name] = &e.Variants[i]
	}
	return m
}

// OptionOf returns Option[T] instantiated.
func (u *Universe) OptionOf(t Type) Type {
	e := &Enum{
		Variants: []Variant{
			{Name: "Some", Fields: []Param{{Name: "value", Type: t}}, HasParens: true},
			{Name: "None"},
		},
	}
	e.byName = variantByName(e)
	return &Named{
		Name:       "Option",
		Origin:     u.Option,
		TypeArgs:   []Type{t},
		Underlying: e,
	}
}

// ResultOf returns Result[T, E] instantiated.
func (u *Universe) ResultOf(t, e Type) Type {
	enum := &Enum{
		Variants: []Variant{
			{Name: "Ok", Fields: []Param{{Name: "value", Type: t}}, HasParens: true},
			{Name: "Err", Fields: []Param{{Name: "error", Type: e}}, HasParens: true},
		},
	}
	enum.byName = variantByName(enum)
	return &Named{
		Name:       "Result",
		Origin:     u.Result,
		TypeArgs:   []Type{t, e},
		Underlying: enum,
	}
}

// IsOption reports whether t is an instantiation of Option and returns its
// element type.
func (u *Universe) IsOption(t Type) (elem Type, ok bool) {
	n, ok := t.(*Named)
	if !ok || n.Origin != u.Option {
		return nil, false
	}
	if len(n.TypeArgs) != 1 {
		return nil, false
	}
	return n.TypeArgs[0], true
}

// IsResult reports whether t is an instantiation of Result and returns its
// Ok and Err types.
func (u *Universe) IsResult(t Type) (ok_ Type, err Type, isOk bool) {
	n, ok := t.(*Named)
	if !ok || n.Origin != u.Result {
		return nil, nil, false
	}
	if len(n.TypeArgs) != 2 {
		return nil, nil, false
	}
	return n.TypeArgs[0], n.TypeArgs[1], true
}
