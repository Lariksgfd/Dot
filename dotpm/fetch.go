package dotpm

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
)

// Fetch downloads a .dot file from a URL and saves it to dot_modules/.
// The filename is derived from the URL path (e.g. "https://example.com/foo.dot" -> "dot_modules/foo.dot").
func Fetch(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: unexpected status %d", url, resp.StatusCode)
	}

	filename := path.Base(url)
	if !strings.HasSuffix(filename, ".dot") {
		filename += ".dot"
	}

	if err := os.MkdirAll("dot_modules", 0755); err != nil {
		return fmt.Errorf("create dot_modules/: %w", err)
	}

	dest := "dot_modules/" + filename
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}

	fmt.Printf("fetched %s -> %s\n", url, dest)
	return nil
}
