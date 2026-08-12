package dotpm

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// ResolveURL constructs a download URL for a dependency.
// Can be overridden in tests.
var ResolveURL = func(name, version string) string {
	return "https://registry.dotlang.sh/" + name + "/" + version + ".dot"
}

// Install reads dot-lock.json and ensures all dependencies are present
// and match their recorded hashes. Missing or mismatched deps are downloaded.
func Install(workDir string) error {
	lockPath := filepath.Join(workDir, "dot-lock.json")
	lock, err := ParseLock(lockPath)
	if err != nil {
		return fmt.Errorf("parse lock: %w", err)
	}

	modulesDir := filepath.Join(workDir, "dot_modules")
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		return fmt.Errorf("create dot_modules/: %w", err)
	}

	for name, entry := range lock.Dependencies {
		if !needsInstall(modulesDir, name, entry.Hash) {
			continue
		}

		url := entry.URL
		if url == "" {
			url = ResolveURL(name, entry.Version)
		}

		if err := downloadDep(modulesDir, name, url); err != nil {
			return fmt.Errorf("install %s: %w", name, err)
		}
	}

	return nil
}

// needsInstall returns true if the dependency is missing or its hash doesn't match.
func needsInstall(modulesDir, name, expectedHash string) bool {
	depPath, isDir, found := findDepPath(modulesDir, name)
	if !found {
		return true
	}

	var hash string
	var err error
	if isDir {
		hash, err = hashDir(depPath)
	} else {
		hash, err = computeFileHash(depPath)
	}
	if err != nil {
		return true
	}
	return hash != expectedHash
}

// downloadDep downloads a dependency from url into dot_modules/<name>.dot.
func downloadDep(modulesDir, name, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: unexpected status %d", url, resp.StatusCode)
	}

	dest := filepath.Join(modulesDir, name+".dot")
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}

	return nil
}
