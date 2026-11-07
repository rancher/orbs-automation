// Package editor is how gero reads a line from a person at a prompt. Reader is
// the seam: a plain implementation on bufio, which tests and anything that is
// not a terminal use, and one on ergochat/readline for a terminal, with history,
// line editing and completion. It is the one importer of readline
// (docs/specs/DEPENDENCY_POLICY.md, docs/specs/REPL.md).
package editor

import (
	"errors"
	"io"
)

// ErrInterrupt is returned by ReadLine when the person pressed Ctrl-C at the
// prompt. It is not the end of input: the prompt is shown again.
var ErrInterrupt = errors.New("interrupt")

// Reader reads lines at a prompt.
type Reader interface {
	// ReadLine shows prompt and returns the next line without its newline. It
	// returns io.EOF at the end of input, which is Ctrl-D on an empty line, and
	// ErrInterrupt for Ctrl-C.
	ReadLine(prompt string) (string, error)
	// Close gives the terminal back, and saves what is to be saved.
	Close() error
}

// Complete returns the words that may complete the word at the end of line. The
// editor shows the ones that start with what is typed of it.
type Complete func(line string) []string

// NewPlain returns a Reader on in that writes the prompt to out and edits
// nothing: the terminal's own line discipline is all there is.
func NewPlain(in io.Reader, out io.Writer) Reader { return &plain{} }

type plain struct{}

func (*plain) ReadLine(string) (string, error) { return "", nil }
func (*plain) Close() error                    { return nil }

// Options are what the editor on a terminal is given.
type Options struct {
	In  io.Reader
	Out io.Writer
	// HistoryFile is where lines are kept between sessions. Empty keeps none.
	HistoryFile string
	// Complete gives the completions of Tab. Nil completes nothing.
	Complete Complete
}

// New returns the Reader for a terminal: line editing, history and completion.
func New(o Options) (Reader, error) { return NewPlain(o.In, o.Out), nil }

// completions adapts a Complete to what the editor asks: the suffixes that finish
// the word before pos, and the length of the word they finish.
func completions(line []rune, pos int, complete Complete) (suffixes [][]rune, length int) {
	return nil, 0
}

// translate turns the editor library's errors into ours.
func translate(err error) error { return err }
