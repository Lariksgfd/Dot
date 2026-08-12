package main

import (
	"fmt"

	"github.com/dotlang/dot/dotpm"
)

func runFetch(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: dot fetch <url>")
	}
	return dotpm.Fetch(args[0])
}
