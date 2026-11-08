// Command genschema writes the JSON Schemas of gero's config files, for editors
// to validate and complete them: one for each kind of the application config, one
// that holds them all and picks by kind (for a file of several documents), and one
// for the user file. Run it from the repository root:
//
//	go run ./tools/genschema
//
// The schemas are derived from the Go types the files decode into, so they follow
// the decoder; what tags cannot say (the values a field may have) is in hints.go.
// The output is committed under generated/jsonschema, and a test fails when it is
// stale.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	out := flag.String("out", filepath.Join("generated", "jsonschema"), "directory to write the schemas to")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "genschema:", err)
		os.Exit(1)
	}
}

func run(out string) error {
	if _, err := os.Stat("go.mod"); err != nil {
		return fmt.Errorf("run it from the repository root, where go.mod is: %w", err)
	}
	files, err := build()
	if err != nil {
		return err
	}
	for name, data := range files {
		path := filepath.Join(out, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
