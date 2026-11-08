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
	os.Exit(m.Run())
}

func writeFile(path string, data []byte) error { return os.WriteFile(path, data, 0o644) }
