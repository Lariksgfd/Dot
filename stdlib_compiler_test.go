package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestStdlibCompiler(t *testing.T) {
	for _, useLLVM := range []bool{false, true} {
		t.Run(fmt.Sprintf("llvm=%v", useLLVM), func(t *testing.T) {
			out, err := buildAndRun(t, "testdata/compiler_test.dot", useLLVM)
			if err != nil {
				t.Fatalf("build+run failed: %v\noutput: %s", err, out)
			}
			if !strings.Contains(out, "OK: happy path") {
				t.Errorf("expected 'OK: happy path' in output, got: %s", out)
			}
			if !strings.Contains(out, "OK: caught type error") {
				t.Errorf("expected 'OK: caught type error' in output, got: %s", out)
			}
			if !strings.Contains(out, "OK: caught syntax error") {
				t.Errorf("expected 'OK: caught syntax error' in output, got: %s", out)
			}
			for _, want := range []string{
				"OK: fn ret ok",
				"OK: fib recursion",
				"OK: fn as value",
				"OK: fn var",
				"OK: tail expr",
				"OK: import skip",
				"OK: missing return",
				"OK: ret mismatch",
				"OK: tail mismatch",
				"OK: arity",
				"OK: arg type",
				"OK: fn type mismatch",
				"OK: generic identity",
				"OK: generic two params",
				"OK: generic subst",
				"OK: generic var arg",
				"OK: none return",
				"OK: some ctor",
				"OK: option is_some",
				"OK: result is_ok",
				"OK: option map",
				"OK: generic ret mismatch",
				"OK: generic conflict",
				"OK: unknown method",
				"OK: closure basic",
				"OK: closure two params",
				"OK: closure capture",
				"OK: closure as arg",
				"OK: struct lit",
				"OK: payload ctor",
				"OK: payload is_some",
				"OK: closure arg type",
				"OK: closure ret mismatch",
				"OK: struct field mismatch",
				"OK: struct unknown field",
				"OK: payload type",
				"OK: map lit",
				"OK: typed map",
				"OK: empty map",
				"OK: map val mismatch",
				"OK: map key mismatch",
				"OK: named tuple",
				"OK: mixed tuple",
				"OK: tuple trailing comma",
				"OK: str_concat",
				"OK: str_concat empty lhs",
				"OK: str_concat empty rhs",
				"OK: str_concat both empty",
				"OK: starts true",
				"OK: starts false",
				"OK: starts prefix longer",
				"OK: starts empty prefix",
				"OK: starts equal",
				"OK: ends true",
				"OK: ends false",
				"OK: ends suffix longer",
				"OK: ends empty suffix",
				"OK: ends equal",
				"OK: contains true",
				"OK: contains false",
				"OK: contains empty sub",
				"OK: contains sub longer",
				"OK: contains at end",
				"OK: index_of found",
				"OK: index_of not found",
				"OK: index_of empty sub",
				"OK: index_of second occurrence",
				"OK: index_of sub longer",
				"OK: index_of at end",
				"OK: index_of both empty",
				"OK: index_of first of many",
				"OK: escape quote backslash",
				"OK: escape ws",
				"OK: escape nul",
				"OK: escape byte128",
				"OK: escape byte127",
				"OK: escape byte1",
				"OK: escape byte31",
				"OK: escape plain",
				"OK: int_to_str neg",
				"OK: int_to_str zero",
				"OK: int_to_str pos",
				"OK: ty_to_c int",
				"OK: ty_to_c float",
				"OK: ty_to_c bool",
				"OK: ty_to_c string",
				"OK: ty_to_c void",
				"OK: ty_to_c nil",
				"OK: ty_to_c unknown",
				"OK: ty_to_c error",
				"OK: ty_to_c slice",
				"OK: ty_to_c map",
				"OK: ty_to_c struct",
				"OK: ty_to_c func",
				"OK: ty_to_c array",
				"OK: ty_to_c typevar",
				"OK: is_heap true string",
				"OK: is_heap true func",
				"OK: is_heap true array",
				"OK: is_heap true slice",
				"OK: is_heap true struct",
				"OK: is_heap true map",
				"OK: is_heap false int",
				"OK: is_heap false float",
				"OK: is_heap false bool",
				"OK: is_heap false void",
				"OK: is_heap false nil",
				"OK: zero_c_value int",
				"OK: zero_c_value float",
				"OK: zero_c_value bool",
				"OK: zero_c_value string",
				"OK: zero_c_value nil",
				"OK: zero_c_value slice",
				"OK: zero_c_value map",
				"OK: zero_c_value struct",
				"OK: zero_c_value func",
				"OK: zero_c_value array",
				"OK: zero_c_value void",
				"OK: zero_c_value fallthrough typevar",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("expected %q in output, got: %s", want, out)
				}
			}
		})
	}
}

func TestStdlibParser(t *testing.T) {
	for _, useLLVM := range []bool{false, true} {
		t.Run(fmt.Sprintf("llvm=%v", useLLVM), func(t *testing.T) {
			out, err := buildAndRun(t, "testdata/parser_test.dot", useLLVM)
			if err != nil {
				t.Fatalf("build+run failed: %v\noutput: %s", err, out)
			}
			if !strings.Contains(out, "OK: parser happy path") {
				t.Errorf("expected 'OK: parser happy path' in output, got: %s", out)
			}
		})
	}
}
