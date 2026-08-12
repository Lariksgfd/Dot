package main

import (
	"fmt"
	"os"

	"github.com/dotlang/dot/dotpm"
)

func runInstall(args []string) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	return dotpm.Install(workDir)
}
