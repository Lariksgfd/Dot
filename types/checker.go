// Package types implements the Dot type system: type representations, scopes,
// inference and the type checker.
//
// The checker never rewrites the AST. Everything it learns is recorded in an
// Info side table keyed by AST nodes (D50), so the ast package stays free of
// any dependency on types.
package types

import (
	"fmt"
	"reflect"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/errors"
)

// Selection records how a field or method expression was resolved.
type Selection struct {
	// Recv is the receiver's type.
	Recv Type
	// Kind tells what the selector denotes.
	Kind SelectionKind
	// Field is the resolved struct field, nil unless Kind == SelectField.
	Field *Field
	// Path is the embedding path to Field, len 1 for a direct field.
	Path []int
	// Method is the resolved method, nil unless Kind is a method kind.
	Method *Method
	// Sig is the resolved signature for method and builtin selections.
	Sig *Fn
	// Variant is the resolved enum variant, nil unless Kind == SelectVariant.
	Variant *Variant
	// Owner is the named type that declared the member, when applicable.
	Owner *Named
}

// SelectionKind classifies what a selector expression resolved to.
type SelectionKind int

const (
	// SelectField is a struct field access, possibly through embedding.
	SelectField SelectionKind = iota
	// SelectMethod is an inherent or trait method.
	SelectMethod
	// SelectBuiltinMethod is a builtin method such as items.push.
	SelectBuiltinMethod
	// SelectBuiltinProp is a builtin property such as items.len.
	SelectBuiltinProp
	// SelectVariant is an enum variant such as Color.Red.
	SelectVariant
	// SelectTupleIndex is a tuple index such as t.0.
	SelectTupleIndex
	// SelectModule is a member of an imported module.
	SelectModule
)

// CallInfo records how a call expression's arguments were matched to
// parameters, which codegen needs in order to materialise defaults and pack
// variadics.
type CallInfo struct {
	// Sig is the callee's signature.
	Sig *Fn
	// ArgOrder maps parameter index to the index in CallExpr.Args that
	// supplied it, or -1 when the default was used.
	ArgOrder []int
	// Defaults lists parameter indices filled from their default expression.
	Defaults []int
	// VariadicFrom is the first argument index packed into the variadic
	// parameter, or -1 when the callee is not variadic.
	VariadicFrom int
	// Builtin is the builtin identity of the callee, BuiltinNone otherwise.
	Builtin BuiltinID
	// IsMethod reports whether the call went through a receiver.
	IsMethod bool
	// Result is the call's result type.
	Result Type
}

// Instance records one instantiation of a generic function or type, so that
// Phase 5 can monomorphise without re-running inference.
type Instance struct {
	// Generic is the uninstantiated symbol.
	Generic *Symbol
	// TypeArgs are the inferred or explicit type arguments.
	TypeArgs []Type
	// Result is the instantiated type.
	Result Type
	// Pos is the instantiation site.
	Pos ast.Position
	// Mangled is the codegen-facing unique name.
	Mangled string
}

// Info is everything the checker learned about a program (D50).
type Info struct {
	// Types maps every checked expression to its type.
	Types map[ast.Expr]Type
	// Defs maps a declaring node to the symbol it introduces.
	Defs map[ast.Node]*Symbol
	// Uses maps an identifier reference to the symbol it denotes.
	Uses map[*ast.Ident]*Symbol
	// Selections maps a field expression to its resolution.
	Selections map[*ast.FieldExpr]*Selection
	// Calls maps a call expression to its argument matching.
	Calls map[*ast.CallExpr]*CallInfo
	// Instances maps a generic use site to its instantiation record.
	Instances map[ast.Expr]*Instance
	// InstanceList is every instantiation in deterministic discovery order.
	InstanceList []*Instance
	// instanceByMangled deduplicates InstanceList by Mangled.
	instanceByMangled map[string]*Instance
	// Scopes maps a scope-introducing node to its scope.
	Scopes map[ast.Node]*Scope
	// FileScope is the top-level scope of the checked file.
	FileScope *Scope
	// Universe is the predeclared scope used for this check.
	Universe *Universe
	// Implicits records types the checker synthesised at nodes that have no
	// syntax of their own, such as an inferred loop variable type.
	Implicits map[ast.Node]Type
	// Captures maps a closure to the variables it captures from outer scopes.
	Captures map[*ast.FnLit][]*Symbol
	// Escapes marks whether an expression is heap or stack allocated.
	Escapes map[ast.Expr]Escape
	// Heap marks expressions whose value is heap allocated and so ARC managed.
	Heap map[ast.Expr]bool
	// Arena marks nodes that live inside an @perf block.
	Arena map[ast.Node]bool
	// Terminates marks statements after which control cannot fall through.
	Terminates map[ast.Stmt]bool
	// NeededRuntime lists the C runtime modules required by imported
	// stdlib packages (e.g. "io", "math"). Deduplicated by caller.
	NeededRuntime []string
	// Diagnostics collects every error and warning produced by the check.
	Diagnostics *errors.ErrorList
}

// newInfo allocates an Info with every map ready for use.
func newInfo(u *Universe) *Info {
	return &Info{
		Types:       make(map[ast.Expr]Type),
		Defs:        make(map[ast.Node]*Symbol),
		Uses:        make(map[*ast.Ident]*Symbol),
		Selections:  make(map[*ast.FieldExpr]*Selection),
		Calls:       make(map[*ast.CallExpr]*CallInfo),
		Instances:   make(map[ast.Expr]*Instance),
		Scopes:      make(map[ast.Node]*Scope),
		Universe:    u,
		Implicits:   make(map[ast.Node]Type),
		Captures:    make(map[*ast.FnLit][]*Symbol),
		Heap:        make(map[ast.Expr]bool),
		Arena:       make(map[ast.Node]bool),
		Terminates:  make(map[ast.Stmt]bool),
		Diagnostics: &errors.ErrorList{},
	}
}

// TypeOf returns the recorded type of e, or Invalid when the expression was
// never checked.
func (in *Info) TypeOf(e ast.Expr) Type {
	if t, ok := in.Types[e]; ok && t != nil {
		return t
	}
	return Invalid
}

// globalChecker points to the checker currently running Check. It is used by
// the generic helpers (checkBounds, checkObjectSafe) to emit diagnostics
// without threading the *Checker through every call.
var globalChecker *Checker

// Checker walks a program and fills an Info.
type Checker struct {
	info *Info
	univ *Universe
	file string

	scope *Scope // the current lexical scope

	// fnStack is the stack of enclosing function signatures; the innermost
	// one determines what `return` and `?` are allowed to produce.
	fnStack []*fnContext
	// selfType is the type `self` and `Self` denote inside an impl or trait.
	selfType Type
	// selfTrait is the trait being implemented or declared, nil otherwise.
	selfTrait *Trait

	loopDepth  int
	arenaDepth int
	nextTypeID int

	// instantiateCache memoises instantiate() by (origin, TypeArgs) so that
	// repeated instantiations of the same generic type return the same *Named
	// pointer, which is critical for type identity.
	instantiateCache map[instantiateCacheKey]*Named

	bail bool // the diagnostic cap was reached
}

// fnContext is the per-function state the checker needs while checking a body.
type fnContext struct {
	sig    *Fn
	node   ast.Node // the *ast.FnLit, *ast.FnDecl or spawn block owning this context
	async  bool
	result Type
	// spawn marks the context of a `spawn { ... }` block: a value-return
	// determines the task's result type instead of being checked against
	// a declared signature.
	spawn bool
}

// Check type checks a parsed program and returns the collected information.
//
// The returned Info is always non-nil, even on failure: checking recovers and
// keeps going so that as many diagnostics as possible are reported at once.
// The returned error is nil on success, otherwise a *errors.ErrorList.
func Check(prog *ast.Program, filename string) (*Info, error) {
	return check(prog, filename, "")
}

// CheckWithImports type checks a program and resolves stdlib imports from
// stdlibDir before the normal two-pass check. Imported declarations are merged
// into the program so that codegen sees them.
func CheckWithImports(prog *ast.Program, filename string, stdlibDir string) (*Info, error) {
	return check(prog, filename, stdlibDir)
}

// check is the shared implementation for Check and CheckWithImports.
func check(prog *ast.Program, filename string, stdlibDir string) (*Info, error) {
	u := NewUniverse()
	c := &Checker{
		info: newInfo(u),
		univ: u,
		file: filename,
	}
	globalChecker = c
	c.scope = NewScope(u.Scope, ScopeFile)
	c.info.FileScope = c.scope
	if prog != nil {
		c.info.Scopes[prog] = c.scope
		if stdlibDir != "" {
			c.resolveImports(prog, stdlibDir)
		}
		c.collectDecls(prog)
		c.checkBodies(prog)

		// Finalize InstanceList: update Mangled names using resolved bounds
		// so that codegen deduplicates correctly.
		finalInstances := make([]*Instance, 0, len(c.info.InstanceList))
		seen := make(map[string]bool)
		for _, inst := range c.info.InstanceList {
			if inst.Generic == nil || inst.Result == nil {
				continue
			}
			// Re-mangle using fully resolved type args
			var newMangled string
			if named, ok := inst.Result.(*Named); ok {
				newMangled = instanceKey(named.Name, named.TypeArgs)
			} else if inst.Generic.Kind == SymFunc {
				newMangled = instanceKey(inst.Generic.Name, inst.TypeArgs)
			} else {
				newMangled = inst.Mangled // Fallback
			}
			inst.Mangled = newMangled

			if !seen[newMangled] {
				seen[newMangled] = true
				finalInstances = append(finalInstances, inst)
			}
		}
		c.info.InstanceList = finalInstances
	}
	return c.info, c.info.Diagnostics.Err()
}

// --- scope helpers -------------------------------------------------------

// push enters a fresh child scope of the given kind and returns a function
// that restores the previous scope.
func (c *Checker) push(kind ScopeKind, node ast.Node) func() {
	prev := c.scope
	c.scope = NewScope(prev, kind)
	if node != nil {
		c.info.Scopes[node] = c.scope
	}
	return func() { c.scope = prev }
}

// declare inserts sym into the current scope, reporting a redeclaration when
// the name already exists locally (SPEC §3, D53).
func (c *Checker) declare(sym *Symbol, node ast.Node) *Symbol {
	if existing, ok := c.scope.Insert(sym); !ok {
		d := c.errorf(node, "%q is already declared in this scope", sym.Name)
		c.hint(d, fmt.Sprintf("the earlier declaration is at %s", existing.Pos))
		return existing
	}
	if node != nil {
		c.info.Defs[node] = sym
	}
	return sym
}

// currentFn returns the innermost enclosing function context, or nil at top
// level.
func (c *Checker) currentFn() *fnContext {
	if n := len(c.fnStack); n > 0 {
		return c.fnStack[n-1]
	}
	return nil
}

// enterFn pushes a function context.
func (c *Checker) enterFn(node ast.Node, sig *Fn) {
	result := Type(Void)
	if sig != nil && sig.Result != nil {
		result = sig.Result
	}
	async := sig != nil && sig.Async
	c.fnStack = append(c.fnStack, &fnContext{sig: sig, node: node, async: async, result: result})
}

// leaveFn pops a function context.
func (c *Checker) leaveFn() {
	if n := len(c.fnStack); n > 0 {
		c.fnStack = c.fnStack[:n-1]
	}
}

// --- recording helpers ---------------------------------------------------

// recordType associates an expression with its type and returns the type.
func (c *Checker) recordType(e ast.Expr, t Type) Type {
	if t == nil {
		t = Invalid
	}
	if e != nil {
		c.info.Types[e] = t
	}
	return t
}

// recordUse associates an identifier reference with the symbol it denotes.
func (c *Checker) recordUse(id *ast.Ident, sym *Symbol) {
	if id != nil && sym != nil {
		c.info.Uses[id] = sym
	}
}

// recordDef associates a declaring node with the symbol it introduces.
func (c *Checker) recordDef(node ast.Node, sym *Symbol) {
	if node != nil && sym != nil {
		c.info.Defs[node] = sym
	}
}

// recordCapture marks sym as captured by the closure fnLit.
func (c *Checker) recordCapture(fnLit *ast.FnLit, sym *Symbol) {
	captures := c.info.Captures[fnLit]
	for _, existing := range captures {
		if existing == sym {
			return
		}
	}
	c.info.Captures[fnLit] = append(captures, sym)
}

// --- diagnostics ---------------------------------------------------------

// errorf records a type error at node's position.
func (c *Checker) errorf(node ast.Node, format string, args ...any) *errors.Diagnostic {
	return c.diag(node, false, format, args...)
}

// warnf records a warning at node's position.
func (c *Checker) warnf(node ast.Node, format string, args ...any) *errors.Diagnostic {
	return c.diag(node, true, format, args...)
}

// diag builds and records a diagnostic.
func (c *Checker) diag(node ast.Node, warn bool, format string, args ...any) *errors.Diagnostic {
	pos, length := c.span(node)
	msg := fmt.Sprintf(format, args...)
	var d *errors.Diagnostic
	if warn {
		d = errors.NewTypeWarning(pos.File, pos.Line, pos.Column, pos.Offset, length, msg)
	} else {
		d = errors.NewTypeError(pos.File, pos.Line, pos.Column, pos.Offset, length, msg)
	}
	if !c.info.Diagnostics.Add(d) {
		c.bail = true
	}
	if c.info.Diagnostics.Len() >= errors.MaxErrors {
		c.bail = true
	}
	return d
}

// hint attaches a hint to a diagnostic.
func (c *Checker) hint(d *errors.Diagnostic, hint string) {
	if d != nil {
		d.Hint = hint
	}
}

// span returns the position and byte length covered by a node, falling back to
// the file start for a nil node.
func (c *Checker) span(node ast.Node) (ast.Position, int) {
	if node == nil || isTypedNil(node) {
		return ast.Position{File: c.file, Line: 1, Column: 1}, 1
	}
	pos, end := node.Pos(), node.End()
	if pos.File == "" {
		pos.File = c.file
	}
	length := end.Offset - pos.Offset
	if length < 1 {
		length = 1
	}
	return pos, length
}

// isTypedNil reports whether v is a non-nil interface wrapping a nil pointer
// (or another nilable value). Such values still satisfy ast.Node but panic
// when methods dereference their receiver.
func isTypedNil(v any) bool {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface,
		reflect.Chan, reflect.Func:
		return rv.IsNil()
	}
	return false
}

// newTypeVar allocates a fresh inference variable.
func (c *Checker) newTypeVar(pos ast.Position) *TypeVar {
	c.nextTypeID++
	return &TypeVar{ID: c.nextTypeID, Pos: pos}
}
