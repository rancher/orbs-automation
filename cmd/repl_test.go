package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/rancher/orbs-automation/internal/cli/repl"
	"github.com/rancher/orbs-automation/internal/clicfg"

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

func TestTreeCompleteIsWhatCobraCompletes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	assert.Contains(t, treeComplete(nil, "do"), "doctor")
	assert.Contains(t, treeComplete([]string{"repo"}, "st"), "status")
	assert.Contains(t, treeComplete([]string{"repo", "status"}, "--jso"), "--json")
	assert.Empty(t, treeComplete(nil, "zzzz"))
}

func TestTreeCompleteDoesNotOfferCobrasOwnCommandsAsDescriptions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for _, w := range treeComplete([]string{"repo"}, "") {
		assert.NotContains(t, w, "\t", "a word, without the description cobra puts after a tab")
		assert.NotContains(t, w, ":", "and not the directive line")
	}
}

func TestReplCompletionHasTheTreeTheREPLsOwnCommandsAndTheDirectory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "docs"), 0o755))
	c := replCompletion(clicfg.Settings{}, &repl.Session{Dir: dir})

	first := c.Complete("p")
	assert.Contains(t, first, "pwd", "the REPL's own")
	assert.Contains(t, c.Complete("do"), "doctor", "the tree's")
	assert.Contains(t, c.Complete("ls d"), "docs/", "a directory of the session's")
	assert.Contains(t, c.Complete("repo st"), "status")
}

func TestReplCompletionFollowsTheWorkingDirectoryOfTheSession(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, b := t.TempDir(), t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(a, "alpha"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(b, "beta"), 0o755))
	sess := &repl.Session{Dir: a}
	c := replCompletion(clicfg.Settings{}, sess)

	assert.Contains(t, c.Complete("ls a"), "alpha/")
	sess.Dir = b

	assert.Contains(t, c.Complete("ls b"), "beta/")
	assert.NotContains(t, c.Complete("ls a"), "alpha/")
}

func TestASessionStylesItsPromptFromTheTerminal(t *testing.T) {
	r := &promptRecorder{}
	s := &repl.Session{Dir: "/"}
	s.UseTerminal(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})

	require.NoError(t, repl.Loop(context.Background(), r, io.Discard, io.Discard, s))

	require.Len(t, r.prompts, 1)
	assert.Contains(t, r.prompts[0], "\x1b[", "a terminal with color gets a styled prompt")
	assert.Equal(t, "gero> ", regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(r.prompts[0], ""))

	plain := &promptRecorder{}
	p := &repl.Session{Dir: "/"}
	p.UseTerminal(clicfg.Terminal{})
	require.NoError(t, repl.Loop(context.Background(), plain, io.Discard, io.Discard, p))
	assert.Equal(t, []string{"gero> "}, plain.prompts)
}

// promptRecorder is a line reader that records its prompts and ends at once.
type promptRecorder struct{ prompts []string }

func (p *promptRecorder) ReadLine(prompt string) (string, error) {
	p.prompts = append(p.prompts, prompt)
	return "", io.EOF
}

func (*promptRecorder) Close() error { return nil }
