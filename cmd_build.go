package main

import (
	"fmt"
)

func runBuild(args []string) error {
	output := ""
	input := ""
	useLLVM := false
	for i := 0; i < len(args); i++ {
		if args[i] == "-llvm" {
			useLLVM = true
		} else if args[i] == "-o" {
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		} else if input == "" {
			input = args[i]
		}
	}
	if input == "" {
		return fmt.Errorf("usage: dot build [-llvm] <file.dot> [-o output]")
	}

	src, _, _, _, err := pipeline(input, useLLVM)
	if err != nil {
		return err
	}

	if output == "" {
		fmt.Print(src)
		return nil
	}

	if useLLVM {
		if err := compileLLVMToBinary(src, output); err != nil {
			return err
		}
	} else {
		if err := compileToBinary(src, output); err != nil {
			return err
		}
	}
	fmt.Printf("built %s -> %s\n", input, absPath(output))
	return nil
}
