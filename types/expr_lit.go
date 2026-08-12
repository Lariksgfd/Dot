package types

import (
	"github.com/dotlang/dot/ast"
)

// checkTupleLit types `(a, b)` and `(x: 1, y: 2)`.
func (c *Checker) checkTupleLit(x *ast.TupleLit, want Type) Type {
	var wantElems []Type
	if want != nil {
		if tup, ok := Underlying(want).(*Tuple); ok && len(tup.Elems) == len(x.Elems) {
			wantElems = tup.Elems
		}
	}
	elems := make([]Type, len(x.Elems))
	for i, e := range x.Elems {
		if wantElems != nil {
			elems[i] = c.checkExprExpect(e.Value, wantElems[i])
		} else {
			elems[i] = Default(c.checkExpr(e.Value))
		}
	}
	return NewTuple(elems...)
}

// checkArrayLit types `[1, 2, 3]`, `[]int{...}` and `[5]int{...}`.
func (c *Checker) checkArrayLit(x *ast.ArrayLit, want Type) Type {
	var elemType Type
	var result Type

	if x.Type != nil {
		declared := c.resolveType(x.Type)
		switch u := Underlying(declared).(type) {
		case *Slice:
			elemType, result = u.Elem, declared
		case *Array:
			elemType, result = u.Elem, declared
		default:
			if !IsInvalid(declared) {
				c.errorf(x, "%s is not an array or slice type", declared)
			}
			return Invalid
		}
	} else if want != nil {
		switch u := Underlying(want).(type) {
		case *Slice:
			elemType, result = u.Elem, want
		case *Array:
			elemType, result = u.Elem, want
		}
	}

	for _, e := range x.Elems {
		got := c.checkExprExpect(e, elemType)
		if elemType == nil {
			elemType = Default(got)
			result = NewSlice(elemType)
			continue
		}
		c.assignCompatible(e, got, elemType, "array element")
	}

	if result != nil {
		if arr, ok := Underlying(result).(*Array); ok && x.Type != nil &&
			int64(len(x.Elems)) != arr.Len && len(x.Elems) > 0 {
			c.errorf(x, "array literal has %d element(s) but the type declares %d",
				len(x.Elems), arr.Len)
		}
		return result
	}
	if elemType == nil {
		d := c.errorf(x, "cannot infer the element type of an empty array literal")
		c.hint(d, "write the type explicitly, as in []int{}")
		return Invalid
	}
	return NewSlice(elemType)
}

// checkMapLit types `map[string]int{...}`.
func (c *Checker) checkMapLit(x *ast.MapLit, want Type) Type {
	var keyType, valType Type

	if x.Type != nil {
		declared := c.resolveType(x.Type)
		m, ok := Underlying(declared).(*Map)
		if !ok {
			if !IsInvalid(declared) {
				c.errorf(x, "%s is not a map type", declared)
			}
			return Invalid
		}
		keyType, valType = m.Key, m.Value
	} else if want != nil {
		if m, ok := Underlying(want).(*Map); ok {
			keyType, valType = m.Key, m.Value
		}
	}

	for _, entry := range x.Entries {
		k := c.checkExprExpect(entry.Key, keyType)
		v := c.checkExprExpect(entry.Value, valType)
		if keyType == nil {
			keyType = Default(k)
		} else {
			c.assignCompatible(entry.Key, k, keyType, "map key")
		}
		if valType == nil {
			valType = Default(v)
		} else {
			c.assignCompatible(entry.Value, v, valType, "map value")
		}
	}

	if keyType == nil || valType == nil {
		d := c.errorf(x, "cannot infer the type of an empty map literal")
		c.hint(d, "write the type explicitly, as in map[string]int{}")
		return Invalid
	}
	if !Comparable(keyType) {
		c.errorf(x, "invalid map key type %s", keyType)
		return Invalid
	}
	return NewMap(keyType, valType)
}

// checkStructLit types `Point { x: 1, y: 2 }`.
func (c *Checker) checkStructLit(x *ast.StructLit) Type {
	declared := c.resolveType(x.Type)
	if IsInvalid(declared) {
		for _, f := range x.Fields {
			c.checkExpr(f.Value)
		}
		return Invalid
	}
	st, ok := Underlying(declared).(*Struct)
	if !ok {
		d := c.errorf(x, "%s is not a struct type", declared)
		c.hint(d, "only struct types support the { field: value } literal form")
		for _, f := range x.Fields {
			c.checkExpr(f.Value)
		}
		return Invalid
	}

	seen := make(map[string]bool, len(x.Fields))
	for _, lf := range x.Fields {
		fp, ok := st.Flat[lf.Name]
		if !ok {
			d := c.errorf(x, "%s has no field %q", declared, lf.Name)
			c.hint(d, "check the field name against the struct declaration")
			c.checkExpr(lf.Value)
			continue
		}
		if seen[lf.Name] {
			c.errorf(x, "field %q is set twice", lf.Name)
		}
		seen[lf.Name] = true
		got := c.checkExprExpect(lf.Value, fp.Field.Type)
		c.assignCompatible(lf.Value, got, fp.Field.Type, "struct field "+lf.Name)
	}

	for i := range st.Fields {
		f := &st.Fields[i]
		if f.Name == "" || seen[f.Name] || f.HasDflt || f.Embedded {
			continue
		}
		d := c.errorf(x, "missing field %q of type %s", f.Name, f.Type)
		c.hint(d, "every field without a default must be initialised")
	}
	return declared
}

// checkFnLit types a lambda, inferring untyped parameters from the expected
// function type when one is available.
func (c *Checker) checkFnLit(x *ast.FnLit, want Type) Type {
	var wantSig *Fn
	if want != nil {
		wantSig, _ = Underlying(want).(*Fn)
	}

	sig := &Fn{}
	if x.Sig != nil {
		for i, p := range x.Sig.Params {
			var pt Type
			switch {
			case p.Type != nil:
				pt = c.resolveType(p.Type)
			case wantSig != nil && i < len(wantSig.Params):
				pt = wantSig.Params[i].Type
			default:
				d := c.errorf(p, "cannot infer the type of parameter %q", p.Name)
				c.hint(d, "annotate the parameter, as in fn(x int) { ... }")
				pt = Invalid
			}
			sig.Params = append(sig.Params, Param{
				Name:     p.Name,
				Type:     pt,
				HasDflt:  p.Default != nil,
				Default:  p.Default,
				Variadic: p.Variadic,
				Mut:      p.Mut,
			})
			if p.Variadic {
				sig.Variadic = true
			}
		}
		if x.Sig.Result != nil {
			sig.Result = c.resolveType(x.Sig.Result)
		}
	}
	if sig.Result == nil {
		if wantSig != nil && wantSig.Result != nil {
			sig.Result = wantSig.Result
		} else {
			sig.Result = Void
		}
	}

	pop := c.push(ScopeFunc, x)
	prevLoop := c.loopDepth
	c.loopDepth = 0
	if x.Sig != nil {
		c.declareParams(x.Sig.Params, sig)
	}
	c.enterFn(sig)

	switch {
	case x.ExprBody != nil:
		got := c.checkExprExpect(x.ExprBody, sig.Result)
		if IsVoid(sig.Result) {
			sig.Result = Default(got)
		} else {
			c.assignCompatible(x.ExprBody, got, sig.Result, "lambda result")
		}
	case x.Body != nil:
		c.checkBlock(x.Body, false)
		if IsVoid(sig.Result) {
			if t, ok := c.blockValue(x.Body); ok {
				sig.Result = t
			}
		}
	}

	c.leaveFn()
	c.loopDepth = prevLoop
	pop()
	return sig
}

// blockValue returns the value type of a block whose last statement is an
// expression statement.
func (c *Checker) blockValue(block *ast.BlockStmt) (Type, bool) {
	if block == nil || len(block.Stmts) == 0 {
		return Void, false
	}
	last, ok := block.Stmts[len(block.Stmts)-1].(*ast.ExprStmt)
	if !ok {
		return Void, false
	}
	t := c.info.TypeOf(last.X)
	if IsVoid(t) || IsInvalid(t) {
		return t, false
	}
	return t, true
}

// checkIfExpr types `if`, including `else if` chains and the if-let form.
// The result is Void unless every branch yields a value.
func (c *Checker) checkIfExpr(x *ast.IfExpr, want Type) Type {
	pop := c.push(ScopeBlock, x)
	defer pop()

	if x.Bind != nil {
		subject := c.checkExpr(x.Cond)
		elem, ok := c.univ.IsOption(subject)
		if !ok {
			if okT, _, isRes := c.univ.IsResult(subject); isRes {
				elem = okT
			} else if !IsInvalid(subject) {
				d := c.errorf(x.Cond, "if-let requires an Option or Result, found %s", subject)
				c.hint(d, "use a plain condition, or match on the value")
				elem = Invalid
			}
		}
		c.bindPattern(x.Bind, elem)
	} else {
		cond := c.checkExpr(x.Cond)
		c.requireBool(x.Cond, cond, "if condition")
	}

	thenType := c.branchValue(x.Then, want)

	switch {
	case x.ElseIf != nil:
		elseType := c.checkExprExpect(x.ElseIf, want)
		return c.mergeBranches(x, thenType, elseType)
	case x.Else != nil:
		elseType := c.branchValue(x.Else, want)
		return c.mergeBranches(x, thenType, elseType)
	}
	return Void
}

// branchValue checks a branch block and returns the value it yields.
func (c *Checker) branchValue(block *ast.BlockStmt, want Type) Type {
	if block == nil {
		return Void
	}
	pop := c.push(ScopeBlock, block)
	defer pop()
	for i, s := range block.Stmts {
		last := i == len(block.Stmts)-1
		if es, ok := s.(*ast.ExprStmt); ok && last {
			return c.checkExprExpect(es.X, want)
		}
		c.checkStmt(s)
	}
	return Void
}

// mergeBranches unifies the value types of two branches.
func (c *Checker) mergeBranches(node ast.Node, a, b Type) Type {
	switch {
	case IsNever(a):
		return b
	case IsNever(b):
		return a
	case IsVoid(a) || IsVoid(b):
		return Void
	case IsInvalid(a) || IsInvalid(b):
		return Invalid
	case AssignableTo(b, a):
		return a
	case AssignableTo(a, b):
		return b
	}
	d := c.errorf(node, "branches have incompatible types: %s and %s", a, b)
	c.hint(d, "every branch of an if expression must produce the same type")
	return Invalid
}

// checkBlockExpr types a block used in expression position.
func (c *Checker) checkBlockExpr(x *ast.BlockExpr, want Type) Type {
	return c.branchValue(x.Block, want)
}

// checkMatchExpr types a match, checks its patterns and merges the arm values.
func (c *Checker) checkMatchExpr(x *ast.MatchExpr, want Type) Type {
	subject := c.checkExpr(x.Subject)

	var result Type
	for _, arm := range x.Arms {
		pop := c.push(ScopeMatchArm, arm)
		c.checkPattern(arm.Pattern, subject)
		armType := c.checkExprExpect(arm.Body, want)
		pop()
		if result == nil {
			result = armType
			continue
		}
		result = c.mergeBranches(arm, result, armType)
	}
	c.checkExhaustive(x, subject)
	if result == nil {
		return Void
	}
	return result
}
