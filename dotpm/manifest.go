package dotpm

import (
	"bufio"
	"os"
	"strings"
)

type Manifest struct {
	Name         string
	Version      string
	Dependencies map[string]string
}

// ParseManifest reads and parses the dot.toml file.
func ParseManifest(path string) (*Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	m := &Manifest{
		Dependencies: make(map[string]string),
	}

	scanner := bufio.NewScanner(file)
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.Trim(strings.TrimSpace(parts[0]), `"`)
		value := strings.Trim(strings.TrimSpace(parts[1]), `"`)

		switch currentSection {
		case "package":
			if key == "name" {
				m.Name = value
			} else if key == "version" {
				m.Version = value
			}
		case "dependencies":
			m.Dependencies[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return m, nil
}
