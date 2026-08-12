package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/parser"
)

func runTest(args []string) error {
	target := "main.dot"
	useLLVM := false
	for _, arg := range args {
		if arg == "-llvm" {
			useLLVM = true
		} else {
			target = arg
		}
	}

	source, err := readSource(target)
	if err != nil {
		return err
	}

	prog, parseErr := parser.ParseFile(source, target)
	if parseErr != nil {
		return reportErr("parsing", parseErr, source)
	}

	var tests []*ast.FnDecl
	for _, d := range prog.Decls {
		fn, ok := d.(*ast.FnDecl)
		if !ok {
			continue
		}
		for _, a := range fn.Annotations {
			if a.Name == "test" {
				tests = append(tests, fn)
				break
			}
		}
	}

	if len(tests) == 0 {
		fmt.Println("no tests found")
		return nil
	}

	src, _, _, _, err := pipeline(target, useLLVM)
	if err != nil {
		return err
	}

	if useLLVM {
		return fmt.Errorf("dot test -llvm is not yet fully implemented for runner generation")
	}

	runner := buildTestRunner(tests, src)

	exePath, err := compileTempExe(runner)
	if err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Dir(exePath))

	runCmd := exec.Command(exePath)
	runCmd.Stdin = os.Stdin
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}
	return nil
}

func buildTestRunner(tests []*ast.FnDecl, userCode string) string {
	var b strings.Builder

	b.WriteString(`#include "dot_runtime.h"
#include <stdio.h>

static int tests_passed = 0;
static int tests_failed = 0;

int main(void) {
`)

	for _, fn := range tests {
		fmt.Fprintf(&b, "\tfprintf(stdout, \"  %s ... \");\n", fn.Name)
		fmt.Fprintf(&b, "\tfflush(stdout);\n")
		fmt.Fprintf(&b, "\tDot_%s();\n", fn.Name)
		fmt.Fprintf(&b, "\tfprintf(stdout, \"OK\\n\");\n")
		fmt.Fprintf(&b, "\ttests_passed++;\n")
	}

	b.WriteString(`
	fprintf(stdout, "\n");
	fprintf(stdout, "OK: %d passed, %d failed\n", tests_passed, tests_failed);
	dot_atexit_cleanup();
	return tests_failed > 0 ? 1 : 0;
}
`)

	b.WriteString("\n" + userCode)

	return b.String()
}
