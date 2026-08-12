package main

import (
	"fmt"
	"os"

	"github.com/dotlang/dot/lsp"
)

func runAnalyz(args []string) error {
	server := lsp.NewServer(os.Stdin, os.Stdout)
	if err := server.Run(); err != nil {
		return fmt.Errorf("lsp server error: %w", err)
	}
	return nil
}
