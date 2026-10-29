package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execRoot runs the root command in process with args and returns what it
// wrote to stdout and stderr. It resets the shared root command afterwards.
func execRoot(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	// Keep a developer's real user file out of the tests.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		resetFlags()
	})

	err = rootCmd.Execute()
	return out.String(), errOut.String(), err
}

func TestRootVersionFlag(t *testing.T) {
	stdout, _, err := execRoot(t, "--version")

	require.NoError(t, err)
	assert.Equal(t, "gero version dev (commit none, built unknown)\n", stdout)
}

// -v belongs to --verbose, so --version must not claim it.
func TestRootVersionFlagHasNoShorthand(t *testing.T) {
	f := rootCmd.Flags().Lookup("version")
	require.NotNil(t, f)
	assert.Empty(t, f.Shorthand)
	v := rootCmd.PersistentFlags().ShorthandLookup("v")
	require.NotNil(t, v)
	assert.Equal(t, "verbose", v.Name)
}

func TestVersionSubcommand(t *testing.T) {
	stdout, _, err := execRoot(t, "version")

	require.NoError(t, err)
	assert.Equal(t, "gero version dev (commit none, built unknown)\n", stdout)
}

func TestVersionSubcommandJSON(t *testing.T) {
	stdout, _, err := execRoot(t, "version", "--json")

	require.NoError(t, err)
	var got map[string]string
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	assert.Equal(t, map[string]string{"version": "dev", "commit": "none", "date": "unknown"}, got)
}

func TestVerboseLogsToStderrOnly(t *testing.T) {
	stdout, stderr, err := execRoot(t, "-vv", "version")

	require.NoError(t, err)
	assert.Equal(t, "gero version dev (commit none, built unknown)\n", stdout)
	assert.Contains(t, stderr, "level=DEBUG")
	assert.Contains(t, stderr, "settings loaded")
}

func TestVerboseLeavesJSONOutputClean(t *testing.T) {
	stdout, stderr, err := execRoot(t, "-vv", "version", "--json")

	require.NoError(t, err)
	assert.True(t, json.Valid([]byte(stdout)), stdout)
	assert.Contains(t, stderr, "settings loaded")
}

func TestDefaultLevelIsQuiet(t *testing.T) {
	_, stderr, err := execRoot(t, "version")

	require.NoError(t, err)
	assert.Empty(t, stderr)
}

func TestInfoLevelHidesDebug(t *testing.T) {
	_, stderr, err := execRoot(t, "-v", "version")

	require.NoError(t, err)
	assert.NotContains(t, stderr, "settings loaded")
}

func TestGeroLogSetsLevel(t *testing.T) {
	t.Setenv("GERO_LOG", "debug")

	_, stderr, err := execRoot(t, "version")

	require.NoError(t, err)
	assert.Contains(t, stderr, "settings loaded")
}

func TestVerboseFlagBeatsGeroLog(t *testing.T) {
	t.Setenv("GERO_LOG", "error")

	_, stderr, err := execRoot(t, "-vv", "version")

	require.NoError(t, err)
	assert.Contains(t, stderr, "settings loaded")
}

func TestInvalidGeroLogFails(t *testing.T) {
	t.Setenv("GERO_LOG", "loud")

	_, _, err := execRoot(t, "version")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "log (from env)")
}

func TestLogsAreJSONInCI(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")

	_, stderr, err := execRoot(t, "-vv", "version")

	require.NoError(t, err)
	var line map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stderr)), &line), stderr)
	assert.Equal(t, "settings loaded", line["msg"])
}

func TestForcedContextChoosesLogHandler(t *testing.T) {
	t.Run("GERO_CONTEXT=ci logs JSON without CI variables", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "")
		t.Setenv("CI", "")
		t.Setenv("GERO_CONTEXT", "ci")

		_, stderr, err := execRoot(t, "-vv", "version")

		require.NoError(t, err)
		var line map[string]any
		require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stderr)), &line), stderr)
	})

	t.Run("GERO_CONTEXT=workstation logs text under GITHUB_ACTIONS", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("GERO_CONTEXT", "workstation")

		_, stderr, err := execRoot(t, "-vv", "version")

		require.NoError(t, err)
		var line map[string]any
		assert.Error(t, json.Unmarshal([]byte(strings.TrimSpace(stderr)), &line), stderr)
		assert.Contains(t, stderr, "settings loaded")
	})
}

func TestInvalidGeroContextFails(t *testing.T) {
	t.Setenv("GERO_CONTEXT", "laptop")

	_, _, err := execRoot(t, "version")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GERO_CONTEXT")
}

// resetFlags returns the global flags to their defaults and unmarks them as
// set, since the shared root command would otherwise carry them from one test
// into the next.
func resetFlags() {
	for _, name := range []string{"config-dir", "upstream", "output", "json", "verbose"} {
		f := rootCmd.PersistentFlags().Lookup(name)
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	}
}

// stubAuth is a github.Auth with a fixed token, swapped in for the real one
// that CI-only gating consults.
type stubAuth struct{ token string }

func (a stubAuth) TokenForHost(string) (string, string) { return a.token, "GH_TOKEN" }
func (stubAuth) DefaultHost() (string, string)          { return "github.com", "default" }
func (stubAuth) IsEnterprise(string) bool               { return false }

func TestCIOnlyCommandsNeedCIContextAndToken(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		token   string
		wantErr string // empty: the gate passes
	}{
		{"workstation", nil, "t0ken", "CI context"},
		{"CI=true", map[string]string{"CI": "true"}, "t0ken", ""},
		{"GITHUB_ACTIONS=true", map[string]string{"GITHUB_ACTIONS": "true"}, "t0ken", ""},
		{"GERO_CONTEXT=ci alone, to debug locally", map[string]string{"GERO_CONTEXT": "ci"}, "t0ken", ""},
		{"GERO_CONTEXT=workstation on a runner", map[string]string{"GITHUB_ACTIONS": "true", "GERO_CONTEXT": "workstation"}, "t0ken", "CI context"},
		{"CI without a token", map[string]string{"GITHUB_ACTIONS": "true"}, "", "no GitHub token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"GITHUB_ACTIONS", "CI", "GERO_CONTEXT"} {
				t.Setenv(k, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			old := gateAuth
			gateAuth = stubAuth{token: tt.token}
			t.Cleanup(func() { gateAuth = old })

			_, _, err := execRoot(t, "ci:reconcile")

			require.Error(t, err) // the command itself is not implemented yet
			if tt.wantErr != "" {
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NotContains(t, err.Error(), "CI context")
				assert.NotContains(t, err.Error(), "no GitHub token")
			}
		})
	}
}

func TestOtherCommandsRunOutsideCI(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")

	_, _, err := execRoot(t, "version")

	require.NoError(t, err)
}
