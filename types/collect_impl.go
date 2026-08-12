package types

import (
	"github.com/dotlang/dot/ast"
)

// collectImpl registers the methods of one `impl` block on their target type,
// and for `impl Trait for T` records the trait implementation and checks that
// the type conforms.
func (c *Checker) collectImpl(decl *ast.ImplDecl) {
	pop := c.push(ScopeImpl, decl)
	defer pop()

	implParams := c.declareTypeParams(decl.TypeParams)
	_ = implParams

	target := c.resolveType(decl.Type)
	if IsInvalid(target) {
		return
	}
	named, ok := target.(*Named)
	if !ok {
		d := c.errorf(decl, "cannot implement methods on %s", target)
		c.hint(d, "methods may only be attached to a struct or enum declared in this file")
		return
	}
	if named.Origin != nil {
		named = named.Origin
	}
	if named.Methods == nil {
		named.Methods = make(map[string]*Method)
	}

	prevSelf, prevTrait := c.selfType, c.selfTrait
	c.selfType = named
	defer func() { c.selfType, c.selfTrait = prevSelf, prevTrait }()

	var trait *Trait
	if decl.Trait != nil {
		trait = c.resolveTraitRef(decl.Trait)
		c.selfTrait = trait
	}

	methods := make(map[string]*Method, len(decl.Methods))
	for _, m := range decl.Methods {
		tps, popFn := c.pushFnTypeParams(m.TypeParams)
		sig := c.signatureOf(m, named)
		sig.TypeParams = tps
		popFn()

		method := &Method{
			Name:    m.Name,
			Sig:     sig,
			HasBody: m.Body != nil || m.ExprBody != nil,
			Static:  sig.Recv == nil,
			Decl:    m,
			Pos:     m.NamePos,
		}
		methods[m.Name] = method

		if trait == nil {
			if _, dup := named.Methods[m.Name]; dup {
				d := c.errorf(m, "method %q is declared more than once on %s", m.Name, named.Name)
				c.hint(d, "each method name may appear only once per type")
				continue
			}
			named.Methods[m.Name] = method
		}
	}

	if trait == nil {
		return
	}
	impl := &TraitImpl{Trait: trait, Target: named, Methods: methods, Decl: decl}
	named.Impls = append(named.Impls, impl)
	c.checkConformance(decl, named, trait, methods)
}

// resolveTraitRef resolves the trait named in `impl Trait for T`.
func (c *Checker) resolveTraitRef(expr ast.Type) *Trait {
	named, ok := expr.(*ast.NamedType)
	if !ok {
		c.errorf(expr, "expected a trait name before 'for'")
		return nil
	}
	sym, ok := c.scope.Lookup(named.Name)
	if !ok {
		c.errorf(expr, "undefined trait %q", named.Name)
		return nil
	}
	if sym.Kind != SymTrait || sym.Trait == nil {
		d := c.errorf(expr, "%q is not a trait", named.Name)
		c.hint(d, "write 'impl "+named.Name+" { }' for inherent methods")
		return nil
	}
	return sym.Trait
}

// checkConformance verifies that an `impl Trait for T` block supplies every
// required method with a matching signature.
func (c *Checker) checkConformance(decl *ast.ImplDecl, named *Named, trait *Trait, methods map[string]*Method) {
	if trait == nil {
		return
	}
	for i := range trait.Methods {
		req := &trait.Methods[i]
		got, ok := methods[req.Name]
		if !ok {
			if req.HasBody {
				// A default implementation covers the requirement.
				continue
			}
			d := c.errorf(decl, "%s does not implement %s: missing method %q",
				named.Name, trait.Name, req.Name)
			c.hint(d, "add 'fn "+req.Name+"' to this impl block")
			continue
		}
		if !signaturesMatch(req.Sig, got.Sig) {
			d := c.errorf(got.Decl, "method %q has the wrong signature for trait %s",
				req.Name, trait.Name)
			c.hint(d, "the trait declares "+req.Sig.String())
		}
	}
	for name, m := range methods {
		if _, ok := trait.Method(name); !ok {
			d := c.errorf(m.Decl, "%s is not a method of trait %s", name, trait.Name)
			c.hint(d, "move it into a plain 'impl "+named.Name+" { }' block")
		}
	}
}

// signaturesMatch compares an implementation signature against a trait
// requirement, ignoring the receiver type since Self differs per impl.
func signaturesMatch(want, got *Fn) bool {
	if want == nil || got == nil {
		return want == got
	}
	if (want.Recv == nil) != (got.Recv == nil) {
		return false
	}
	if len(want.Params) != len(got.Params) {
		return false
	}
	for i := range want.Params {
		if !compatibleParam(want.Params[i].Type, got.Params[i].Type) {
			return false
		}
	}
	return compatibleParam(want.Result, got.Result)
}

// compatibleParam compares two types treating a trait's Self placeholder as a
// wildcard, since the implementing type substitutes for it.
func compatibleParam(want, got Type) bool {
	if want == nil || got == nil {
		return want == got
	}
	if tp, ok := want.(*TypeParam); ok && tp.Name == "Self" {
		return true
	}
	return Identical(want, got)
}
