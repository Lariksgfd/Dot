package main

import (
	"fmt"

	"github.com/dotlang/dot/parser"
	"github.com/dotlang/dot/types"
)

func checkFile(path string) error {
	source, err := readSource(path)
	if err != nil {
		return err
	}
	prog, parseErr := parser.ParseFile(source, path)
	if parseErr != nil {
		return reportErr("parsing", parseErr, source)
	}
	if _, checkErr := types.CheckWithImports(prog, path, findStdlibDir()); checkErr != nil {
		return reportErr("type checking", checkErr, source)
	}
	fmt.Printf("ok\t%s\n", path)
	return nil
}
