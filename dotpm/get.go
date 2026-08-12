package dotpm

import (
	"fmt"
	"os"
)

// Get reads the manifest and creates a dummy dot_modules folder.
func Get(manifestPath string) error {
	m, err := ParseManifest(manifestPath)
	if err != nil {
		return err
	}

	err = os.MkdirAll("dot_modules", 0755)
	if err != nil {
		return err
	}

	for dep, ver := range m.Dependencies {
		fmt.Printf("Fetching dependency %s@%s...\n", dep, ver)
		depPath := "dot_modules/" + dep
		if err := os.MkdirAll(depPath, 0755); err != nil {
			return err
		}
	}
	return nil
}
