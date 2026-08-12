package types

import (
	"github.com/dotlang/dot/ast"
)

// checkCall types a function, method, constructor or builtin call.
func (c *Checker) checkCall(x *ast.CallExpr) Type {
	callee := c.checkCallee(x)
	if IsInvalid(callee) {
		for _, a := range x.Args {
			c.checkExpr(a.Value)
		}
		return Invalid
	}
	sig, ok := Underlying(callee).(*Fn)
	if !ok {
		d := c.errorf(x, "cannot call %s", callee)
		c.hint(d, "only functions, methods and enum constructors may be called")
		for _, a := range x.Args {
			c.checkExpr(a.Value)
		}
		return Invalid
	}
	result := c.checkArgs(x, sig)
	if id, ok := x.Fn.(*ast.Ident); ok {
		if sym, ok := c.info.Uses[id]; ok && sym.Kind == SymBuiltin {
			if ci, ok := c.info.Calls[x]; ok {
				ci.Builtin = sym.Builtin
			}
		}
	}
	c.recordGenericCallInstance(x, sig)
	return result
}

// recordGenericCallInstance records an *Instance for a call to a generic
// function, so Phase 5 can monomorphise it.
func (c *Checker) recordGenericCallInstance(x *ast.CallExpr, sig *Fn) {
	if sig == nil || len(sig.TypeParams) == 0 {
		return
	}
	var args []Type
	if idx, ok := x.Fn.(*ast.IndexExpr); ok {
		for _, a := range idx.Indices {
			args = append(args, c.typeFromExpr(a))
		}
	}
	if len(args) == 0 {
		for _, tp := range sig.TypeParams {
			args = append(args, tp)
		}
	}
	base := genericFuncName(x)
	mangled := instanceKey(base, args)
	inst := c.recordInstance(&Instance{
		Generic:  c.genericFuncSymbol(x),
		TypeArgs: args,
		Result:   sig,
		Pos:      x.Pos(),
		Mangled:  mangled,
	})
	if c.info.Instances == nil {
		c.info.Instances = make(map[ast.Expr]*Instance)
	}
	c.info.Instances[x] = inst
}

// genericFuncName returns the base name of the callee for mangling.
func genericFuncName(x *ast.CallExpr) string {
	switch f := x.Fn.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.FieldExpr:
		return f.Name
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return "fn"
}

// genericFuncSymbol returns the symbol of the callee, if resolvable.
func (c *Checker) genericFuncSymbol(x *ast.CallExpr) *Symbol {
	switch fn := x.Fn.(type) {
	case *ast.Ident:
		if sym, ok := c.scope.Lookup(fn.Name); ok {
			return sym
		}
	case *ast.IndexExpr:
		if id, ok := fn.X.(*ast.Ident); ok {
			if sym, ok := c.scope.Lookup(id.Name); ok {
				return sym
			}
		}
	}
	return nil
}

// checkCallee types the callee expression, treating a bare enum variant and a
// method selector specially.
func (c *Checker) checkCallee(x *ast.CallExpr) Type {
	return c.checkExpr(x.Fn)
}

// checkArgs matches the arguments of a call against a signature, handling
// named arguments, defaults and variadics, and records the result in Info.
func (c *Checker) checkArgs(x *ast.CallExpr, sig *Fn) Type {
	info := &CallInfo{Sig: sig, VariadicFrom: -1}
	nparams := len(sig.Params)
	filled := make([]bool, nparams)
	info.ArgOrder = make([]int, nparams)
	for i := range info.ArgOrder {
		info.ArgOrder[i] = -1
	}

	positional := 0
	seenNamed := false
	for ai, arg := range x.Args {
		if arg.Name != "" {
			seenNamed = true
			idx := paramIndex(sig, arg.Name)
			if idx < 0 {
				c.errorf(arg.Value, "unknown parameter name %q", arg.Name)
				c.checkExpr(arg.Value)
				continue
			}
			if filled[idx] {
				c.errorf(arg.Value, "parameter %q supplied twice", arg.Name)
			}
			filled[idx] = true
			info.ArgOrder[idx] = ai
			c.checkArgValue(arg, sig.Params[idx].Type)
			continue
		}
		if seenNamed {
			d := c.errorf(arg.Value, "positional argument after a named argument")
			c.hint(d, "put every positional argument before the named ones")
		}
		// Variadic tail: everything from here on packs into the last param.
		if sig.Variadic && positional >= nparams-1 {
			last := nparams - 1
			if last >= 0 {
				if info.VariadicFrom < 0 {
					info.VariadicFrom = ai
					filled[last] = true
					info.ArgOrder[last] = ai
				}
				c.checkArgValue(arg, sig.Params[last].Type)
			}
			positional++
			continue
		}
		if positional >= nparams {
			required, total := sig.Arity()
			c.errorf(arg.Value, "too many arguments: expected %d..%d, found %d",
				required, total, len(x.Args))
			c.checkExpr(arg.Value)
			positional++
			continue
		}
		filled[positional] = true
		info.ArgOrder[positional] = ai
		c.checkArgValue(arg, sig.Params[positional].Type)
		positional++
	}

	for i := range sig.Params {
		if filled[i] {
			continue
		}
		p := &sig.Params[i]
		switch {
		case p.HasDflt:
			info.Defaults = append(info.Defaults, i)
		case p.Variadic:
			// An empty variadic tail is fine.
		default:
			d := c.errorf(x, "missing argument for parameter %q of type %s", p.Name, p.Type)
			c.hint(d, "supply it positionally or as "+p.Name+": value")
		}
	}

	result := sig.Result
	if result == nil {
		result = Void
	}
	if sig.Async {
		result = NewFuture(result)
	}
	info.Result = result
	info.IsMethod = sig.Recv != nil
	c.info.Calls[x] = info
	return result
}

// checkArgValue checks one argument against its parameter type.
//
// A parameter typed Void is the "any" placeholder used by the variadic
// builtins (print and friends): it accepts every value unchecked.
func (c *Checker) checkArgValue(arg ast.Arg, want Type) {
	if want != nil && IsVoid(want) {
		c.checkExpr(arg.Value)
		return
	}
	got := c.checkExprExpect(arg.Value, want)
	if arg.Spread {
		if s, ok := Underlying(got).(*Slice); ok {
			got = s.Elem
		} else if !IsInvalid(got) {
			d := c.errorf(arg.Value, "cannot spread %s", got)
			c.hint(d, "'...' spreads a slice into a variadic parameter")
			return
		}
	}
	c.assignCompatible(arg.Value, got, want, "argument")
}

// paramIndex returns the index of the named parameter, or -1.
func paramIndex(sig *Fn, name string) int {
	for i := range sig.Params {
		if sig.Params[i].Name == name {
			return i
		}
	}
	return -1
}
