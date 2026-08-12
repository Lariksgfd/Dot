package main

import (
	"fmt"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/parser"
)

func parseFile(path string) error {
	source, err := readSource(path)
	if err != nil {
		return err
	}
	prog, parseErr := parser.ParseFile(source, path)
	fmt.Print(ast.Print(prog))
	return reportErr("parsing", parseErr, source)
}
