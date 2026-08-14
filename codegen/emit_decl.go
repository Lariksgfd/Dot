package codegen

import (
	"fmt"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/types"
)

// emitStructFwd emits the typedef declaration for a struct, so that pointer
// fields and parameters can reference it before its full definition appears.
func (g *generator) emitStructFwd(name string) {
	g.line(fmt.Sprintf("typedef struct Dot%s Dot%s;", name, name))
}

// emitStructDef emits a struct definition: `struct DotFoo { ... };`.
// The typedef declaration is emitted separately by emitStructFwd so that all
// forward declarations can be grouped before any definition (see emit_types.go).
func (g *generator) emitStructDef(name string, st *types.Struct) {
	g.line(fmt.Sprintf("struct Dot%s {", name))
	for _, f := range st.Fields {
		if f.Embedded {
			continue
		}
		ft := cFieldType(g, f.Type)
		g.line(fmt.Sprintf("    %s %s;", ft, cFieldName(f.Name)))
	}
	g.line("};")
	g.line("")
}

// emitEnumFwd emits the typedef declaration for a tagged-union enum.
func (g *generator) emitEnumFwd(name string) {
	g.line(fmt.Sprintf("typedef struct Dot%s Dot%s;", name, name))
}

// emitEnumDef emits a tagged-union enum definition.
func (g *generator) emitEnumDef(name string, en *types.Enum) {
	g.line(fmt.Sprintf("struct Dot%s {", name))
	g.line("    int32_t tag;")
	if len(en.Variants) > 0 {
		hasPayload := false
		for _, v := range en.Variants {
			if len(v.Fields) > 0 {
				hasPayload = true
				break
			}
		}
		if hasPayload {
			g.line("    union {")
			for _, v := range en.Variants {
				if len(v.Fields) == 0 {
					continue
				}
				g.line(fmt.Sprintf("        /* %s */", v.Name))
				for _, p := range v.Fields {
					pt := cFieldType(g, p.Type)
					g.line(fmt.Sprintf("        %s %s;", pt, variantFieldName(v.Name, p.Name)))
				}
			}
			g.line("    };")
		}
	}
	g.line("};")
	g.line("")
}

// emitTraitVtable emits a vtable struct for a trait.
func (g *generator) emitTraitVtable(name string, tr *types.Trait) {
	g.line(fmt.Sprintf("struct Dot%s_vtable {", name))
	for _, m := range tr.Methods {
		if m.Static {
			continue
		}
		result := cType(g, m.Sig.Result)
		g.line(fmt.Sprintf("    %s (*%s)(void*", result, cFieldName(m.Name)))
		for _, p := range m.Sig.Params {
			if p.Name == "self" {
				continue
			}
			g.line(fmt.Sprintf(", %s", cType(g, p.Type)))
		}
		g.line(");")
	}
	g.line("};")
	g.line("")
}

// emitFuncDef emits a single function definition.
// @extern("C") functions have no body and are skipped here; their
// declarations are emitted as extern by emitFuncDecls.
func (g *generator) emitFuncDef(fn *ast.FnDecl, recv *types.Named) {
	if isExtern(fn) || (fn.Body == nil && fn.ExprBody == nil) {
		return
	}
	fnType := g.fnType(fn)
	if fnType == nil {
		// For impl methods, look up the type from the receiver's Methods
		if recv != nil {
			fnType = g.methodType(fn, recv)
		}
	}
	if fnType == nil {
		return
	}

	result := cType(g, fnType.Result)
	if isEnumType(fnType.Result) {
		result += "*"
	}
	funcName := g.funcName(fn, recv)

	params := g.emitParams(fn.Sig, fnType, recv)

	g.line(fmt.Sprintf("%s %s(%s) {", result, funcName, params))
	if fn.Body != nil {
		saved := g.scopeVars
		g.scopeVars = nil
		for _, s := range fn.Body.Stmts {
			g.emitStmt(s)
		}

		needsCleanup := true
		if len(fn.Body.Stmts) > 0 {
			if _, isReturn := fn.Body.Stmts[len(fn.Body.Stmts)-1].(*ast.ReturnStmt); isReturn {
				needsCleanup = false
			}
		}

		if needsCleanup && len(g.scopeVars) > 0 {
			cleanup := emitScopeCleanup(g, g.scopeVars)
			for _, cl := range strings.Split(strings.TrimRight(cleanup, "\n"), "\n") {
				if cl != "" {
					g.line(cl)
				}
			}
		}
		g.scopeVars = saved
	} else if fn.ExprBody != nil {
		expr := g.emitExpr(fn.ExprBody)
		g.line(fmt.Sprintf("    return %s;", expr))
	}
	g.line("}")
	g.line("")
}

// methodType looks up the function type for an impl method from the receiver's Methods.
func (g *generator) methodType(fn *ast.FnDecl, recv *types.Named) *types.Fn {
	for _, m := range recv.Methods {
		if m == nil || m.Name != fn.Name {
			continue
		}
		return m.Sig
	}
	return nil
}

// emitForwardParams renders the C parameter type list for a forward declaration.
func (g *generator) emitForwardParams(fnType *types.Fn) string {
	if len(fnType.Params) == 0 {
		return "void"
	}
	var parts []string
	for _, p := range fnType.Params {
		ct := cType(g, p.Type)
		if isEnumType(p.Type) {
			ct += "*"
		}
		parts = append(parts, ct)
	}
	return strings.Join(parts, ", ")
}

// emitParams renders the C parameter list for a function.
func (g *generator) emitParams(sig *ast.FnSig, fnType *types.Fn, recv *types.Named) string {
	var parts []string
	paramIdx := 0
	if recv != nil && sig.Recv != nil {
		parts = append(parts, fmt.Sprintf("Dot%s* self", recv.Name))
	}
	for _, p := range sig.Params {
		var pt types.Type = types.Invalid
		if paramIdx < len(fnType.Params) {
			pt = fnType.Params[paramIdx].Type
		}
		ct := cType(g, pt)
		if isEnum := isEnumType(pt); isEnum {
			ct += "*"
		}
		parts = append(parts, fmt.Sprintf("%s %s", ct, p.Name))
		paramIdx++
	}
	return strings.Join(parts, ", ")
}

// funcName returns the C name for a function, handling methods and generics.
func (g *generator) funcName(fn *ast.FnDecl, recv *types.Named) string {
	if recv != nil {
		return fmt.Sprintf("Dot_%s_%s", recv.Name, fn.Name)
	}
	return fmt.Sprintf("Dot_%s", fn.Name)
}

// fnType returns the *types.Fn for a function declaration, looking it up
// from the symbol table (info.Defs) since FnDecl is not an ast.Expr.
func (g *generator) fnType(fn *ast.FnDecl) *types.Fn {
	sym, ok := g.info.Defs[fn]
	if !ok || sym == nil {
		return nil
	}
	if ft, ok := sym.Type.(*types.Fn); ok {
		return ft
	}
	if named, ok := sym.Type.(*types.Named); ok && named.Underlying != nil {
		if ft, ok := named.Underlying.(*types.Fn); ok {
			return ft
		}
	}
	return nil
}

// emitTypeDefs emits all type definitions (structs, enums, traits) plus the
// monomorphised type instances, in dependency order: forward typedefs first,
// then definitions topologically sorted by value-field dependencies.
// The implementation lives in emit_types.go.
func (g *generator) emitTypeDefs() error {
	return g.emitSortedTypeDefs()
}

// emitFuncDecls emits forward declarations for all user functions.
// @extern("C") functions receive `extern` declarations so that the linker
// can find their implementation in an external C library.
func (g *generator) emitFuncDecls() error {
	for _, decl := range g.prog.Decls {
		switch d := decl.(type) {
		case *ast.FnDecl:
			if len(d.TypeParams) > 0 {
				continue
			}
			if isExtern(d) {
				fnType := g.fnType(d)
				if fnType == nil {
					continue
				}
				result := cType(g, fnType.Result)
				if isEnumType(fnType.Result) {
					result += "*"
				}
				paramTypes := g.emitForwardParams(fnType)
				g.line(fmt.Sprintf("extern %s %s(%s);", result, g.funcName(d, nil), paramTypes))
				continue
			}
			if d.Body == nil && d.ExprBody == nil {
				continue
			}
			fnType := g.fnType(d)
			if fnType == nil {
				continue
			}
			result := cType(g, fnType.Result)
			if isEnumType(fnType.Result) {
				result += "*"
			}
			paramTypes := g.emitForwardParams(fnType)
			g.line(fmt.Sprintf("%s %s(%s);", result, g.funcName(d, nil), paramTypes))
		case *ast.ImplDecl:
			recv := g.implRecvType(d)
			if recv == nil {
				continue
			}
			for _, m := range d.Methods {
				if isExtern(m) {
					fnType := g.fnType(m)
					if fnType == nil {
						continue
					}
					result := cType(g, fnType.Result)
					paramTypes := g.emitForwardParams(fnType)
					g.line(fmt.Sprintf("extern %s %s(%s);", result, g.funcName(m, recv), paramTypes))
					continue
				}
				if m.Body == nil && m.ExprBody == nil {
					continue
				}
				fnType := g.fnType(m)
				if fnType == nil {
					continue
				}
				result := cType(g, fnType.Result)
				paramTypes := g.emitForwardParams(fnType)
				g.line(fmt.Sprintf("%s %s(%s);", result, g.funcName(m, recv), paramTypes))
			}
		}
	}
	g.line("")
	return nil
}

// emitMonomorphisedTypeDecls emits forward declarations for generic types.
func (g *generator) emitMonomorphisedTypeDecls() error {
	for _, inst := range g.info.InstanceList {
		if inst == nil || inst.Generic == nil || inst.Generic.Kind != types.SymType {
			continue
		}
		mangled := inst.Mangled
		if !strings.HasPrefix(mangled, "Dot") {
			mangled = "Dot" + mangled
		}
		g.line(fmt.Sprintf("typedef struct %s %s;", mangled, mangled))
	}
	return nil
}

// emitMonomorphisedDecls emits forward declarations for every monomorphised
// generic function so that call sites appearing before the definitions still
// see a complete prototype.
func (g *generator) emitMonomorphisedDecls() error {
	for _, inst := range g.info.InstanceList {
		if inst == nil || inst.Generic == nil || inst.Generic.Kind != types.SymFunc {
			continue
		}
		fn := inst.Generic.Fn
		if fn == nil || inst.Mangled == "" {
			continue
		}
		result := cType(g, fn.Result)
		if isEnumType(fn.Result) {
			result += "*"
		}
		var params []string
		for _, p := range fn.Params {
			ct := cType(g, p.Type)
			if isEnumType(p.Type) {
				ct += "*"
			}
			params = append(params, ct)
		}
		if len(params) == 0 {
			params = append(params, "void")
		}
		mangled := inst.Mangled
		if !strings.HasPrefix(mangled, "Dot") {
			mangled = "Dot" + mangled
		}
		g.line(fmt.Sprintf("%s %s(%s);", result, mangled, strings.Join(params, ", ")))
	}
	return nil
}

// emitFuncDefs emits all function definitions (user + monomorphised).
// @extern("C") functions are skipped because their implementation lives
// in external C code.
func (g *generator) emitFuncDefs() error {
	for _, decl := range g.prog.Decls {
		switch d := decl.(type) {
		case *ast.FnDecl:
			if isExtern(d) {
				continue
			}
			if len(d.TypeParams) > 0 {
				continue
			}
			g.emitFuncDef(d, nil)
		case *ast.ImplDecl:
			recv := g.implRecvType(d)
			if recv == nil {
				continue
			}
			for _, m := range d.Methods {
				g.emitFuncDef(m, recv)
			}
		}
	}
	return nil
}

// implRecvType resolves the receiver type for an impl block.
func (g *generator) implRecvType(d *ast.ImplDecl) *types.Named {
	switch t := d.Type.(type) {
	case *ast.NamedType:
		// Look up the type symbol by name
		for _, sym := range g.info.Uses {
			if sym != nil && sym.Name == t.Name && sym.Kind == types.SymType {
				if named, ok := sym.Type.(*types.Named); ok {
					return named
				}
			}
		}
		// Also check Defs
		for _, sym := range g.info.Defs {
			if sym != nil && sym.Name == t.Name && sym.Kind == types.SymType {
				if named, ok := sym.Type.(*types.Named); ok {
					return named
				}
			}
		}
	}
	return nil
}

// emitMonomorphised emits monomorphised generic functions from InstanceList.
// Monomorphised type definitions are emitted earlier, by emitSortedTypeDefs,
// so that they participate in the dependency-ordered type section.
func (g *generator) emitMonomorphised() error {
	seen := make(map[string]bool)
	for _, inst := range g.info.InstanceList {
		if inst == nil || seen[inst.Mangled] {
			continue
		}
		seen[inst.Mangled] = true
		if inst.Generic == nil {
			continue
		}
		if inst.Generic.Kind == types.SymFunc {
			g.emitMonomorphisedFunc(inst)
		}
	}
	return nil
}

// emitMonomorphisedFunc emits a monomorphised generic function: the mangled
// name carries the type arguments and the body is emitted from the generic
// declaration's AST.
func (g *generator) emitMonomorphisedFunc(inst *types.Instance) {
	fn := inst.Generic.Fn
	if fn == nil {
		return
	}
	result := cType(g, fn.Result)
	if isEnumType(fn.Result) {
		result += "*"
	}
	var params []string
	for _, p := range fn.Params {
		ct := cType(g, p.Type)
		if isEnumType(p.Type) {
			ct += "*"
		}
		params = append(params, fmt.Sprintf("%s %s", ct, p.Name))
	}
	mangled := inst.Mangled
	if !strings.HasPrefix(mangled, "Dot") {
		g.line(fmt.Sprintf("%s %s(%s) {", result, "Dot"+inst.Mangled, strings.Join(params, ", ")))
	} else {
		g.line(fmt.Sprintf("%s %s(%s) {", result, inst.Mangled, strings.Join(params, ", ")))
	}
	g.line(fmt.Sprintf("    /* monomorphised: %s */", inst.Generic.Name))
	if decl, ok := inst.Generic.Decl.(*ast.FnDecl); ok && decl != nil {
		if decl.Body != nil {
			saved := g.scopeVars
			g.scopeVars = nil
			for _, s := range decl.Body.Stmts {
				g.emitStmt(s)
			}

			needsCleanup := true
			if len(decl.Body.Stmts) > 0 {
				if _, isReturn := decl.Body.Stmts[len(decl.Body.Stmts)-1].(*ast.ReturnStmt); isReturn {
					needsCleanup = false
				}
			}

			if needsCleanup && len(g.scopeVars) > 0 {
				cleanup := emitScopeCleanup(g, g.scopeVars)
				for _, cl := range strings.Split(strings.TrimRight(cleanup, "\n"), "\n") {
					if cl != "" {
						g.line(cl)
					}
				}
			}
			g.scopeVars = saved
		} else if decl.ExprBody != nil {
			expr := g.emitExpr(decl.ExprBody)
			g.line(fmt.Sprintf("    return %s;", expr))
		}
	}
	g.line("}")
	g.line("")
}

// emitMonomorphisedType emits a monomorphised generic type.
func (g *generator) emitMonomorphisedType(inst *types.Instance) {
	named, ok := inst.Result.(*types.Named)
	if !ok {
		return
	}
	mangled := inst.Mangled
	if strings.HasPrefix(mangled, "Dot") {
		mangled = mangled[3:]
	}
	switch u := named.Underlying.(type) {
	case *types.Struct:
		g.emitStructDef(mangled, u)
	case *types.Enum:
		g.emitEnumDef(mangled, u)
	}
}
