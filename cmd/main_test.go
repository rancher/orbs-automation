package cmd

import (
	"os"
	"testing"
)

// TestMain starts every test as a workstation, whatever runs the tests: on a CI
// runner CI=true is set, which turns commands into their CI behavior (JSON logs,
// no user file, a different set of doctor checks). A test about CI sets the
// variables itself with t.Setenv.
func TestMain(m *testing.M) {
	for _, k := range []string{"CI", "GITHUB_ACTIONS", "GERO_CONTEXT"} {
		_ = os.Unsetenv(k)
	}
	// With no local config a command looks for the central bundle. A test must not
	// reach the network, nor read the cache of whoever runs it, so every test is
	// offline with a cache of its own, which a test that wants a bundle fills.
	cache, err := os.MkdirTemp("", "gero-cmd-test-cache-")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("XDG_CACHE_HOME", cache)
	_ = os.Setenv("GERO_OFFLINE", "true")
	code := m.Run()
	_ = os.RemoveAll(cache)
	os.Exit(code)
}

func writeFile(path string, data []byte) error { return os.WriteFile(path, data, 0o644) }
