package main

import (
	"fmt"
	"os"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/parser"
)

func runFmt(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: dot fmt <file.dot>")
	}
	checkOnly := false
	filePath := args[0]
	if filePath == "--check" {
		checkOnly = true
		if len(args) < 2 {
			return fmt.Errorf("usage: dot fmt --check <file.dot>")
		}
		filePath = args[1]
	}

	source, err := readSource(filePath)
	if err != nil {
		return err
	}

	prog, parseErr := parser.ParseFile(source, filePath)
	if parseErr != nil {
		return reportErr("formatting", parseErr, source)
	}

	formatted := ast.Print(prog)

	if checkOnly {
		if source != formatted {
			return fmt.Errorf("%s: not formatted", filePath)
		}
		return nil
	}

	if err := os.WriteFile(filePath, []byte(formatted), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filePath, err)
	}
	return nil
}
