// Package types defines the type system of the Dot language: the Kind
// classification, the Type interface, every primitive (Basic) type as a
// canonical singleton, composite and named types, and the predicates that
// relate them (identity, assignability, convertibility, ...).
//
// It imports ast and lexer and the standard library only; it never imports
// parser or codegen, so those packages may import types without a cycle.
package types

import (
	"fmt"
	"strings"
)

// Kind classifies a Type. Basic kinds come first so `k < KindVoid` is a
// cheap "is a basic type" test.
type Kind int

const (
	// KindInvalid is the poison kind; produced after an error, never reported
	// twice (D50).
	KindInvalid Kind = iota

	// --- basic, ordered so the numeric ranges below are contiguous ---
	KindBool
	KindInt
	KindInt8
	KindInt16
	KindInt32
	KindUint
	KindUint8
	KindUint16
	KindUint32
	KindFloat
	KindFloat32
	KindRune // distinct type, C repr int32_t
	KindString

	// --- untyped literal kinds (D51) ---
	KindUntypedInt
	KindUntypedFloat
	KindUntypedBool
	KindUntypedString
	KindUntypedNil

	// KindVoid is the unit/void type: what a bodyless statement or a void fn
	// yields.
	KindVoid
	// KindNever is the diverging type: panic(), an infinite loop, a
	// return/break path.
	KindNever

	// --- composite ---
	KindSlice
	KindArray
	KindMap
	KindTuple
	KindFn
	KindPointer
	KindDyn
	KindWeak
	KindChan
	KindFuture
	KindStruct
	KindEnum
	KindTrait
	KindNamed
	KindTypeVar
	KindTypeParam
)

// kindNames maps every Kind to its source-level spelling without the "Kind"
// prefix. It is indexed directly by the Kind value.
var kindNames = [KindTypeParam + 1]string{
	KindInvalid: "invalid type",

	KindBool:    "bool",
	KindInt:     "int",
	KindInt8:    "int8",
	KindInt16:   "int16",
	KindInt32:   "int32",
	KindUint:    "uint",
	KindUint8:   "uint8",
	KindUint16:  "uint16",
	KindUint32:  "uint32",
	KindFloat:   "float",
	KindFloat32: "float32",
	KindRune:    "rune",
	KindString:  "string",

	KindUntypedInt:    "untyped int",
	KindUntypedFloat:  "untyped float",
	KindUntypedBool:   "untyped bool",
	KindUntypedString: "untyped string",
	KindUntypedNil:    "nil",

	KindVoid:  "void",
	KindNever: "never",

	KindSlice:     "slice",
	KindArray:     "array",
	KindMap:       "map",
	KindTuple:     "tuple",
	KindFn:        "fn",
	KindPointer:   "pointer",
	KindDyn:       "dyn",
	KindWeak:      "weak",
	KindChan:      "chan",
	KindFuture:    "future",
	KindStruct:    "struct",
	KindEnum:      "enum",
	KindTrait:     "trait",
	KindNamed:     "named",
	KindTypeVar:   "typevar",
	KindTypeParam: "typeparam",
}

// String returns the constant name without the "Kind" prefix ("int", "slice").
func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// Type is the interface implemented by every Dot type.
type Type interface {
	// Kind returns the type's classification.
	Kind() Kind
	// String returns the source-level spelling used in diagnostics
	// ("[]int", "map[string]Option[User]", "fn(int, int) -> int").
	String() string
}

// Basic is a primitive type. There is exactly ONE canonical *Basic value per
// Kind, held in package-level variables, so `a == b` is a valid identity test
// for basics (D49).
type Basic struct {
	kind Kind
	name string
}

// Kind returns the Basic's kind.
func (b *Basic) Kind() Kind { return b.kind }

// String returns the Basic's source spelling.
func (b *Basic) String() string { return b.name }

// Name returns the Basic's source spelling. It is identical to String() and
// exists so call sites that want to emphasise "the name of a basic type" read
// clearly.
func (b *Basic) Name() string { return b.name }

// The canonical basic types. `byte` is an ALIAS of Uint8 (the same pointer),
// per SPEC §2; `rune` is its own type.
var (
	Invalid = &Basic{KindInvalid, "invalid type"}
	Bool    = &Basic{KindBool, "bool"}
	Int     = &Basic{KindInt, "int"}
	Int8    = &Basic{KindInt8, "int8"}
	Int16   = &Basic{KindInt16, "int16"}
	Int32   = &Basic{KindInt32, "int32"}
	Uint    = &Basic{KindUint, "uint"}
	Uint8   = &Basic{KindUint8, "uint8"}
	Uint16  = &Basic{KindUint16, "uint16"}
	Uint32  = &Basic{KindUint32, "uint32"}
	Byte    = Uint8 // alias, same pointer (SPEC §2)
	Float   = &Basic{KindFloat, "float"}
	Float32 = &Basic{KindFloat32, "float32"}
	Rune    = &Basic{KindRune, "rune"}
	String_ = &Basic{KindString, "string"} // exported as `String_` to avoid
	// clashing with the String() method convention; see D50

	UntypedInt    = &Basic{KindUntypedInt, "untyped int"}
	UntypedFloat  = &Basic{KindUntypedFloat, "untyped float"}
	UntypedBool   = &Basic{KindUntypedBool, "untyped bool"}
	UntypedString = &Basic{KindUntypedString, "untyped string"}
	UntypedNil    = &Basic{KindUntypedNil, "nil"}

	Void  = &Basic{KindVoid, "void"}
	Never = &Basic{KindNever, "never"}
)

// IsBasic reports whether t is a *Basic.
func IsBasic(t Type) bool {
	_, ok := t.(*Basic)
	return ok
}

// IsNumeric reports whether t is an integer, float or rune type, including the
// untyped int and untyped float kinds.
func IsNumeric(t Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case KindInt, KindInt8, KindInt16, KindInt32,
		KindUint, KindUint8, KindUint16, KindUint32,
		KindFloat, KindFloat32,
		KindRune,
		KindUntypedInt, KindUntypedFloat:
		return true
	}
	return false
}

// IsInteger reports whether t is a signed or unsigned integer type or a rune,
// including untyped int.
func IsInteger(t Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case KindInt, KindInt8, KindInt16, KindInt32,
		KindUint, KindUint8, KindUint16, KindUint32,
		KindRune,
		KindUntypedInt:
		return true
	}
	return false
}

// IsFloat reports whether t is a float type, including untyped float.
func IsFloat(t Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case KindFloat, KindFloat32, KindUntypedFloat:
		return true
	}
	return false
}

// IsSigned reports whether t is a signed integer type or a rune. Untyped int
// counts as signed.
func IsSigned(t Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case KindInt, KindInt8, KindInt16, KindInt32, KindRune, KindUntypedInt:
		return true
	}
	return false
}

// IsUnsigned reports whether t is an unsigned integer type. Byte (uint8) and
// untyped int are NOT unsigned: byte is an alias of uint8 and IS unsigned;
// untyped int is signed.
func IsUnsigned(t Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case KindUint, KindUint8, KindUint16, KindUint32:
		return true
	}
	return false
}

// IsBoolean reports whether t is bool or untyped bool.
func IsBoolean(t Type) bool {
	if t == nil {
		return false
	}
	return t.Kind() == KindBool || t.Kind() == KindUntypedBool
}

// IsStringType reports whether t is string or untyped string.
func IsStringType(t Type) bool {
	if t == nil {
		return false
	}
	return t.Kind() == KindString || t.Kind() == KindUntypedString
}

// IsUntyped reports whether t is one of the untyped literal kinds.
func IsUntyped(t Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case KindUntypedInt, KindUntypedFloat, KindUntypedBool,
		KindUntypedString, KindUntypedNil:
		return true
	}
	return false
}

// IsInvalid reports whether t is the poison type (Kind() == KindInvalid).
func IsInvalid(t Type) bool {
	if t == nil {
		return false
	}
	return t.Kind() == KindInvalid
}

// IsNever reports whether t is the diverging type (Kind() == KindNever).
func IsNever(t Type) bool {
	if t == nil {
		return false
	}
	return t.Kind() == KindNever
}

// IsVoid reports whether t is the unit/void type (Kind() == KindVoid).
func IsVoid(t Type) bool {
	if t == nil {
		return false
	}
	return t.Kind() == KindVoid
}

// IsOrdered reports whether the ordering operators < > <= >= are defined on t:
// numeric types and string.
func IsOrdered(t Type) bool {
	if t == nil {
		return false
	}
	if IsNumeric(t) {
		return true
	}
	return t.Kind() == KindString
}

// IsComparable reports whether == != is defined on t: every ordered type plus
// bool.
func IsComparable(t Type) bool {
	if t == nil {
		return false
	}
	if IsOrdered(t) {
		return true
	}
	return t.Kind() == KindBool
}

// joinStrings is a small helper that joins a non-empty slice of type spellings
// with ", " — used by several composite String() methods.
func joinStrings(ss []string) string {
	return strings.Join(ss, ", ")
}
