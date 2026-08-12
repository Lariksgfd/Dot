package types

import (
	"sort"
)

// methodSpec declares one builtin method in a Go table (D62). Params/Result
// are built lazily by the closure so that they can depend on the receiver's
// element type (e.g. push(T) on []T).
type methodSpec struct {
	Name string
	// Build returns the method's signature given the concrete receiver.
	Build func(u *Universe, recv Type) *Fn
	// Field is true for a property accessed without parentheses (.len).
	Field bool
}

// lookupBuiltinMethod finds a method or property on a builtin type. recv is
// the receiver type; for Option/Result it is the *Named itself (identity is
// needed), for all other types it is the Underlying()-resolved type.
func lookupBuiltinMethod(u *Universe, recv Type, name string) (*Fn, bool) {
	spec, ok := builtinMethodSpec(u, recv, name)
	if !ok {
		return nil, false
	}
	return spec.Build(u, recv), true
}

// lookupBuiltinMethodField returns the builtin method's signature and whether it is a field.
func lookupBuiltinMethodField(u *Universe, recv Type, name string) (*Fn, bool, bool) {
	spec, ok := builtinMethodSpec(u, recv, name)
	if !ok {
		return nil, false, false
	}
	return spec.Build(u, recv), spec.Field, true
}

// builtinMethodNames returns every method name available on recv, sorted -
// used to build did-you-mean hints.
func builtinMethodNames(recv Type) []string {
	var names []string
	if n, ok := recv.(*Named); ok {
		switch len(n.TypeArgs) {
		case 1:
			names = optionNames
		case 2:
			names = resultNames
		}
	} else {
		names = methodNames(recv)
	}
	sort.Strings(names)
	return names
}

// builtinMethodSpec dispatches on receiver shape and returns the spec for
// name, or (zero, false) when no such method exists.
func builtinMethodSpec(u *Universe, recv Type, name string) (methodSpec, bool) {
	switch x := recv.(type) {
	case *Slice:
		return sliceMethod(x, name)
	case *Map:
		return mapMethod(x, name)
	case *Array:
		return arrayMethod(x, name)
	case *Chan:
		return chanMethod(x, name)
	case *Future:
		return futureMethod(x, name)
	case *Named:
		if elem, ok := u.IsOption(x); ok {
			return optionMethod(elem, name)
		}
		if ok_, err, ok := u.IsResult(x); ok {
			return resultMethod(ok_, err, name)
		}
	case *Basic:
		switch {
		case x.Kind() == KindString:
			return stringMethod(name)
		case x.Kind() == KindBool:
			return boolMethod(name)
		case IsNumeric(x):
			return numericMethod(x, name)
		}
	}
	return methodSpec{}, false
}

// methodNames returns all method names for a non-named receiver shape.
func methodNames(recv Type) []string {
	switch recv.(type) {
	case *Slice:
		return sliceNames
	case *Map:
		return mapNames
	case *Array:
		return arrayNames
	case *Chan:
		return chanNames
	case *Future:
		return futureNames
	case *Basic:
		switch recv.(interface{ Kind() Kind }).Kind() {
		case KindString:
			return stringNames
		case KindBool:
			return boolNames
		default:
			if IsNumeric(recv) {
				return numericNames
			}
		}
	}
	return nil
}

// fnOf builds a *Fn with the given param types and result. Each param becomes
// Param{Type: p}.
func fnOf(result Type, params ...Type) *Fn {
	ps := make([]Param, len(params))
	for i, p := range params {
		ps[i] = Param{Type: p}
	}
	return &Fn{Params: ps, Result: result}
}

// predFn builds a predicate fn(T) -> bool.
func predFn(t Type) *Fn { return fnOf(Bool, t) }

// cmpFn builds a comparison fn(T, T) -> int.
func cmpFn(t Type) *Fn { return fnOf(Int, t, t) }

// mapFn builds a mapping fn(T) -> U with a fresh type param U.
func mapFn(t Type) (*Fn, *TypeParam) {
	up := &TypeParam{Name: "U", Index: 0}
	return fnOf(up, t), up
}

// reduceFn builds a reduction fn(U, T) -> U with a fresh type param U.
func reduceFn(t Type) (*Fn, *TypeParam) {
	up := &TypeParam{Name: "U", Index: 0}
	return fnOf(up, up, t), up
}

// mkString wraps a fixed signature as a methodSpec with a no-op Build.
func mkString(name string, sig *Fn) methodSpec {
	return methodSpec{Name: name, Build: func(u *Universe, r Type) *Fn { return sig }}
}
