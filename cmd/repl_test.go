package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runLine(t *testing.T, run func(context.Context, string, []string) error, args ...string) error {
	t.Helper()
	return run(context.Background(), t.TempDir(), args)
}

func TestReplLineRunsACommandOfTheTree(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	run := lineRunner(nil, bytes.NewReader(nil), &out, &errOut)

	err := runLine(t, run, "version")

	require.NoError(t, err)
	assert.Equal(t, "gero version dev (commit none, built unknown)\n", out.String())
}

// cobra keeps flag values on a command, so a --json on one line must not reach
// the next: each line has a tree of its own.
func TestReplFlagsOfALineDoNotCarryToTheNext(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	run := lineRunner(nil, bytes.NewReader(nil), &out, &errOut)

	require.NoError(t, runLine(t, run, "version", "--json"))
	assert.Contains(t, out.String(), `"version"`)
	out.Reset()
	require.NoError(t, runLine(t, run, "version"))

	assert.Equal(t, "gero version dev (commit none, built unknown)\n", out.String())
}

func TestReplFlagsGivenToReplApplyToEveryLine(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	run := lineRunner([]string{"--json"}, bytes.NewReader(nil), &out, &errOut)

	require.NoError(t, runLine(t, run, "version"))
	assert.Contains(t, out.String(), `"version"`)
	out.Reset()
	require.NoError(t, runLine(t, run, "version"))

	assert.Contains(t, out.String(), `"version"`)
}

func TestReplLineErrorIsReturnedAndNotPrintedByCobra(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	run := lineRunner(nil, bytes.NewReader(nil), &out, &errOut)

	err := runLine(t, run, "nosuchcommand")

	require.Error(t, err)
	assert.Empty(t, errOut.String(), "the loop prints the error, once")
}

// Wiring only: the checks are tested in internal/cli/repl.
func TestReplRefusesWithoutATerminal(t *testing.T) {
	_, _, err := execRoot(t, "repl")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal")
}

func TestReplRefusesInCI(t *testing.T) {
	t.Setenv("CI", "true")

	_, _, err := execRoot(t, "repl")

	require.Error(t, err)
}

func TestBaseFlagsCarriesWhatWasGivenToRepl(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"repl", "--dry-run", "--config-dir", "/cfg", "-vv", "--color", "never", "--no-color"})
	var got []string
	for _, c := range root.Commands() {
		if c.Name() == "repl" {
			c.RunE = func(cmd *cobra.Command, _ []string) error { got = baseFlags(cmd); return nil }
			c.PersistentPreRunE = func(*cobra.Command, []string) error { return nil }
		}
	}

	require.NoError(t, root.Execute())

	assert.Equal(t, []string{"--config-dir=/cfg", "--color=never", "--no-color=true", "--dry-run=true", "--verbose=2"}, got)
}

// A renamed global flag must be renamed in globalFlags, or it would not reach
// the lines.
func TestGlobalFlagsAreDeclared(t *testing.T) {
	flags := newRootCmd().PersistentFlags()

	for _, name := range globalFlags {
		assert.NotNil(t, flags.Lookup(name), name)
	}
}
