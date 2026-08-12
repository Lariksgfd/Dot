package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func runInit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: dot init <project_name>")
	}
	name := args[0]

	if err := os.Mkdir(name, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", name, err)
	}

	mainContent := fmt.Sprintf(`fn main() {
	print("Hello from %s!")
}
`, name)
	if err := os.WriteFile(filepath.Join(name, "main.dot"), []byte(mainContent), 0644); err != nil {
		return fmt.Errorf("failed to create main.dot: %w", err)
	}

	gitignore := "*.exe\n*.o\n.dot_build/\n"
	if err := os.WriteFile(filepath.Join(name, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	runtimeFiles, err := filepath.Glob(filepath.Join(findRuntimeDir(), "*"))
	if err != nil {
		return fmt.Errorf("failed to list runtime files: %w", err)
	}

	runtimeDest := filepath.Join(name, "runtime")
	if err := os.Mkdir(runtimeDest, 0755); err != nil {
		return fmt.Errorf("failed to create runtime dir: %w", err)
	}

	for _, src := range runtimeFiles {
		base := filepath.Base(src)
		dst := filepath.Join(runtimeDest, base)

		srcFile, err := os.Open(src)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", src, err)
		}
		dstFile, err := os.Create(dst)
		if err != nil {
			srcFile.Close()
			return fmt.Errorf("failed to create %s: %w", dst, err)
		}
		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
		if err != nil {
			return fmt.Errorf("failed to copy %s: %w", base, err)
		}
	}

	fmt.Printf("created project '%s'\n", name)
	return nil
}
