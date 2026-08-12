// Package llvm implements the LLVM IR backend for Dot.
//
// This file holds enum (sum type) codegen for part 3: tagged-union type
// declarations with real payload types, variant constructors, match arms
// with payload destructuring, and the Option/Result builtin methods.
package llvm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/types"
)

// enumVariantLayout describes where one enum variant's payload lives inside
// the LLVM struct: FieldIdx are struct indices, FieldTypes the stored LLVM
// types and FieldValueTypes the logical types (used when binding match
// patterns to the payload).
type enumVariantLayout struct {
	Tag             int
	FieldTypes      []string
	FieldValueTypes []string
	FieldIdx        []int
}

// enumFieldInfo is one struct field of an enum's LLVM representation.
type enumFieldInfo struct {
	Idx  int
	Type string
}

// enumPayloadFieldLLVMType coerces a payload field's LLVM type for storage:
// named aggregates and slice headers are stored as pointers, everything else
// (scalars, strings) inline.
func enumPayloadFieldLLVMType(t string) string {
	if isNamedAgg(t) || t == "{ ptr, i32, i32 }" {
		return "ptr"
	}
	return t
}

// emitEnumDecl emits the LLVM type for an AST-declared enum: a struct with a
// leading i32 tag followed by the concatenated payload fields of every
// variant (mirrors the C backend's tag + union layout).
func (e *emitter) emitEnumDecl(ed *ast.EnumDecl) {
	variantMap := make(map[string]int)
	layouts := make(map[string]enumVariantLayout)
	fields := []string{"i32"}
	fieldInfos := []enumFieldInfo{{Idx: 0, Type: "i32"}}
	idx := 1
	for i, v := range ed.Variants {
		variantMap[v.Name] = i
		l := enumVariantLayout{Tag: i}
		for _, f := range v.Fields {
			vt := e.resolveType(f.Type)
			st := enumPayloadFieldLLVMType(vt)
			l.FieldTypes = append(l.FieldTypes, st)
			l.FieldValueTypes = append(l.FieldValueTypes, vt)
			l.FieldIdx = append(l.FieldIdx, idx)
			fields = append(fields, st)
			fieldInfos = append(fieldInfos, enumFieldInfo{Idx: idx, Type: st})
			idx++
		}
		layouts[v.Name] = l
	}
	e.enumVariants[ed.Name] = variantMap
	e.enumLayouts[ed.Name] = layouts
	e.enumFieldList[ed.Name] = fieldInfos
	e.emit("%%%s = type { %s }", ed.Name, strings.Join(fields, ", "))
}

// emitInstantiatedEnumDecl emits the LLVM type for an instantiated generic
// enum (Option[int], Result[int, string], ...). name is the mangled LLVM type
// name without the '%' prefix.
func (e *emitter) emitInstantiatedEnumDecl(name string, en *types.Enum) {
	if _, done := e.enumLayouts[name]; done {
		return
	}
	variantMap := make(map[string]int)
	layouts := make(map[string]enumVariantLayout)
	fields := []string{"i32"}
	fieldInfos := []enumFieldInfo{{Idx: 0, Type: "i32"}}
	idx := 1
	for i, v := range en.Variants {
		variantMap[v.Name] = i
		l := enumVariantLayout{Tag: i}
		for _, f := range v.Fields {
			vt := e.llvmType(f.Type)
			st := enumPayloadFieldLLVMType(vt)
			l.FieldTypes = append(l.FieldTypes, st)
			l.FieldValueTypes = append(l.FieldValueTypes, vt)
			l.FieldIdx = append(l.FieldIdx, idx)
			fields = append(fields, st)
			fieldInfos = append(fieldInfos, enumFieldInfo{Idx: idx, Type: st})
			idx++
		}
		layouts[v.Name] = l
	}
	e.enumVariants[name] = variantMap
	e.enumLayouts[name] = layouts
	e.enumFieldList[name] = fieldInfos
	e.emit("%%%s = type { %s }", name, strings.Join(fields, ", "))
}

// typeIsConcrete reports whether a type has no unresolved type parameters,
// so it can be used for a storage layout.
func typeIsConcrete(t types.Type) bool {
	switch x := t.(type) {
	case *types.TypeParam:
		return false
	case *types.TypeVar:
		if x.Bound == nil {
			return false
		}
		return typeIsConcrete(x.Bound)
	case *types.Named:
		for _, a := range x.TypeArgs {
			if !typeIsConcrete(a) {
				return false
			}
		}
		return true
	case *types.Slice:
		return typeIsConcrete(x.Elem)
	case *types.Array:
		return typeIsConcrete(x.Elem)
	case *types.Pointer:
		return typeIsConcrete(x.Elem)
	case *types.Fn:
		for _, p := range x.Params {
			if !typeIsConcrete(p.Type) {
				return false
			}
		}
		return typeIsConcrete(x.Result)
	}
	return true
}

// enumIsConcrete reports whether every variant payload field of en is fully
// instantiated.
func enumIsConcrete(en *types.Enum) bool {
	for _, v := range en.Variants {
		for _, f := range v.Fields {
			if !typeIsConcrete(f.Type) {
				return false
			}
		}
	}
	return true
}

// collectInstantiatedEnumDecls scans the checker's recorded types for
// instantiated generic enums and emits their LLVM declarations, in sorted
// order for determinism. Declarations must come before any use. When several
// type objects share a mangled name, the fully instantiated one wins because
// it carries the concrete payload layouts.
func (e *emitter) collectInstantiatedEnumDecls() {
	if e.info == nil {
		return
	}
	found := make(map[string]*types.Enum)
	for _, t := range e.info.Types {
		n, ok := t.(*types.Named)
		if !ok || len(n.TypeArgs) == 0 {
			continue
		}
		en, ok := types.Underlying(n).(*types.Enum)
		if !ok {
			continue
		}
		name := strings.TrimPrefix(namedLLVMType(n), "%")
		if _, exists := e.enumLayouts[name]; exists {
			continue
		}
		if prev, ok := found[name]; !ok || (enumIsConcrete(en) && !enumIsConcrete(prev)) {
			found[name] = en
		}
	}
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		e.emitInstantiatedEnumDecl(name, found[name])
	}
}

// enumNameOfExpr resolves the LLVM enum type name (without '%') of an
// expression whose value is an enum, or "".
func (e *emitter) enumNameOfExpr(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	if e.info != nil {
		t := e.info.TypeOf(expr)
		if t != nil {
			if _, ok := types.Underlying(t).(*types.Enum); ok {
				if n, ok := t.(*types.Named); ok {
					return strings.TrimPrefix(namedLLVMType(n), "%")
				}
			}
		}
	}
	switch x := expr.(type) {
	case *ast.FieldExpr:
		if ident, ok := x.X.(*ast.Ident); ok {
			if vmap, ok := e.enumVariants[ident.Name]; ok {
				if _, ok := vmap[x.Name]; ok {
					return ident.Name
				}
			}
		}
	case *ast.Ident:
		for en, vmap := range e.enumVariants {
			if _, ok := vmap[x.Name]; ok {
				return en
			}
		}
	case *ast.CallExpr:
		return e.enumNameOfExpr(x.Fn)
	}
	return ""
}

// variantTag returns the discriminant of variantName in enumName, 0 when the
// enum is unknown.
func (e *emitter) variantTag(enumName, variantName string) int {
	if vmap, ok := e.enumLayouts[enumName]; ok {
		if l, ok := vmap[variantName]; ok {
			return l.Tag
		}
	}
	if vmap, ok := e.enumVariants[enumName]; ok {
		if tag, ok := vmap[variantName]; ok {
			return tag
		}
	}
	return 0
}

// emitEnumLit emits an enum literal: an alloca of the enum type, a tag store
// and a payload store per constructor argument. It returns the alloca pointer
// (enums flow as pointers in this backend).
func (e *emitter) emitEnumLit(enumName, variantName string, args []ast.Expr) string {
	if enumName == "" {
		return "0"
	}
	tmp := e.nextTmp()
	e.emit("  %s = alloca %%%s", tmp, enumName)

	tagPtr := e.nextTmp()
	e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 0", tagPtr, enumName, tmp)
	e.emit("  store i32 %d, ptr %s", e.variantTag(enumName, variantName), tagPtr)

	var layout enumVariantLayout
	if vmap, ok := e.enumLayouts[enumName]; ok {
		layout = vmap[variantName]
	}
	for i, arg := range args {
		val := e.emitExpr(arg)
		ft := "i64"
		idx := i + 1
		vt := "i64"
		if i < len(layout.FieldTypes) {
			ft = layout.FieldTypes[i]
			idx = layout.FieldIdx[i]
			vt = layout.FieldValueTypes[i]
		}
		if ft == "ptr" && isNamedAgg(vt) {
			// The payload aggregate lives on the stack; copy it to the
			// heap so the enum keeps owning it after the current frame
			// returns.
			val = e.materializeAgg(strings.TrimPrefix(vt, "%"), val)
		}
		fieldPtr := e.nextTmp()
		e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", fieldPtr, enumName, tmp, idx)
		e.emit("  store %s %s, ptr %s", ft, val, fieldPtr)
	}
	return tmp
}

// emitEnumTag loads the discriminant of an enum value (a pointer to the
// enum alloca/heap object) and returns the i32 register.
func (e *emitter) emitEnumTag(enumName, enumPtr string) string {
	tagPtr := e.nextTmp()
	e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 0", tagPtr, enumName, enumPtr)
	tagVal := e.nextTmp()
	e.emit("  %s = load i32, ptr %s", tagVal, tagPtr)
	return tagVal
}

// enumBindings holds a deferred payload binding for one match arm.
type enumBindings struct {
	enumName   string
	variant    string
	args       []ast.Pattern
	subjectPtr string
}

// emitEnumBindings loads the payload fields of variantName from subjectPtr
// and binds each sub-pattern (IdentPattern or wildcard) to an alloca.
func (e *emitter) emitEnumBindings(b *enumBindings) {
	if b == nil || b.enumName == "" {
		return
	}
	var layout enumVariantLayout
	if vmap, ok := e.enumLayouts[b.enumName]; ok {
		layout = vmap[b.variant]
	}
	for i, p := range b.args {
		ip, ok := p.(*ast.IdentPattern)
		if !ok {
			continue
		}
		ft := "i64"
		idx := i + 1
		vt := "i64"
		if i < len(layout.FieldTypes) {
			ft = layout.FieldTypes[i]
			idx = layout.FieldIdx[i]
			vt = layout.FieldValueTypes[i]
		}
		fieldPtr := e.nextTmp()
		e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", fieldPtr, b.enumName, b.subjectPtr, idx)
		val := e.nextTmp()
		e.emit("  %s = load %s, ptr %s", val, ft, fieldPtr)
		allocName := e.bindName(ip.Name, isNamedAgg(vt), vt, ft)
		e.emit("  store %s %s, ptr %%%s", ft, val, allocName)
	}
}

// matchEnumNameFromPatterns recovers the enum name from qualified variant
// patterns when the checker is unavailable (unit-test fallback).
func (e *emitter) matchEnumNameFromPatterns(me *ast.MatchExpr) string {
	for _, arm := range me.Arms {
		pat := arm.Pattern
		if gp, ok := pat.(*ast.GuardedPattern); ok {
			pat = gp.Pattern
		}
		if ep, ok := pat.(*ast.EnumPattern); ok {
			if ep.Enum != "" {
				return ep.Enum
			}
		}
	}
	return ""
}

// emitMatchExpr emits a match expression. For enum subjects it loads the tag
// and branches per arm, binding the payload sub-patterns before each arm
// body. Literal/ident/wildcard arms behave as before.
func (e *emitter) emitMatchExpr(me *ast.MatchExpr) string {
	enumName := ""
	if e.info != nil {
		if _, ok := types.Underlying(e.info.TypeOf(me.Subject)).(*types.Enum); ok {
			enumName = e.enumNameOfExpr(me.Subject)
		}
	}
	if enumName == "" {
		enumName = e.matchEnumNameFromPatterns(me)
	}

	subjectPtr := e.emitExpr(me.Subject)

	matchType := e.exprType(me)
	if matchType == "void" {
		matchType = "i64"
	}
	if isNamedAgg(matchType) {
		matchType = "ptr"
	}

	resPtr := "%" + e.nextLabel("match.res")
	e.emit("  %s = alloca %s", resPtr, matchType)

	endBlock := e.nextLabel("match.end")
	failBlock := e.nextLabel("match.default")

	var armLabels []string
	for i := range me.Arms {
		armLabels = append(armLabels, e.nextLabel(fmt.Sprintf("match.arm%d", i)))
	}
	nextLabels := make([]string, len(me.Arms))
	for i := range me.Arms {
		nextLabels[i] = e.nextLabel(fmt.Sprintf("match.next%d", i))
	}

	var tagVal string
	tagType := "i32"
	if enumName != "" {
		tagVal = e.emitEnumTag(enumName, subjectPtr)
	} else {
		tagVal = subjectPtr
		tagType = e.exprType(me.Subject)
	}

	pendingBindings := make([]*enumBindings, len(me.Arms))

	for i, arm := range me.Arms {
		func() {
			saved := e.snapshotScope()
			defer e.restoreScope(saved)

			if i == 0 {
				e.emit("  br label %%%s", armLabels[0])
			}

			e.emit("\n%s:", armLabels[i])
			pat := arm.Pattern
			var guardExpr ast.Expr
			if gp, ok := pat.(*ast.GuardedPattern); ok {
				pat = gp.Pattern
				guardExpr = gp.Guard
			}

			var branchCond string

			if ep, ok := pat.(*ast.EnumPattern); ok {
				epEnumName := ep.Enum
				if epEnumName == "" {
					epEnumName = enumName
				}
				cmpTmp := e.nextTmp()
				e.emit("  %s = icmp eq i32 %s, %d", cmpTmp, tagVal, e.variantTag(epEnumName, ep.Variant))
				branchCond = cmpTmp
				if len(ep.Args) > 0 {
					b := &enumBindings{enumName: epEnumName, variant: ep.Variant, args: ep.Args, subjectPtr: subjectPtr}
					if guardExpr != nil {
						// The guard reads the payload, so bind before it.
						e.emitEnumBindings(b)
					} else {
						pendingBindings[i] = b
					}
				}
			} else if lp, ok := pat.(*ast.LiteralPattern); ok {
				litVal := e.emitExpr(lp.Value)
				if tagType == "{ ptr, i32 }" {
					branchCond = e.emitStringEq(tagVal, litVal)
				} else {
					cmpTmp := e.nextTmp()
					e.emit("  %s = icmp eq %s %s, %s", cmpTmp, tagType, tagVal, litVal)
					branchCond = cmpTmp
				}
			} else if _, ok := pat.(*ast.WildcardPattern); ok {
				bodyVal := e.emitExpr(arm.Body)
				e.emit("  store %s %s, ptr %s", matchType, bodyVal, resPtr)
				e.emit("  br label %%%s", endBlock)
				return
			} else if ip, ok := pat.(*ast.IdentPattern); ok {
				allocName := e.bindName(ip.Name, false, tagType, tagType)
				e.emit("  store %s %s, ptr %%%s", tagType, tagVal, allocName)
				if guardExpr != nil {
					guardVal := e.emitExpr(guardExpr)
					branchCond = guardVal
					if i+1 < len(armLabels) {
						e.emit("  br i1 %s, label %%%s, label %%%s", branchCond, nextLabels[i], armLabels[i+1])
					} else {
						e.emit("  br i1 %s, label %%%s, label %%%s", branchCond, nextLabels[i], failBlock)
					}
					e.emit("\n%s:", nextLabels[i])
				}
				bodyVal := e.emitExpr(arm.Body)
				e.emit("  store %s %s, ptr %s", matchType, bodyVal, resPtr)
				e.emit("  br label %%%s", endBlock)
				return
			} else {
				bodyVal := e.emitExpr(arm.Body)
				e.emit("  store %s %s, ptr %s", matchType, bodyVal, resPtr)
				e.emit("  br label %%%s", endBlock)
				return
			}

			if guardExpr != nil {
				guardVal := e.emitExpr(guardExpr)
				andTmp := e.nextTmp()
				e.emit("  %s = and i1 %s, %s", andTmp, branchCond, guardVal)
				branchCond = andTmp
			}

			if i+1 < len(armLabels) {
				e.emit("  br i1 %s, label %%%s, label %%%s", branchCond, nextLabels[i], armLabels[i+1])
			} else {
				e.emit("  br i1 %s, label %%%s, label %%%s", branchCond, nextLabels[i], failBlock)
			}

			e.emit("\n%s:", nextLabels[i])
			if pendingBindings[i] != nil {
				e.emitEnumBindings(pendingBindings[i])
			}
			bodyVal := e.emitExpr(arm.Body)
			e.emit("  store %s %s, ptr %s", matchType, bodyVal, resPtr)
			e.emit("  br label %%%s", endBlock)
		}()
	}

	e.emit("\n%s:", failBlock)
	e.emit("  store %s %s, ptr %s", matchType, zeroConst(matchType), resPtr)
	e.emit("  br label %%%s", endBlock)

	e.emit("\n%s:", endBlock)
	tmp := e.nextTmp()
	e.emit("  %s = load %s, ptr %s", tmp, matchType, resPtr)
	return tmp
}

// emitBuiltinMethodCall emits a builtin method call on an Option or Result
// receiver. Methods map directly to tag checks and payload loads.
func (e *emitter) emitBuiltinMethodCall(ex *ast.CallExpr, fe *ast.FieldExpr) string {
	if e.info == nil {
		return "0"
	}
	recvT := e.info.TypeOf(fe.X)
	n, ok := recvT.(*types.Named)
	if !ok {
		return "0"
	}
	if n.Name == "Option" && len(n.TypeArgs) == 1 {
		return e.emitOptionMethodCall(ex, fe, n)
	}
	if n.Name == "Result" && len(n.TypeArgs) == 2 {
		return e.emitResultMethodCall(ex, fe, n)
	}
	return "0"
}

// optionPayloadInfo returns the stored field type and struct index of the
// Some payload for the given Option instantiation.
func (e *emitter) optionPayloadInfo(enumName string) (string, int) {
	if vmap, ok := e.enumLayouts[enumName]; ok {
		if l, ok := vmap["Some"]; ok && len(l.FieldTypes) > 0 {
			return l.FieldTypes[0], l.FieldIdx[0]
		}
	}
	return "i64", 1
}

func (e *emitter) emitOptionMethodCall(ex *ast.CallExpr, fe *ast.FieldExpr, n *types.Named) string {
	enumName := strings.TrimPrefix(namedLLVMType(n), "%")
	recv := e.emitExpr(fe.X)
	tagVal := e.emitEnumTag(enumName, recv)

	switch fe.Name {
	case "is_some":
		tmp := e.nextTmp()
		e.emit("  %s = icmp eq i32 %s, 0", tmp, tagVal)
		return tmp
	case "is_none":
		tmp := e.nextTmp()
		e.emit("  %s = icmp eq i32 %s, 1", tmp, tagVal)
		return tmp
	case "unwrap":
		ft, idx := e.optionPayloadInfo(enumName)
		return e.emitUnwrap(enumName, recv, tagVal, 0, ft, idx, fe.Name)
	case "unwrap_or":
		ft, idx := e.optionPayloadInfo(enumName)
		if len(ex.Args) > 0 {
			return e.emitUnwrapOr(enumName, recv, tagVal, 0, ft, idx, ex.Args[0].Value)
		}
	}
	return "0"
}

func (e *emitter) emitResultMethodCall(ex *ast.CallExpr, fe *ast.FieldExpr, n *types.Named) string {
	enumName := strings.TrimPrefix(namedLLVMType(n), "%")
	recv := e.emitExpr(fe.X)
	tagVal := e.emitEnumTag(enumName, recv)

	payloadInfo := func(variant string) (string, int) {
		if vmap, ok := e.enumLayouts[enumName]; ok {
			if l, ok := vmap[variant]; ok && len(l.FieldTypes) > 0 {
				return l.FieldTypes[0], l.FieldIdx[0]
			}
		}
		return "i64", 1
	}

	switch fe.Name {
	case "is_ok":
		tmp := e.nextTmp()
		e.emit("  %s = icmp eq i32 %s, 0", tmp, tagVal)
		return tmp
	case "is_err":
		tmp := e.nextTmp()
		e.emit("  %s = icmp eq i32 %s, 1", tmp, tagVal)
		return tmp
	case "unwrap":
		ft, idx := payloadInfo("Ok")
		return e.emitUnwrap(enumName, recv, tagVal, 0, ft, idx, fe.Name)
	case "unwrap_err":
		ft, idx := payloadInfo("Err")
		return e.emitUnwrap(enumName, recv, tagVal, 1, ft, idx, fe.Name)
	case "unwrap_or":
		ft, idx := payloadInfo("Ok")
		if len(ex.Args) > 0 {
			return e.emitUnwrapOr(enumName, recv, tagVal, 0, ft, idx, ex.Args[0].Value)
		}
	}
	return "0"
}

// emitUnwrap branches on the tag: the payload is loaded when the tag equals
// wantTag, otherwise the runtime unwrap panic runs.
func (e *emitter) emitUnwrap(enumName, recv, tagVal string, wantTag int, ftype string, idx int, method string) string {
	okBlock := e.nextLabel("unwrap.ok")
	badBlock := e.nextLabel("unwrap.bad")
	endBlock := e.nextLabel("unwrap.end")

	cond := e.nextTmp()
	e.emit("  %s = icmp eq i32 %s, %d", cond, tagVal, wantTag)
	e.emit("  br i1 %s, label %%%s, label %%%s", cond, okBlock, badBlock)

	e.emit("\n%s:", okBlock)
	fieldPtr := e.nextTmp()
	e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", fieldPtr, enumName, recv, idx)
	val := e.nextTmp()
	e.emit("  %s = load %s, ptr %s", val, ftype, fieldPtr)
	e.emit("  br label %%%s", endBlock)

	e.emit("\n%s:", badBlock)
	e.emit("  call void @dot_unwrap_panic()")
	e.emit("  unreachable")

	e.emit("\n%s:", endBlock)
	res := e.nextTmp()
	e.emit("  %s = phi %s [ %s, %%%s ]", res, ftype, val, okBlock)
	return res
}

// emitUnwrapOr selects the payload when the tag equals wantTag and the
// default argument otherwise.
func (e *emitter) emitUnwrapOr(enumName, recv, tagVal string, wantTag int, ftype string, idx int, dfltExpr ast.Expr) string {
	payloadBlock := e.nextLabel("unworp.some")
	dfltBlock := e.nextLabel("unworp.none")
	endBlock := e.nextLabel("unworp.end")

	cond := e.nextTmp()
	e.emit("  %s = icmp eq i32 %s, %d", cond, tagVal, wantTag)
	e.emit("  br i1 %s, label %%%s, label %%%s", cond, payloadBlock, dfltBlock)

	e.emit("\n%s:", payloadBlock)
	fieldPtr := e.nextTmp()
	e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", fieldPtr, enumName, recv, idx)
	val := e.nextTmp()
	e.emit("  %s = load %s, ptr %s", val, ftype, fieldPtr)
	e.emit("  br label %%%s", endBlock)

	e.emit("\n%s:", dfltBlock)
	dflt := e.emitExpr(dfltExpr)
	e.emit("  br label %%%s", endBlock)

	e.emit("\n%s:", endBlock)
	res := e.nextTmp()
	e.emit("  %s = phi %s [ %s, %%%s ], [ %s, %%%s ]", res, ftype, val, payloadBlock, dflt, dfltBlock)
	return res
}
