package types

import (
	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// checkCompoundAssign validates `+=` and friends.
func (c *Checker) checkCompoundAssign(x *ast.AssignExpr, want, got Type) {
	if IsInvalid(want) || IsInvalid(got) {
		return
	}
	u := Underlying(want)
	switch x.Op {
	case lexer.TokenPlusAssign:
		if IsStringType(u) {
			// `+=` on a string target is concatenation: the right-hand side
			// must be usable as a string.
			if !AssignableTo(got, want) {
				c.errorf(x, "cannot use %s as %s in assignment", got, want)
			}
			return
		}
		fallthrough
	case lexer.TokenMinusAssign, lexer.TokenStarAssign, lexer.TokenSlashAssign:
		if !IsNumeric(u) {
			c.errorf(x, "operator %s requires a numeric target, found %s", x.OpString(), want)
			return
		}
	case lexer.TokenPercentAssign, lexer.TokenAmpAssign, lexer.TokenPipeAssign,
		lexer.TokenCaretAssign, lexer.TokenShlAssign, lexer.TokenShrAssign:
		if !IsInteger(u) {
			c.errorf(x, "operator %s requires an integer target, found %s", x.OpString(), want)
			return
		}
	}
	if !AssignableTo(got, want) {
		c.errorf(x, "cannot use %s as %s in assignment", got, want)
	}
}

// requireAssignable reports an assignment to something that cannot be assigned.
func (c *Checker) requireAssignable(target ast.Expr) {
	switch t := target.(type) {
	case *ast.Ident:
		if sym, ok := c.scope.Lookup(t.Name); ok && !sym.Mutable && sym.Kind != SymParam {
			d := c.errorf(t, "cannot assign to constant %q", t.Name)
			c.hint(d, "declare it without 'const' to allow reassignment")
		}
	case *ast.FieldExpr, *ast.IndexExpr, *ast.ParenExpr, *ast.SelfExpr:
		// Assignable in principle; mutability of the receiver is checked
		// where the receiver itself is resolved.
	default:
		d := c.errorf(target, "cannot assign to this expression")
		c.hint(d, "the left-hand side must be a variable, field or index")
	}
}

// checkRange types `a..b` and `a..=b`.
func (c *Checker) checkRange(x *ast.RangeExpr) Type {
	var elem Type = Int
	if x.Low != nil {
		elem = c.checkExpr(x.Low)
	}
	if x.High != nil {
		high := c.checkExprExpect(x.High, elem)
		if x.Low == nil {
			elem = high
		} else if !AssignableTo(high, elem) && !IsInvalid(high) && !IsInvalid(elem) {
			c.errorf(x, "range bounds have different types: %s and %s", elem, high)
		}
	}
	if !IsInvalid(elem) && !IsInteger(Underlying(elem)) {
		d := c.errorf(x, "range bounds must be integers, found %s", elem)
		c.hint(d, "ranges iterate over integers only")
		return Invalid
	}
	return elem
}

// checkCast types `x as T`.
func (c *Checker) checkCast(x *ast.CastExpr) Type {
	src := c.checkExpr(x.X)
	dst := c.resolveType(x.Type)
	if IsInvalid(src) || IsInvalid(dst) {
		return dst
	}
	if !ConvertibleTo(src, dst) {
		d := c.errorf(x, "cannot convert %s to %s", src, dst)
		c.hint(d, "'as' converts between numeric types, string and []byte, and to 'dyn'")
		return Invalid
	}
	return dst
}

// checkTry types the `?` operator, which is legal only inside a function
// returning Option or Result.
func (c *Checker) checkTry(x *ast.TryExpr) Type {
	operand := c.checkExpr(x.X)
	if IsInvalid(operand) {
		return Invalid
	}
	fn := c.currentFn()
	if fn == nil {
		c.errorf(x, "'?' is only valid inside a function")
		return Invalid
	}

	if elem, ok := c.univ.IsOption(operand); ok {
		if _, isOpt := c.univ.IsOption(fn.result); !isOpt {
			d := c.errorf(x, "'?' on Option requires the function to return Option, found %s", fn.result)
			c.hint(d, "change the result type to Option[...] or handle the value with match")
		}
		return elem
	}
	if okT, errT, ok := c.univ.IsResult(operand); ok {
		if _, fnErr, isRes := c.univ.IsResult(fn.result); !isRes {
			d := c.errorf(x, "'?' on Result requires the function to return Result, found %s", fn.result)
			c.hint(d, "change the result type to Result[...] or handle the value with match")
		} else if !AssignableTo(errT, fnErr) && !IsInvalid(errT) && !IsInvalid(fnErr) {
			d := c.errorf(x, "'?' propagates %s but the function returns errors of type %s", errT, fnErr)
			c.hint(d, "convert the error before propagating it")
		}
		return okT
	}
	d := c.errorf(x, "'?' requires an Option or Result, found %s", operand)
	c.hint(d, "only Option and Result support error propagation")
	return Invalid
}

// checkAwait types `await x`.
func (c *Checker) checkAwait(x *ast.AwaitExpr) Type {
	operand := c.checkExpr(x.X)
	if fn := c.currentFn(); fn != nil && !fn.async {
		d := c.errorf(x, "'await' is only valid inside an async function")
		c.hint(d, "mark the enclosing function 'async fn'")
	}
	if fut, ok := Underlying(operand).(*Future); ok {
		return fut.Result
	}
	if IsInvalid(operand) {
		return Invalid
	}
	d := c.errorf(x, "cannot await %s", operand)
	c.hint(d, "await applies to the result of an async call or a spawn handle")
	return Invalid
}

// checkSpawn types `spawn { ... }`, whose value is a task handle.
func (c *Checker) checkSpawn(x *ast.SpawnExpr) Type {
	sig := &Fn{Async: true}
	c.enterFn(sig)
	c.fnStack[len(c.fnStack)-1].spawn = true
	pop := c.push(ScopeFunc, x)
	prevLoop := c.loopDepth
	c.loopDepth = 0
	c.checkBlock(x.Block, false)
	c.loopDepth = prevLoop
	pop()
	c.leaveFn()
	return NewFuture(c.spawnResult(x))
}

// spawnResult finds the type a spawned block returns, Void when it returns
// nothing.
func (c *Checker) spawnResult(x *ast.SpawnExpr) Type {
	if x.Block == nil {
		return Void
	}
	for _, s := range x.Block.Stmts {
		if ret, ok := s.(*ast.ReturnStmt); ok && len(ret.Values) == 1 {
			return c.info.TypeOf(ret.Values[0])
		}
	}
	return Void
}
