package backport

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/backport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bg = context.Background()

// session is a terminal with its answers typed ahead and what it did recorded.
type session struct {
	*Terminal
	out      *bytes.Buffer
	started  []string // "command dir"
	remember []string
}

func newSession(input, editor string, installed ...string) *session {
	s := &session{out: &bytes.Buffer{}}
	s.Terminal = &Terminal{
		In: strings.NewReader(input), Out: s.out, Editor: editor,
		Remember:  func(c string) error { s.remember = append(s.remember, c); return nil },
		Installed: func(c string) bool { return contains(installed, c) },
		Start:     func(_ context.Context, c, dir string) error { s.started = append(s.started, c+" "+dir); return nil },
	}
	return s
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

var conflict = backport.Conflict{
	Branch: "release/v2.15", Line: "v2.15", PR: 15, Commit: "abcdef1234", Subject: "Update image",
	Dir: "/work/tree", Files: []string{"build.yaml", "app/gen.go"},
}

func TestTerminalTellsWhereTheConflictIsOpensTheEditorAndWaitsForContinue(t *testing.T) {
	s := newSession("c\n", "goland")

	got, err := s.Resolve(bg, conflict)

	require.NoError(t, err)
	assert.Equal(t, backport.Continue, got)
	assert.Equal(t, []string{"goland /work/tree"}, s.started)
	for _, want := range []string{"v2.15", "#15", "Update image", "abcdef1", "/work/tree", "build.yaml", "app/gen.go"} {
		assert.Contains(t, s.out.String(), want)
	}
}

func TestTerminalAbortsWhenTheAnswerIsAbort(t *testing.T) {
	got, err := newSession("a\n", "goland").Resolve(bg, conflict)

	require.NoError(t, err)
	assert.Equal(t, backport.Abort, got)
}

func TestTerminalOpensTheEditorAgainWhenAsked(t *testing.T) {
	s := newSession("o\nc\n", "goland")

	got, err := s.Resolve(bg, conflict)

	require.NoError(t, err)
	assert.Equal(t, backport.Continue, got)
	assert.Len(t, s.started, 2)
}

func TestTerminalAsksAgainForAnAnswerItDoesNotKnow(t *testing.T) {
	s := newSession("what\nc\n", "goland")

	got, err := s.Resolve(bg, conflict)

	require.NoError(t, err)
	assert.Equal(t, backport.Continue, got)
	assert.Equal(t, 2, strings.Count(s.out.String(), "continue"), "the options were said again after the answer it did not know")
}

func TestTerminalAbortsWhenTheInputEnds(t *testing.T) {
	got, err := newSession("", "goland").Resolve(bg, conflict)

	require.NoError(t, err)
	assert.Equal(t, backport.Abort, got, "nobody is there to answer")
}

func TestTerminalNamesTheFilesThatStillHoldMarkers(t *testing.T) {
	c := conflict
	c.Left = []string{"build.yaml"}
	s := newSession("c\n", "goland")

	_, err := s.Resolve(bg, c)

	require.NoError(t, err)
	assert.Contains(t, s.out.String(), "still has a conflict marker")
	assert.Contains(t, s.out.String(), "build.yaml")
}

func TestTerminalAsksWhichEditorOnTheFirstConflictAndRemembersIt(t *testing.T) {
	s := newSession("2\nc\n", "", "code", "goland", "zed")

	got, err := s.Resolve(bg, conflict)

	require.NoError(t, err)
	assert.Equal(t, backport.Continue, got)
	assert.Equal(t, []string{"code /work/tree"}, s.started, "the second of the editors found, in the order they are listed")
	assert.Equal(t, []string{"code"}, s.remember)
	assert.Contains(t, s.out.String(), "GoLand")
	assert.NotContains(t, s.out.String(), "Sublime", "an editor that is not installed is not offered")

	s.Terminal.In = strings.NewReader("c\n")
	s.Terminal.in = nil
	_, err = s.Resolve(bg, conflict)

	require.NoError(t, err)
	assert.Equal(t, []string{"code"}, s.remember, "asked once")
	assert.Equal(t, []string{"code /work/tree", "code /work/tree"}, s.started)
}

func TestTerminalTakesACommandTypedInPlaceOfAChoice(t *testing.T) {
	s := newSession("nvim-qt\nc\n", "", "goland")

	_, err := s.Resolve(bg, conflict)

	require.NoError(t, err)
	assert.Equal(t, []string{"nvim-qt /work/tree"}, s.started)
	assert.Equal(t, []string{"nvim-qt"}, s.remember)
}

func TestTerminalOpensNothingWhenThePersonEditsThemselves(t *testing.T) {
	s := newSession("n\nc\n", "", "goland")

	got, err := s.Resolve(bg, conflict)

	require.NoError(t, err)
	assert.Equal(t, backport.Continue, got)
	assert.Empty(t, s.started)
	assert.Empty(t, s.remember, "doing it by hand is not a choice of editor to keep")
	assert.Contains(t, s.out.String(), "/work/tree")
}

func TestTerminalSaysWhenTheEditorWouldNotStartAndStillWaits(t *testing.T) {
	s := newSession("c\n", "goland")
	s.Terminal.Start = func(context.Context, string, string) error { return errors.New("not found") }

	got, err := s.Resolve(bg, conflict)

	require.NoError(t, err)
	assert.Equal(t, backport.Continue, got)
	assert.Contains(t, s.out.String(), "could not open goland")
}
