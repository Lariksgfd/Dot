package types

// unify attempts to make a and b the same type by binding inference
// variables, and reports whether it succeeded.
//
// Binding is destructive: a solved *TypeVar keeps its Bound forever, which is
// what lets `Some(x)` learn its element type from the surrounding context
// (D52). Unification never widens: two concrete types must already be
// assignable to each other.
func unify(a, b Type) bool {
	a, b = resolve(a), resolve(b)
	if a == nil || b == nil {
		return false
	}
	if a == b {
		return true
	}
	if IsInvalid(a) || IsInvalid(b) {
		return true
	}

	if v, ok := a.(*TypeVar); ok {
		return bindVar(v, b)
	}
	if v, ok := b.(*TypeVar); ok {
		return bindVar(v, a)
	}

	// An untyped literal takes the shape of whatever it meets.
	if IsUntyped(a) || IsUntyped(b) {
		return AssignableTo(a, b) || AssignableTo(b, a)
	}

	switch x := a.(type) {
	case *Slice:
		y, ok := b.(*Slice)
		return ok && unify(x.Elem, y.Elem)
	case *Array:
		y, ok := b.(*Array)
		return ok && x.Len == y.Len && unify(x.Elem, y.Elem)
	case *Map:
		y, ok := b.(*Map)
		return ok && unify(x.Key, y.Key) && unify(x.Value, y.Value)
	case *Pointer:
		y, ok := b.(*Pointer)
		return ok && unify(x.Elem, y.Elem)
	case *Weak:
		y, ok := b.(*Weak)
		return ok && unify(x.Elem, y.Elem)
	case *Chan:
		y, ok := b.(*Chan)
		return ok && unify(x.Elem, y.Elem)
	case *Future:
		y, ok := b.(*Future)
		return ok && unify(x.Result, y.Result)
	case *Tuple:
		y, ok := b.(*Tuple)
		if !ok || len(x.Elems) != len(y.Elems) {
			return false
		}
		for i := range x.Elems {
			if !unify(x.Elems[i], y.Elems[i]) {
				return false
			}
		}
		return true
	case *Fn:
		y, ok := b.(*Fn)
		if !ok || len(x.Params) != len(y.Params) {
			return false
		}
		for i := range x.Params {
			if !unify(x.Params[i].Type, y.Params[i].Type) {
				return false
			}
		}
		return unify(x.Result, y.Result)
	case *Named:
		y, ok := b.(*Named)
		if !ok || !sameOrigin(x, y) || len(x.TypeArgs) != len(y.TypeArgs) {
			return false
		}
		for i := range x.TypeArgs {
			if !unify(x.TypeArgs[i], y.TypeArgs[i]) {
				return false
			}
		}
		return true
	}
	return Identical(a, b)
}

// bindVar binds an inference variable to t after an occurs check.
func bindVar(v *TypeVar, t Type) bool {
	if v.Bound != nil {
		return unify(v.Bound, t)
	}
	if other, ok := t.(*TypeVar); ok && other == v {
		return true
	}
	if occurs(v, t) {
		return false
	}
	v.Bound = t
	return true
}

// occurs reports whether v appears inside t, which would create an infinite
// type.
func occurs(v *TypeVar, t Type) bool {
	switch x := resolve(t).(type) {
	case *TypeVar:
		return x == v
	case *Slice:
		return occurs(v, x.Elem)
	case *Array:
		return occurs(v, x.Elem)
	case *Map:
		return occurs(v, x.Key) || occurs(v, x.Value)
	case *Pointer:
		return occurs(v, x.Elem)
	case *Weak:
		return occurs(v, x.Elem)
	case *Chan:
		return occurs(v, x.Elem)
	case *Future:
		return occurs(v, x.Result)
	case *Tuple:
		for _, e := range x.Elems {
			if occurs(v, e) {
				return true
			}
		}
	case *Fn:
		for _, p := range x.Params {
			if occurs(v, p.Type) {
				return true
			}
		}
		return occurs(v, x.Result)
	case *Named:
		for _, a := range x.TypeArgs {
			if occurs(v, a) {
				return true
			}
		}
	}
	return false
}

// resolve follows a chain of solved inference variables to the type they
// ultimately stand for.
func resolve(t Type) Type {
	for {
		v, ok := t.(*TypeVar)
		if !ok || v.Bound == nil {
			return t
		}
		t = v.Bound
	}
}

// sameOrigin reports whether two named types come from the same declaration.
func sameOrigin(a, b *Named) bool {
	x, y := a, b
	if x.Origin != nil {
		x = x.Origin
	}
	if y.Origin != nil {
		y = y.Origin
	}
	return x == y
}

// hasUnresolvedVars reports whether t still contains an unbound inference
// variable, which means inference failed to pin the type down.
func hasUnresolvedVars(t Type) bool {
	switch x := resolve(t).(type) {
	case *TypeVar:
		return true
	case *Slice:
		return hasUnresolvedVars(x.Elem)
	case *Array:
		return hasUnresolvedVars(x.Elem)
	case *Map:
		return hasUnresolvedVars(x.Key) || hasUnresolvedVars(x.Value)
	case *Tuple:
		for _, e := range x.Elems {
			if hasUnresolvedVars(e) {
				return true
			}
		}
	case *Named:
		for _, a := range x.TypeArgs {
			if hasUnresolvedVars(a) {
				return true
			}
		}
	}
	return false
}
