// Package codegen translates a typed Dot AST into C source code.
//
// This file implements dependency-ordered type emission. In C, a type stored
// by value (a struct field, an array element) requires the full definition of
// its value type to precede it; pointer fields only need a typedef
// declaration. All named types (user structs/enums, monomorphised generic
// instances and tuple typedefs) are therefore collected into one graph keyed
// by C name, forward typedefs are emitted first, and the definitions follow
// in topological order of their value dependencies.
package codegen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/types"
)

// cTypeDef is one pending C type definition participating in the topo sort.
type cTypeDef struct {
	cName string   // key in byName: the Dot-prefixed typedef name
	deps  []string // C names of types that must be defined first
	emit  func()
}

// emitSortedTypeDefs emits the complete type section: forward typedefs for
// every named struct/enum, trait vtables, then all definitions (user types,
// monomorphised instances and tuple typedefs) in value-dependency order.
func (g *generator) emitSortedTypeDefs() error {
	byName := make(map[string]*cTypeDef)
	var order []string

	addDef := func(cName string, deps []string, emit func()) {
		if _, dup := byName[cName]; dup {
			return
		}
		byName[cName] = &cTypeDef{cName: cName, deps: deps, emit: emit}
		order = append(order, cName)
	}

	// Phase A: user-declared types. Forwards for every named struct/enum so
	// that pointer fields can reference any type regardless of order; trait
	// vtables hold only function pointers and are emitted inline.
	for _, decl := range g.prog.Decls {
		switch d := decl.(type) {
		case *ast.StructDecl:
			if isExternType(d.Annotations) {
				continue
			}
			named := g.declNamedType(d)
			if named == nil {
				continue
			}
			if st, ok := named.Underlying.(*types.Struct); ok {
				g.emitStructFwd(d.Name)
				addDef("Dot"+d.Name, structDeps(g, st), func() {
					g.emitStructDef(d.Name, st)
				})
			}
		case *ast.EnumDecl:
			named := g.declNamedType(d)
			if named == nil {
				continue
			}
			if en, ok := named.Underlying.(*types.Enum); ok {
				g.emitEnumFwd(d.Name)
				addDef("Dot"+d.Name, enumDeps(g, en), func() {
					g.emitEnumDef(d.Name, en)
				})
			}
		case *ast.TraitDecl:
			named := g.declNamedType(d)
			if named == nil {
				continue
			}
			if tr, ok := named.Underlying.(*types.Trait); ok {
				g.emitTraitVtable(d.Name, tr)
			}
		}
	}

	// Phase B: monomorphised generic type instances. Their forward typedefs
	// were already emitted by emitMonomorphisedTypeDecls; only definitions
	// join the graph so that value dependencies on user types (and on other
	// instances) are respected.
	for _, inst := range g.info.InstanceList {
		if inst == nil || inst.Generic == nil || inst.Generic.Kind != types.SymType {
			continue
		}
		named, ok := inst.Result.(*types.Named)
		if !ok {
			continue
		}
		cName := dotTypeName(inst.Mangled)
		var deps []string
		switch u := named.Underlying.(type) {
		case *types.Struct:
			deps = structDeps(g, u)
		case *types.Enum:
			deps = enumDeps(g, u)
		default:
			continue
		}
		instCopy := inst
		addDef(cName, deps, func() {
			g.emitMonomorphisedType(instCopy)
		})
	}

	// Phase C: generic Named types discovered in field/signature/expression
	// types but absent from InstanceList (e.g. Option[string] in a struct
	// field). Their forward typedefs are emitted by
	// emitMonomorphisedTypeDecls; here only the definitions join the graph.
	for _, cName := range g.genericTypeList {
		registerGenericDef(byName, addDef, g, cName)
	}

	// Phase D: tuple typedefs. cTupleName may register new tuple shapes while
	// dependencies are collected above, so the list is drained index-wise.
	for i := 0; i < len(g.tupleList); i++ {
		t := g.tupleList[i]
		var deps []string
		for _, et := range t.ElemTypes {
			collectValueDeps(g, et, &deps)
		}
		addDef(t.Name, deps, func() {
			g.emitTupleDef(t)
		})
	}

	// Phase E: any generic Named types whose dependencies (collected in
	// Phase C) introduced new tuple shapes. Runs to a fixpoint; normally
	// only one or two extra iterations.
	for {
		added := false
		for _, cName := range g.genericTypeList {
			if registerGenericDef(byName, addDef, g, cName) {
				added = true
			}
		}
		if !added {
			break
		}
	}

	// Emit every definition in dependency order (depth-first, cycle-safe).
	visited := make(map[string]bool)
	var visit func(cName string)
	visit = func(cName string) {
		if visited[cName] {
			return
		}
		visited[cName] = true
		d, ok := byName[cName]
		if !ok {
			return
		}
		for _, dep := range d.deps {
			visit(dep)
		}
		d.emit()
	}
	for _, cName := range order {
		visit(cName)
	}
	g.line("")
	return nil
}

// declNamedType resolves the *types.Named introduced by a type declaration.
func (g *generator) declNamedType(d ast.Node) *types.Named {
	sym, ok := g.info.Defs[d]
	if !ok || sym == nil || sym.Type == nil {
		return nil
	}
	named, ok := sym.Type.(*types.Named)
	if !ok {
		return nil
	}
	return named
}

// isExternType reports whether a struct declaration carries @extern.
func isExternType(anns []*ast.Annotation) bool {
	for _, ann := range anns {
		if ann.Name == "extern" {
			return true
		}
	}
	return false
}

// structDeps collects the value dependencies of a struct definition: every
// field stored by value whose type needs a prior full definition.
func structDeps(g *generator, st *types.Struct) []string {
	var deps []string
	for _, f := range st.Fields {
		if f.Embedded {
			continue
		}
		collectFieldDeps(g, f.Type, &deps)
	}
	return deps
}

// enumDeps collects the value dependencies of an enum definition from the
// payload fields of all variants.
func enumDeps(g *generator, en *types.Enum) []string {
	var deps []string
	for _, v := range en.Variants {
		for _, p := range v.Fields {
			collectFieldDeps(g, p.Type, &deps)
		}
	}
	return deps
}

// collectFieldDeps gathers dependencies for one struct or enum-union field.
// Enum-typed fields are always stored as pointers (cFieldType), so they need
// no definition; everything else is stored by value.
func collectFieldDeps(g *generator, t types.Type, deps *[]string) {
	if t == nil {
		return
	}
	if isEnumType(t) {
		return
	}
	collectValueDeps(g, t, deps)
}

// collectValueDeps gathers the C type names whose full definitions must
// precede a definition that stores t by value.
func collectValueDeps(g *generator, t types.Type, deps *[]string) {
	if t == nil {
		return
	}
	switch x := t.(type) {
	case *types.Named:
		if isEnumType(x) {
			return
		}
		if _, ok := x.Underlying.(*types.Struct); ok {
			*deps = append(*deps, dotTypeName(cNamed(g, x)))
		}
	case *types.Struct:
		for _, f := range x.Fields {
			if f.Embedded {
				continue
			}
			collectFieldDeps(g, f.Type, deps)
		}
	case *types.Enum:
		for _, v := range x.Variants {
			for _, p := range v.Fields {
				collectFieldDeps(g, p.Type, deps)
			}
		}
	case *types.Array:
		// cType renders arrays as anonymous structs whose data member holds
		// elements by value, with no pointer promotion for enums.
		switch e := x.Elem.(type) {
		case *types.Named:
			if isEnumType(e) {
				*deps = append(*deps, dotTypeName(cNamed(g, e)))
				return
			}
		case *types.Enum:
			for _, v := range e.Variants {
				for _, p := range v.Fields {
					collectFieldDeps(g, p.Type, deps)
				}
			}
			return
		}
		collectValueDeps(g, x.Elem, deps)
	case *types.Tuple:
		name := cTupleName(g, x)
		*deps = append(*deps, name)
		for _, e := range x.Elems {
			collectValueDeps(g, e, deps)
		}
	case *types.TypeVar:
		if x.Bound != nil {
			collectValueDeps(g, x.Bound, deps)
		}
	}
}

// dotTypeName normalises a type name to its Dot-prefixed C typedef name.
func dotTypeName(name string) string {
	if strings.HasPrefix(name, "Dot") {
		return name
	}
	return "Dot" + name
}

// registerGenericDef adds the definition of one scanned generic Named type to
// the topo-sort graph. It reports whether a new definition was registered.
func registerGenericDef(byName map[string]*cTypeDef, addDef func(string, []string, func()), g *generator, cName string) bool {
	if _, dup := byName[cName]; dup {
		return false
	}
	named := g.genericTypes[cName]
	if named == nil {
		return false
	}
	var deps []string
	switch u := named.Underlying.(type) {
	case *types.Struct:
		deps = structDeps(g, u)
	case *types.Enum:
		deps = enumDeps(g, u)
	default:
		return false
	}
	n := named
	addDef(cName, deps, func() {
		g.emitNamedTypeDef(cName, n)
	})
	return true
}

// emitNamedTypeDef emits the struct or enum definition for a generic Named
// type whose instantiation was never recorded in Info.InstanceList.
func (g *generator) emitNamedTypeDef(cName string, named *types.Named) {
	short := strings.TrimPrefix(cName, "Dot")
	switch u := named.Underlying.(type) {
	case *types.Struct:
		g.emitStructDef(short, u)
	case *types.Enum:
		g.emitEnumDef(short, u)
	}
}

// collectGenericTypes scans every type the program mentions (expression
// types, signatures, declared-type fields and instance results) and
// registers generic Named types that have no entry in Info.InstanceList.
// Definitions for these types are emitted by emitSortedTypeDefs and their
// forward typedefs by emitMonomorphisedTypeDecls.
func (g *generator) collectGenericTypes() {
	if g.genericTypes != nil {
		return
	}
	g.genericTypes = make(map[string]*types.Named)
	seen := make(map[types.Type]bool)

	scan := func(t types.Type) { g.scanTypeForGenerics(t, seen) }

	for _, decl := range g.prog.Decls {
		switch d := decl.(type) {
		case *ast.StructDecl, *ast.EnumDecl, *ast.TraitDecl:
			if named := g.declNamedType(d); named != nil {
				scan(named)
			}
		case *ast.FnDecl:
			if len(d.TypeParams) > 0 || isExtern(d) {
				continue
			}
			if ft := g.fnType(d); ft != nil {
				scan(ft)
			}
		case *ast.ImplDecl:
			recv := g.implRecvType(d)
			if recv == nil {
				continue
			}
			for _, m := range d.Methods {
				ft := g.fnType(m)
				if ft == nil {
					ft = g.methodType(m, recv)
				}
				if ft != nil {
					scan(ft)
				}
			}
		}
	}
	for _, inst := range g.info.InstanceList {
		if inst == nil || inst.Generic == nil {
			continue
		}
		switch inst.Generic.Kind {
		case types.SymFunc:
			if inst.Generic.Fn != nil {
				scan(inst.Generic.Fn)
			}
			if f, ok := inst.Result.(*types.Fn); ok {
				scan(f)
			}
		case types.SymType:
			scan(inst.Result)
		}
	}
	for _, t := range g.info.Types {
		scan(t)
	}
	for _, t := range g.info.Implicits {
		scan(t)
	}

	// Deterministic emission order: identical input always yields the same
	// generic type section even though info.Types is a map.
	sort.Strings(g.genericTypeList)
}

// scanTypeForGenerics walks t structurally, registering every generic Named
// type it meets and descending into underlying types to catch nested generic
// instantiations (e.g. Option[Box[int]]).
func (g *generator) scanTypeForGenerics(t types.Type, seen map[types.Type]bool) {
	if t == nil {
		return
	}
	if seen[t] {
		return
	}
	seen[t] = true
	switch x := t.(type) {
	case *types.Named:
		if len(x.TypeArgs) > 0 {
			cName := cNamed(g, x)
			if _, ok := g.genericTypes[cName]; !ok {
				g.genericTypes[cName] = x
				g.genericTypeList = append(g.genericTypeList, cName)
			}
		}
		g.scanTypeForGenerics(x.Underlying, seen)
	case *types.Slice:
		g.scanTypeForGenerics(x.Elem, seen)
	case *types.Array:
		g.scanTypeForGenerics(x.Elem, seen)
	case *types.Map:
		g.scanTypeForGenerics(x.Key, seen)
		g.scanTypeForGenerics(x.Value, seen)
	case *types.Tuple:
		for _, e := range x.Elems {
			g.scanTypeForGenerics(e, seen)
		}
	case *types.Pointer:
		g.scanTypeForGenerics(x.Elem, seen)
	case *types.Weak:
		g.scanTypeForGenerics(x.Elem, seen)
	case *types.Chan:
		g.scanTypeForGenerics(x.Elem, seen)
	case *types.Future:
		g.scanTypeForGenerics(x.Result, seen)
	case *types.Fn:
		for _, p := range x.Params {
			g.scanTypeForGenerics(p.Type, seen)
		}
		g.scanTypeForGenerics(x.Result, seen)
		g.scanTypeForGenerics(x.Recv, seen)
	case *types.Struct:
		for _, f := range x.Fields {
			g.scanTypeForGenerics(f.Type, seen)
		}
	case *types.Enum:
		for _, v := range x.Variants {
			for _, p := range v.Fields {
				g.scanTypeForGenerics(p.Type, seen)
			}
		}
	case *types.Trait:
		for i := range x.Methods {
			g.scanTypeForGenerics(x.Methods[i].Sig, seen)
		}
	case *types.TypeVar:
		if x.Bound != nil {
			g.scanTypeForGenerics(x.Bound, seen)
		}
	}
}

// emitTupleDef emits one positional tuple typedef.
func (g *generator) emitTupleDef(t *tupleInfo) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("typedef struct { "))
	for i, f := range t.Fields {
		b.WriteString(fmt.Sprintf("%s _%d; ", f, i))
	}
	b.WriteString(fmt.Sprintf("} %s;", t.Name))
	g.line(b.String())
}
