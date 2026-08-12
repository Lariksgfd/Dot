package types

import (
	"github.com/dotlang/dot/ast"
)

// checkFor checks all five loop forms.
func (c *Checker) checkFor(stmt *ast.ForStmt) {
	pop := c.push(ScopeLoop, stmt)
	defer pop()
	c.loopDepth++
	defer func() { c.loopDepth-- }()

	switch stmt.Kind {
	case ast.ForInfinite:
		// Nothing to check in the head.
	case ast.ForCond:
		cond := c.checkExpr(stmt.Cond)
		c.requireBool(stmt.Cond, cond, "loop condition")
	case ast.ForIn:
		c.checkForIn(stmt)
	}
	c.checkBlock(stmt.Body, false)
}

// checkForIn binds the loop variables of a `for x in xs` head.
func (c *Checker) checkForIn(stmt *ast.ForStmt) {
	iter := c.checkExpr(stmt.Iterable)
	keyType, valType, ok := c.iterationTypes(iter)
	if !ok {
		if !IsInvalid(iter) {
			d := c.errorf(stmt.Iterable, "cannot iterate over %s", iter)
			c.hint(d, "iterate over a range, a slice, an array, a map or a channel")
		}
		keyType, valType = Invalid, Invalid
	}

	if stmt.Key != nil {
		c.bindLoopVar(stmt.Key, keyType)
		c.bindLoopVar(stmt.Value, valType)
		return
	}
	// With a single variable, a map yields its keys and everything else its
	// elements.
	single := valType
	if _, isMap := Underlying(iter).(*Map); isMap {
		single = keyType
	}
	c.bindLoopVar(stmt.Value, single)
}

// iterationTypes returns the (key, value) types produced by iterating t.
func (c *Checker) iterationTypes(t Type) (Type, Type, bool) {
	switch x := Underlying(t).(type) {
	case *Slice:
		return Int, x.Elem, true
	case *Array:
		return Int, x.Elem, true
	case *Map:
		return x.Key, x.Value, true
	case *Chan:
		return Int, x.Elem, true
	case *Basic:
		switch {
		case IsStringType(x):
			return Int, Rune, true
		case IsInteger(x):
			// The value of a range expression is its element type.
			return Int, x, true
		}
	}
	if IsInvalid(t) {
		return Invalid, Invalid, true
	}
	return Invalid, Invalid, false
}

// bindLoopVar declares one loop variable.
func (c *Checker) bindLoopVar(e ast.Expr, t Type) {
	id, ok := e.(*ast.Ident)
	if !ok {
		return
	}
	sym := &Symbol{
		Kind:    SymVar,
		Name:    id.Name,
		Type:    t,
		Pos:     id.Pos(),
		Decl:    id,
		Mutable: true,
		Private: isPrivate(id.Name),
	}
	c.declare(sym, id)
	c.recordType(id, t)
}

// blockTerminates reports whether control cannot fall off the end of a block.
func (c *Checker) blockTerminates(block *ast.BlockStmt) bool {
	if block == nil || len(block.Stmts) == 0 {
		return false
	}
	last := block.Stmts[len(block.Stmts)-1]
	if c.info.Terminates[last] {
		return true
	}
	// A trailing expression statement supplies the value of the block.
	if es, ok := last.(*ast.ExprStmt); ok {
		t := c.info.TypeOf(es.X)
		return !IsVoid(t) && !IsInvalid(t)
	}
	return false
}

// requireBool reports a non-boolean condition.
func (c *Checker) requireBool(node ast.Node, t Type, what string) {
	if IsInvalid(t) || IsNever(t) {
		return
	}
	if !IsBoolean(Underlying(t)) {
		d := c.errorf(node, "%s must be bool, found %s", what, t)
		c.hint(d, "Dot has no truthiness: write an explicit comparison")
	}
}

// assignCompatible reports an assignment whose value type does not fit.
func (c *Checker) assignCompatible(node ast.Node, got, want Type, what string) {
	if got == nil || want == nil || IsInvalid(got) || IsInvalid(want) || IsNever(got) {
		return
	}
	// Inference variables introduced by a generic constructor such as Some(x)
	// are solved against the expected type before assignability is judged.
	if hasUnresolvedVars(got) || hasUnresolvedVars(want) {
		if unify(got, want) {
			return
		}
	}
	if AssignableTo(resolve(got), resolve(want)) {
		return
	}
	d := c.errorf(node, "cannot use %s as %s in %s", got, want, what)
	if IsNumeric(Underlying(got)) && IsNumeric(Underlying(want)) {
		c.hint(d, "Dot has no implicit numeric conversion: write 'as "+want.String()+"'")
	}
}
