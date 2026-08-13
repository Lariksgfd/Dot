package types

import (
	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// checkExpr infers the type of an expression with no contextual expectation.
func (c *Checker) checkExpr(e ast.Expr) Type {
	return c.checkExprExpect(e, nil)
}

// checkExprExpect infers the type of an expression, using want as the
// bidirectional expectation when one is available (D52). It never returns nil.
func (c *Checker) checkExprExpect(e ast.Expr, want Type) Type {
	if e == nil {
		return Invalid
	}
	if c.bail {
		return Invalid
	}
	t := c.inferExpr(e, want)
	if t == nil {
		t = Invalid
	}
	return c.recordType(e, t)
}

// inferExpr dispatches on the expression kind.
func (c *Checker) inferExpr(e ast.Expr, want Type) Type {
	switch x := e.(type) {
	case *ast.IntLit:
		return c.literalInt(x, want)
	case *ast.FloatLit:
		return c.literalFloat(want)
	case *ast.StringLit:
		return c.checkStringLit(x)
	case *ast.RawStringLit:
		return String_
	case *ast.BoolLit:
		return Bool
	case *ast.NilLit:
		if want != nil && !IsInvalid(want) {
			return want
		}
		return UntypedNil
	case *ast.Ident:
		return c.checkIdent(x)
	case *ast.UnderscoreExpr:
		c.errorf(x, "'_' may only be used as an assignment target")
		return Invalid
	case *ast.SelfExpr:
		return c.checkSelf(x)
	case *ast.ParenExpr:
		return c.checkExprExpect(x.X, want)
	case *ast.UnaryExpr:
		return c.checkUnary(x, want)
	case *ast.BinaryExpr:
		return c.checkBinary(x)
	case *ast.AssignExpr:
		return c.checkAssign(x)
	case *ast.RangeExpr:
		return c.checkRange(x)
	case *ast.CastExpr:
		return c.checkCast(x)
	case *ast.IsExpr:
		c.checkExpr(x.X)
		c.resolveType(x.Type)
		return Bool
	case *ast.TryExpr:
		return c.checkTry(x)
	case *ast.AwaitExpr:
		return c.checkAwait(x)
	case *ast.SpawnExpr:
		return c.checkSpawn(x)
	case *ast.CallExpr:
		return c.checkCall(x)
	case *ast.IndexExpr:
		return c.checkIndex(x)
	case *ast.SliceExpr:
		return c.checkSlice(x)
	case *ast.FieldExpr:
		return c.checkField(x)
	case *ast.PipeExpr:
		return c.checkPipe(x)
	case *ast.TupleLit:
		return c.checkTupleLit(x, want)
	case *ast.ArrayLit:
		return c.checkArrayLit(x, want)
	case *ast.MapLit:
		return c.checkMapLit(x, want)
	case *ast.StructLit:
		return c.checkStructLit(x)
	case *ast.FnLit:
		return c.checkFnLit(x, want)
	case *ast.IfExpr:
		return c.checkIfExpr(x, want)
	case *ast.MatchExpr:
		return c.checkMatchExpr(x, want)
	case *ast.BlockExpr:
		return c.checkBlockExpr(x, want)
	case *ast.BadExpr:
		return Invalid
	}
	c.errorf(e, "unsupported expression %s", ast.NodeName(e))
	return Invalid
}

// literalInt types an integer literal, honouring a float expectation so that
// `x float = 1` works without an explicit conversion.
func (c *Checker) literalInt(x *ast.IntLit, want Type) Type {
	if want != nil {
		u := Underlying(want)
		if IsInteger(u) || IsFloat(u) {
			return want
		}
	}
	return Int
}

// literalFloat types a float literal.
func (c *Checker) literalFloat(want Type) Type {
	if want != nil && IsFloat(Underlying(want)) {
		return want
	}
	return Float
}

// checkStringLit checks every interpolated expression and yields string.
func (c *Checker) checkStringLit(x *ast.StringLit) Type {
	for _, part := range x.Parts {
		if part.Kind == ast.PartExpr && part.Expr != nil {
			c.checkExpr(part.Expr)
		}
	}
	return String_
}

// checkIdent resolves an identifier reference.
func (c *Checker) checkIdent(x *ast.Ident) Type {
	sym, ok := c.scope.Lookup(x.Name)
	if !ok {
		d := c.errorf(x, "undefined: %q", x.Name)
		c.hint(d, "declare it before use, or check the spelling")
		return Invalid
	}
	c.recordUse(x, sym)
	
	if sym.Kind == SymVar || sym.Kind == SymParam {
		if sym.Scope != nil && sym.Scope.Depth() > 1 {
			for i := len(c.fnStack) - 1; i >= 0; i-- {
				fnCtx := c.fnStack[i]
				fnLit, isLit := fnCtx.node.(*ast.FnLit)
				if !isLit {
					continue
				}
				fnScope := c.info.Scopes[fnLit]
				if fnScope != nil && fnScope.Depth() > sym.Scope.Depth() {
					c.recordCapture(fnLit, sym)
				}
			}
		}
	}

	switch sym.Kind {
	case SymVariant:
		return c.variantValue(sym, x)
	case SymType, SymTrait:
		d := c.errorf(x, "%q is a type, not a value", x.Name)
		c.hint(d, "a type name cannot be used as a value here")
		return Invalid
	}
	if sym.Type == nil {
		return Invalid
	}
	return sym.Type
}

// variantValue types a bare enum variant reference: a unit variant is a value
// of its enum, while a payload variant is a constructor function.
func (c *Checker) variantValue(sym *Symbol, x ast.Expr) Type {
	v := sym.Variant
	if v == nil || sym.Owner == nil {
		return Invalid
	}
	result := c.freshEnumValue(sym.Owner, x)
	if len(v.Fields) == 0 {
		return result
	}
	// Take the payload types from the freshly instantiated enum so that the
	// constructor's parameters mention the same inference variables as its
	// result, letting Some(x) learn T from x.
	fields := v.Fields
	if e, ok := Underlying(result).(*Enum); ok {
		if iv, ok := e.Variant(v.Name); ok {
			fields = iv.Fields
		}
	}
	fn := &Fn{Result: result}
	for _, f := range fields {
		fn.Params = append(fn.Params, Param{Name: f.Name, Type: f.Type})
	}
	return fn
}

// freshEnumValue instantiates a generic enum with inference variables so that
// `None` and `Ok(1)` can be used before their type arguments are known.
func (c *Checker) freshEnumValue(named *Named, at ast.Expr) Type {
	if len(named.TypeParams) == 0 {
		return named
	}
	args := make([]Type, len(named.TypeParams))
	for i := range args {
		args[i] = c.newTypeVar(at.Pos())
	}
	switch named {
	case c.univ.Option:
		inst := c.univ.OptionOf(args[0])
		c.recordGenericEnumInstance(named, args, inst, at)
		return inst
	case c.univ.Result:
		inst := c.univ.ResultOf(args[0], args[1])
		c.recordGenericEnumInstance(named, args, inst, at)
		return inst
	}
	return c.instantiate(named, args)
}

// recordGenericEnumInstance records an *Instance for a generic enum use
// (Option[?T], Result[?T,?E]) so Phase 5 can monomorphise it.
func (c *Checker) recordGenericEnumInstance(named *Named, args []Type, inst Type, at ast.Expr) {
	mangled := instanceKey(named.Name, args)
	c.recordInstance(&Instance{
		Generic:  named.Sym,
		TypeArgs: args,
		Result:   inst,
		Pos:      at.Pos(),
		Mangled:  mangled,
	})
}

// checkSelf types the `self` receiver.
func (c *Checker) checkSelf(x *ast.SelfExpr) Type {
	sym, ok := c.scope.Lookup("self")
	if !ok {
		d := c.errorf(x, "'self' is only valid inside a method")
		c.hint(d, "add a 'self' receiver to the method signature")
		return Invalid
	}
	return sym.Type
}

// checkUnary types `-x`, `+x`, `not x` and `~x`.
func (c *Checker) checkUnary(x *ast.UnaryExpr, want Type) Type {
	operand := c.checkExprExpect(x.X, want)
	if IsInvalid(operand) {
		return Invalid
	}
	u := Underlying(operand)
	switch x.Op {
	case lexer.TokenMinus, lexer.TokenPlus:
		if !IsNumeric(u) {
			c.errorf(x, "operator %s requires a numeric operand, found %s", x.OpString(), operand)
			return Invalid
		}
		return operand
	case lexer.TokenNot:
		if !IsBoolean(u) {
			d := c.errorf(x, "operator 'not' requires bool, found %s", operand)
			c.hint(d, "Dot has no truthiness: compare explicitly")
			return Invalid
		}
		return operand
	case lexer.TokenTilde:
		if !IsInteger(u) {
			c.errorf(x, "operator '~' requires an integer, found %s", operand)
			return Invalid
		}
		return operand
	}
	c.errorf(x, "unsupported unary operator %s", x.OpString())
	return Invalid
}

// checkBinary types every binary operator.
func (c *Checker) checkBinary(x *ast.BinaryExpr) Type {
	left := c.checkExpr(x.X)
	right := c.checkExprExpect(x.Y, left)
	if IsInvalid(left) || IsInvalid(right) {
		return Invalid
	}

	switch x.Op {
	case lexer.TokenAnd, lexer.TokenOr:
		c.requireBool(x.X, left, "operand of "+x.OpString())
		c.requireBool(x.Y, right, "operand of "+x.OpString())
		return Bool

	case lexer.TokenEqEq, lexer.TokenBangEq:
		if !c.sameOperands(x, left, right) {
			return Bool
		}
		if !Comparable(left) {
			d := c.errorf(x, "%s cannot be compared for equality", left)
			c.hint(d, "implement a trait or compare the fields individually")
		}
		return Bool

	case lexer.TokenLt, lexer.TokenGt, lexer.TokenLtEq, lexer.TokenGtEq:
		if !c.sameOperands(x, left, right) {
			return Bool
		}
		if !Ordered(left) {
			c.errorf(x, "%s cannot be ordered with %s", left, x.OpString())
		}
		return Bool

	case lexer.TokenPlus:
		if IsStringType(Underlying(left)) && IsStringType(Underlying(right)) {
			return left
		}
		return c.arithmetic(x, left, right)

	case lexer.TokenMinus, lexer.TokenStar, lexer.TokenSlash, lexer.TokenStarStar:
		return c.arithmetic(x, left, right)

	case lexer.TokenPercent:
		if !c.sameOperands(x, left, right) {
			return Invalid
		}
		if !IsInteger(Underlying(left)) {
			c.errorf(x, "operator '%%' requires integers, found %s", left)
			return Invalid
		}
		return left

	case lexer.TokenAmp, lexer.TokenPipe, lexer.TokenCaret,
		lexer.TokenShl, lexer.TokenShr:
		if !IsInteger(Underlying(left)) || !IsInteger(Underlying(right)) {
			c.errorf(x, "operator %s requires integers, found %s and %s",
				x.OpString(), left, right)
			return Invalid
		}
		return left
	}
	c.errorf(x, "unsupported binary operator %s", x.OpString())
	return Invalid
}

// arithmetic checks a numeric binary operator.
func (c *Checker) arithmetic(x *ast.BinaryExpr, left, right Type) Type {
	if !c.sameOperands(x, left, right) {
		return Invalid
	}
	if !IsNumeric(Underlying(left)) {
		c.errorf(x, "operator %s requires numeric operands, found %s", x.OpString(), left)
		return Invalid
	}
	return left
}

// sameOperands reports whether both operands have the same type, emitting a
// diagnostic when they do not.
func (c *Checker) sameOperands(x *ast.BinaryExpr, left, right Type) bool {
	if AssignableTo(right, left) || AssignableTo(left, right) {
		return true
	}
	d := c.errorf(x, "mismatched types: %s %s %s", left, x.OpString(), right)
	if IsNumeric(Underlying(left)) && IsNumeric(Underlying(right)) {
		c.hint(d, "Dot has no implicit numeric conversion: use 'as'")
	}
	return false
}

// checkAssign types `x = v` and the compound assignment operators.
func (c *Checker) checkAssign(x *ast.AssignExpr) Type {
	for i, target := range x.Targets {
		if _, ok := target.(*ast.UnderscoreExpr); ok {
			if i < len(x.Values) {
				c.checkExpr(x.Values[i])
			}
			continue
		}
		want := c.checkExpr(target)
		c.requireAssignable(target)
		if i < len(x.Values) {
			got := c.checkExprExpect(x.Values[i], want)
			if x.Op == lexer.TokenAssign {
				c.assignCompatible(x.Values[i], got, want, "assignment")
			} else {
				c.checkCompoundAssign(x, want, got)
			}
		}
	}
	return Void
}
