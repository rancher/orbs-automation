package cmd

import (
	"os"
	"strings"
	"testing"
)

// TestMain gives every test the same environment, whatever runs the tests: a clean
// one. The commands read a lot of it (GERO_* settings, GitHub tokens and hosts, the
// XDG directories, colors, and whether this is CI), so a test that did not clear
// it would pass on one machine and fail on another: on a CI runner CI=true is set,
// and on a developer's machine whatever they exported or configured. A test about
// any of it sets what it needs with t.Setenv.
func TestMain(m *testing.M) {
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if scrubbed(key) {
			_ = os.Unsetenv(key)
		}
	}
	// A home and a cache of their own, so no test reads the user file, the gh
	// config, the git config or the cached bundle of whoever runs it, and offline,
	// so none reaches the network for the central bundle.
	home, err := os.MkdirTemp("", "gero-cmd-test-home-")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("HOME", home)
	_ = os.Setenv("USERPROFILE", home)
	_ = os.Setenv("XDG_CACHE_HOME", home+"/.cache")
	_ = os.Setenv("GERO_OFFLINE", "true")
	// The binary carries a config bundle, so a command with no config directory
	// would read that and not find none. Tests of the directory lookup and of local
	// config want the lookup alone; those of the bundle set the source themselves.
	_ = os.Setenv("GERO_CONFIG_SOURCE", "dir")
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

// scrubbed is whether the variable can change what a command does.
func scrubbed(key string) bool {
	for _, prefix := range []string{"GERO_", "GH_", "GITHUB_", "GIT_", "XDG_", "RUNNER_", "CLICOLOR"} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	switch key {
	case "CI", "NO_COLOR", "COLORTERM", "FORCE_COLOR", "TERM", "SHELL", "EDITOR", "VISUAL":
		return true
	}
	return false
}

func writeFile(path string, data []byte) error { return os.WriteFile(path, data, 0o644) }
