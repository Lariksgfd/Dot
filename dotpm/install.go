package dotpm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ResolveURL constructs a download URL for a dependency.
var ResolveURL = func(name, version string) string {
	return "https://" + name
}

// Install reads dot.toml and clones all dependencies into .dot/modules/
func Install(workDir string) error {
	manifestPath := filepath.Join(workDir, "dot.toml")
	m, err := ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	modulesDir := filepath.Join(workDir, ".dot", "modules")
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		return fmt.Errorf("create .dot/modules/: %w", err)
	}

	for repo, version := range m.Dependencies {
		dest := filepath.Join(modulesDir, repo)
		
		// If already exists, skip or we could pull/checkout, but skipping is simplest for now
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("Dependency %s already exists, skipping.\n", repo)
			continue
		}

		fmt.Printf("Installing %s@%s...\n", repo, version)
		
		url := "https://" + repo
		
		cmd := exec.Command("git", "clone", "--branch", version, "--depth", "1", url, dest)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Run(); err != nil {
			// Fallback: try without branch if the version might not be a valid branch/tag
			fmt.Printf("Failed to clone with branch %s, trying default branch...\n", version)
			cmd = exec.Command("git", "clone", url, dest)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("install %s: %w", repo, err)
			}
			
			// Try to checkout the specific version after clone
			checkoutCmd := exec.Command("git", "-C", dest, "checkout", version)
			checkoutCmd.Stdout = os.Stdout
			checkoutCmd.Stderr = os.Stderr
			if err := checkoutCmd.Run(); err != nil {
				fmt.Printf("Warning: failed to checkout version %s for %s\n", version, repo)
			}
		}
	}

	return nil
}
