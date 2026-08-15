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
