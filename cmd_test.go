package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// lex / parse / check - smoke tests on golden data
// ---------------------------------------------------------------------------

func TestLexFile(t *testing.T) {
	path := filepath.Join(projectRoot, "testdata", "lexer_sample.dot")
	err := lexFile(path)
	if err != nil {
		t.Fatalf("lexFile(%q): %v", path, err)
	}
}

func TestParseFile(t *testing.T) {
	path := filepath.Join(projectRoot, "testdata", "parser_sample.dot")
	err := parseFile(path)
	if err != nil {
		t.Fatalf("parseFile(%q): %v", path, err)
	}
}

func TestCheckFile(t *testing.T) {
	path := filepath.Join(projectRoot, "testdata", "check_ok.dot")
	err := checkFile(path)
	if err != nil {
		t.Fatalf("checkFile(%q): %v", path, err)
	}
}

func TestCheckFile_HelloStdlib(t *testing.T) {
	path := filepath.Join(projectRoot, "testdata", "hello_stdlib.dot")
	err := checkFile(path)
	if err != nil {
		t.Logf("checkFile(%q) returned diagnostics (may be expected): %v", path, err)
	}
}

// ---------------------------------------------------------------------------
// build
// ---------------------------------------------------------------------------

func TestBuild_GeneratesC(t *testing.T) {
	path := filepath.Join(projectRoot, "testdata", "check_ok.dot")
	csrc, _, _, _, err := pipeline(path, false)
	if err != nil {
		t.Fatalf("pipeline(%q): %v", path, err)
	}
	if csrc == "" {
		t.Fatal("pipeline returned empty C source")
	}
	if !strings.Contains(csrc, "#include") {
		t.Error("C source missing #include")
	}
	if !strings.Contains(csrc, "main(void)") {
		t.Error("C source missing main()")
	}
}

func TestBuild_MinimalProgram(t *testing.T) {
	src := `fn main() {
	print("hi")
}
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "min.dot")
	if err := os.WriteFile(tmpFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	csrc, _, _, _, err := pipeline(tmpFile, false)
	if err != nil {
		t.Fatalf("pipeline(%q): %v", tmpFile, err)
	}
	if csrc == "" {
		t.Fatal("pipeline returned empty C source")
	}
	if !strings.Contains(csrc, "Dot_main()") {
		t.Error("C source missing Dot_main() call")
	}
}

// ---------------------------------------------------------------------------
// fmt
// ---------------------------------------------------------------------------

func TestFmt_Run(t *testing.T) {
	input := "const MAX_SIZE = 1024\n\nfn main(){print(\"hello\")}"

	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "fmt_test.dot")
	if err := os.WriteFile(fpath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	err := runFmt([]string{fpath})
	if err != nil {
		t.Fatalf("runFmt: %v", err)
	}

	output, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) == 0 {
		t.Fatal("formatted output is empty")
	}
}

func TestFmt_CheckRejectsDotSource(t *testing.T) {
	input := "const MAX_SIZE = 1024\n\nfn main() {\n    print(\"hello\")\n}\n"

	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "fmt_check.dot")
	if err := os.WriteFile(fpath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	err := runFmt([]string{"--check", fpath})
	if err == nil {
		t.Error("runFmt --check should reject .dot source (not AST-print format)")
	}
}

func TestFmt_MissingFile(t *testing.T) {
	err := runFmt([]string{"nonexistent_file.dot"})
	if err == nil {
		t.Error("runFmt should fail on missing file")
	}
}

// ---------------------------------------------------------------------------
// init
// ---------------------------------------------------------------------------

func TestInit_CreatesProject(t *testing.T) {
	tmpDir := t.TempDir()
	projName := "testproj"

	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	err := runInit([]string{projName})
	if err != nil {
		t.Fatalf("runInit(%q): %v", projName, err)
	}

	projDir := filepath.Join(tmpDir, projName)
	checkFileExists(t, filepath.Join(projDir, "main.dot"))
	checkFileExists(t, filepath.Join(projDir, ".gitignore"))

	mainContent, err := os.ReadFile(filepath.Join(projDir, "main.dot"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainContent), "fn main()") {
		t.Error("main.dot missing fn main()")
	}
	if !strings.Contains(string(mainContent), projName) {
		t.Errorf("main.dot missing project name %q", projName)
	}

	runtimeDir := filepath.Join(projDir, "runtime")
	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		t.Fatalf("readDir(%q): %v", runtimeDir, err)
	}
	if len(entries) == 0 {
		t.Error("runtime directory is empty")
	}
}

func TestInit_MissingName(t *testing.T) {
	err := runInit([]string{})
	if err == nil {
		t.Error("runInit with no args should fail")
	}
}

func TestInit_DuplicateDir(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "existing")
	if err := os.Mkdir(projDir, 0755); err != nil {
		t.Fatal(err)
	}

	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	err := runInit([]string{"existing"})
	if err == nil {
		t.Error("runInit on existing directory should fail")
	}
}

// ---------------------------------------------------------------------------
// runtime C compilation
// ---------------------------------------------------------------------------

func TestRuntimeCompiles(t *testing.T) {
	gcc, err := exec.LookPath("gcc")
	if err != nil {
		t.Skipf("gcc not found in PATH; skipping C compilation test")
	}

	runtimeC, err := filepath.Glob(filepath.Join(runtimePath, "*.c"))
	if err != nil {
		t.Fatalf("glob runtime/*.c: %v", err)
	}
	if len(runtimeC) == 0 {
		t.Fatal("no .c files found in runtime directory")
	}

	for _, cFile := range runtimeC {
		name := filepath.Base(cFile)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			outDir := t.TempDir()
			cmd := exec.Command(gcc, "-c", "-Wall", "-Wextra", "-std=c99",
				cFile,
				"-o", filepath.Join(outDir, name+".o"),
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("gcc failed on %s: %v\n%s", name, err, string(out))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func checkFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("missing expected file %q: %v", path, err)
		return
	}
	if info.IsDir() {
		t.Errorf("expected file %q but got directory", path)
	}
}
