package dotpm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LockEntry represents a single locked dependency with its version, content hash, and source URL.
type LockEntry struct {
	Version string `json:"version"`
	Hash    string `json:"hash"`
	URL     string `json:"url,omitempty"`
}

// LockFile represents the dot-lock.json structure.
type LockFile struct {
	Dependencies map[string]LockEntry `json:"dependencies"`
}

// computeFileHash returns the SHA-256 hash of a file's contents.
func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashDir computes a combined hash for all .dot files in a directory.
func hashDir(dir string) (string, error) {
	h := sha256.New()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".dot" {
			hash, err := computeFileHash(path)
			if err != nil {
				return err
			}
			h.Write([]byte(hash))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// findDepPath resolves a dependency name to its path in dot_modules/.
// It looks for: dot_modules/<name>/ (directory) or dot_modules/<name>.dot (file).
// Returns (path, isDir, found).
func findDepPath(modulesDir, name string) (string, bool, bool) {
	dirPath := filepath.Join(modulesDir, name)
	if info, err := os.Stat(dirPath); err == nil {
		return dirPath, info.IsDir(), true
	}
	filePath := filepath.Join(modulesDir, name+".dot")
	if _, err := os.Stat(filePath); err == nil {
		return filePath, false, true
	}
	return "", false, false
}

// Lock reads the manifest, scans dot_modules/, and writes dot-lock.json
// with exact versions and content hashes.
// The workDir is the project root (where dot.json and dot_modules/ reside).
func Lock(workDir string) error {
	manifestPath := filepath.Join(workDir, "dot.json")
	m, err := ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	lock := LockFile{
		Dependencies: make(map[string]LockEntry),
	}

	modulesDir := filepath.Join(workDir, "dot_modules")
	for dep, ver := range m.Dependencies {
		depPath, isDir, found := findDepPath(modulesDir, dep)
		if !found {
			return fmt.Errorf("dependency %s: not found in %s", dep, modulesDir)
		}

		var hash string
		if isDir {
			hash, err = hashDir(depPath)
		} else {
			hash, err = computeFileHash(depPath)
		}
		if err != nil {
			return fmt.Errorf("hash %s: %w", dep, err)
		}

		lock.Dependencies[dep] = LockEntry{
			Version: ver,
			Hash:    hash,
			URL:     ResolveURL(dep, ver),
		}
	}

	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lock: %w", err)
	}
	lockPath := filepath.Join(workDir, "dot-lock.json")
	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		return fmt.Errorf("write lock: %w", err)
	}

	return nil
}

// ParseLock reads and parses the dot-lock.json file.
func ParseLock(path string) (*LockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var l LockFile
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

// Verify checks that installed dependencies match the lock file.
// Returns nil if all dependencies are valid, or an error describing mismatches.
func Verify(workDir string) error {
	manifestPath := filepath.Join(workDir, "dot.json")
	m, err := ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	lockPath := filepath.Join(workDir, "dot-lock.json")
	lock, err := ParseLock(lockPath)
	if err != nil {
		return fmt.Errorf("parse lock: %w", err)
	}

	modulesDir := filepath.Join(workDir, "dot_modules")
	var issues []string

	// Check all manifest deps are in lock
	for dep, ver := range m.Dependencies {
		entry, ok := lock.Dependencies[dep]
		if !ok {
			issues = append(issues, fmt.Sprintf("missing in lock: %s", dep))
			continue
		}
		if entry.Version != ver {
			issues = append(issues, fmt.Sprintf("version mismatch: %s (manifest=%s, lock=%s)", dep, ver, entry.Version))
		}

		depPath, isDir, found := findDepPath(modulesDir, dep)
		if !found {
			issues = append(issues, fmt.Sprintf("not installed: %s", dep))
			continue
		}

		var hash string
		if isDir {
			hash, err = hashDir(depPath)
		} else {
			hash, err = computeFileHash(depPath)
		}
		if err != nil {
			issues = append(issues, fmt.Sprintf("hash error: %s (%v)", dep, err))
			continue
		}

		if hash != entry.Hash {
			issues = append(issues, fmt.Sprintf("content mismatch: %s (hash changed)", dep))
		}
	}

	// Check for orphaned lock entries
	for dep := range lock.Dependencies {
		if _, ok := m.Dependencies[dep]; !ok {
			issues = append(issues, fmt.Sprintf("orphaned lock entry: %s", dep))
		}
	}

	if len(issues) > 0 {
		msg := strings.Join(issues, "; ")
		return fmt.Errorf("verification failed: %s", msg)
	}
	return nil
}
