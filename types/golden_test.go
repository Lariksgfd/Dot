package types

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotlang/dot/parser"
)

func testdataDir() string {
	return filepath.Join(projectRoot(), "testdata")
}

func TestGolden_CheckOk(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(testdataDir(), "check_ok.dot"))
	if err != nil {
		t.Fatal(err)
	}
	prog, err := parser.ParseFile(string(src), "check_ok.dot")
	if err != nil {
		t.Fatalf("parse check_ok.dot: %v", err)
	}
	info, err := Check(prog, "check_ok.dot")
	if err != nil {
		t.Fatalf("check_ok.dot has errors: %v", err)
	}
	if info == nil {
		t.Fatal("info is nil")
	}
	if info.Defs == nil {
		t.Error("Defs is nil")
	}
	if info.Uses == nil {
		t.Error("Uses is nil")
	}
	if info.Types == nil {
		t.Error("Types is nil")
	}
	if info.FileScope == nil {
		t.Error("FileScope is nil")
	}
	if info.Diagnostics == nil {
		t.Error("Diagnostics is nil")
	}
	if info.Diagnostics.Len() != 0 {
		t.Errorf("expected 0 diagnostics, got %d", info.Diagnostics.Len())
	}
}

func TestGolden_CheckErr(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(testdataDir(), "check_err.dot"))
	if err != nil {
		t.Fatal(err)
	}
	prog, err := parser.ParseFile(string(src), "check_err.dot")
	if err != nil {
		t.Fatalf("parse check_err.dot: %v", err)
	}
	info, err := Check(prog, "check_err.dot")
	if err == nil {
		t.Fatal("check_err.dot should have errors, got nil")
	}
	if info == nil {
		t.Fatal("info is nil")
	}
	if info.Diagnostics.Len() < 5 {
		t.Errorf("expected at least 5 diagnostics, got %d", info.Diagnostics.Len())
	}
}

func TestGolden_CheckErr_ContainsExpectedMessages(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(testdataDir(), "check_err.dot"))
	if err != nil {
		t.Fatal(err)
	}
	prog, err := parser.ParseFile(string(src), "check_err.dot")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = Check(prog, "check_err.dot")
	if err == nil {
		t.Fatal("expected errors")
	}
	errStr := strings.ToLower(err.Error())

	expected := []string{
		"undefined",
		"mismatch",
		"cannot call",
		"must be bool",
		"exhaustive",
	}
	for _, exp := range expected {
		if !strings.Contains(errStr, exp) {
			t.Errorf("expected error containing %q, but not found in: %s", exp, errStr)
		}
	}
}
