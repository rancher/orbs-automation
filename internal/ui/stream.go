// Package ui is how gero writes to the person running it. Stream prints as a
// command works, and holds output only while a table is being built, so the
// table can be aligned.
package ui

import (
	"io"
	"strings"
)

// Stream writes lines as they are given, except while a table is open: a table
// cannot be aligned until every row is known, so output is held from BeginTable
// until EndTable, which prints the table and then what was held, and streaming
// goes on. It is not safe for use from several goroutines.
type Stream struct {
	w      io.Writer
	indent string
	width  func(string) int // how wide a cell is; nil counts runes
	err    error

	open bool       // a table is open
	rows [][]string // its header and rows
	held []string   // lines given while it was open
}

// NewStream returns a Stream that writes to w. Tables are indented by two
// spaces, as the reports of gero are.
func NewStream(w io.Writer, opts ...Option) *Stream {
	s := &Stream{w: w, indent: "  "}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Line prints a line now, or holds it while a table is open. A newline is added.
func (s *Stream) Line(text string) {
	if s.open {
		s.held = append(s.held, text)
		return
	}
	s.write(text + "\n")
}

// BeginTable starts a table with a header row. Output is held until EndTable.
// A table that is already open is ended first.
func (s *Stream) BeginTable(header ...string) {
	s.EndTable()
	s.open, s.rows = true, [][]string{header}
}

// Row adds a row to the open table. With no table open it prints the cells as a
// line, separated by two spaces.
func (s *Stream) Row(cells ...string) {
	if !s.open {
		s.Line(strings.Join(cells, "  "))
		return
	}
	s.rows = append(s.rows, cells)
}

// EndTable prints the open table, aligned, and then the lines held while it was
// open. A table with no rows prints nothing but the held lines. With no table
// open it does nothing.
func (s *Stream) EndTable() {
	if !s.open {
		return
	}
	if len(s.rows) > 1 {
		rows := make([][]string, len(s.rows))
		for i, row := range s.rows {
			if len(row) == 0 {
				rows[i] = []string{s.indent}
				continue
			}
			rows[i] = append([]string{s.indent + row[0]}, row[1:]...)
		}
		s.write(Align(rows, s.width))
	}
	held := s.held
	s.open, s.rows, s.held = false, nil, nil
	for _, line := range held {
		s.write(line + "\n")
	}
}

// Flush ends a table that is still open, so nothing is lost when a command
// stops early.
func (s *Stream) Flush() { s.EndTable() }

// Err is the first error from the writer, or nil.
func (s *Stream) Err() error { return s.err }

func (s *Stream) write(text string) {
	if _, err := io.WriteString(s.w, text); err != nil && s.err == nil {
		s.err = err
	}
}
