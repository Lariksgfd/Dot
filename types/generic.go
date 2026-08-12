package types

import (
	"fmt"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// instantiateCacheKey is the key for the generic instantiation cache.
type instantiateCacheKey struct {
	origin *Named
	args   string
}

// getOrAddCache returns a cached instantiation of named with args, or creates
// a fresh one and caches it under (origin pointer, TypeArgs). This guarantees
// that the same instantiation site always returns the same *Named pointer,
// which is critical for type identity.
func (c *Checker) getOrAddCache(named *Named, args []Type) *Named {
	if c.instantiateCache == nil {
		c.instantiateCache = make(map[instantiateCacheKey]*Named)
	}
	key := instantiateCacheKey{origin: named, args: typeListString(args)}
	if cached, ok := c.instantiateCache[key]; ok {
		return cached
	}
	inst := c.instantiate(named, args)
	if n, ok := inst.(*Named); ok {
		c.instantiateCache[key] = n
		return n
	}
	return nil
}

// typeListString renders a slice of types as a deterministic string for use as
// a cache key.
func typeListString(ts []Type) string {
	if len(ts) == 0 {
		return ""
	}
	parts := make([]string, len(ts))
	for i, t := range ts {
		if t == nil {
			parts[i] = "<nil>"
		} else {
			parts[i] = t.String()
		}
	}
	return strings.Join(parts, ",")
}

// checkBounds verifies that arg implements every trait in tp.Bounds. On
// failure it emits a diagnostic and returns false. pos is the instantiation
// site for the error location.
func checkBounds(tp *TypeParam, arg Type, pos ast.Position) bool {
	for _, tr := range tp.Bounds {
		if !implements(arg, tr) {
			d := globalChecker.errorf(positionNode(pos),
				"%s = %s does not implement %s", tp.Name, arg, tr.Name)
			if len(tr.Methods) > 0 && tr.Methods[0].Pos.Offset > 0 {
				globalChecker.hint(d, fmt.Sprintf(
					"the bound %s is declared at %s:%d",
					tr.Name, tr.Methods[0].Pos.File, tr.Methods[0].Pos.Line))
			}
			return false
		}
	}
	return true
}

// positionNode wraps an ast.Position so it satisfies ast.Node for error
// reporting.
type positionNode ast.Position

func (n positionNode) Pos() ast.Position { return ast.Position(n) }

func (n positionNode) End() ast.Position {
	return ast.Position{File: n.File, Line: n.Line, Column: n.Column + 1, Offset: n.Offset + 1}
}
func (n positionNode) SetDoc(doc []lexer.Token)    {}
func (n positionNode) GetDoc() []lexer.Token       { return nil }
func (n positionNode) SetComment(c []lexer.Token) {}
func (n positionNode) GetComment() []lexer.Token { return nil }

// checkObjectSafe verifies that tr may be used as `dyn Tr`. `dyn Tr` is legal
// iff every method of Tr: (1) has a self receiver, (2) does not mention Self
// except as the receiver, and (3) is not generic. On the first violation it
// emits a diagnostic and returns false.
func checkObjectSafe(tr *Trait, pos ast.Position) bool {
	selfParam := &TypeParam{Name: "Self"}
	for i := range tr.Methods {
		m := &tr.Methods[i]
		if m.Static {
			globalChecker.errorf(positionNode(pos),
				"%s cannot be used as 'dyn' because '%s' is a static method",
				tr.Name, m.Name)
			return false
		}
		if mentionsSelf(m.Sig, selfParam) {
			globalChecker.errorf(positionNode(pos),
				"%s cannot be used as 'dyn' because '%s' takes a 'Self' parameter",
				tr.Name, m.Name)
			return false
		}
		if len(m.Sig.TypeParams) > 0 {
			globalChecker.errorf(positionNode(pos),
				"%s cannot be used as 'dyn' because '%s' is generic",
				tr.Name, m.Name)
			return false
		}
	}
	return true
}

// mentionsSelf reports whether sig references selfParam anywhere except as the
// receiver.
func mentionsSelf(sig *Fn, selfParam *TypeParam) bool {
	if sig == nil {
		return false
	}
	for _, p := range sig.Params {
		if typeContainsParam(p.Type, selfParam) {
			return true
		}
	}
	return typeContainsParam(sig.Result, selfParam)
}

// typeContainsParam reports whether t references param anywhere structurally.
// A type parameter matches by identity or by name, so that the Self
// placeholder embedded in trait signatures (a fresh pointer per trait) is
// recognised even though it is not pointer-identical to the probe.
func typeContainsParam(t Type, param *TypeParam) bool {
	if t == nil || param == nil {
		return false
	}
	if tp, ok := t.(*TypeParam); ok {
		return tp == param || tp.Name == param.Name
	}
	switch x := t.(type) {
	case *Slice:
		return typeContainsParam(x.Elem, param)
	case *Array:
		return typeContainsParam(x.Elem, param)
	case *Map:
		return typeContainsParam(x.Key, param) || typeContainsParam(x.Value, param)
	case *Pointer:
		return typeContainsParam(x.Elem, param)
	case *Weak:
		return typeContainsParam(x.Elem, param)
	case *Chan:
		return typeContainsParam(x.Elem, param)
	case *Future:
		return typeContainsParam(x.Result, param)
	case *Tuple:
		for _, e := range x.Elems {
			if typeContainsParam(e, param) {
				return true
			}
		}
		return false
	case *Fn:
		for _, p := range x.Params {
			if typeContainsParam(p.Type, param) {
				return true
			}
		}
		return typeContainsParam(x.Result, param)
	case *Struct:
		for _, f := range x.Fields {
			if typeContainsParam(f.Type, param) {
				return true
			}
		}
		return false
	case *Enum:
		for _, v := range x.Variants {
			for _, f := range v.Fields {
				if typeContainsParam(f.Type, param) {
					return true
				}
			}
		}
		return false
	case *Named:
		for _, a := range x.TypeArgs {
			if typeContainsParam(a, param) {
				return true
			}
		}
		return false
	}
	return false
}

// instanceKey builds a deterministic, collision-free mangled name for a
// generic instantiation: base + "__" + each arg's mangled name joined by "_".
func instanceKey(base string, args []Type) string {
	var b strings.Builder
	b.WriteString(base)
	b.WriteString("__")
	for i, a := range args {
		if i > 0 {
			b.WriteString("_")
		}
		b.WriteString(mangleType(a))
	}
	return b.String()
}

// mangleType returns a valid C identifier spelling of t for use in
// instanceKey.
func mangleType(t Type) string {
	if t == nil {
		return "?"
	}
	switch x := t.(type) {
	case *Basic:
		return x.Name()
	case *Slice:
		return "Slice_" + mangleType(x.Elem)
	case *Array:
		return fmt.Sprintf("Array_%d_%s", x.Len, mangleType(x.Elem))
	case *Map:
		return "Map_" + mangleType(x.Key) + "_" + mangleType(x.Value)
	case *Pointer:
		return "Ptr_" + mangleType(x.Elem)
	case *Weak:
		return "Weak_" + mangleType(x.Elem)
	case *Chan:
		return "Chan_" + mangleType(x.Elem)
	case *Future:
		return "Fut_" + mangleType(x.Result)
	case *Tuple:
		parts := make([]string, len(x.Elems))
		for i, e := range x.Elems {
			parts[i] = mangleType(e)
		}
		return "Tup_" + strings.Join(parts, "_")
	case *Fn:
		parts := make([]string, len(x.Params))
		for i, p := range x.Params {
			parts[i] = mangleType(p.Type)
		}
		return "Fn_" + strings.Join(parts, "_") + "_" + mangleType(x.Result)
	case *Named:
		if x.Origin != nil {
			return instanceKey(x.Origin.Name, x.TypeArgs)
		}
		if len(x.TypeArgs) > 0 {
			return instanceKey(x.Name, x.TypeArgs)
		}
		return x.Name
	case *TypeParam:
		return x.Name
	case *Trait:
		return x.Name
	case *Dyn:
		return "dyn_" + x.Trait.Name
	case *Struct:
		return "struct"
	case *Enum:
		return "enum"
	default:
		return strings.ReplaceAll(t.String(), " ", "_")
	}
}

// recordInstance adds inst to Info.Instances and Info.InstanceList, keyed by
// Mangled for deduplication. The first occurrence wins; later duplicates are
// dropped so InstanceList stays deterministic.
func (c *Checker) recordInstance(inst *Instance) *Instance {
	if inst == nil {
		return nil
	}
	if c.info.Instances == nil {
		c.info.Instances = make(map[ast.Expr]*Instance)
	}
	if existing, ok := c.info.instanceByMangled[inst.Mangled]; ok {
		return existing
	}
	if c.info.instanceByMangled == nil {
		c.info.instanceByMangled = make(map[string]*Instance)
	}
	c.info.instanceByMangled[inst.Mangled] = inst
	c.info.InstanceList = append(c.info.InstanceList, inst)
	return inst
}

// substituteSelf replaces every occurrence of a Self type param in t with
// concrete. It is used when a trait default method is used on a concrete
// type.
func substituteSelf(t Type, selfParam *TypeParam, concrete Type) Type {
	if t == nil {
		return nil
	}
	if tp, ok := t.(*TypeParam); ok && tp == selfParam {
		return concrete
	}
	switch x := t.(type) {
	case *Slice:
		return &Slice{Elem: substituteSelf(x.Elem, selfParam, concrete)}
	case *Array:
		return &Array{Elem: substituteSelf(x.Elem, selfParam, concrete), Len: x.Len}
	case *Map:
		return &Map{Key: substituteSelf(x.Key, selfParam, concrete), Value: substituteSelf(x.Value, selfParam, concrete)}
	case *Pointer:
		return &Pointer{Elem: substituteSelf(x.Elem, selfParam, concrete)}
	case *Weak:
		return &Weak{Elem: substituteSelf(x.Elem, selfParam, concrete)}
	case *Chan:
		return &Chan{Elem: substituteSelf(x.Elem, selfParam, concrete)}
	case *Future:
		return &Future{Result: substituteSelf(x.Result, selfParam, concrete)}
	case *Tuple:
		elems := make([]Type, len(x.Elems))
		for i, e := range x.Elems {
			elems[i] = substituteSelf(e, selfParam, concrete)
		}
		return &Tuple{Elems: elems}
	case *Fn:
		out := &Fn{
			Result:   substituteSelf(x.Result, selfParam, concrete),
			Recv:     substituteSelf(x.Recv, selfParam, concrete),
			RecvMut:  x.RecvMut,
			Async:    x.Async,
			Variadic: x.Variadic,
		}
		out.Params = make([]Param, len(x.Params))
		for i, p := range x.Params {
			q := p
			q.Type = substituteSelf(p.Type, selfParam, concrete)
			out.Params[i] = q
		}
		return out
	case *Struct:
		out := &Struct{Fields: make([]Field, len(x.Fields))}
		for i, f := range x.Fields {
			g := f
			g.Type = substituteSelf(f.Type, selfParam, concrete)
			out.Fields[i] = g
		}
		out.Flat, out.Ambiguous = flattenFields(out)
		return out
	case *Enum:
		out := &Enum{Variants: make([]Variant, len(x.Variants))}
		for i, v := range x.Variants {
			w := v
			w.Fields = make([]Param, len(v.Fields))
			for j, f := range v.Fields {
				g := f
				g.Type = substituteSelf(f.Type, selfParam, concrete)
				w.Fields[j] = g
			}
			out.Variants[i] = w
		}
		out.byName = variantByName(out)
		return out
	case *Named:
		args := make([]Type, len(x.TypeArgs))
		for i, a := range x.TypeArgs {
			args[i] = substituteSelf(a, selfParam, concrete)
		}
		return &Named{
			Name:       x.Name,
			Sym:        x.Sym,
			Origin:     x.Origin,
			Underlying: substituteSelf(x.Underlying, selfParam, concrete),
			TypeArgs:   args,
			Methods:    x.Methods,
			Impls:      x.Impls,
			Pos:        x.Pos,
		}
	}
	return t
}
