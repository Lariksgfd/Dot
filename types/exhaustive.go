package types

import (
	"github.com/dotlang/dot/ast"
)

// checkExhaustive reports a match that does not cover every possible value.
//
// Coverage is computed exactly for enums (including Option and Result) and for
// bool. Every other subject type must supply a wildcard arm, because the value
// space cannot be enumerated (D66). Guarded arms never count as covering,
// since their guard may fail at run time.
func (c *Checker) checkExhaustive(x *ast.MatchExpr, subject Type) {
	if IsInvalid(subject) || len(x.Arms) == 0 {
		return
	}
	if c.hasWildcardArm(x) {
		return
	}

	switch u := Underlying(subject).(type) {
	case *Enum:
		c.checkEnumExhaustive(x, subject, u)
	case *Basic:
		if IsBoolean(u) {
			c.checkBoolExhaustive(x, subject)
			return
		}
		c.reportInexhaustive(x, subject, nil)
	default:
		c.reportInexhaustive(x, subject, nil)
	}
}

// hasWildcardArm reports whether some unguarded arm matches every value.
func (c *Checker) hasWildcardArm(x *ast.MatchExpr) bool {
	for _, arm := range x.Arms {
		if irrefutable(arm.Pattern) {
			return true
		}
	}
	return false
}

// irrefutable reports whether a pattern matches every possible value.
func irrefutable(p ast.Pattern) bool {
	switch pat := p.(type) {
	case *ast.WildcardPattern:
		return true
	case *ast.IdentPattern:
		return true
	case *ast.TypePattern:
		return false
	case *ast.OrPattern:
		for _, alt := range pat.Alts {
			if irrefutable(alt) {
				return true
			}
		}
	}
	return false
}

// checkEnumExhaustive verifies that every variant of an enum is covered.
func (c *Checker) checkEnumExhaustive(x *ast.MatchExpr, subject Type, e *Enum) {
	covered := make(map[string]bool, len(e.Variants))
	for _, arm := range x.Arms {
		if _, guarded := arm.Pattern.(*ast.GuardedPattern); guarded {
			continue
		}
		collectCoveredVariants(arm.Pattern, covered)
	}
	var missing []string
	for _, v := range e.Variants {
		if !covered[v.Name] {
			missing = append(missing, v.Name)
		}
	}
	if len(missing) > 0 {
		c.reportInexhaustive(x, subject, missing)
	}
}

// checkBoolExhaustive verifies that both true and false are covered.
func (c *Checker) checkBoolExhaustive(x *ast.MatchExpr, subject Type) {
	seenTrue, seenFalse := false, false
	for _, arm := range x.Arms {
		if _, guarded := arm.Pattern.(*ast.GuardedPattern); guarded {
			continue
		}
		collectCoveredBools(arm.Pattern, &seenTrue, &seenFalse)
	}
	if seenTrue && seenFalse {
		return
	}
	var missing []string
	if !seenTrue {
		missing = append(missing, "true")
	}
	if !seenFalse {
		missing = append(missing, "false")
	}
	c.reportInexhaustive(x, subject, missing)
}

// collectCoveredVariants records which enum variants a pattern covers.
func collectCoveredVariants(p ast.Pattern, out map[string]bool) {
	switch pat := p.(type) {
	case *ast.EnumPattern:
		out[pat.Variant] = true
	case *ast.OrPattern:
		for _, alt := range pat.Alts {
			collectCoveredVariants(alt, out)
		}
	}
}

// collectCoveredBools records which boolean literals a pattern covers.
func collectCoveredBools(p ast.Pattern, seenTrue, seenFalse *bool) {
	switch pat := p.(type) {
	case *ast.LiteralPattern:
		if b, ok := pat.Value.(*ast.BoolLit); ok {
			if b.Value {
				*seenTrue = true
			} else {
				*seenFalse = true
			}
		}
	case *ast.OrPattern:
		for _, alt := range pat.Alts {
			collectCoveredBools(alt, seenTrue, seenFalse)
		}
	}
}

// reportInexhaustive emits the non-exhaustive match diagnostic.
func (c *Checker) reportInexhaustive(x *ast.MatchExpr, subject Type, missing []string) {
	if len(missing) > 0 {
		d := c.errorf(x, "match on %s is not exhaustive: missing %s",
			subject, joinStrings(missing))
		c.hint(d, "add the missing arm(s), or a '_' arm to cover the rest")
		return
	}
	d := c.errorf(x, "match on %s is not exhaustive", subject)
	c.hint(d, "add a '_' arm: values of this type cannot be enumerated")
}
