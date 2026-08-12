package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func buildAndRunLLVM(t *testing.T, dotFile string) (string, error) {
	t.Helper()

	llsrc, _, _, _, err := pipeline(dotFile, true)
	if err != nil {
		return "", err
	}

	exePath := filepath.Join(t.TempDir(), "test.exe")
	if err := compileLLVMToBinary(llsrc, exePath); err != nil {
		return "", err
	}

	out, err := exec.Command(exePath).CombinedOutput()
	return string(out), err
}

func TestE2ELLVM_Hello(t *testing.T) {
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not found in PATH, skipping LLVM E2E test")
	}

	out, err := buildAndRunLLVM(t, "testdata/hello.dot")
	if err != nil {
		t.Fatalf("build+run failed: %v\noutput: %s", err, out)
	}
	if !contains(out, "Hello, world!") {
		t.Errorf("expected 'Hello, world!' in output, got: %s", out)
	}
}
