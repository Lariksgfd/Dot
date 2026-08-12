package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func runRun(args []string) error {
	input := ""
	useLLVM := false
	for _, arg := range args {
		if arg == "-llvm" {
			useLLVM = true
		} else if input == "" {
			input = arg
		}
	}
	if input == "" {
		return fmt.Errorf("usage: dot run [-llvm] <file.dot>")
	}

	src, _, _, _, err := pipeline(input, useLLVM)
	if err != nil {
		return err
	}

	var exePath string
	if useLLVM {
		exePath, err = compileTempExeLLVM(src)
	} else {
		exePath, err = compileTempExe(src)
	}
	if err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Dir(exePath))

	runCmd := exec.Command(exePath)
	runCmd.Stdin = os.Stdin
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}
	return nil
}
