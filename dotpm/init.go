package dotpm

import (
	"encoding/json"
	"os"
)

// Init generates a default dot.json manifest file in the current directory.
func Init(name string) error {
	m := Manifest{
		Name:         name,
		Version:      "0.1.0",
		Dependencies: make(map[string]string),
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("dot.json", data, 0644)
}
