package dotpm

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestInitAndParse(t *testing.T) {
	// Clean up after test
	defer os.Remove("dot.json")

	err := Init("test-pkg")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	m, err := ParseManifest("dot.json")
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
	defer os.Remove("dot.json")
	defer os.RemoveAll("dot_modules")

	err := Init("test-pkg")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Add dummy dependency
	m, _ := ParseManifest("dot.json")
	m.Dependencies["dummy-lib"] = "1.0.0"
	os.WriteFile("dot.json", []byte(`{"name":"test-pkg","version":"0.1.0","dependencies":{"dummy-lib":"1.0.0"}}`), 0644)

	err = Get("dot.json")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if _, err := os.Stat("dot_modules/dummy-lib"); os.IsNotExist(err) {
		t.Errorf("Expected dot_modules/dummy-lib to be created")
	}
}

func TestFetch(t *testing.T) {
	defer os.RemoveAll("dot_modules")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fn main() { print(\"hello\") }\n"))
	}))
	defer ts.Close()

	url := ts.URL + "/mylib.dot"
	if err := Fetch(url); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	data, err := os.ReadFile("dot_modules/mylib.dot")
	if err != nil {
		t.Fatalf("Expected dot_modules/mylib.dot to exist: %v", err)
	}
	if string(data) != "fn main() { print(\"hello\") }\n" {
		t.Errorf("Unexpected file contents: %q", string(data))
	}
}

func TestFetchServerError(t *testing.T) {
	defer os.RemoveAll("dot_modules")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	err := Fetch(ts.URL + "/missing.dot")
	if err == nil {
		t.Fatal("Expected error for non-200 response, got nil")
	}
}

func TestLockCreatesFile(t *testing.T) {
	dir := t.TempDir()

	manifestPath := filepath.Join(dir, "dot.json")
	modulesDir := filepath.Join(dir, "dot_modules")
	lockPath := filepath.Join(dir, "dot-lock.json")

	// Write manifest directly
	manifest := `{"name":"test-pkg","version":"0.1.0","dependencies":{"lib-a":"1.0.0","lib-b":"2.3.1"}}`
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

	lb, ok := lock.Dependencies["lib-b"]
	if !ok {
		t.Fatal("lib-b not in lock")
	}
	if lb.Version != "2.3.1" {
		t.Errorf("lib-b version: expected 2.3.1, got %s", lb.Version)
	}
}

func TestVerifySuccess(t *testing.T) {
	dir := t.TempDir()

	manifestPath := filepath.Join(dir, "dot.json")
	modulesDir := filepath.Join(dir, "dot_modules")

	manifest := `{"name":"test-pkg","version":"0.1.0","dependencies":{"mylib":"1.2.0"}}`
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

	manifest := `{"name":"test-pkg","version":"0.1.0","dependencies":{"mylib":"1.2.0"}}`
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

	manifest := `{"name":"test-pkg","version":"0.1.0","dependencies":{"mylib":"1.2.0"}}`
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

	// Same content -> same hash
	hash2, _ := computeFileHash(filePath)
	if hash != hash2 {
		t.Error("Same content produced different hashes")
	}

	// Different content -> different hash
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

func TestInstallDownloadsMissing(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "dot.json")
	modulesDir := filepath.Join(dir, "dot_modules")

	manifest := `{"name":"test-pkg","version":"0.1.0","dependencies":{"mylib":"1.0.0"}}`
	_ = os.WriteFile(manifestPath, []byte(manifest), 0644)

	_ = os.MkdirAll(modulesDir, 0755)
	content := []byte("fn hello() { print(\"hi\") }\n")
	_ = os.WriteFile(filepath.Join(modulesDir, "mylib.dot"), content, 0644)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer ts.Close()

	oldResolve := ResolveURL
	ResolveURL = func(name, version string) string {
		return ts.URL + "/" + name + ".dot"
	}
	defer func() { ResolveURL = oldResolve }()

	if err := Lock(dir); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	_ = os.RemoveAll(modulesDir)

	if err := Install(dir); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(modulesDir, "mylib.dot"))
	if err != nil {
		t.Fatalf("Expected mylib.dot to exist: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("Unexpected content: %q", string(data))
	}
}

func TestInstallSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "dot.json")
	modulesDir := filepath.Join(dir, "dot_modules")

	manifest := `{"name":"test-pkg","version":"0.1.0","dependencies":{"mylib":"1.0.0"}}`
	_ = os.WriteFile(manifestPath, []byte(manifest), 0644)

	_ = os.MkdirAll(modulesDir, 0755)
	content := []byte("fn hello() {}\n")
	_ = os.WriteFile(filepath.Join(modulesDir, "mylib.dot"), content, 0644)

	serverCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer ts.Close()

	oldResolve := ResolveURL
	ResolveURL = func(name, version string) string {
		return ts.URL + "/" + name + ".dot"
	}
	defer func() { ResolveURL = oldResolve }()

	if err := Lock(dir); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	if err := Install(dir); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if serverCalled {
		t.Error("Server was called even though dep was already installed with correct hash")
	}
}

func TestInstallRedownloadsOnHashMismatch(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "dot.json")
	modulesDir := filepath.Join(dir, "dot_modules")

	manifest := `{"name":"test-pkg","version":"0.1.0","dependencies":{"mylib":"1.0.0"}}`
	_ = os.WriteFile(manifestPath, []byte(manifest), 0644)

	_ = os.MkdirAll(modulesDir, 0755)
	originalContent := []byte("fn hello() { print(\"hi\") }\n")
	_ = os.WriteFile(filepath.Join(modulesDir, "mylib.dot"), originalContent, 0644)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(originalContent)
	}))
	defer ts.Close()

	oldResolve := ResolveURL
	ResolveURL = func(name, version string) string {
		return ts.URL + "/" + name + ".dot"
	}
	defer func() { ResolveURL = oldResolve }()

	if err := Lock(dir); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	_ = os.WriteFile(filepath.Join(modulesDir, "mylib.dot"), []byte("tampered\n"), 0644)

	if err := Install(dir); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(modulesDir, "mylib.dot"))
	if err != nil {
		t.Fatalf("Expected mylib.dot to exist: %v", err)
	}
	if string(data) != string(originalContent) {
		t.Errorf("Content not restored after tampering: %q", string(data))
	}
}

func TestInstallMissingLockFile(t *testing.T) {
	dir := t.TempDir()
	err := Install(dir)
	if err == nil {
		t.Fatal("Expected error when lock file missing, got nil")
	}
}
