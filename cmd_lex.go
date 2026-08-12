package main

import (
	"fmt"

	"github.com/dotlang/dot/lexer"
)

func lexFile(path string) error {
	source, err := readSource(path)
	if err != nil {
		return err
	}
	tokens, lexErr := lexer.Tokenize(source, path)
	for _, tok := range tokens {
		fmt.Printf("%s\t%s\t%s\n", tok.Pos, tok.Type, display(tok))
	}
	return reportErr("lexing", lexErr, source)
}
