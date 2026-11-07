package ui

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// viaTabwriter is what Stream printed before Align: the reference.
func viaTabwriter(rows [][]string) string {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	for _, row := range rows {
		_, _ = fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	_ = w.Flush()
	return buf.String()
}

func tables() map[string][][]string {
	return map[string][][]string{
		"one column":                 {{"a"}, {"bbb"}},
		"header and rows":            {{"LINE", "BRANCH", "STATUS"}, {"v2.13", "release/v2.13", "behind"}, {"v2.14", "main", "current"}},
		"empty cells":                {{"a", "", "c"}, {"", "b", ""}, {"aaa", "bb", "c"}},
		"wide characters":            {{"名前", "x"}, {"a", "y"}, {"世界你好", "z"}},
		"a trailing empty cell":      {{"a", ""}, {"bbb", "x"}},
		"ragged rows":                {{"a", "b", "c"}, {"aaaa", "bbbb"}, {"a"}, {"aa", "bb", "cc"}, {"aaaaa", "b", "c"}},
		"a short row ends the block": {{"x", "y", "z"}, {"longer", "y"}, {"x", "y", "z"}},
		"unicode arrows":             {{"✔", "ok"}, {"⚠", "warning"}},
	}
}

func TestAlignIsWhatTabwriterDoes(t *testing.T) {
	all := tables()
	require.NotEmpty(t, all)
	for name, rows := range all {
		require.NotEmpty(t, rows, name)

		got := Align(rows, nil)

		assert.Equal(t, viaTabwriter(rows), got, name)
	}
}

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

func visible(s string) int { return utf8.RuneCountInString(strip(s)) }

// A cell with escapes is as wide as what it shows: take the escapes away and
// the table is the one the plain cells make.
func TestAlignMeasuresWhatIsShownAndNotTheEscapes(t *testing.T) {
	all := tables()
	require.NotEmpty(t, all)
	for name, rows := range all {
		styled := make([][]string, len(rows))
		for i, row := range rows {
			styled[i] = make([]string, len(row))
			for j, cell := range row {
				styled[i][j] = "\x1b[1;31m" + cell + "\x1b[m"
			}
		}

		got := Align(styled, visible)

		assert.Equal(t, viaTabwriter(rows), strip(got), name)
	}
}

func TestAlignWithoutAWidthIsTabwritersEvenWithEscapes(t *testing.T) {
	rows := [][]string{{"\x1b[31mab\x1b[m", "x"}, {"abcdefg", "y"}}

	got := Align(rows, nil)

	assert.Equal(t, viaTabwriter(rows), got, "the default is tabwriter's, which does not know escapes")
}

func TestAlignOfNothingIsNothing(t *testing.T) {
	assert.Empty(t, Align(nil, nil))
}

func TestStreamAlignsStyledCellsByWhatTheyShow(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf, WithWidth(visible))

	s.BeginTable("STATUS", "LINE")
	s.Row("\x1b[32mcurrent\x1b[m", "v2.13")
	s.Row("\x1b[31mbehind\x1b[m", "v2.14")
	s.EndTable()

	assert.Equal(t, "  STATUS   LINE\n  current  v2.13\n  behind   v2.14\n", strip(buf.String()))
	assert.Contains(t, buf.String(), "\x1b[32mcurrent")
}

func TestStreamWithoutAWidthAlignsAsBefore(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf)

	s.BeginTable("A", "B")
	s.Row("aaa", "1")
	s.EndTable()

	assert.Equal(t, "  A    B\n  aaa  1\n", buf.String())
}
