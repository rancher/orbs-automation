package termfacts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// No pty is needed to know what is not a terminal. What is one is checked by
// hand on a real terminal.
func notTerminals(t *testing.T) map[string]*os.File {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })
	file, err := os.Create(filepath.Join(t.TempDir(), "out"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })
	null, err := os.Open(os.DevNull)
	require.NoError(t, err)
	t.Cleanup(func() { _ = null.Close() })
	return map[string]*os.File{"a pipe": r, "a file": file, "/dev/null": null}
}

func TestIsTerminalIsFalseForWhatIsNotOne(t *testing.T) {
	files := notTerminals(t)
	require.NotEmpty(t, files)
	for name, f := range files {
		assert.False(t, IsTerminal(f), name)
	}
}

func TestDetectWithoutTerminalsHasNoTTYAndNoWidth(t *testing.T) {
	files := notTerminals(t)

	got := Detect(files["a pipe"], files["/dev/null"])

	assert.False(t, got.StdinTTY)
	assert.False(t, got.StdoutTTY)
	assert.Zero(t, got.Width)
}

func TestDetectTellsStdinFromStdout(t *testing.T) {
	files := notTerminals(t)

	got := Detect(files["a file"], files["a pipe"])

	assert.False(t, got.StdinTTY)
	assert.False(t, got.StdoutTTY)
}
