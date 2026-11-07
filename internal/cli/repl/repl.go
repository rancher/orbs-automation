// Package repl is the core of gero repl: the working directory as session
// state, the line splitting, the REPL-only commands and the loop that runs a
// line. It has no terminal code and no cobra: a front end gives it lines and
// a runner for the commands of the tree (docs/specs/REPL.md).
package repl

import (
	"context"
	"io"
)

// Runner runs one line of the command tree, already split into words and
// without the leading gero, in the working directory cwd.
type Runner func(ctx context.Context, cwd string, args []string) error

// ProjectDir returns the clone root of the project named by its config name or
// owner/name slug, for ls.
type ProjectDir func(key string) (string, error)

// Session is what the REPL carries between lines: the working directory. The
// other fields are how it reaches the rest of gero, and are injected so a test
// needs neither.
type Session struct {
	Dir        string
	Run        Runner
	ProjectDir ProjectDir
}

// Split cuts a line into words at blanks. Single or double quotes keep blanks
// inside one word. There is no escaping, expansion or any other shell syntax.
func Split(line string) ([]string, error) { return nil, nil }

// Prompt is the text shown before each line.
func Prompt() string { return "" }

// Exec runs one line. It reports true when the line leaves the REPL. An error
// ends the line, not the session.
func (s *Session) Exec(ctx context.Context, out io.Writer, line string) (bool, error) {
	return false, nil
}

// Loop reads lines from in until it ends or a line leaves the REPL, showing the
// prompt and results on out and the errors of lines on errOut.
func Loop(ctx context.Context, in io.Reader, out, errOut io.Writer, s *Session) error {
	return nil
}
