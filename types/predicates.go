package types

// Underlying unwraps a *Named to its underlying type; every other type is
// returned unchanged. It also follows a resolved *TypeVar.
func Underlying(t Type) Type {
	for {
		switch x := t.(type) {
		case *Named:
			if x.Underlying == nil {
				return x
			}
			t = x.Underlying
			continue
		case *TypeVar:
			if x.Bound == nil {
				return x
			}
			t = x.Bound
			continue
		}
		return t
	}
}

// Identical reports whether a and b are the same type.
// Nominal for Named/TypeParam/Trait (pointer identity plus identical type
// arguments); structural for every other type (D48).
func Identical(a, b Type) bool {
	if a == nil || b == nil {
		return false
	}
	// A solved inference variable stands for the type it was bound to.
	a, b = resolve(a), resolve(b)
	if a == b {
		return true
	}
	if a.Kind() != b.Kind() {
		return false
	}
	switch x := a.(type) {
	case *Basic:
		return a == b // pointer equality — basics are singletons
	case *Slice:
		y := b.(*Slice)
		return Identical(x.Elem, y.Elem)
	case *Array:
		y := b.(*Array)
		return x.Len == y.Len && Identical(x.Elem, y.Elem)
	case *Map:
		y := b.(*Map)
		return Identical(x.Key, y.Key) && Identical(x.Value, y.Value)
	case *Tuple:
		y := b.(*Tuple)
		if len(x.Elems) != len(y.Elems) {
			return false
		}
		for i := range x.Elems {
			if !Identical(x.Elems[i], y.Elems[i]) {
				return false
			}
		}
		return true
	case *Fn:
		y := b.(*Fn)
		if len(x.Params) != len(y.Params) {
			return false
		}
		for i := range x.Params {
			if !Identical(x.Params[i].Type, y.Params[i].Type) {
				return false
			}
		}
		return Identical(x.Result, y.Result)
	case *Pointer:
		y := b.(*Pointer)
		return Identical(x.Elem, y.Elem)
	case *Dyn:
		y := b.(*Dyn)
		return x.Trait == y.Trait // pointer identity on the trait
	case *Weak:
		y := b.(*Weak)
		return Identical(x.Elem, y.Elem)
	case *Chan:
		y := b.(*Chan)
		return Identical(x.Elem, y.Elem)
	case *Future:
		y := b.(*Future)
		return Identical(x.Result, y.Result)
	case *Struct:
		y := b.(*Struct)
		if len(x.Fields) != len(y.Fields) {
			return false
		}
		for i := range x.Fields {
			if x.Fields[i].Name != y.Fields[i].Name {
				return false
			}
			if !Identical(x.Fields[i].Type, y.Fields[i].Type) {
				return false
			}
		}
		return true
	case *Enum:
		// Nominal: identity is pointer identity, but structurally compare
		// variants so two enum literals of the same shape compare equal.
		y := b.(*Enum)
		if len(x.Variants) != len(y.Variants) {
			return false
		}
		for i := range x.Variants {
			if x.Variants[i].Name != y.Variants[i].Name {
				return false
			}
		}
		return true
	case *Named, *TypeParam, *Trait:
		// Nominal: pointer identity (already handled at the top for ==).
		return false
	case *TypeVar:
		return false // pointer identity only
	}
	return false
}

// AssignableTo reports whether a value of type src may be assigned to a
// location of type dst. There is NO implicit numeric widening (D51).
func AssignableTo(src, dst Type) bool {
	if src == nil || dst == nil {
		return false
	}
	// Poison and divergence.
	if IsInvalid(src) || IsInvalid(dst) {
		return true
	}
	if IsNever(src) {
		return true
	}
	if Identical(src, dst) {
		return true
	}
	// Two named types with the same origin are assignable when their type
	// arguments are assignable element-wise. This covers Option[?T] vs
	// Option[User] once ?T has been bound, and generic instantiations that
	// share an Origin but are different pointers.
	if n, ok := src.(*Named); ok {
		if m, ok := dst.(*Named); ok && sameOrigin(n, m) && len(n.TypeArgs) == len(m.TypeArgs) {
			for i := range n.TypeArgs {
				if !AssignableTo(resolve(n.TypeArgs[i]), resolve(m.TypeArgs[i])) {
					return false
				}
			}
			return true
		}
	}
	// Untyped literal flexibility (D51 rule 4).
	if IsUntyped(src) {
		return untypedAssignableTo(src, dst)
	}
	// dst is dyn Tr and src implements Tr (D51 rule 5).
	if d, ok := dst.(*Dyn); ok {
		return implements(src, d.Trait)
	}
	return false
}

// untypedAssignableTo checks whether an untyped literal of kind src's kind is
// representable in dst.
func untypedAssignableTo(src, dst Type) bool {
	switch src.Kind() {
	case KindUntypedInt:
		return IsInteger(dst) || IsFloat(dst)
	case KindUntypedFloat:
		return IsFloat(dst)
	case KindUntypedBool:
		return dst.Kind() == KindBool
	case KindUntypedString:
		return dst.Kind() == KindString
	case KindUntypedNil:
		switch dst.(type) {
		case *Pointer, *Weak, *Dyn:
			return true
		}
		// nil -> Option[T] (None).
		if n, ok := Underlying(dst).(*Named); ok && len(n.TypeArgs) > 0 {
			// Heuristic: any single-arg generic named type may be Option;
			// the real check is Universe.IsOption, which lives in builtins.
			return true
		}
		return false
	}
	return false
}

// implements reports whether t implements trait tr. In v0.1 this is a
// structural/trait-impl lookup; the full table lives with the checker. Here we
// handle the dyn coercion entry point conservatively: a *Named implements tr
// when its Impls list contains it.
func implements(t Type, tr *Trait) bool {
	if t == nil || tr == nil {
		return false
	}
	if t == tr {
		return true
	}
	if n, ok := t.(*Named); ok {
		for _, impl := range n.Impls {
			if impl.Trait == tr {
				return true
			}
		}
	}
	// A *TypeParam implements tr when one of its bounds is tr.
	if tp, ok := t.(*TypeParam); ok {
		for _, b := range tp.Bounds {
			if b == tr {
				return true
			}
		}
	}
	return false
}

// ConvertibleTo reports whether `expr as dst` is legal for a source type src
// (D55). It is deliberately wider than AssignableTo: any numeric ↔ any
// numeric; rune ↔ int32/uint32/int; string ↔ []byte; T → dyn Tr when T
// implements Tr. bool converts to nothing.
func ConvertibleTo(src, dst Type) bool {
	if src == nil || dst == nil {
		return false
	}
	if IsInvalid(src) || IsInvalid(dst) {
		return true
	}
	if Identical(src, dst) {
		return true
	}
	// Untyped literals are convertible to their default concrete type and to
	// any numeric type.
	if IsUntyped(src) {
		return untypedConvertibleTo(src, dst)
	}
	// Any numeric ↔ any numeric (D55).
	if IsNumeric(src) && IsNumeric(dst) {
		return true
	}
	// string ↔ []byte.
	if IsStringType(src) && isByteSlice(dst) {
		return true
	}
	if isByteSlice(src) && IsStringType(dst) {
		return true
	}
	// T → dyn Tr when T implements Tr.
	if d, ok := dst.(*Dyn); ok {
		return implements(src, d.Trait)
	}
	return false
}

// untypedConvertibleTo reports whether an untyped literal src may be converted
// to dst via `as`.
func untypedConvertibleTo(src, dst Type) bool {
	switch src.Kind() {
	case KindUntypedInt, KindUntypedFloat:
		return IsInteger(dst) || IsFloat(dst)
	case KindUntypedBool:
		return dst.Kind() == KindBool
	case KindUntypedString:
		return dst.Kind() == KindString || isByteSlice(dst)
	case KindUntypedNil:
		switch dst.(type) {
		case *Pointer, *Weak, *Dyn:
			return true
		}
		return false
	}
	return false
}

// isByteSlice reports whether t is []byte / []uint8.
func isByteSlice(t Type) bool {
	if s, ok := t.(*Slice); ok {
		if s.Elem == Byte || s.Elem == Uint8 {
			return true
		}
	}
	return false
}

// Comparable reports whether `==` / `!=` is defined on t. It is the static
// property; see also the structural recursion over composite types handled by
// the checker.
func Comparable(t Type) bool {
	if t == nil {
		return false
	}
	if IsComparable(t) {
		return true
	}
	switch t.(type) {
	case *Tuple:
		// Elementwise; the checker recurses, the static answer is optimistic.
		return true
	case *Struct:
		return true
	case *Pointer, *Dyn:
		return true
	}
	return false
}

// Ordered reports whether `<` `>` `<=` `>=` are defined on t.
func Ordered(t Type) bool {
	if t == nil {
		return false
	}
	return IsOrdered(t)
}

// Default returns the type an untyped value takes on when no context
// constrains it: untyped int -> int, untyped float -> float,
// untyped bool -> bool, untyped string -> string, nil -> Invalid (D51).
// Any other type is returned unchanged.
func Default(t Type) Type {
	if t == nil {
		return Invalid
	}
	switch t.Kind() {
	case KindUntypedInt:
		return Int
	case KindUntypedFloat:
		return Float
	case KindUntypedBool:
		return Bool
	case KindUntypedString:
		return String_
	case KindUntypedNil:
		return Invalid
	}
	return t
}

  // IsHeap reports whether values of t are heap allocated and therefore
  // ARC-managed: string, slice, map, enum with payload, dyn, chan, future (D67).
  // Structs are value types in Dot — NOT heap-allocated.
  func IsHeap(t Type) bool {
  	if t == nil {
  		return false
  	}
  	switch x := Underlying(t).(type) {
  	case *Basic:
  		return t.Kind() == KindString
  	case *Slice, *Map, *Dyn, *Chan, *Future:
  		return true
  	case *Named:
  		return IsHeap(x.Underlying)
  	case *Struct:
  		return false
	case *Enum:
		// Any variant with a payload makes the enum heap-allocated.
		for _, v := range x.Variants {
			if len(v.Fields) > 0 {
				return true
			}
		}
		return false
	case *Tuple:
		for _, e := range x.Elems {
			if IsHeap(e) {
				return true
			}
		}
		return false
	}
	return false
}
