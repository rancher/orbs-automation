package ui

// Align lays rows out in columns: every cell but the last of a row is padded to
// the widest cell of its column, plus two spaces, and each row ends in a
// newline. It is what text/tabwriter does with a padding of 2, except that the
// width of a cell comes from width, so a cell with escapes in it takes only the
// cells it shows. A nil width counts runes, as tabwriter does.
//
// A column is a run of consecutive rows that have a cell in it, as in
// tabwriter, so a row with fewer cells ends the column above it.
func Align(rows [][]string, width func(string) int) string { return "" }

// Option changes how a Stream shows what it is given.
type Option func(*Stream)

// WithWidth sets how a Stream measures a cell, for a table whose cells carry
// escapes. The default counts runes.
func WithWidth(width func(string) int) Option { return func(*Stream) {} }
