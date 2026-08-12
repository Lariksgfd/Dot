package types

import (
	"github.com/dotlang/dot/ast"
)

// checkPattern checks a pattern against the type of the value being matched
// and introduces every binding it declares into the current scope.
func (c *Checker) checkPattern(p ast.Pattern, subject Type) {
	if p == nil {
		return
	}
	switch pat := p.(type) {
	case *ast.WildcardPattern:
		// Matches anything, binds nothing.

	case *ast.IdentPattern:
		c.bindPattern(pat, subject)

	case *ast.LiteralPattern:
		got := c.checkExprExpect(pat.Value, subject)
		if !IsInvalid(got) && !IsInvalid(subject) && !AssignableTo(got, subject) {
			d := c.errorf(pat, "pattern of type %s cannot match %s", got, subject)
			c.hint(d, "a literal pattern must have the same type as the subject")
		}

	case *ast.OrPattern:
		for _, alt := range pat.Alts {
			c.checkPattern(alt, subject)
		}

	case *ast.GuardedPattern:
		c.checkPattern(pat.Pattern, subject)
		guard := c.checkExpr(pat.Guard)
		c.requireBool(pat.Guard, guard, "match guard")

	case *ast.TypePattern:
		c.checkTypePattern(pat, subject)

	case *ast.EnumPattern:
		c.checkEnumPattern(pat, subject)

	case *ast.StructPattern:
		c.checkStructPattern(pat, subject)

	case *ast.TuplePattern:
		c.checkTuplePattern(pat, subject)

	case *ast.RangePattern:
		c.checkRangePattern(pat, subject)

	case *ast.BadPattern:
		// The parser already reported this.
	}
}

// bindPattern introduces the binding of an identifier pattern.
func (c *Checker) bindPattern(p ast.Pattern, t Type) {
	id, ok := p.(*ast.IdentPattern)
	if !ok {
		c.checkPattern(p, t)
		return
	}
	if id.Name == "_" {
		return
	}
	sym := &Symbol{
		Kind:    SymVar,
		Name:    id.Name,
		Type:    t,
		Pos:     id.Pos(),
		Decl:    id,
		Mutable: id.Mut,
		Private: isPrivate(id.Name),
	}
	c.declare(sym, id)
}

// checkTypePattern checks `int n` and `[]int arr`.
func (c *Checker) checkTypePattern(pat *ast.TypePattern, subject Type) {
	t := c.resolveType(pat.Type)
	if pat.Binding != nil {
		c.bindPattern(pat.Binding, t)
	}
	if IsInvalid(t) || IsInvalid(subject) {
		return
	}
	if _, isDyn := Underlying(subject).(*Dyn); isDyn {
		return
	}
	if !AssignableTo(t, subject) && !AssignableTo(subject, t) {
		d := c.errorf(pat, "%s can never match a subject of type %s", t, subject)
		c.hint(d, "type patterns are only useful on 'dyn' values")
	}
}

// checkEnumPattern checks `Color.Red`, `Shape.Circle(r)`, `Some(v)` and `None`.
func (c *Checker) checkEnumPattern(pat *ast.EnumPattern, subject Type) {
	e, ok := Underlying(subject).(*Enum)
	if !ok {
		if !IsInvalid(subject) {
			d := c.errorf(pat, "%s is not an enum and cannot be matched by a variant pattern", subject)
			c.hint(d, "match a value whose type is an enum, Option or Result")
		}
		c.bindUnknownArgs(pat)
		return
	}
	if pat.Enum != "" {
		if named, ok := subject.(*Named); ok && named.Name != pat.Enum {
			d := c.errorf(pat, "%s is not a variant of %s", pat.Enum, named.Name)
			c.hint(d, "the qualifier must name the subject's enum")
		}
	}
	v, ok := e.Variant(pat.Variant)
	if !ok {
		d := c.errorf(pat, "%s has no variant %q", subject, pat.Variant)
		c.hint(d, "available variants: "+variantNames(e))
		c.bindUnknownArgs(pat)
		return
	}
	if len(pat.Args) != len(v.Fields) {
		if len(pat.Args) > 0 || len(v.Fields) > 0 {
			c.errorf(pat, "variant %s takes %d value(s) but the pattern binds %d",
				v.Name, len(v.Fields), len(pat.Args))
		}
	}
	for i, arg := range pat.Args {
		var ft Type = Invalid
		if i < len(v.Fields) {
			ft = v.Fields[i].Type
		}
		c.checkPattern(arg, ft)
	}
}

// bindUnknownArgs binds the sub-patterns of an unresolvable variant pattern to
// Invalid so later statements do not cascade "undefined" errors.
func (c *Checker) bindUnknownArgs(pat *ast.EnumPattern) {
	for _, arg := range pat.Args {
		c.checkPattern(arg, Invalid)
	}
}

// checkStructPattern checks `Point { x: 0, y }`.
func (c *Checker) checkStructPattern(pat *ast.StructPattern, subject Type) {
	declared := c.resolveType(pat.Type)
	target := declared
	if IsInvalid(target) {
		target = subject
	}
	st, ok := Underlying(target).(*Struct)
	if !ok {
		if !IsInvalid(target) {
			c.errorf(pat, "%s is not a struct", target)
		}
		for _, f := range pat.Fields {
			c.checkPattern(f.Pattern, Invalid)
		}
		return
	}
	if !IsInvalid(declared) && !IsInvalid(subject) &&
		!AssignableTo(declared, subject) && !AssignableTo(subject, declared) {
		c.errorf(pat, "%s can never match a subject of type %s", declared, subject)
	}
	for _, f := range pat.Fields {
		fp, ok := st.Flat[f.Name]
		if !ok {
			c.errorf(pat, "%s has no field %q", target, f.Name)
			c.checkPattern(f.Pattern, Invalid)
			continue
		}
		c.checkPattern(f.Pattern, fp.Field.Type)
	}
}

// checkTuplePattern checks `(a, b)`.
func (c *Checker) checkTuplePattern(pat *ast.TuplePattern, subject Type) {
	tup, ok := Underlying(subject).(*Tuple)
	if !ok {
		if !IsInvalid(subject) {
			c.errorf(pat, "%s is not a tuple", subject)
		}
		for _, e := range pat.Elems {
			c.checkPattern(e, Invalid)
		}
		return
	}
	if len(pat.Elems) != len(tup.Elems) {
		c.errorf(pat, "tuple pattern binds %d element(s) but %s has %d",
			len(pat.Elems), subject, len(tup.Elems))
	}
	for i, e := range pat.Elems {
		var et Type = Invalid
		if i < len(tup.Elems) {
			et = tup.Elems[i]
		}
		c.checkPattern(e, et)
	}
}

// checkRangePattern checks `1..10` and `1..=10`.
func (c *Checker) checkRangePattern(pat *ast.RangePattern, subject Type) {
	for _, bound := range []ast.Expr{pat.Low, pat.High} {
		if bound == nil {
			continue
		}
		got := c.checkExprExpect(bound, subject)
		if !IsInvalid(got) && !IsInvalid(subject) && !AssignableTo(got, subject) {
			c.errorf(bound, "range bound of type %s cannot match %s", got, subject)
		}
	}
	if !IsInvalid(subject) && !Ordered(subject) {
		d := c.errorf(pat, "%s cannot be matched by a range pattern", subject)
		c.hint(d, "range patterns need an ordered type such as int or float")
	}
}

// variantNames renders an enum's variant names for a diagnostic hint.
func variantNames(e *Enum) string {
	names := make([]string, len(e.Variants))
	for i, v := range e.Variants {
		names[i] = v.Name
	}
	return joinStrings(names)
}
