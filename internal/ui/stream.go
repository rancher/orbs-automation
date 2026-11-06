// Package ui is how gero writes to the person running it. Stream prints as a
// command works, and holds output only while a table is being built, so the
// table can be aligned.
package ui

import (
	"io"
)

// Stream writes lines as they are given, except while a table is open: a table
// cannot be aligned until every row is known, so output is held from BeginTable
// until EndTable, which prints the table and then what was held, and streaming
// goes on. It is not safe for use from several goroutines.
type Stream struct {
	w      io.Writer
	indent string
	err    error
}

// NewStream returns a Stream that writes to w. Tables are indented by two
// spaces, as the reports of gero are.
func NewStream(w io.Writer) *Stream { return &Stream{w: w, indent: "  "} }

// Line prints a line now, or holds it while a table is open. A newline is added.
//
// STUB: prints nothing until a later commit.
func (s *Stream) Line(text string) {}

// BeginTable starts a table with a header row. Output is held until EndTable.
//
// STUB: does nothing until a later commit.
func (s *Stream) BeginTable(header ...string) {}

// Row adds a row to the open table. With no table open it prints the cells as a
// line, separated by two spaces.
//
// STUB: does nothing until a later commit.
func (s *Stream) Row(cells ...string) {}

// EndTable prints the open table, aligned, and then the lines held while it was
// open. A table with no rows prints nothing but the held lines. With no table
// open it does nothing.
//
// STUB: does nothing until a later commit.
func (s *Stream) EndTable() {}

// Flush ends a table that is still open, so nothing is lost when a command
// stops early.
//
// STUB: does nothing until a later commit.
func (s *Stream) Flush() {}

// Err is the first error from the writer, or nil.
func (s *Stream) Err() error { return s.err }
