package repl

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/editor"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scripted is a Reader that answers each ReadLine with the next of its steps and
// remembers the prompts it was shown.
type scripted struct {
	steps   []step
	prompts []string
}

type step struct {
	line string
	err  error
}

func (s *scripted) ReadLine(prompt string) (string, error) {
	s.prompts = append(s.prompts, prompt)
	if len(s.steps) == 0 {
		return "", io.EOF
	}
	next := s.steps[0]
	s.steps = s.steps[1:]
	return next.line, next.err
}

func (*scripted) Close() error { return nil }

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

func TestCtrlCClearsTheLineAndTheLoopGoesOn(t *testing.T) {
	r := &scripted{steps: []step{
		{"half a comm", editor.ErrInterrupt},
		{"pwd", nil},
		{"", editor.ErrInterrupt},
		{"pwd", nil},
	}}
	s := &Session{Dir: "/work"}
	var out, errOut bytes.Buffer

	err := Loop(context.Background(), r, &out, &errOut, s)

	require.NoError(t, err)
	assert.Equal(t, "/work\n/work\n\n", out.String(), "the lines that ran, and a newline at the end of input")
	assert.Empty(t, errOut.String(), "an interrupt is not an error")
	assert.Len(t, r.prompts, 5, "a prompt each time: four lines and the end")
}

func TestCtrlCDoesNotRunWhatWasTyped(t *testing.T) {
	var ran []string
	s := &Session{Dir: "/", Run: func(_ context.Context, _ string, args []string) error {
		ran = append(ran, strings.Join(args, " "))
		return nil
	}}
	r := &scripted{steps: []step{{"repo sta", editor.ErrInterrupt}, {"repo status", nil}}}

	require.NoError(t, Loop(context.Background(), r, io.Discard, io.Discard, s))

	assert.Equal(t, []string{"repo status"}, ran)
}

func TestCtrlDLeavesAndExitLeaves(t *testing.T) {
	for name, steps := range map[string][]step{
		"end of input": {{"pwd", nil}},
		"exit":         {{"exit", nil}, {"pwd", nil}},
	} {
		r := &scripted{steps: steps}
		var out bytes.Buffer

		err := Loop(context.Background(), r, &out, io.Discard, &Session{Dir: "/work"})

		require.NoError(t, err, name)
		assert.NotContains(t, out.String(), "unreachable", name)
	}
}

func TestAReaderErrorEndsTheLoopWithIt(t *testing.T) {
	boom := errors.New("terminal gone")
	r := &scripted{steps: []step{{"", boom}}}

	err := Loop(context.Background(), r, io.Discard, io.Discard, &Session{Dir: "/"})

	assert.ErrorIs(t, err, boom)
}

func TestThePromptIsTheDefaultOneInThePlainLook(t *testing.T) {
	for who, st := range map[string]*style.Renderer{"nil": nil, "plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
		r := &scripted{}

		require.NoError(t, Loop(context.Background(), r, io.Discard, io.Discard, &Session{Dir: "/", Style: st}))

		assert.Equal(t, []string{"gero> "}, r.prompts, who)
	}
}

func TestThePromptTakesTheLookOfAPromptWhereThereIsColor(t *testing.T) {
	v := style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
	r := &scripted{}

	require.NoError(t, Loop(context.Background(), r, io.Discard, io.Discard, &Session{Dir: "/", Style: v}))

	require.Len(t, r.prompts, 1)
	assert.Equal(t, v.Style(style.Prompt, "gero>")+" ", r.prompts[0], "the space after it is not styled, so nothing bleeds into what is typed")
	assert.Equal(t, "gero> ", strip(r.prompts[0]))
	assert.Equal(t, 6, v.Width(r.prompts[0]), "the editor will measure what shows: one line, six cells")
	assert.NotContains(t, r.prompts[0], "\n", "a prompt is one line")
}

func TestNewReaderMakesTheDirectoryOfTheHistory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state", "gero", "repl_history")

	r, err := NewReader(strings.NewReader(""), io.Discard, file, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	_, statErr := os.Stat(filepath.Dir(file))
	assert.NoError(t, statErr)
}

func TestNewReaderReadsLinesAndEndsAtTheEnd(t *testing.T) {
	r, err := NewReader(strings.NewReader("pwd\n"), io.Discard, "", nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	line, err := r.ReadLine("gero> ")
	require.NoError(t, err)
	_, err2 := r.ReadLine("gero> ")

	assert.Equal(t, "pwd", line)
	assert.ErrorIs(t, err2, io.EOF)
}
