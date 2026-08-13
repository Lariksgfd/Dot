package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func buildAndRun(t *testing.T, dotFile string, useLLVM bool) (string, error) {
	t.Helper()

	csrc, _, _, _, err := pipeline(dotFile, useLLVM)
	if err != nil {
		return "", err
	}

	exePath := filepath.Join(t.TempDir(), "test.exe")
	if useLLVM {
		if err := compileLLVMToBinary(csrc, exePath); err != nil {
			return "", err
		}
	} else {
		if err := compileToBinary(csrc, exePath); err != nil {
			return "", err
		}
	}

	out, err := exec.Command(exePath).CombinedOutput()
	return string(out), err
}

func TestE2E_Hello(t *testing.T) {
	for _, useLLVM := range []bool{false, true} {
		t.Run(fmt.Sprintf("llvm=%v", useLLVM), func(t *testing.T) {
			out, err := buildAndRun(t, "testdata/hello.dot", useLLVM)
			if err != nil {
				t.Fatalf("build+run failed: %v\noutput: %s", err, out)
			}
			if !contains(out, "Hello, world!") {
				t.Errorf("expected 'Hello, world!' in output, got: %s", out)
			}
		})
	}
}

func TestE2E_Fibonacci(t *testing.T) {
	for _, useLLVM := range []bool{false, true} {
		t.Run(fmt.Sprintf("llvm=%v", useLLVM), func(t *testing.T) {
			out, err := buildAndRun(t, "testdata/fibonacci.dot", useLLVM)
			if err != nil {
				t.Fatalf("build+run failed: %v\noutput: %s", err, out)
			}
			if !contains(out, "55") {
				t.Errorf("expected '55' in output, got: %s", out)
			}
		})
	}
}

func TestE2E_Structs(t *testing.T) {
	for _, useLLVM := range []bool{false, true} {
		t.Run(fmt.Sprintf("llvm=%v", useLLVM), func(t *testing.T) {
			csrc, _, _, _, err := pipeline("testdata/structs.dot", useLLVM)
			if err != nil {
				t.Fatalf("structs.dot pipeline failed: %v", err)
			}
			if len(csrc) == 0 {
				t.Error("generated source is empty")
			}
		})
	}
}

func TestE2E_HelloStdlib(t *testing.T) {
	for _, useLLVM := range []bool{false, true} {
		t.Run(fmt.Sprintf("llvm=%v", useLLVM), func(t *testing.T) {
			csrc, _, _, _, err := pipeline("testdata/hello_stdlib.dot", useLLVM)
			if err != nil {
				t.Fatalf("hello_stdlib.dot pipeline failed: %v", err)
			}
			if len(csrc) == 0 {
				t.Error("generated source is empty")
			}
		})
	}
}

func TestE2E_CheckOk(t *testing.T) {
	for _, useLLVM := range []bool{false, true} {
		t.Run(fmt.Sprintf("llvm=%v", useLLVM), func(t *testing.T) {
			csrc, _, _, _, err := pipeline("testdata/check_ok.dot", useLLVM)
			if err != nil {
				t.Fatalf("check_ok.dot pipeline failed: %v", err)
			}
			if len(csrc) == 0 {
				t.Error("generated source is empty")
			}
		})
	}
}

func TestE2E_RelativeImport(t *testing.T) {
	for _, useLLVM := range []bool{false, true} {
		t.Run(fmt.Sprintf("llvm=%v", useLLVM), func(t *testing.T) {
			out, err := buildAndRun(t, "testdata/relative_main.dot", useLLVM)
			if err != nil {
				t.Fatalf("build+run failed: %v\noutput: %s", err, out)
			}
			if !contains(out, "42") {
				t.Errorf("expected '42' in output, got: %s", out)
			}
		})
	}
}

func TestE2E_ReassignLoop(t *testing.T) {
	for _, useLLVM := range []bool{false, true} {
		t.Run(fmt.Sprintf("llvm=%v", useLLVM), func(t *testing.T) {
			out, err := buildAndRun(t, "testdata/reassign_loop.dot", useLLVM)
			if err != nil {
				t.Fatalf("build+run failed: %v\noutput: %s", err, out)
			}
			if !contains(out, "12345|ab|21|inner42|ok") {
				t.Errorf("expected reassignment output, got: %s", out)
			}
		})
	}
}

func TestE2E_StrCmpMatch(t *testing.T) {
	for _, useLLVM := range []bool{false, true} {
		t.Run(fmt.Sprintf("llvm=%v", useLLVM), func(t *testing.T) {
			out, err := buildAndRun(t, "testdata/strcmp_match.dot", useLLVM)
			if err != nil {
				t.Fatalf("build+run failed: %v\noutput: %s", err, out)
			}
			if !contains(out, "eq|ne|empty|notempty|WfallbackEMPTY") {
				t.Errorf("expected string comparison output, got: %s", out)
			}
		})
	}
}

func TestE2E_TupleReturn(t *testing.T) {
	for _, useLLVM := range []bool{false, true} {
		t.Run(fmt.Sprintf("llvm=%v", useLLVM), func(t *testing.T) {
			out, err := buildAndRun(t, "testdata/tuple_return.dot", useLLVM)
			if err != nil {
				t.Fatalf("build+run failed: %v\noutput: %s", err, out)
			}
			if !contains(out, "7|ok|7|ok|") {
				t.Errorf("expected tuple return output, got: %s", out)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
