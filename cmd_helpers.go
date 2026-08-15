package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/codegen"
	"github.com/dotlang/dot/codegen/llvm"
	"github.com/dotlang/dot/errors"
	"github.com/dotlang/dot/lexer"
	"github.com/dotlang/dot/parser"
	"github.com/dotlang/dot/types"
)

const (
	projectRoot = `D:\Programing\Go\Projects\dot`
	stdlibPath  = projectRoot + `\stdlib`
	runtimePath = projectRoot + `\runtime`
)

func readSource(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func reportErr(phase string, err error, source string) error {
	if err == nil {
		return nil
	}
	if list, ok := err.(*errors.ErrorList); ok {
		if list.Len() > 0 {
			fmt.Fprintln(os.Stderr, errors.FormatAll(list, source))
		}
		// Warnings do not prevent compilation (SeverityWarning): fail only
		// on actual errors.
		if n := list.ErrorCount(); n > 0 {
			return fmt.Errorf("%s failed: %d error(s)", phase, n)
		}
		return nil
	}
	return err
}

func findStdlibDir() string {
	return stdlibPath
}

func findRuntimeDir() string {
	return runtimePath
}

func pipeline(sourceFile string, useLLVM bool) (string, *types.Info, *ast.Program, string, error) {
	source, err := readSource(sourceFile)
	if err != nil {
		return "", nil, nil, source, err
	}

	prog, parseErr := parser.ParseFile(source, sourceFile)
	if parseErr != nil {
		return "", nil, nil, source, reportErr("parsing", parseErr, source)
	}

	info, checkErr := types.CheckWithImports(prog, sourceFile, findStdlibDir())
	if checkErr != nil {
		// reportErr fails only on actual errors; a warnings-only list must
		// not abort the pipeline.
		if err := reportErr("type checking", checkErr, source); err != nil {
			return "", nil, nil, source, err
		}
	} else if info != nil && info.Diagnostics.WarningCount() > 0 {
		// Err() is nil for a warnings-only list, but the warnings still
		// deserve to be printed.
		reportErr("type checking", info.Diagnostics, source)
	}

	var generatedCode string
	var genErr error
	if useLLVM {
		generatedCode, genErr = llvm.Generate(info, prog)
	} else {
		generatedCode, genErr = codegen.Generate(info, prog)
	}
	if genErr != nil {
		return "", nil, nil, source, fmt.Errorf("codegen: %w", genErr)
	}

	return generatedCode, info, prog, source, nil
}

func compileToBinary(csrc string, outputPath string) error {
	tmpFile, err := os.CreateTemp("", "dotbuild-*.c")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(csrc); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp C file: %w", err)
	}
	tmpFile.Close()

	rtObjs, err := runtimeObjectFiles()
	if err != nil {
		return err
	}

	outPath, _ := filepath.Abs(outputPath)
	gccArgs := []string{"-O2", "-Wno-implicit-function-declaration", "-I", findRuntimeDir(), "-o", outPath, tmpFile.Name()}
	gccArgs = append(gccArgs, rtObjs...)
	gccArgs = append(gccArgs, "-lws2_32", "-ladvapi32", "-lbcrypt")

	gccCmd := exec.Command("gcc", gccArgs...)
	gccOut, gccErr := gccCmd.CombinedOutput()
	if gccErr != nil {
		return fmt.Errorf("gcc failed: %s\n%s", gccErr, string(gccOut))
	}
	return nil
}

func compileTempExe(csrc string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "dotrun-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	cPath := filepath.Join(tmpDir, "program.c")
	if err := os.WriteFile(cPath, []byte(csrc), 0644); err != nil {
		return "", fmt.Errorf("failed to write temp C file: %w", err)
	}

	rtObjs, err := runtimeObjectFiles()
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	exePath := filepath.Join(tmpDir, "program.exe")
	gccArgs := []string{"-O2", "-Wno-implicit-function-declaration", "-I", findRuntimeDir(), "-o", exePath, cPath}
	gccArgs = append(gccArgs, rtObjs...)
	gccArgs = append(gccArgs, "-lws2_32", "-ladvapi32", "-lbcrypt")

	gccCmd := exec.Command("gcc", gccArgs...)
	gccOut, gccErr := gccCmd.CombinedOutput()
	if gccErr != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("gcc failed: %s\n%s", gccErr, string(gccOut))
	}
	return exePath, nil
}

var (
	runtimeObjOnce  sync.Once
	runtimeObjPaths []string
	runtimeObjErr   error
)

func runtimeObjectFiles() ([]string, error) {
	runtimeObjOnce.Do(func() {
		rtDir := findRuntimeDir()
		rtCFiles, err := filepath.Glob(filepath.Join(rtDir, "*.c"))
		if err != nil {
			runtimeObjErr = fmt.Errorf("failed to glob runtime files: %w", err)
			return
		}
		if len(rtCFiles) == 0 {
			runtimeObjErr = fmt.Errorf("no .c files found in runtime dir %s", rtDir)
			return
		}

		objDir, err := os.MkdirTemp("", "dot-rtobj-*")
		if err != nil {
			runtimeObjErr = fmt.Errorf("failed to create temp dir for runtime objects: %w", err)
			return
		}

		objs := make([]string, 0, len(rtCFiles))
		for _, cf := range rtCFiles {
			objPath := filepath.Join(objDir, filepath.Base(cf)+".o")
			cmd := exec.Command("gcc", "-c", "-O2", "-Wno-implicit-function-declaration", "-I", rtDir, cf, "-o", objPath)
			out, cerr := cmd.CombinedOutput()
			if cerr != nil {
				runtimeObjErr = fmt.Errorf("gcc -c failed on %s: %s\n%s", cf, cerr, string(out))
				os.RemoveAll(objDir)
				return
			}
			objs = append(objs, objPath)
		}
		runtimeObjPaths = objs
	})
	return runtimeObjPaths, runtimeObjErr
}

func display(t lexer.Token) string {
	if t.Type == lexer.TokenEOF {
		return ""
	}
	return fmt.Sprintf("%q", t.Lexeme)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: dot (lex|parse|check|build|run|test|fmt|init|analyz) <file.dot> [flags]")
	os.Exit(2)
}

func absPath(s string) string {
	p, _ := filepath.Abs(s)
	return p
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

func compileLLVMToBinary(llsrc string, outputPath string) error {
	tmpFile, err := os.CreateTemp("", "dotbuild-*.ll")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(llsrc); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp LLVM IR file: %w", err)
	}
	tmpFile.Close()

	outPath, _ := filepath.Abs(outputPath)
	clangArgs := []string{"-O2", "-Wno-override-module", "-DWIN32_LEAN_AND_MEAN", "-I", findRuntimeDir(), "-o", outPath, tmpFile.Name()}

	// Also link C runtime just in case
	rtCFiles, err := filepath.Glob(filepath.Join(findRuntimeDir(), "*.c"))
	if err != nil {
		return fmt.Errorf("failed to glob runtime files: %w", err)
	}
	clangArgs = append(clangArgs, rtCFiles...)
	clangArgs = append(clangArgs, "-lws2_32", "-ladvapi32", "-lbcrypt", "-lshell32")

	clangCmd := exec.Command("clang", clangArgs...)
	clangOut, clangErr := clangCmd.CombinedOutput()
	if clangErr != nil {
		return fmt.Errorf("clang failed: %s\n%s", clangErr, string(clangOut))
	}
	return nil
}

func compileTempExeLLVM(llsrc string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "dotrun-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	llPath := filepath.Join(tmpDir, "program.ll")
	if err := os.WriteFile(llPath, []byte(llsrc), 0644); err != nil {
		return "", fmt.Errorf("failed to write temp LLVM IR file: %w", err)
	}

	exePath := filepath.Join(tmpDir, "program.exe")
	clangArgs := []string{"-O2", "-Wno-override-module", "-DWIN32_LEAN_AND_MEAN", "-I", findRuntimeDir(), "-o", exePath, llPath}

	rtCFiles, err := filepath.Glob(filepath.Join(findRuntimeDir(), "*.c"))
	if err != nil {
		return "", fmt.Errorf("failed to glob runtime files: %w", err)
	}
	clangArgs = append(clangArgs, rtCFiles...)
	clangArgs = append(clangArgs, "-lws2_32", "-ladvapi32", "-lbcrypt", "-lshell32")

	clangCmd := exec.Command("clang", clangArgs...)
	clangOut, clangErr := clangCmd.CombinedOutput()
	if clangErr != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("clang failed: %s\n%s", clangErr, string(clangOut))
	}
	return exePath, nil
}

