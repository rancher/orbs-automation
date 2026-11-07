package ui

import (
	"strings"
	"unicode/utf8"
)

// Align lays rows out in columns: every cell but the last of a row is padded to
// the widest cell of its column, plus two spaces, and each row ends in a
// newline. It is what text/tabwriter does with a padding of 2, except that the
// width of a cell comes from width, so a cell with escapes in it takes only the
// cells it shows. A nil width counts runes, as tabwriter does.
//
// A column is a run of consecutive rows that have a cell in it, as in
// tabwriter, so a row with fewer cells ends the column above it.
func Align(rows [][]string, width func(string) int) string {
	if width == nil {
		width = utf8.RuneCountInString
	}
	// padded[i][c] is the width cell c of row i is padded to. A cell is padded
	// when it is not the last of its row.
	padded := make([][]int, len(rows))
	widest := 0
	for i, row := range rows {
		padded[i] = make([]int, max(len(row)-1, 0))
		widest = max(widest, len(row)-1)
	}
	for c := 0; c < widest; c++ {
		for i := 0; i < len(rows); {
			if c >= len(rows[i])-1 {
				i++
				continue
			}
			end, w := i, 0
			for end < len(rows) && c < len(rows[end])-1 {
				w = max(w, width(rows[end][c]))
				end++
			}
			for k := i; k < end; k++ {
				padded[k][c] = w + gap
			}
			i = end
		}
	}
	var b strings.Builder
	for i, row := range rows {
		for c, cell := range row {
			b.WriteString(cell)
			if c < len(row)-1 {
				b.WriteString(strings.Repeat(" ", padded[i][c]-width(cell)))
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// gap is the space between two columns, as tabwriter's padding.
const gap = 2

// Option changes how a Stream shows what it is given.
type Option func(*Stream)

// WithWidth sets how a Stream measures a cell, for a table whose cells carry
// escapes. The default counts runes.
func WithWidth(width func(string) int) Option { return func(s *Stream) { s.width = width } }
