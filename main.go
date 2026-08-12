package main

import (
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
	}

	var err error
	switch args[0] {
	case "lex":
		if len(args) < 2 {
			printUsage()
		}
		err = lexFile(args[1])
	case "parse":
		if len(args) < 2 {
			printUsage()
		}
		err = parseFile(args[1])
	case "check":
		if len(args) < 2 {
			printUsage()
		}
		err = checkFile(args[1])
	case "build":
		err = runBuild(args[1:])
	case "run":
		err = runRun(args[1:])
	case "test":
		err = runTest(args[1:])
	case "fmt":
		err = runFmt(args[1:])
	case "init":
		err = runInit(args[1:])
	case "fetch":
		err = runFetch(args[1:])
	case "install":
		err = runInstall(args[1:])
	case "analyz":
		err = runAnalyz(args[1:])
	default:
		printUsage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
