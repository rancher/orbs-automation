// Command compilebundle writes the central config bundle: the config/ directory
// of the central repository as one file, for a gero release to carry beside its
// binaries (docs/specs/CENTRAL_BUNDLE.md). It is not part of gero. Run it from
// the repository root, after GoReleaser, which empties dist/ when it starts:
//
//	go run ./tools/compilebundle --version v0.5.0 --commit "$(git rev-parse HEAD)" \
//	    --date "$(git show -s --format=%cI HEAD)"
//
// The version, commit and date go in the header, and are flags and not read from
// git or the clock so that the same input gives the same bytes.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	configDir := flag.String("config", "config", "the central config directory to bundle")
	out := flag.String("out", "dist", "directory to write "+bundleName+" to")
	version := flag.String("version", "dev", "the gero release the bundle is for")
	commit := flag.String("commit", "none", "the commit the config was read from")
	date := flag.String("date", time.Unix(0, 0).UTC().Format(time.RFC3339), "the compile time, RFC 3339")
	flag.Parse()
	compiledAt, err := time.Parse(time.RFC3339, *date)
	if err != nil {
		fmt.Fprintln(os.Stderr, "compilebundle: --date:", err)
		os.Exit(2)
	}
	h := Header{GeroVersion: *version, SourceCommit: *commit, CompiledAt: compiledAt}
	if err := run(*configDir, *out, h); err != nil {
		fmt.Fprintln(os.Stderr, "compilebundle:", err)
		os.Exit(1)
	}
}

func run(configDir, out string, h Header) error {
	if _, err := os.Stat("go.mod"); err != nil {
		return fmt.Errorf("run it from the repository root, where go.mod is: %w", err)
	}
	data, err := Compile(os.DirFS(configDir), h)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, bundleName), data, 0o644)
}
