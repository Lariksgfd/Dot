package types

import (
	"github.com/dotlang/dot/ast"
)

// checkBodies is pass 2: it checks every function body and top-level
// initialiser now that all declarations are known.
func (c *Checker) checkBodies(prog *ast.Program) {
	for _, d := range prog.Decls {
		if c.bail {
			return
		}
		switch decl := d.(type) {
		case *ast.FnDecl:
			c.checkFnBody(decl, nil)
		case *ast.ImplDecl:
			c.checkImplBodies(decl)
		case *ast.VarDecl:
			c.checkTopLevelVar(decl)
		}
	}
}

// checkImplBodies checks the method bodies of one impl block.
func (c *Checker) checkImplBodies(decl *ast.ImplDecl) {
	pop := c.push(ScopeImpl, nil)
	defer pop()
	c.declareTypeParams(decl.TypeParams)

	target := c.resolveType(decl.Type)
	prevSelf := c.selfType
	c.selfType = target
	defer func() { c.selfType = prevSelf }()

	for _, m := range decl.Methods {
		c.checkFnBody(m, target)
	}
}

// checkFnBody checks one function or method body against its signature.
func (c *Checker) checkFnBody(decl *ast.FnDecl, recv Type) {
	pop := c.push(ScopeFunc, decl)
	defer pop()

	c.declareTypeParams(decl.TypeParams)
	sig := c.signatureOf(decl, recv)

	if decl.Sig != nil && decl.Sig.Recv != nil && recv != nil {
		c.declare(&Symbol{
			Kind:    SymParam,
			Name:    "self",
			Type:    recv,
			Pos:     decl.Sig.Recv.Pos(),
			Decl:    decl.Sig.Recv,
			Mutable: decl.Sig.Recv.Mut,
		}, decl.Sig.Recv)
	}
	if decl.Sig != nil {
		c.declareParams(decl.Sig.Params, sig)
	}

	c.enterFn(decl, sig)
	defer c.leaveFn()

	switch {
	case decl.Body != nil:
		c.checkBlock(decl.Body, false)
		c.checkMissingReturn(decl, sig)
	case decl.ExprBody != nil:
		got := c.checkExpr(decl.ExprBody)
		c.assignCompatible(decl.ExprBody, got, sig.Result, "function result")
	}
	if decl.Body == nil && decl.ExprBody == nil && !isExternFn(decl) {
		c.errorf(decl, "function %q has no body", decl.Name)
	}
}

// declareParams binds a signature's parameters in the current scope and checks
// their default expressions.
func (c *Checker) declareParams(params []*ast.Param, sig *Fn) {
	for i, p := range params {
		var pt Type = Invalid
		if i < len(sig.Params) {
			pt = sig.Params[i].Type
		}
		bound := pt
		if p.Variadic {
			bound = NewSlice(pt)
		}
		c.declare(&Symbol{
			Kind:    SymParam,
			Name:    p.Name,
			Type:    bound,
			Pos:     p.NamePos,
			Decl:    p,
			Mutable: p.Mut,
			Private: isPrivate(p.Name),
		}, p)
		if p.Default != nil {
			got := c.checkExpr(p.Default)
			c.assignCompatible(p.Default, got, pt, "default value")
		}
	}
}

// checkMissingReturn reports a non-void function whose body can fall off the
// end without returning.
func (c *Checker) checkMissingReturn(decl *ast.FnDecl, sig *Fn) {
	if sig.Result == nil || IsVoid(sig.Result) || IsInvalid(sig.Result) {
		return
	}
	if decl.Body == nil || c.blockTerminates(decl.Body) {
		return
	}
	d := c.errorf(decl.Body, "missing return: %q must return %s on every path", decl.Name, sig.Result)
	c.hint(d, "add a return statement at the end of the function")
}

// checkTopLevelVar checks a top-level variable or constant initialiser.
func (c *Checker) checkTopLevelVar(decl *ast.VarDecl) {
	c.checkVarDecl(decl, true)
}

// --- statements ----------------------------------------------------------

// checkBlock checks every statement in a block. When ownScope is false the
// caller has already opened a scope for it (function bodies, loop bodies).
func (c *Checker) checkBlock(block *ast.BlockStmt, ownScope bool) {
	if block == nil {
		return
	}
	if ownScope {
		pop := c.push(ScopeBlock, block)
		defer pop()
	}
	for i, s := range block.Stmts {
		if c.bail {
			return
		}
		c.checkStmt(s)
		if c.info.Terminates[s] && i < len(block.Stmts)-1 {
			c.warnf(block.Stmts[i+1], "unreachable code")
			break
		}
	}
}

// checkStmt checks one statement.
func (c *Checker) checkStmt(s ast.Stmt) {
	switch stmt := s.(type) {
	case *ast.BlockStmt:
		c.checkBlock(stmt, true)
	case *ast.ExprStmt:
		t := c.checkExpr(stmt.X)
		if IsNever(t) {
			c.info.Terminates[s] = true
		}
	case *ast.VarDecl:
		c.checkVarDecl(stmt, false)
	case *ast.ReturnStmt:
		c.checkReturn(stmt)
	case *ast.ForStmt:
		c.checkFor(stmt)
	case *ast.BreakStmt:
		if c.loopDepth == 0 {
			c.errorf(stmt, "break outside of a loop")
		}
		c.info.Terminates[s] = true
	case *ast.ContinueStmt:
		if c.loopDepth == 0 {
			c.errorf(stmt, "continue outside of a loop")
		}
		c.info.Terminates[s] = true
	case *ast.DeferStmt:
		c.checkExpr(stmt.Call)
	case *ast.PerfBlock:
		c.arenaDepth++
		c.info.Arena[stmt] = true
		c.checkBlock(stmt.Block, true)
		c.arenaDepth--
	case *ast.FnDecl, *ast.StructDecl, *ast.EnumDecl, *ast.TraitDecl,
		*ast.ImplDecl, *ast.ImportDecl:
		c.errorf(s, "%s is only allowed at the top level", ast.NodeName(s))
	case *ast.BadStmt:
		// The parser already reported this.
	}
}

// checkVarDecl implements the declaration-versus-reassignment rule (D53):
// a name that already exists in the CURRENT scope is reassigned, a name found
// only in an outer scope is shadowed, and an unknown name is declared.
func (c *Checker) checkVarDecl(decl *ast.VarDecl, topLevel bool) {
	var declared Type
	if decl.Type != nil {
		declared = c.resolveType(decl.Type)
	}

	values := c.checkVarValues(decl, declared, len(decl.Names))

	for i, n := range decl.Names {
		var got Type = Invalid
		if i < len(values) {
			got = values[i]
		}
		want := declared
		if want == nil {
			want = Default(got)
		} else if i < len(values) {
			c.assignCompatible(valueNode(decl, i), got, want, "variable initialiser")
		}
		if want == nil || IsVoid(want) {
			want = Invalid
		}
		c.bindName(decl, n, want, topLevel)
	}
}

// checkVarValues checks a declaration's initialisers and returns one type per
// declared name, expanding a single multi-value call across all names.
func (c *Checker) checkVarValues(decl *ast.VarDecl, want Type, names int) []Type {
	if len(decl.Values) == 0 {
		if decl.Type == nil {
			d := c.errorf(decl, "cannot infer a type without an initialiser")
			c.hint(d, "write an explicit type, as in 'count int', or assign a value")
		}
		return nil
	}
	if len(decl.Values) == 1 && names > 1 {
		t := c.checkExpr(decl.Values[0])
		if tup, ok := Underlying(t).(*Tuple); ok {
			if len(tup.Elems) != names {
				c.errorf(decl, "assignment count mismatch: %d name(s) but %d value(s)",
					names, len(tup.Elems))
			}
			return tup.Elems
		}
		c.errorf(decl, "assignment count mismatch: %d name(s) but 1 value", names)
		return []Type{t}
	}
	if len(decl.Values) != names {
		c.errorf(decl, "assignment count mismatch: %d name(s) but %d value(s)",
			names, len(decl.Values))
	}
	out := make([]Type, 0, len(decl.Values))
	for _, v := range decl.Values {
		if want != nil {
			out = append(out, c.checkExprExpect(v, want))
		} else {
			out = append(out, c.checkExpr(v))
		}
	}
	return out
}

// valueNode returns the initialiser node matching name index i, for accurate
// diagnostic positions.
func valueNode(decl *ast.VarDecl, i int) ast.Node {
	if i < len(decl.Values) {
		return decl.Values[i]
	}
	if len(decl.Values) > 0 {
		return decl.Values[0]
	}
	return decl
}

// bindName declares or reassigns one name of a variable declaration.
func (c *Checker) bindName(decl *ast.VarDecl, n ast.Expr, t Type, topLevel bool) {
	id, ok := n.(*ast.Ident)
	if !ok {
		// `_` discards the value.
		return
	}
	if topLevel {
		if sym, ok := c.scope.LookupLocal(id.Name); ok {
			if sym.Type == nil || IsInvalid(sym.Type) {
				sym.Type = t
			}
			c.recordUse(id, sym)
			return
		}
	}
	if !decl.Const && decl.Type == nil {
		if sym, ok := c.scope.LookupLocal(id.Name); ok {
			// Reassignment of an existing binding in this very scope.
			if !sym.Mutable {
				d := c.errorf(id, "cannot assign to constant %q", id.Name)
				c.hint(d, "declare it without 'const' to allow reassignment")
			} else if !AssignableTo(t, sym.Type) && !IsInvalid(t) && !IsInvalid(sym.Type) {
				d := c.errorf(id, "cannot assign %s to %q of type %s", t, id.Name, sym.Type)
				c.hint(d, "a variable keeps the type it was first given")
			}
			c.recordUse(id, sym)
			return
		}
	}
	kind := SymVar
	if decl.Const {
		kind = SymConst
	}
	sym := &Symbol{
		Kind:    kind,
		Name:    id.Name,
		Type:    t,
		Pos:     id.Pos(),
		Decl:    decl,
		Mutable: !decl.Const,
		Private: isPrivate(id.Name),
	}
	c.declare(sym, id)
	c.recordType(id, t)
}

// checkReturn checks a return statement against the enclosing signature.
func (c *Checker) checkReturn(stmt *ast.ReturnStmt) {
	c.info.Terminates[stmt] = true
	fn := c.currentFn()
	if fn == nil {
		c.errorf(stmt, "return outside of a function")
		for _, v := range stmt.Values {
			c.checkExpr(v)
		}
		return
	}
	want := fn.result
	if fn.spawn {
		switch len(stmt.Values) {
		case 0:
			// A bare return supplies no value for the spawned task.
		case 1:
			got := c.checkExpr(stmt.Values[0])
			if IsVoid(want) {
				fn.result = Default(got)
			} else {
				c.assignCompatible(stmt.Values[0], got, want, "return value")
			}
		default:
			for _, v := range stmt.Values {
				c.checkExpr(v)
			}
		}
		return
	}

	switch len(stmt.Values) {
	case 0:
		if !IsVoid(want) && !IsInvalid(want) && !IsNever(want) {
			d := c.errorf(stmt, "missing return value: expected %s", want)
			c.hint(d, "return a value of type "+want.String())
		}
	case 1:
		got := c.checkExprExpect(stmt.Values[0], want)
		c.assignCompatible(stmt.Values[0], got, want, "return value")
	default:
		tup, ok := Underlying(want).(*Tuple)
		if !ok {
			c.errorf(stmt, "function returns %s but %d values were given", want, len(stmt.Values))
			for _, v := range stmt.Values {
				c.checkExpr(v)
			}
			return
		}
		if len(tup.Elems) != len(stmt.Values) {
			c.errorf(stmt, "function returns %d value(s) but %d were given",
				len(tup.Elems), len(stmt.Values))
		}
		for i, v := range stmt.Values {
			var wantElem Type = Invalid
			if i < len(tup.Elems) {
				wantElem = tup.Elems[i]
			}
			got := c.checkExprExpect(v, wantElem)
			c.assignCompatible(v, got, wantElem, "return value")
		}
	}
}
