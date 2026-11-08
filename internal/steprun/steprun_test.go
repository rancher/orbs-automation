package steprun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/execx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// script writes an executable shell script at rel in a new clone directory and
// returns the directory.
func script(t *testing.T, rel, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755))
	return dir
}

func run(dir string, cmd ...string) error {
	return Runner{}.Run(context.Background(), chain.Step{Run: cmd}, bump.StepEnv{Dir: dir})
}

func TestARepositoryScriptRunsInTheCloneWithTheCallsVariables(t *testing.T) {
	dir := script(t, "scripts/gen", `printf '%s|%s|%s' "$PWD" "$GERO_TAG" "$1" > out.txt`)

	err := Runner{}.Run(context.Background(),
		chain.Step{Run: v1alpha2.Command{"scripts/gen", "arg"}},
		bump.StepEnv{Dir: dir, Env: map[string]string{"GERO_TAG": "v1.2.3"}})

	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	require.NoError(t, err)
	wd, _ := filepath.EvalSymlinks(dir)
	gotWD, _, _ := strings.Cut(string(got), "|")
	gotReal, _ := filepath.EvalSymlinks(gotWD)
	assert.Equal(t, wd, gotReal)
	assert.Contains(t, string(got), "|v1.2.3|arg")
}

func TestABareNameIsAToolOnThePath(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, run(dir, "true"))

	err := run(dir, "false")
	var exit *execx.ExitError
	require.ErrorAs(t, err, &exit)
	assert.Equal(t, 1, exit.ExitCode)
}

func TestAFailureEndsWithWhatTheStepWroteToStderr(t *testing.T) {
	dir := script(t, "fail.sh", "echo one >&2; echo 'no go toolchain' >&2; exit 3")

	err := run(dir, "./fail.sh")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "one; no go toolchain")
	var exit *execx.ExitError
	require.ErrorAs(t, err, &exit)
	assert.Equal(t, 3, exit.ExitCode)
}

func TestTheEnvironmentIsScrubbed(t *testing.T) {
	t.Setenv("GH_TOKEN", "secret")
	t.Setenv("GERO_TEST_LEAK", "leak")
	dir := script(t, "env.sh", `env > env.txt`)

	require.NoError(t, run(dir, "./env.sh"))

	got, err := os.ReadFile(filepath.Join(dir, "env.txt"))
	require.NoError(t, err)
	assert.NotContains(t, string(got), "GH_TOKEN")
	assert.NotContains(t, string(got), "GERO_TEST_LEAK")
	assert.Contains(t, string(got), "PATH=")
}

func TestAScriptCannotLeaveTheClone(t *testing.T) {
	dir := script(t, "ok.sh", "")
	outside := script(t, "evil.sh", "touch pwned")
	require.NoError(t, os.Symlink(filepath.Join(outside, "evil.sh"), filepath.Join(dir, "link.sh")))

	for _, cmd := range []string{"../evil.sh", "scripts/../../evil.sh", filepath.Join(outside, "evil.sh"), "./link.sh"} {
		err := run(dir, cmd)

		require.ErrorIs(t, err, ErrOutsideClone, cmd)
	}
	_, err := os.Stat(filepath.Join(outside, "pwned"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestAStepNeedsACommand(t *testing.T) {
	require.ErrorIs(t, run(t.TempDir()), ErrEmptyCommand)
	require.ErrorIs(t, run(t.TempDir(), ""), ErrEmptyCommand)
}

func TestOnlyTheConsumersFilesAreSupportedForNow(t *testing.T) {
	dir := script(t, "x.sh", "")
	for _, from := range []string{FromProducer, FromCentral, "elsewhere"} {
		err := Runner{}.Run(context.Background(), chain.Step{Run: v1alpha2.Command{"./x.sh"}, From: from},
			bump.StepEnv{Dir: dir})

		require.ErrorIs(t, err, ErrFromUnsupported, from)
	}
	require.NoError(t, Runner{}.Run(context.Background(), chain.Step{Run: v1alpha2.Command{"./x.sh"}, From: FromConsumer},
		bump.StepEnv{Dir: dir}))
}

func TestACancelledContextStopsTheStep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Runner{}.Run(ctx, chain.Step{Run: v1alpha2.Command{"true"}}, bump.StepEnv{Dir: t.TempDir()})

	require.ErrorIs(t, err, context.Canceled)
}
