package types

import (
	"github.com/dotlang/dot/ast"
)

// SymbolKind classifies what a name denotes.
type SymbolKind int

const (
	SymVar SymbolKind = iota
	SymConst
	SymParam
	SymFunc
	SymType
	SymTrait
	SymVariant
	SymTypeParam
	SymModule
	SymBuiltin
	SymLabel
)

// symbolKindNames indexes the human-readable name of each SymbolKind.
var symbolKindNames = [...]string{
	SymVar:       "var",
	SymConst:     "const",
	SymParam:     "param",
	SymFunc:      "func",
	SymType:      "type",
	SymTrait:     "trait",
	SymVariant:   "variant",
	SymTypeParam: "typeparam",
	SymModule:    "module",
	SymBuiltin:   "builtin",
	SymLabel:     "label",
}

// String returns the SymbolKind's name.
func (k SymbolKind) String() string {
	if int(k) < len(symbolKindNames) {
		return symbolKindNames[k]
	}
	return "SymbolKind(?)"
}

// Symbol is one named entity. Exactly one of the Kind-specific payload fields
// is meaningful, selected by Kind.
type Symbol struct {
	Kind    SymbolKind
	Name    string
	Type    Type         // the entity's type; nil until resolved
	Pos     ast.Position // declaration site
	Decl    ast.Node     // the declaring node; nil for builtins
	Mutable bool         // false for const, a non-mut param, and a fn
	Private bool         // Name starts with '_' (SPEC §15)
	Used    bool         // set on every Lookup, drives unused-variable lints
	Scope   *Scope       // the scope that owns the symbol

	// Kind-specific payload (exactly one is meaningful).
	Named   *Named     // SymType
	Trait   *Trait     // SymTrait
	Variant *Variant   // SymVariant, with Owner set
	Owner   *Named     // SymVariant / SymFunc (method): the receiver type
	Fn      *Fn        // SymFunc / SymBuiltin
	Builtin BuiltinID  // SymBuiltin
	TP      *TypeParam // SymTypeParam
}

// ScopeKind labels a scope so the checker can ask "am I inside a loop / fn /
// impl" without a separate stack.
type ScopeKind int

const (
	ScopeUniverse ScopeKind = iota
	ScopeFile
	ScopeFunc
	ScopeBlock
	ScopeLoop
	ScopeMatchArm
	ScopeImpl
	ScopeTypeParams
)

// Scope is a lexical scope with a parent chain.
type Scope struct {
	Parent   *Scope
	Children []*Scope
	Symbols  map[string]*Symbol
	Order    []string // insertion order, for deterministic diagnostics
	Kind     ScopeKind
	Pos      ast.Position
	End      ast.Position
}

// NewScope creates a child scope of parent. When parent is nil the result is
// a universe scope.
func NewScope(parent *Scope, kind ScopeKind) *Scope {
	s := &Scope{
		Parent:  parent,
		Symbols: make(map[string]*Symbol),
		Kind:    kind,
	}
	if parent != nil {
		parent.Children = append(parent.Children, s)
	}
	return s
}

// Lookup walks the parent chain and marks the symbol Used. It returns the
// symbol and true on success, or (nil, false) when the name is undeclared.
func (s *Scope) Lookup(name string) (*Symbol, bool) {
	for cur := s; cur != nil; cur = cur.Parent {
		if sym, ok := cur.Symbols[name]; ok {
			sym.Used = true
			return sym, true
		}
	}
	return nil, false
}

// LookupLocal searches only this scope and does not mark Used.
func (s *Scope) LookupLocal(name string) (*Symbol, bool) {
	sym, ok := s.Symbols[name]
	return sym, ok
}

// Insert adds sym to this scope. It returns (nil, true) on success. When the
// name is already declared in THIS scope it returns (existing, false) — the
// caller reports the redeclaration (SPEC §3, D53). A name of "_" is never
// inserted and always returns (nil, true).
func (s *Scope) Insert(sym *Symbol) (*Symbol, bool) {
	if sym == nil || sym.Name == "_" {
		return nil, true
	}
	if existing, ok := s.Symbols[sym.Name]; ok {
		return existing, false
	}
	sym.Scope = s
	s.Symbols[sym.Name] = sym
	s.Order = append(s.Order, sym.Name)
	return nil, true
}

// Names returns the locally declared names in insertion order.
func (s *Scope) Names() []string {
	return s.Order
}

// Depth returns the distance to the universe scope.
func (s *Scope) Depth() int {
	d := 0
	for cur := s.Parent; cur != nil; cur = cur.Parent {
		d++
	}
	return d
}

// Enclosing returns the nearest enclosing scope of the given kind, or nil.
func (s *Scope) Enclosing(k ScopeKind) *Scope {
	for cur := s.Parent; cur != nil; cur = cur.Parent {
		if cur.Kind == k {
			return cur
		}
	}
	return nil
}
