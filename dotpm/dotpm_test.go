package dotpm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitAndParse(t *testing.T) {
	// Clean up after test
	defer os.Remove("dot.json")

	// Init still generates dot.json in the current code
	err := Init("test-pkg")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// But ParseManifest expects TOML. Let's create a manual dot.toml to test it.
	manifestPath := "dot.toml"
	defer os.Remove(manifestPath)

	tomlContent := `[package]
name = "test-pkg"
version = "0.1.0"
`
	err = os.WriteFile(manifestPath, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write toml: %v", err)
	}

	m, err := ParseManifest(manifestPath)
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}

	if m.Name != "test-pkg" {
		t.Errorf("Expected name 'test-pkg', got '%s'", m.Name)
	}
	if m.Version != "0.1.0" {
		t.Errorf("Expected version '0.1.0', got '%s'", m.Version)
	}
}

func TestGet(t *testing.T) {
	// Clean up after test
	defer os.Remove("dot.toml")
	defer os.RemoveAll("dot_modules")

	tomlContent := `[package]
name = "test-pkg"
version = "0.1.0"

[dependencies]
dummy-lib = "1.0.0"
`
	os.WriteFile("dot.toml", []byte(tomlContent), 0644)

	err := Get("dot.toml")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if _, err := os.Stat("dot_modules/dummy-lib"); os.IsNotExist(err) {
		t.Errorf("Expected dot_modules/dummy-lib to be created")
	}
}

func TestLockCreatesFile(t *testing.T) {
	dir := t.TempDir()

	manifestPath := filepath.Join(dir, "dot.json") // Lock still reads dot.json in code
	modulesDir := filepath.Join(dir, "dot_modules")
	lockPath := filepath.Join(dir, "dot-lock.json")

	// Write manifest as JSON because Lock in lock.go hardcodes "dot.json" and the old parse didn't error if we pass something ParseManifest can read. Wait, ParseManifest only parses TOML now!
	// If ParseManifest parses TOML, and Lock uses ParseManifest("dot.json"), Lock will parse it as TOML!
	// So we must write TOML format but name it "dot.json" because of hardcoding in Lock.
	manifest := `[package]
name = "test-pkg"
version = "0.1.0"

[dependencies]
lib-a = "1.0.0"
lib-b = "2.3.1"
`
	_ = os.WriteFile(manifestPath, []byte(manifest), 0644)

	// Create dummy dep files
	_ = os.MkdirAll(modulesDir, 0755)
	_ = os.MkdirAll(filepath.Join(modulesDir, "lib-a"), 0755)
	_ = os.WriteFile(filepath.Join(modulesDir, "lib-a", "index.dot"), []byte("fn foo() {}\n"), 0644)
	_ = os.WriteFile(filepath.Join(modulesDir, "lib-b.dot"), []byte("fn bar() {}\n"), 0644)

	if err := Lock(dir); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	lock, err := ParseLock(lockPath)
	if err != nil {
		t.Fatalf("ParseLock failed: %v", err)
	}

	if len(lock.Dependencies) != 2 {
		t.Errorf("Expected 2 deps in lock, got %d", len(lock.Dependencies))
	}

	la, ok := lock.Dependencies["lib-a"]
	if !ok {
		t.Fatal("lib-a not in lock")
	}
	if la.Version != "1.0.0" {
		t.Errorf("lib-a version: expected 1.0.0, got %s", la.Version)
	}
	if la.Hash == "" {
		t.Error("lib-a hash is empty")
	}
}

func TestVerifySuccess(t *testing.T) {
	dir := t.TempDir()

	manifestPath := filepath.Join(dir, "dot.json") // hardcoded in Verify
	modulesDir := filepath.Join(dir, "dot_modules")

	manifest := `[package]
name = "test-pkg"
version = "0.1.0"

[dependencies]
mylib = "1.2.0"
`
	_ = os.WriteFile(manifestPath, []byte(manifest), 0644)

	_ = os.MkdirAll(modulesDir, 0755)
	_ = os.WriteFile(filepath.Join(modulesDir, "mylib.dot"), []byte("const x = 42\n"), 0644)

	if err := Lock(dir); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	if err := Verify(dir); err != nil {
		t.Fatalf("Verify failed on valid state: %v", err)
	}
}

func TestDetectContentMismatch(t *testing.T) {
	dir := t.TempDir()

	manifestPath := filepath.Join(dir, "dot.json")
	modulesDir := filepath.Join(dir, "dot_modules")

	manifest := `[dependencies]
mylib = "1.2.0"
`
	_ = os.WriteFile(manifestPath, []byte(manifest), 0644)

	_ = os.MkdirAll(modulesDir, 0755)
	_ = os.WriteFile(filepath.Join(modulesDir, "mylib.dot"), []byte("const x = 42\n"), 0644)

	if err := Lock(dir); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	// Tamper with the file after locking
	_ = os.WriteFile(filepath.Join(modulesDir, "mylib.dot"), []byte("const x = 1337\n"), 0644)

	if err := Verify(dir); err == nil {
		t.Fatal("Expected Verify to fail after content tampering, got nil")
	}
}

func TestDetectMissingDep(t *testing.T) {
	dir := t.TempDir()

	manifestPath := filepath.Join(dir, "dot.json")
	modulesDir := filepath.Join(dir, "dot_modules")

	manifest := `[dependencies]
mylib = "1.2.0"
`
	_ = os.WriteFile(manifestPath, []byte(manifest), 0644)

	_ = os.MkdirAll(modulesDir, 0755)
	_ = os.WriteFile(filepath.Join(modulesDir, "mylib.dot"), []byte("const x = 42\n"), 0644)

	if err := Lock(dir); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	// Remove the installed dep
	_ = os.Remove(filepath.Join(modulesDir, "mylib.dot"))

	if err := Verify(dir); err == nil {
		t.Fatal("Expected Verify to fail with missing dep, got nil")
	}
}

func TestComputeFileHash(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "test_hash.tmp")

	content := []byte("hello world\n")
	_ = os.WriteFile(filePath, content, 0644)

	hash, err := computeFileHash(filePath)
	if err != nil {
		t.Fatalf("computeFileHash failed: %v", err)
	}
	if len(hash) != 64 {
		t.Errorf("Expected 64-char hex hash, got %d chars", len(hash))
	}

	hash2, _ := computeFileHash(filePath)
	if hash != hash2 {
		t.Error("Same content produced different hashes")
	}

	_ = os.WriteFile(filePath, []byte("changed\n"), 0644)
	hash3, _ := computeFileHash(filePath)
	if hash == hash3 {
		t.Error("Different content produced same hash")
	}
}

func TestHashDir(t *testing.T) {
	dir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "a.dot"), []byte("a"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "sub", "b.dot"), []byte("b"), 0644)

	hash, err := hashDir(dir)
	if err != nil {
		t.Fatalf("hashDir failed: %v", err)
	}
	if len(hash) != 64 {
		t.Errorf("Expected 64-char hex hash, got %d chars", len(hash))
	}
}

func TestInstallSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "dot.toml")
	modulesDir := filepath.Join(dir, ".dot", "modules")

	manifest := `[dependencies]
mylib = "1.0.0"
`
	_ = os.WriteFile(manifestPath, []byte(manifest), 0644)

	_ = os.MkdirAll(filepath.Join(modulesDir, "mylib"), 0755)
	content := []byte("fn hello() {}\n")
	_ = os.WriteFile(filepath.Join(modulesDir, "mylib", "index.dot"), content, 0644)

	// Install should skip because the directory already exists
	if err := Install(dir); err != nil {
		t.Fatalf("Install failed: %v", err)
	}
}
