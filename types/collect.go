package types

import (
	"github.com/dotlang/dot/ast"
)

// collectDecls is pass 1: it registers every top-level declaration so that
// declaration order does not matter, then completes each type's underlying
// structure and its impl blocks.
func (c *Checker) collectDecls(prog *ast.Program) {
	// 1a: register the names of all types and traits, with empty bodies.
	for _, d := range prog.Decls {
		switch decl := d.(type) {
		case *ast.StructDecl:
			c.declareNamed(decl.Name, decl, decl.NamePos)
		case *ast.EnumDecl:
			c.declareNamed(decl.Name, decl, decl.NamePos)
		case *ast.TraitDecl:
			c.declareTrait(decl)
		case *ast.ImportDecl:
			c.reportUnsupportedImport(decl)
		}
	}

	// 1b: fill in the underlying structure now that every name resolves.
	for _, d := range prog.Decls {
		switch decl := d.(type) {
		case *ast.StructDecl:
			c.completeStruct(decl)
		case *ast.EnumDecl:
			c.completeEnum(decl)
		case *ast.TraitDecl:
			c.completeTrait(decl)
		}
	}

	// 1c: register free functions and top-level constants.
	for _, d := range prog.Decls {
		switch decl := d.(type) {
		case *ast.FnDecl:
			c.declareFn(decl)
		case *ast.VarDecl:
			c.declareTopLevelVar(decl)
		}
	}

	// 1d: attach methods declared in impl blocks.
	for _, d := range prog.Decls {
		if decl, ok := d.(*ast.ImplDecl); ok {
			c.collectImpl(decl)
		}
	}
}

// declareNamed registers a nominal type with an as yet unknown underlying
// structure, so that mutually recursive declarations resolve.
func (c *Checker) declareNamed(name string, decl ast.Node, pos ast.Position) *Named {
	named := &Named{Name: name, Pos: pos, Methods: make(map[string]*Method)}
	sym := &Symbol{
		Kind:    SymType,
		Name:    name,
		Type:    named,
		Pos:     pos,
		Decl:    decl,
		Named:   named,
		Private: isPrivate(name),
	}
	named.Sym = sym
	c.declare(sym, decl)
	return named
}

// declareTrait registers a trait name before its methods are known.
func (c *Checker) declareTrait(decl *ast.TraitDecl) *Trait {
	tr := &Trait{Name: decl.Name}
	sym := &Symbol{
		Kind:    SymTrait,
		Name:    decl.Name,
		Type:    tr,
		Pos:     decl.NamePos,
		Decl:    decl,
		Trait:   tr,
		Private: isPrivate(decl.Name),
	}
	c.declare(sym, decl)
	return tr
}

// namedOf returns the *Named registered for a declaration name.
func (c *Checker) namedOf(name string) *Named {
	if sym, ok := c.scope.LookupLocal(name); ok && sym.Named != nil {
		return sym.Named
	}
	return nil
}

// traitOf returns the *Trait registered for a declaration name.
func (c *Checker) traitOf(name string) *Trait {
	if sym, ok := c.scope.LookupLocal(name); ok && sym.Trait != nil {
		return sym.Trait
	}
	return nil
}

// completeStruct fills in a struct's fields, including `embed`.
func (c *Checker) completeStruct(decl *ast.StructDecl) {
	named := c.namedOf(decl.Name)
	if named == nil {
		return
	}
	restore := c.pushTypeParams(named, decl.TypeParams)
	defer restore()

	st := &Struct{}
	for i, f := range decl.Fields {
		ft := c.resolveType(f.Type)
		name := f.Name
		if f.Embed {
			name = embeddedName(ft)
		}
		st.Fields = append(st.Fields, Field{
			Name:     name,
			Type:     ft,
			Embedded: f.Embed,
			HasDflt:  f.Default != nil,
			Default:  f.Default,
			Private:  isPrivate(name),
			Pos:      f.Pos(),
			Index:    i,
		})
	}
	st.Flat, st.Ambiguous = flattenFields(st)
	named.Underlying = st
}

// completeEnum fills in an enum's variants and registers its constructors.
func (c *Checker) completeEnum(decl *ast.EnumDecl) {
	named := c.namedOf(decl.Name)
	if named == nil {
		return
	}
	restore := c.pushTypeParams(named, decl.TypeParams)
	defer restore()

	e := &Enum{}
	for i, v := range decl.Variants {
		variant := Variant{
			Name:      v.Name,
			Tag:       i,
			Pos:       v.NamePos,
			HasParens: v.HasParens,
		}
		for _, f := range v.Fields {
			variant.Fields = append(variant.Fields, Param{
				Name: f.Name,
				Type: c.resolveType(f.Type),
			})
		}
		e.Variants = append(e.Variants, variant)
	}
	e.byName = variantByName(e)
	named.Underlying = e
}

// completeTrait fills in a trait's method signatures.
func (c *Checker) completeTrait(decl *ast.TraitDecl) {
	tr := c.traitOf(decl.Name)
	if tr == nil {
		return
	}
	prevSelf, prevTrait := c.selfType, c.selfTrait
	c.selfTrait = tr
	// Inside a trait body `Self` is the implementing type, which is unknown
	// here; a type parameter stands in for it.
	selfParam := &TypeParam{Name: "Self"}
	c.selfType = selfParam
	defer func() { c.selfType, c.selfTrait = prevSelf, prevTrait }()

	for _, m := range decl.Methods {
		// Inside a trait the receiver is the Self placeholder, so that a
		// method with a `self` parameter is not mistaken for a static one.
		tps, popFn := c.pushFnTypeParams(m.TypeParams)
		sig := c.signatureOf(m, selfParam)
		sig.TypeParams = tps
		popFn()
		
		tr.Methods = append(tr.Methods, Method{
			Name:    m.Name,
			Sig:     sig,
			HasBody: m.Body != nil || m.ExprBody != nil,
			Static:  sig.Recv == nil,
			Decl:    m,
			Pos:     m.NamePos,
		})
	}
	tr.byName = make(map[string]*Method, len(tr.Methods))
	for i := range tr.Methods {
		tr.byName[tr.Methods[i].Name] = &tr.Methods[i]
	}
}

// declareFn registers a free function.
func (c *Checker) declareFn(decl *ast.FnDecl) {
	tps, restore := c.pushFnTypeParams(decl.TypeParams)
	sig := c.signatureOf(decl, nil)
	sig.TypeParams = tps
	restore()

	sym := &Symbol{
		Kind:    SymFunc,
		Name:    decl.Name,
		Type:    sig,
		Pos:     decl.NamePos,
		Decl:    decl,
		Fn:      sig,
		Private: isPrivate(decl.Name),
	}
	c.declare(sym, decl)
}

// declareTopLevelVar registers a top-level variable or constant. Its type is
// resolved here when written explicitly and inferred in pass 2 otherwise.
func (c *Checker) declareTopLevelVar(decl *ast.VarDecl) {
	var declared Type
	if decl.Type != nil {
		declared = c.resolveType(decl.Type)
	}
	for _, n := range decl.Names {
		id, ok := n.(*ast.Ident)
		if !ok {
			continue
		}
		kind := SymVar
		if decl.Const {
			kind = SymConst
		}
		sym := &Symbol{
			Kind:    kind,
			Name:    id.Name,
			Type:    declared,
			Pos:     id.Pos(),
			Decl:    decl,
			Mutable: !decl.Const,
			Private: isPrivate(id.Name),
		}
		c.declare(sym, id)
	}
}

// reportUnsupportedImport records that module loading is not implemented yet
// (Phase 7), one diagnostic per import.
func (c *Checker) reportUnsupportedImport(decl *ast.ImportDecl) {
	d := c.errorf(decl, "module %q cannot be resolved: imports are not implemented yet", decl.PathString())
	c.hint(d, "single-file programs only until the module loader lands in Phase 7")
}

// signatureOf builds the semantic signature of a function declaration. recv is
// the receiver type for methods, nil for free functions.
func (c *Checker) signatureOf(decl *ast.FnDecl, recv Type) *Fn {
	sig := &Fn{Async: decl.Async}
	if decl.Sig == nil {
		sig.Result = Void
		return sig
	}
	if decl.Sig.Recv != nil {
		sig.Recv = recv
		sig.RecvMut = decl.Sig.Recv.Mut
	}
	for _, p := range decl.Sig.Params {
		var pt Type = Invalid
		if p.Type != nil {
			pt = c.resolveType(p.Type)
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
	if decl.Sig.Result != nil {
		sig.Result = c.resolveType(decl.Sig.Result)
	} else {
		sig.Result = Void
	}
	return sig
}

// pushTypeParams opens a scope holding a named type's generic parameters and
// records them on the type. The returned function closes the scope.
func (c *Checker) pushTypeParams(named *Named, params []*ast.TypeParam) func() {
	if len(params) == 0 {
		return func() {}
	}
	pop := c.push(ScopeTypeParams, nil)
	named.TypeParams = c.declareTypeParams(params)
	return pop
}

// pushFnTypeParams opens a scope holding a function's generic parameters.
func (c *Checker) pushFnTypeParams(params []*ast.TypeParam) ([]*TypeParam, func()) {
	if len(params) == 0 {
		return nil, func() {}
	}
	pop := c.push(ScopeTypeParams, nil)
	tps := c.declareTypeParams(params)
	return tps, pop
}

// declareTypeParams registers generic parameters in the current scope and
// resolves their trait bounds.
func (c *Checker) declareTypeParams(params []*ast.TypeParam) []*TypeParam {
	out := make([]*TypeParam, 0, len(params))
	for i, p := range params {
		tp := &TypeParam{Name: p.Name, Index: i, Pos: p.NamePos}
		for _, b := range p.Bounds {
			named, ok := b.(*ast.NamedType)
			if !ok {
				c.errorf(b, "a trait bound must be a trait name")
				continue
			}
			sym, ok := c.scope.Lookup(named.Name)
			if !ok {
				c.errorf(b, "undefined trait %q in bound", named.Name)
				continue
			}
			if sym.Kind != SymTrait || sym.Trait == nil {
				c.errorf(b, "%q is not a trait and cannot be used as a bound", named.Name)
				continue
			}
			tp.Bounds = append(tp.Bounds, sym.Trait)
		}
		c.declare(&Symbol{
			Kind: SymTypeParam,
			Name: p.Name,
			Type: tp,
			Pos:  p.NamePos,
			Decl: p,
			TP:   tp,
		}, p)
		out = append(out, tp)
	}
	return out
}

// embeddedName returns the field name an `embed T` field is reachable by.
func embeddedName(t Type) string {
	switch x := t.(type) {
	case *Named:
		return x.Name
	case *Basic:
		return x.Name()
	}
	return ""
}

// isPrivate reports whether a name is module-private (SPEC §15).
func isPrivate(name string) bool {
	return len(name) > 0 && name[0] == '_'
}
