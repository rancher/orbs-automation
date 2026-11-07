package repl

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/ui/editor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type shellCall struct {
	shell, dir string
	env        []string
}

// fakeShell records what the REPL asked for and starts nothing.
func fakeShell(calls *[]shellCall, err error) ShellFunc {
	return func(_ context.Context, shell, dir string, env []string) error {
		*calls = append(*calls, shellCall{shell, dir, env})
		return err
	}
}

// userShell is an executable file standing in for $SHELL.
func userShell(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "zsh")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755))
	return path
}

func TestShellStartsTheUsersShellInTheWorkingDirectory(t *testing.T) {
	var calls []shellCall
	sh := userShell(t)
	s := &Session{Dir: "/work", Env: []string{"SHELL=" + sh, "FOO=bar", "GH_TOKEN=secret"}, StartShell: fakeShell(&calls, nil)}

	out, exit, err := exec(t, s, "shell")

	require.NoError(t, err)
	assert.False(t, exit)
	assert.Empty(t, out)
	require.Len(t, calls, 1)
	assert.Equal(t, sh, calls[0].shell)
	assert.Equal(t, "/work", calls[0].dir)
	assert.Equal(t, []string{"SHELL=" + sh, "FOO=bar", "GH_TOKEN=secret", "GERO_REPL=1"}, calls[0].env,
		"the whole environment, tokens included, and the marker")
}

func TestShellMarkerReplacesOneAlreadyThere(t *testing.T) {
	var calls []shellCall
	s := &Session{Dir: "/", Env: []string{"GERO_REPL=old", "SHELL=" + userShell(t)}, StartShell: fakeShell(&calls, nil)}

	_, _, err := exec(t, s, "shell")

	require.NoError(t, err)
	require.Len(t, calls, 1)
	var markers []string
	for _, e := range calls[0].env {
		if strings.HasPrefix(e, "GERO_REPL=") {
			markers = append(markers, e)
		}
	}
	assert.Equal(t, []string{"GERO_REPL=1"}, markers)
}

func TestShellTakesNoArguments(t *testing.T) {
	var calls []shellCall
	s := &Session{Dir: "/", Env: []string{"SHELL=" + userShell(t)}, StartShell: fakeShell(&calls, nil)}

	for _, line := range []string{"shell init zsh", "shell install", "shell -l"} {
		_, _, err := exec(t, s, line)

		require.Error(t, err, line)
		assert.Contains(t, err.Error(), "no arguments", line)
	}
	assert.Empty(t, calls, "never passed on to the tree's gero shell")
}

func TestShellNeedsAUsableDollarShell(t *testing.T) {
	notExec := filepath.Join(t.TempDir(), "plain")
	require.NoError(t, os.WriteFile(notExec, nil, 0o644))
	rows := map[string][]string{
		"unset":          {"FOO=bar"},
		"empty":          {"SHELL="},
		"missing":        {"SHELL=" + filepath.Join(t.TempDir(), "nope")},
		"a directory":    {"SHELL=" + t.TempDir()},
		"not executable": {"SHELL=" + notExec},
		"relative":       {"SHELL=zsh"},
	}
	for name, env := range rows {
		var calls []shellCall
		s := &Session{Dir: "/", Env: env, StartShell: fakeShell(&calls, nil)}

		_, _, err := exec(t, s, "shell")

		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "$SHELL", name)
		assert.Empty(t, calls, name)
	}
}

func TestShellFailureToStartIsAnError(t *testing.T) {
	boom := errors.New("boom")
	var calls []shellCall
	s := &Session{Dir: "/", Env: []string{"SHELL=" + userShell(t)}, StartShell: fakeShell(&calls, boom)}

	_, exit, err := exec(t, s, "shell")

	require.Len(t, calls, 1)
	assert.ErrorIs(t, err, boom)
	assert.False(t, exit)
}

func TestShellSeesWhereCdMovedTo(t *testing.T) {
	var calls []shellCall
	clone := t.TempDir()
	start := t.TempDir()
	t.Chdir(start)
	s := &Session{Dir: start, Env: []string{"SHELL=" + userShell(t)}, StartShell: fakeShell(&calls, nil),
		ProjectDir: projects(map[string]string{"scc": clone})}

	_, _, err := exec(t, s, "cd scc")
	require.NoError(t, err)
	_, _, err = exec(t, s, "shell")

	require.NoError(t, err)
	require.Len(t, calls, 1)
	assert.Equal(t, clone, calls[0].dir)
}

func TestLoopGoesOnAfterTheShellReturns(t *testing.T) {
	var calls []shellCall
	s := &Session{Dir: "/work", Env: []string{"SHELL=" + userShell(t)}, StartShell: fakeShell(&calls, nil)}
	var out, errOut bytes.Buffer

	err := Loop(context.Background(), editor.NewPlain(strings.NewReader("shell\npwd\nexit\n"), &out), &out, &errOut, s)

	require.NoError(t, err)
	assert.Len(t, calls, 1)
	assert.Equal(t, "gero> gero> /work\ngero> ", out.String())
	assert.Empty(t, errOut.String())
}

// ShellOn is the real hand-off, tried on a non-interactive sh: the streams are
// the ones given, and a non-zero exit is not an error.
func TestShellOnRunsTheProgramOnTheGivenStreams(t *testing.T) {
	var out, errOut bytes.Buffer
	start := ShellOn(strings.NewReader("echo \"hi $GERO_REPL\"\nexit 4\n"), &out, &errOut)

	err := start(context.Background(), "/bin/sh", t.TempDir(), []string{"GERO_REPL=1"})

	require.NoError(t, err)
	assert.Equal(t, "hi 1\n", out.String())
}

func TestShellOnFailsToStartAMissingProgram(t *testing.T) {
	start := ShellOn(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})

	err := start(context.Background(), filepath.Join(t.TempDir(), "nope"), t.TempDir(), nil)

	assert.Error(t, err)
}
