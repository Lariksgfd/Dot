package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, exePath).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("running %s: %v\noutput: %s", exePath, err, string(out))
	}
	return string(out), nil
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
