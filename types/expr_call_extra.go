package types

import (
	"github.com/dotlang/dot/ast"
)

// checkIndex types a[i] and generic instantiation Stack[int].
func (c *Checker) checkIndex(x *ast.IndexExpr) Type {
	if t, ok := c.tryTypeInstantiation(x); ok {
		return t
	}
	recv := c.checkExpr(x.X)
	if len(x.Indices) != 1 {
		c.errorf(x, "expected exactly one index, found %d", len(x.Indices))
		for _, i := range x.Indices {
			c.checkExpr(i)
		}
		return Invalid
	}
	switch u := Underlying(recv).(type) {
	case *Slice:
		c.requireIntIndex(x.Indices[0])
		return u.Elem
	case *Array:
		c.requireIntIndex(x.Indices[0])
		return u.Elem
	case *Map:
		key := c.checkExprExpect(x.Indices[0], u.Key)
		c.assignCompatible(x.Indices[0], key, u.Key, "map key")
		// A map lookup yields Option[V] so a missing key must be handled.
		return c.univ.OptionOf(u.Value)
	case *Basic:
		if IsStringType(u) {
			c.requireIntIndex(x.Indices[0])
			return Rune
		}
	}
	if IsInvalid(recv) {
		return Invalid
	}
	d := c.errorf(x, "cannot index %s", recv)
	c.hint(d, "indexing works on slices, arrays, maps and strings")
	return Invalid
}

// tryTypeInstantiation recognises Name[T] used as a generic instantiation
// rather than an index expression.
func (c *Checker) tryTypeInstantiation(x *ast.IndexExpr) (Type, bool) {
	id, ok := x.X.(*ast.Ident)
	if !ok {
		return nil, false
	}
	sym, ok := c.scope.Lookup(id.Name)
	if !ok {
		return nil, false
	}
	switch sym.Kind {
	case SymType:
		args := make([]Type, len(x.Indices))
		for i, a := range x.Indices {
			args[i] = c.typeFromExpr(a)
		}
		named := sym.Named
		if named == nil {
			return Invalid, true
		}
		if len(named.TypeParams) != len(args) {
			c.errorf(x, "%q expects %d type argument(s), got %d",
				id.Name, len(named.TypeParams), len(args))
			return Invalid, true
		}
		switch named {
		case c.univ.Option:
			return c.univ.OptionOf(args[0]), true
		case c.univ.Result:
			return c.univ.ResultOf(args[0], args[1]), true
		case c.univ.Channel:
			return NewChan(args[0]), true
		}
		return c.getOrAddCache(named, args), true
	case SymFunc:
		if sym.Fn != nil && len(sym.Fn.TypeParams) > 0 {
			for _, a := range x.Indices {
				c.typeFromExpr(a)
			}
			return sym.Fn, true
		}
	}
	return nil, false
}

// typeFromExpr reinterprets an expression in type-argument position as a type.
func (c *Checker) typeFromExpr(e ast.Expr) Type {
	switch x := e.(type) {
	case *ast.Ident:
		sym, ok := c.scope.Lookup(x.Name)
		if !ok {
			c.errorf(x, "undefined type %q", x.Name)
			return Invalid
		}
		if sym.Kind != SymType && sym.Kind != SymTypeParam {
			c.errorf(x, "%q is not a type", x.Name)
			return Invalid
		}
		return sym.Type
	case *ast.IndexExpr:
		t, ok := c.tryTypeInstantiation(x)
		if ok {
			return t
		}
	}
	c.errorf(e, "expected a type argument")
	return Invalid
}

// requireIntIndex reports a non-integer index.
func (c *Checker) requireIntIndex(e ast.Expr) {
	t := c.checkExprExpect(e, Int)
	if IsInvalid(t) {
		return
	}
	if !IsInteger(Underlying(t)) {
		c.errorf(e, "index must be an integer, found %s", t)
	}
}

// checkSlice types a[lo..hi].
func (c *Checker) checkSlice(x *ast.SliceExpr) Type {
	recv := c.checkExpr(x.X)
	if x.Low != nil {
		c.requireIntIndex(x.Low)
	}
	if x.High != nil {
		c.requireIntIndex(x.High)
	}
	switch u := Underlying(recv).(type) {
	case *Slice:
		return u
	case *Array:
		return NewSlice(u.Elem)
	case *Basic:
		if IsStringType(u) {
			return String_
		}
	}
	if IsInvalid(recv) {
		return Invalid
	}
	c.errorf(x, "cannot slice %s", recv)
	return Invalid
}

// checkField types a.b, tuple indexing t.0, enum variants Color.Red and
// method values.
func (c *Checker) checkField(x *ast.FieldExpr) Type {
	if t, ok := c.tryEnumVariant(x); ok {
		return t
	}
	recv := c.checkExpr(x.X)
	if IsInvalid(recv) {
		return Invalid
	}

	if x.IsTupleIndex {
		return c.tupleElement(x, recv)
	}
	if t, ok := c.structField(x, recv); ok {
		return t
	}
	if t, ok := c.methodValue(x, recv); ok {
		return t
	}
	if fn, isField, ok := lookupBuiltinMethodField(c.univ, recv, x.Name); ok {
		kind := SelectBuiltinMethod
		if isField {
			kind = SelectBuiltinProp
		}
		c.info.Selections[x] = &Selection{Recv: recv, Kind: kind, Sig: fn}
		if kind == SelectBuiltinProp {
			return fn.Result
		}
		return fn
	}

	d := c.errorf(x, "%s has no member %q", recv, x.Name)
	if names := builtinMethodNames(recv); len(names) > 0 {
		c.hint(d, "available members: "+joinStrings(names))
	}
	return Invalid
}

// tryEnumVariant recognises Color.Red and Shape.Circle.
func (c *Checker) tryEnumVariant(x *ast.FieldExpr) (Type, bool) {
	id, ok := x.X.(*ast.Ident)
	if !ok {
		return nil, false
	}
	sym, ok := c.scope.Lookup(id.Name)
	if !ok || sym.Kind != SymType || sym.Named == nil {
		return nil, false
	}
	named := sym.Named
	if e, ok := Underlying(named).(*Enum); ok {
		if v, ok := e.Variant(x.Name); ok {
			c.info.Selections[x] = &Selection{
				Recv: named, Kind: SelectVariant, Variant: v, Owner: named,
			}
			vs := &Symbol{Kind: SymVariant, Name: v.Name, Variant: v, Owner: named}
			return c.variantValue(vs, x), true
		}
	}
	// A static method such as Point.new or Point.origin.
	if named.Methods != nil {
		if m, ok := named.Methods[x.Name]; ok {
			c.info.Selections[x] = &Selection{
				Recv: named, Kind: SelectMethod, Method: m, Sig: m.Sig, Owner: named,
			}
			return m.Sig, true
		}
	}
	return nil, false
}

// tupleElement types t.0.
func (c *Checker) tupleElement(x *ast.FieldExpr, recv Type) Type {
	tup, ok := Underlying(recv).(*Tuple)
	if !ok {
		c.errorf(x, "%s is not a tuple", recv)
		return Invalid
	}
	if x.Index < 0 || x.Index >= len(tup.Elems) {
		c.errorf(x, "tuple index %d out of range: %s has %d element(s)",
			x.Index, recv, len(tup.Elems))
		return Invalid
	}
	c.info.Selections[x] = &Selection{Recv: recv, Kind: SelectTupleIndex}
	return tup.Elems[x.Index]
}

// structField resolves a struct field, following embedded structs.
func (c *Checker) structField(x *ast.FieldExpr, recv Type) (Type, bool) {
	st, ok := Underlying(recv).(*Struct)
	if !ok {
		return nil, false
	}
	if st.Ambiguous[x.Name] {
		d := c.errorf(x, "field %q is ambiguous in %s", x.Name, recv)
		c.hint(d, "it is reachable through more than one embedded struct")
		return Invalid, true
	}
	fp, ok := st.Flat[x.Name]
	if !ok {
		return nil, false
	}
	c.info.Selections[x] = &Selection{
		Recv: recv, Kind: SelectField, Field: fp.Field, Path: fp.Path,
	}
	return fp.Field.Type, true
}

// methodValue resolves an inherent or trait method on a named receiver.
func (c *Checker) methodValue(x *ast.FieldExpr, recv Type) (Type, bool) {
	named, ok := recv.(*Named)
	if !ok {
		return nil, false
	}
	origin := named
	if named.Origin != nil {
		origin = named.Origin
	}
	if origin.Methods != nil {
		if m, ok := origin.Methods[x.Name]; ok {
			c.info.Selections[x] = &Selection{
				Recv: recv, Kind: SelectMethod, Method: m, Sig: m.Sig, Owner: origin,
			}
			return m.Sig, true
		}
	}
	for _, impl := range origin.Impls {
		if m, ok := impl.Methods[x.Name]; ok {
			c.info.Selections[x] = &Selection{
				Recv: recv, Kind: SelectMethod, Method: m, Sig: m.Sig, Owner: origin,
			}
			return m.Sig, true
		}
		if impl.Trait != nil {
			if m, ok := impl.Trait.Method(x.Name); ok && m.HasBody {
				c.info.Selections[x] = &Selection{
					Recv: recv, Kind: SelectMethod, Method: m, Sig: m.Sig, Owner: origin,
				}
				return m.Sig, true
			}
		}
	}
	return nil, false
}

// checkPipe types x |> f(...), which passes x as the first argument of f.
func (c *Checker) checkPipe(x *ast.PipeExpr) Type {
	left := c.checkExpr(x.X)
	fnType := c.checkExpr(x.Fn)
	sig, ok := Underlying(fnType).(*Fn)
	if !ok {
		if IsInvalid(fnType) {
			return Invalid
		}
		d := c.errorf(x, "the right side of '|>' must be a function, found %s", fnType)
		c.hint(d, "write 'value |> f(args)' where f is callable")
		return Invalid
	}
	if len(sig.Params) == 0 {
		c.errorf(x, "cannot pipe into a function taking no parameters")
		return Invalid
	}
	c.assignCompatible(x.X, left, sig.Params[0].Type, "piped value")
	if sig.Result == nil {
		return Void
	}
	return sig.Result
}
