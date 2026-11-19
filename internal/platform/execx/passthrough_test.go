package execx

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sh(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	return path
}

func TestPassthroughRunsWithTheStreamsDirectoryAndEnvironmentGiven(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer

	code, err := Passthrough(context.Background(), PassCmd{
		Path: sh(t), Dir: dir,
		Args: []string{"-c", `read line; echo "$line|$FOO|$GH_TOKEN"; pwd; echo oops >&2`},
		Env:  []string{"FOO=bar", "GH_TOKEN=secret"},
		In:   strings.NewReader("typed\n"), Out: &out, Err: &errOut,
	})

	require.NoError(t, err)
	assert.Equal(t, 0, code)
	want, evalErr := filepath.EvalSymlinks(dir)
	require.NoError(t, evalErr)
	assert.Equal(t, "typed|bar|secret\n"+want+"\n", out.String(), "the token is not scrubbed")
	assert.Equal(t, "oops\n", errOut.String())
}

func TestPassthroughGivesOnlyTheEnvironmentItIsGiven(t *testing.T) {
	t.Setenv("EXECX_LEAK", "yes")
	var out bytes.Buffer

	_, err := Passthrough(context.Background(), PassCmd{
		Path: sh(t), Args: []string{"-c", `echo "[$EXECX_LEAK]"`}, Env: []string{"A=b"}, Out: &out,
	})

	require.NoError(t, err)
	assert.Equal(t, "[]\n", out.String())
}

func TestPassthroughExitStatusIsNotAnError(t *testing.T) {
	code, err := Passthrough(context.Background(), PassCmd{Path: sh(t), Args: []string{"-c", "exit 3"}})

	require.NoError(t, err)
	assert.Equal(t, 3, code)
}

func TestPassthroughFailsWhenTheProgramCannotStart(t *testing.T) {
	_, err := Passthrough(context.Background(), PassCmd{Path: filepath.Join(t.TempDir(), "nope")})

	assert.Error(t, err)
}

func TestPassthroughStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	code, err := Passthrough(ctx, PassCmd{Path: sh(t), Args: []string{"-c", "sleep 5"}})

	// Killed before it ran, or while it did: it never reports success.
	assert.True(t, err != nil || code != 0, "code %d, err %v", code, err)
}

// fakeEditor puts a program called name on PATH that writes a file called started in the
// directory it is given, and returns that PATH's directory.
func fakeEditor(t *testing.T, name string) {
	t.Helper()
	bin := t.TempDir()
	script := "#!/bin/sh\ntouch \"$1/started\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestOnPathFindsAProgramOnPATHAndNotOneThatIsNot(t *testing.T) {
	fakeEditor(t, "gero-fake-editor")

	assert.True(t, OnPath("gero-fake-editor"))
	assert.False(t, OnPath("gero-no-such-editor"))
}

func TestLaunchStartsTheProgramOnTheDirectory(t *testing.T) {
	fakeEditor(t, "gero-fake-editor")
	dir := t.TempDir()

	require.NoError(t, Launch("gero-fake-editor", dir))

	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(dir, "started"))
		return err == nil
	}, 5*time.Second, 20*time.Millisecond, "the program ran with the directory as its argument")
}

func TestLaunchOfAProgramThatIsNotThereIsAnError(t *testing.T) {
	err := Launch("gero-no-such-editor", t.TempDir())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gero-no-such-editor")
}
