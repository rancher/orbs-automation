package execx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeBin writes an executable shell script called name into a new directory
// and puts that directory first on PATH for the test, so the Runner finds the
// script as if it were a real binary. It returns the directory.
func fakeBin(t *testing.T, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binaries are shell scripts")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func TestLookup(t *testing.T) {
	t.Run("returns the absolute path of an allowed binary", func(t *testing.T) {
		dir := fakeBin(t, "execx-fake", "exit 0")

		got, err := New("execx-fake").Lookup("execx-fake")

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "execx-fake"), got)
	})

	t.Run("refuses a binary that is not on the allow list", func(t *testing.T) {
		fakeBin(t, "execx-fake", "exit 0")

		_, err := New("something-else").Lookup("execx-fake")

		assert.ErrorIs(t, err, ErrNotAllowed)
	})

	t.Run("reports an allowed binary that is not on PATH", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		_, err := New("execx-missing").Lookup("execx-missing")

		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("resolves once, so a later PATH change cannot swap the binary", func(t *testing.T) {
		first := fakeBin(t, "execx-fake", "echo first")
		r := New("execx-fake")
		before, err := r.Lookup("execx-fake")
		require.NoError(t, err)

		fakeBin(t, "execx-fake", "echo second") // a new directory, now first on PATH
		after, err := r.Lookup("execx-fake")

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(first, "execx-fake"), before)
		assert.Equal(t, before, after)
	})
}

func TestRunCapturesOutput(t *testing.T) {
	t.Run("returns raw stdout and stderr, untouched", func(t *testing.T) {
		fakeBin(t, "execx-fake", `printf '  out\n\n'; printf 'err\n' >&2`)

		got, err := New("execx-fake").Run(context.Background(), Cmd{Name: "execx-fake"})

		require.NoError(t, err)
		assert.Equal(t, "  out\n\n", string(got.Stdout), "no trimming")
		assert.Equal(t, "err\n", string(got.Stderr), "stderr is kept apart from stdout")
		assert.Equal(t, 0, got.ExitCode)
	})

	t.Run("passes the argument vector as given, with no shell in between", func(t *testing.T) {
		fakeBin(t, "execx-fake", `printf '%s|' "$@"`)
		args := []string{"a b", "$(touch pwned)", ";", "*", ""}

		got, err := New("execx-fake").Run(context.Background(), Cmd{Name: "execx-fake", Args: args})

		require.NoError(t, err)
		assert.Equal(t, "a b|$(touch pwned)|;|*||", string(got.Stdout))
	})

	t.Run("runs in Dir", func(t *testing.T) {
		fakeBin(t, "execx-fake", `pwd`)
		dir, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)

		got, err := New("execx-fake").Run(context.Background(), Cmd{Name: "execx-fake", Dir: dir})

		require.NoError(t, err)
		assert.Equal(t, dir+"\n", string(got.Stdout))
	})
}

func TestRunEnvironment(t *testing.T) {
	const show = `printf 'secret=%s home=%s' "${EXECX_SECRET-unset}" "${HOME-unset}"`

	t.Run("a nil Env does not hand over the whole environment", func(t *testing.T) {
		fakeBin(t, "execx-fake", show)
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("EXECX_SECRET", "hunter2")

		got, err := New("execx-fake").Run(context.Background(), Cmd{Name: "execx-fake"})

		require.NoError(t, err)
		assert.Equal(t, "secret=unset home="+home, string(got.Stdout), "HOME is in the default, the rest is not")
	})

	t.Run("a set Env is exactly the environment the program gets", func(t *testing.T) {
		fakeBin(t, "execx-fake", show)
		t.Setenv("HOME", t.TempDir())
		t.Setenv("EXECX_SECRET", "hunter2")

		got, err := New("execx-fake").Run(context.Background(), Cmd{Name: "execx-fake", Env: []string{"EXECX_SECRET=given"}})

		require.NoError(t, err)
		assert.Equal(t, "secret=given home=unset", string(got.Stdout))
	})
}

func TestRunFailures(t *testing.T) {
	t.Run("a non-zero exit is an ExitError and the Result is still filled in", func(t *testing.T) {
		fakeBin(t, "execx-fake", `echo out; echo err >&2; exit 3`)

		got, err := New("execx-fake").Run(context.Background(), Cmd{Name: "execx-fake"})

		var exit *ExitError
		require.ErrorAs(t, err, &exit)
		assert.Equal(t, 3, exit.ExitCode)
		assert.Equal(t, "execx-fake", exit.Name)
		assert.Contains(t, exit.Error(), "execx-fake")
		assert.Contains(t, exit.Error(), "3")
		assert.NotNil(t, exit.Unwrap(), "the underlying error stays reachable")
		assert.Equal(t, 3, got.ExitCode)
		assert.Equal(t, "out\n", string(got.Stdout))
		assert.Equal(t, "err\n", string(got.Stderr))
	})

	t.Run("an allowed binary that is not on PATH is ErrNotFound, not an ExitError", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		got, err := New("execx-missing").Run(context.Background(), Cmd{Name: "execx-missing"})

		require.ErrorIs(t, err, ErrNotFound)
		var exit *ExitError
		assert.False(t, errors.As(err, &exit))
		assert.Equal(t, -1, got.ExitCode)
	})

	t.Run("a binary off the allow list is refused and never run", func(t *testing.T) {
		dir := fakeBin(t, "execx-fake", `touch ran`)
		work := t.TempDir()

		got, err := New("something-else").Run(context.Background(), Cmd{Name: "execx-fake", Dir: work})

		require.ErrorIs(t, err, ErrNotAllowed)
		assert.Equal(t, -1, got.ExitCode)
		assert.NoFileExists(t, filepath.Join(work, "ran"))
		assert.NoFileExists(t, filepath.Join(dir, "ran"))
	})

	t.Run("sudo is not special: it must be on the allow list like anything else", func(t *testing.T) {
		fakeBin(t, "sudo", `echo ran`)

		_, err := New("git").Run(context.Background(), Cmd{Name: "sudo", Args: []string{"true"}})

		assert.ErrorIs(t, err, ErrNotAllowed)
	})
}

func TestRunContext(t *testing.T) {
	t.Run("a context that is already cancelled runs nothing", func(t *testing.T) {
		fakeBin(t, "execx-fake", `touch ran`)
		work := t.TempDir()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		got, err := New("execx-fake").Run(ctx, Cmd{Name: "execx-fake", Dir: work})

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, -1, got.ExitCode)
		assert.NoFileExists(t, filepath.Join(work, "ran"))
	})

	t.Run("cancelling kills the process", func(t *testing.T) {
		fakeBin(t, "execx-fake", `exec sleep 30`)
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(100*time.Millisecond, cancel)

		start := time.Now()
		got, err := New("execx-fake").Run(ctx, Cmd{Name: "execx-fake"})

		require.ErrorIs(t, err, context.Canceled)
		var exit *ExitError
		assert.False(t, errors.As(err, &exit), "a killed process is the context's error, not an exit code")
		assert.Equal(t, -1, got.ExitCode)
		assert.Less(t, time.Since(start), 10*time.Second)
	})

	t.Run("a deadline is the caller's time limit", func(t *testing.T) {
		fakeBin(t, "execx-fake", `exec sleep 30`)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := New("execx-fake").Run(ctx, Cmd{Name: "execx-fake"})

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(start), 10*time.Second)
	})
}
