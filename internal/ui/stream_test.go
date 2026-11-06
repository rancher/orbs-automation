package ui

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinesAreWrittenAtOnce(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf)

	s.Line("first")
	assert.Equal(t, "first\n", buf.String(), "written before anything else is said")
	s.Line("second")

	assert.Equal(t, "first\nsecond\n", buf.String())
}

func TestATableIsAlignedAndHoldsTheLinesThatComeWhileItIsOpen(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf)
	s.Line("before")

	s.BeginTable("BRANCH", "STATUS")
	s.Row("release/v2.15", "ready")
	s.Line("held while the table is open")
	s.Row("main", "already up to date")
	assert.Equal(t, "before\n", buf.String(), "nothing of the table is written until it is complete")
	s.EndTable()
	s.Line("after")

	want := strings.Join([]string{
		"before",
		"  BRANCH         STATUS",
		"  release/v2.15  ready",
		"  main           already up to date",
		"held while the table is open",
		"after",
		"",
	}, "\n")
	assert.Equal(t, want, buf.String())
}

func TestHeldLinesComeOutInTheOrderTheyWereGiven(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf)
	s.BeginTable("A")
	s.Row("x")
	s.Line("one")
	s.Line("two")
	s.EndTable()

	assert.Equal(t, "  A\n  x\none\ntwo\n", buf.String())
}

func TestATableWithNoRowsPrintsOnlyTheHeldLines(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf)
	s.BeginTable("BRANCH", "STATUS")
	s.Line("held")

	s.EndTable()

	assert.Equal(t, "held\n", buf.String())
}

func TestStreamingResumesAfterATableAndAnotherCanFollow(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf)
	s.BeginTable("A", "B")
	s.Row("1", "2")
	s.EndTable()
	s.Line("between")
	s.BeginTable("C")
	s.Row("3")
	s.EndTable()

	assert.Equal(t, "  A  B\n  1  2\nbetween\n  C\n  3\n", buf.String())
}

func TestRowWithNoTableOpenIsALine(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf)

	s.Row("a", "b")

	assert.Equal(t, "a  b\n", buf.String())
}

func TestEndTableWithNoTableOpenDoesNothing(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf)

	s.EndTable()

	assert.Empty(t, buf.String())
}

func TestFlushEndsATableThatIsStillOpen(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf)
	s.BeginTable("A")
	s.Row("x")
	s.Line("held")

	s.Flush()

	assert.Equal(t, "  A\n  x\nheld\n", buf.String(), "nothing is lost when a command stops early")
}

func TestBeginningATableEndsTheOneThatIsOpen(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf)
	s.BeginTable("A")
	s.Row("x")

	s.BeginTable("B")
	s.Row("y")
	s.EndTable()

	assert.Equal(t, "  A\n  x\n  B\n  y\n", buf.String())
}

type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }

func TestTheFirstWriteErrorIsKept(t *testing.T) {
	boom := errors.New("closed pipe")
	s := NewStream(failingWriter{boom})
	require.NoError(t, s.Err())

	s.Line("x")
	s.Line("y")

	assert.ErrorIs(t, s.Err(), boom)
}
