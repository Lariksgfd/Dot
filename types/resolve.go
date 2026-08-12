package types

import (
	"github.com/dotlang/dot/ast"
)

// resolveType turns a syntactic type expression into a semantic type.
//
// It never returns nil: an unresolvable type yields Invalid after a diagnostic
// has been recorded, so checking can continue.
func (c *Checker) resolveType(expr ast.Type) Type {
	if expr == nil {
		return Invalid
	}
	switch t := expr.(type) {
	case *ast.NamedType:
		return c.resolveNamed(t)
	case *ast.GenericType:
		return c.resolveGeneric(t)
	case *ast.SliceType:
		return NewSlice(c.resolveType(t.Elem))
	case *ast.ArrayType:
		return c.resolveArray(t)
	case *ast.MapType:
		return c.resolveMap(t)
	case *ast.FnType:
		return c.resolveFnType(t)
	case *ast.TupleType:
		elems := make([]Type, len(t.Elems))
		for i, e := range t.Elems {
			elems[i] = c.resolveType(e)
		}
		return NewTuple(elems...)
	case *ast.PointerType:
		return NewPointer(c.resolveType(t.Elem))
	case *ast.DynType:
		return c.resolveDyn(t)
	case *ast.WeakType:
		return NewWeak(c.resolveType(t.Elem))
	case *ast.OptionalType:
		// `T?` is sugar for Option[T] (D57).
		elem := c.resolveType(t.Elem)
		if IsInvalid(elem) {
			return Invalid
		}
		return c.univ.OptionOf(elem)
	case *ast.SelfTypeNode:
		if c.selfType == nil {
			d := c.errorf(t, "Self is only valid inside a trait or impl block")
			c.hint(d, "use the concrete type name here")
			return Invalid
		}
		return c.selfType
	case *ast.BadType:
		return Invalid
	}
	c.errorf(expr, "unsupported type expression %s", ast.NodeName(expr))
	return Invalid
}

// resolveNamed resolves `int`, `User`, `Self` and the qualified `pkg.T` form.
func (c *Checker) resolveNamed(t *ast.NamedType) Type {
	if t.Pkg != "" {
		d := c.errorf(t, "qualified type %s.%s cannot be resolved: modules are not implemented yet", t.Pkg, t.Name)
		c.hint(d, "cross-module type resolution arrives with the module loader")
		return Invalid
	}
	if t.Name == "Self" {
		if c.selfType == nil {
			c.errorf(t, "Self is only valid inside a trait or impl block")
			return Invalid
		}
		return c.selfType
	}

	sym, ok := c.scope.Lookup(t.Name)
	if !ok {
		d := c.errorf(t, "undefined type %q", t.Name)
		c.hint(d, "check the spelling, or declare the type before using it")
		return Invalid
	}
	switch sym.Kind {
	case SymType:
		if sym.Named != nil && len(sym.Named.TypeParams) > 0 {
			d := c.errorf(t, "generic type %q requires type arguments", t.Name)
			c.hint(d, "write "+t.Name+"[...] with one argument per parameter")
			return Invalid
		}
		return sym.Type
	case SymTypeParam:
		return sym.Type
	case SymTrait:
		d := c.errorf(t, "%q is a trait and cannot be used as a value type", t.Name)
		c.hint(d, "write 'dyn "+t.Name+"' for dynamic dispatch, or use it as a bound")
		return Invalid
	}
	d := c.errorf(t, "%q is a %s, not a type", t.Name, sym.Kind)
	c.hint(d, "only struct, enum and type parameter names may appear here")
	return Invalid
}

// resolveGeneric resolves `Option[int]`, `Result[int, Error]`, `Stack[T]` and
// the builtin `Channel[T]`.
func (c *Checker) resolveGeneric(t *ast.GenericType) Type {
	base, ok := t.Base.(*ast.NamedType)
	if !ok {
		c.errorf(t, "only a named type may take type arguments")
		return Invalid
	}
	args := make([]Type, len(t.Args))
	bad := false
	for i, a := range t.Args {
		args[i] = c.resolveType(a)
		if IsInvalid(args[i]) {
			bad = true
		}
	}
	if bad {
		return Invalid
	}

	sym, ok := c.scope.Lookup(base.Name)
	if !ok {
		c.errorf(t, "undefined type %q", base.Name)
		return Invalid
	}
	if sym.Kind != SymType || sym.Named == nil {
		c.errorf(t, "%q is not a generic type", base.Name)
		return Invalid
	}
	named := sym.Named

	if want := len(named.TypeParams); want != len(args) {
		d := c.errorf(t, "%q expects %d type argument(s), got %d", base.Name, want, len(args))
		c.hint(d, "type arguments are matched positionally")
		return Invalid
	}

	switch named {
	case c.univ.Option:
		return c.univ.OptionOf(args[0])
	case c.univ.Result:
		return c.univ.ResultOf(args[0], args[1])
	case c.univ.Channel:
		return NewChan(args[0])
	}
	return c.getOrAddCache(named, args)
}

// instantiate substitutes type arguments into a generic named type.
//
// Bounds are checked against every declared type parameter, and the result is
// cached by (origin pointer, TypeArgs) so repeated instantiations return the
// same *Named pointer.
func (c *Checker) instantiate(named *Named, args []Type) Type {
	for i, tp := range named.TypeParams {
		if i < len(args) {
			checkBounds(tp, args[i], named.Pos)
		}
	}
	subst := make(map[*TypeParam]Type, len(args))
	for i, tp := range named.TypeParams {
		if i < len(args) {
			subst[tp] = args[i]
		}
	}
	inst := &Named{
		Name:     named.Name,
		Origin:   named,
		TypeArgs: args,
		Pos:      named.Pos,
		Sym:      named.Sym,
	}
	if named.Underlying != nil {
		inst.Underlying = substitute(named.Underlying, subst)
	}
	return inst
}

// resolveArray resolves `[N]T`, requiring a constant non-negative length.
func (c *Checker) resolveArray(t *ast.ArrayType) Type {
	elem := c.resolveType(t.Elem)
	n, ok := constIntValue(t.Len)
	if !ok {
		d := c.errorf(t, "array length must be a constant integer")
		c.hint(d, "use a literal such as [5]int, or a slice []int for a dynamic size")
		return Invalid
	}
	if n < 0 {
		c.errorf(t, "array length cannot be negative")
		return Invalid
	}
	return NewArray(elem, n)
}

// resolveMap resolves `map[K]V`, requiring a comparable key type.
func (c *Checker) resolveMap(t *ast.MapType) Type {
	key := c.resolveType(t.Key)
	value := c.resolveType(t.Value)
	if IsInvalid(key) || IsInvalid(value) {
		return Invalid
	}
	if !Comparable(key) {
		d := c.errorf(t.Key, "invalid map key type %s", key)
		c.hint(d, "map keys must be comparable: a primitive, a string or an enum")
		return Invalid
	}
	return NewMap(key, value)
}

// resolveFnType resolves `fn(A, B) -> C`.
func (c *Checker) resolveFnType(t *ast.FnType) Type {
	fn := &Fn{Variadic: t.Variadic}
	for i, p := range t.Params {
		pt := c.resolveType(p)
		variadic := t.Variadic && i == len(t.Params)-1
		fn.Params = append(fn.Params, Param{Type: pt, Variadic: variadic})
	}
	if t.Result != nil {
		fn.Result = c.resolveType(t.Result)
	} else {
		fn.Result = Void
	}
	return fn
}

// resolveDyn resolves `dyn Trait`, checking object safety.
func (c *Checker) resolveDyn(t *ast.DynType) Type {
	named, ok := t.Trait.(*ast.NamedType)
	if !ok {
		c.errorf(t, "'dyn' must be followed by a trait name")
		return Invalid
	}
	sym, ok := c.scope.Lookup(named.Name)
	if !ok {
		c.errorf(t, "undefined trait %q", named.Name)
		return Invalid
	}
	if sym.Kind != SymTrait || sym.Trait == nil {
		d := c.errorf(t, "%q is not a trait", named.Name)
		c.hint(d, "'dyn' may only be applied to a trait")
		return Invalid
	}
	if safe, reason := sym.Trait.ObjectSafe(); !safe {
		d := c.errorf(t, "%s", reason)
		c.hint(d, "use a generic parameter with this trait as a bound instead")
		return Invalid
	}
	if !checkObjectSafe(sym.Trait, t.Pos()) {
		return Invalid
	}
	return NewDyn(sym.Trait)
}

// constIntValue folds a constant integer expression, handling literals and a
// leading unary minus.
func constIntValue(e ast.Expr) (int64, bool) {
	switch x := e.(type) {
	case *ast.IntLit:
		return int64(x.Value), true
	case *ast.ParenExpr:
		return constIntValue(x.X)
	case *ast.UnaryExpr:
		v, ok := constIntValue(x.X)
		if !ok {
			return 0, false
		}
		switch x.OpString() {
		case "-":
			return -v, true
		case "+":
			return v, true
		}
	}
	return 0, false
}

// substitute replaces type parameters in t according to subst.
func substitute(t Type, subst map[*TypeParam]Type) Type {
	if t == nil || len(subst) == 0 {
		return t
	}
	switch x := t.(type) {
	case *TypeParam:
		if r, ok := subst[x]; ok {
			return r
		}
		return x
	case *Slice:
		return &Slice{Elem: substitute(x.Elem, subst)}
	case *Array:
		return &Array{Elem: substitute(x.Elem, subst), Len: x.Len}
	case *Map:
		return &Map{Key: substitute(x.Key, subst), Value: substitute(x.Value, subst)}
	case *Pointer:
		return &Pointer{Elem: substitute(x.Elem, subst)}
	case *Weak:
		return &Weak{Elem: substitute(x.Elem, subst)}
	case *Chan:
		return &Chan{Elem: substitute(x.Elem, subst)}
	case *Future:
		return &Future{Result: substitute(x.Result, subst)}
	case *Tuple:
		elems := make([]Type, len(x.Elems))
		for i, e := range x.Elems {
			elems[i] = substitute(e, subst)
		}
		return &Tuple{Elems: elems}
	case *Fn:
		out := &Fn{
			Result:   substitute(x.Result, subst),
			Recv:     substitute(x.Recv, subst),
			RecvMut:  x.RecvMut,
			Async:    x.Async,
			Variadic: x.Variadic,
		}
		out.Params = make([]Param, len(x.Params))
		for i, p := range x.Params {
			q := p
			q.Type = substitute(p.Type, subst)
			out.Params[i] = q
		}
		return out
	case *Struct:
		out := &Struct{Fields: make([]Field, len(x.Fields))}
		for i, f := range x.Fields {
			g := f
			g.Type = substitute(f.Type, subst)
			out.Fields[i] = g
		}
		out.Flat, out.Ambiguous = flattenFields(out)
		return out
	case *Enum:
		out := &Enum{Variants: make([]Variant, len(x.Variants))}
		for i, v := range x.Variants {
			w := v
			w.Fields = make([]Param, len(v.Fields))
			for j, f := range v.Fields {
				g := f
				g.Type = substitute(f.Type, subst)
				w.Fields[j] = g
			}
			out.Variants[i] = w
		}
		out.byName = variantByName(out)
		return out
	case *Named:
		if len(x.TypeArgs) == 0 && x.Underlying == nil {
			return x
		}
		args := make([]Type, len(x.TypeArgs))
		for i, a := range x.TypeArgs {
			args[i] = substitute(a, subst)
		}
		return &Named{
			Name:       x.Name,
			Sym:        x.Sym,
			Origin:     x.Origin,
			Underlying: substitute(x.Underlying, subst),
			TypeArgs:   args,
			Methods:    x.Methods,
			Impls:      x.Impls,
			Pos:        x.Pos,
		}
	}
	return t
}

// flattenFields builds the flattened field lookup for a struct, following
// embedded structs and marking names reachable by more than one path (D59).
func flattenFields(s *Struct) (map[string]FieldPath, map[string]bool) {
	flat := make(map[string]FieldPath)
	ambiguous := make(map[string]bool)

	var walk func(st *Struct, prefix []int, depth int)
	walk = func(st *Struct, prefix []int, depth int) {
		for i := range st.Fields {
			f := &st.Fields[i]
			path := append(append([]int{}, prefix...), i)
			if f.Name != "" {
				if prev, ok := flat[f.Name]; ok {
					if prev.Depth == depth {
						ambiguous[f.Name] = true
					}
				} else {
					flat[f.Name] = FieldPath{Path: path, Field: f, Depth: depth}
				}
			}
			if f.Embedded {
				if inner, ok := Underlying(f.Type).(*Struct); ok {
					walk(inner, path, depth+1)
				}
			}
		}
	}
	walk(s, nil, 0)
	return flat, ambiguous
}
