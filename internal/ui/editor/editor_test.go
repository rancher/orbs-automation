package editor

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ergochat/readline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlainReadsLinesAndShowsThePromptBeforeEach(t *testing.T) {
	var out bytes.Buffer
	r := NewPlain(strings.NewReader("first\nsecond\n"), &out)

	a, errA := r.ReadLine("gero> ")
	b, errB := r.ReadLine("gero> ")
	_, errC := r.ReadLine("gero> ")

	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.Equal(t, []string{"first", "second"}, []string{a, b})
	assert.ErrorIs(t, errC, io.EOF)
	assert.Equal(t, "gero> gero> gero> ", out.String(), "the prompt is shown before each read, the last one included")
}

func TestPlainReturnsALastLineWithNoNewlineThenTheEnd(t *testing.T) {
	r := NewPlain(strings.NewReader("only"), io.Discard)

	line, err := r.ReadLine("> ")
	require.NoError(t, err)
	_, err2 := r.ReadLine("> ")

	assert.Equal(t, "only", line)
	assert.ErrorIs(t, err2, io.EOF)
}

func TestPlainOfNothingIsTheEnd(t *testing.T) {
	_, err := NewPlain(strings.NewReader(""), io.Discard).ReadLine("> ")

	assert.ErrorIs(t, err, io.EOF)
}

func TestPlainKeepsACarriageReturnOutOfTheLine(t *testing.T) {
	line, err := NewPlain(strings.NewReader("pwd\r\n"), io.Discard).ReadLine("> ")

	require.NoError(t, err)
	assert.Equal(t, "pwd", line)
}

func TestPlainCloseIsHarmless(t *testing.T) {
	assert.NoError(t, NewPlain(strings.NewReader(""), io.Discard).Close())
}

func runes(ss ...string) [][]rune {
	out := make([][]rune, len(ss))
	for i, s := range ss {
		out[i] = []rune(s)
	}
	return out
}

func TestCompletionsOfferTheSuffixesOfTheWordBeingTyped(t *testing.T) {
	words := func(string) []string { return []string{"repo", "repl", "ship", "pwd"} }
	rows := []struct {
		name string
		line string
		want [][]rune
		long int
	}{
		{"two start the same", "re", runes("po", "pl"), 2},
		{"one", "sh", runes("ip"), 2},
		{"the whole word is typed", "ship", runes(""), 4},
		{"nothing typed", "", runes("repo", "repl", "ship", "pwd"), 0},
		{"after a space the word is new", "repo ", runes("repo", "repl", "ship", "pwd"), 0},
		{"the last word only", "repo re", runes("po", "pl"), 2},
		{"none match", "zz", nil, 2},
	}
	for _, r := range rows {
		got, long := completions([]rune(r.line), len([]rune(r.line)), words)

		assert.Equal(t, r.want, got, r.name)
		assert.Equal(t, r.long, long, r.name)
	}
}

func TestCompletionsAreAskedForTheLineUpToTheCursor(t *testing.T) {
	var asked string
	words := func(line string) []string { asked = line; return []string{"status"} }

	got, long := completions([]rune("repo st status"), 7, words)

	assert.Equal(t, "repo st", asked, "what is after the cursor is not part of the question")
	assert.Equal(t, runes("atus"), got)
	assert.Equal(t, 2, long)
}

func TestCompletionsHandleWideCharacters(t *testing.T) {
	words := func(string) []string { return []string{"世界", "世"} }

	got, long := completions([]rune("世"), 1, words)

	assert.Equal(t, runes("界", ""), got)
	assert.Equal(t, 1, long)
}

func TestCompletionsWithNoCompleterOfferNothing(t *testing.T) {
	got, long := completions([]rune("re"), 2, nil)

	assert.Empty(t, got)
	assert.Zero(t, long)
}

func TestTranslateMakesTheLibrarysInterruptOurs(t *testing.T) {
	assert.ErrorIs(t, translate(readline.ErrInterrupt), ErrInterrupt)
	assert.ErrorIs(t, translate(io.EOF), io.EOF)
	assert.NoError(t, translate(nil))
}

// A terminal is not needed to read lines from the editor: piped, it reads them
// as they come and writes no escape. Editing, history keys and completion on a
// real terminal are checked by hand.
func TestTheEditorReadsLinesFromAPipeAndWritesNoEscapes(t *testing.T) {
	var out bytes.Buffer
	r, err := New(Options{In: strings.NewReader("pwd\nls\n"), Out: &out})
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	a, errA := r.ReadLine("gero> ")
	b, errB := r.ReadLine("gero> ")
	_, errC := r.ReadLine("gero> ")

	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.Equal(t, []string{"pwd", "ls"}, []string{a, b})
	assert.ErrorIs(t, errC, io.EOF)
	assert.NotContains(t, out.String(), "\x1b")
}

func TestTheEditorMakesTheDirectoryOfTheHistoryFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state", "gero", "repl_history")

	r, err := New(Options{In: strings.NewReader(""), Out: io.Discard, HistoryFile: file})
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	info, statErr := os.Stat(filepath.Dir(file))
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(), "history is the user's own")
}

func TestTheEditorWithoutAHistoryFileKeepsNone(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	r, err := New(Options{In: strings.NewReader("a\n"), Out: io.Discard})
	require.NoError(t, err)
	_, _ = r.ReadLine("> ")
	require.NoError(t, r.Close())

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing is written beside the user")
}
