//go:build ignore

// Run with: go run scripts/check-manifests.go
package main

import (
	"fmt"
	"github.com/goccy/go-yaml"
	"os"
)

func main() {
	for _, path := range []string{"compose.yaml", ".github/workflows/quality.yaml", ".github/workflows/release.yaml", "docs/api/openapi.yaml", "editions/linux/docker/compose.yaml", ".github/workflows/linux-edition.yaml", ".github/workflows/mikrotik-edition.yaml", ".github/workflows/systemd-edition.yaml"} {
		raw, e := os.ReadFile(path)
		if e != nil {
			fail(e)
		}
		var doc map[string]any
		if e = yaml.Unmarshal(raw, &doc); e != nil {
			fail(fmt.Errorf("%s: %w", path, e))
		}
		if len(doc) == 0 {
			fail(fmt.Errorf("empty manifest: %s", path))
		}
		fmt.Println(path + ": YAML syntax valid")
	}
}
func fail(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
