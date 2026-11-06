package ui

// Output is how a command shows what it is doing, as it does it. Commands take
// an Output and not an io.Writer, so the same command works in an ad-hoc run,
// where Stream prints plain lines and aligned tables, and in a REPL or a TUI,
// which can draw the same calls their own way. The calls are the few that a
// progressive report needs: a line, and a table that is held until it is whole.
//
// It is defined here, and not where it is used, because every command and every
// front end shares it.
type Output interface {
	// Line shows a line now, or after the open table when one is being built.
	Line(text string)
	// BeginTable starts a table with a header row, and Row adds a row to it.
	// Output is held until EndTable so the table can be aligned.
	BeginTable(header ...string)
	Row(cells ...string)
	// EndTable shows the table, then whatever was held while it was open.
	EndTable()
	// Flush ends a table that is still open, for a command that stops early.
	Flush()
}

var _ Output = (*Stream)(nil)
