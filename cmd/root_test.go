package cmd

import (
	"bytes"
	"encoding/json"
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

// -v is reserved for --verbose, so --version must not claim it.
func TestRootVersionFlagHasNoShorthand(t *testing.T) {
	f := rootCmd.Flags().Lookup("version")
	require.NotNil(t, f)
	assert.Empty(t, f.Shorthand)
	assert.Nil(t, rootCmd.Flags().ShorthandLookup("v"), "-v must stay free for --verbose")
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

// resetFlags returns the global flags to their defaults and unmarks them as
// set, since the shared root command would otherwise carry them from one test
// into the next.
func resetFlags() {
	for _, name := range []string{"config-dir", "upstream", "output", "json"} {
		f := rootCmd.PersistentFlags().Lookup(name)
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	}
}
