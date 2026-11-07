// Package editor is how gero reads a line from a person at a prompt. Reader is
// the seam: a plain implementation on bufio, which tests and anything that is
// not a terminal use, and one on ergochat/readline for a terminal, with history,
// line editing and completion. It is the one importer of readline
// (docs/specs/DEPENDENCY_POLICY.md, docs/specs/REPL.md).
package editor

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ergochat/readline"
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
func NewPlain(in io.Reader, out io.Writer) Reader {
	return &plain{sc: bufio.NewScanner(in), out: out}
}

type plain struct {
	sc  *bufio.Scanner
	out io.Writer
}

func (p *plain) ReadLine(prompt string) (string, error) {
	if _, err := io.WriteString(p.out, prompt); err != nil {
		return "", err
	}
	if !p.sc.Scan() {
		if err := p.sc.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return p.sc.Text(), nil // the scanner drops the newline, and a carriage return before it
}

func (*plain) Close() error { return nil }

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
// When what it reads is not a terminal it reads lines as they come and writes no
// escape, so it can be driven without one.
func New(o Options) (Reader, error) {
	cfg := &readline.Config{
		Stdin:           io.NopCloser(o.In), // the editor does not close the terminal's input
		Stdout:          o.Out,
		Stderr:          o.Out,
		InterruptPrompt: "^C",
		EOFPrompt:       "^D",
	}
	if o.HistoryFile != "" {
		if err := os.MkdirAll(filepath.Dir(o.HistoryFile), 0o700); err != nil {
			return nil, fmt.Errorf("make the directory of the history file: %w", err)
		}
		cfg.HistoryFile = o.HistoryFile
	}
	if o.Complete != nil {
		cfg.AutoComplete = completer(o.Complete)
	}
	rl, err := readline.NewFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &terminal{rl: rl}, nil
}

type terminal struct{ rl *readline.Instance }

func (t *terminal) ReadLine(prompt string) (string, error) {
	t.rl.SetPrompt(prompt)
	line, err := t.rl.ReadLine()
	return line, translate(err)
}

func (t *terminal) Close() error { return t.rl.Close() }

// completer is the editor's way of asking for completions.
type completer Complete

func (c completer) Do(line []rune, pos int) ([][]rune, int) {
	return completions(line, pos, Complete(c))
}

// completions adapts a Complete to what the editor asks: the suffixes that finish
// the word before pos, and the length of the word they finish. A word is what
// follows the last blank before pos.
func completions(line []rune, pos int, complete Complete) (suffixes [][]rune, length int) {
	if complete == nil {
		return nil, 0
	}
	pos = min(max(pos, 0), len(line))
	before := string(line[:pos])
	word := before[strings.LastIndexAny(before, " \t")+1:]
	for _, candidate := range complete(before) {
		if rest, ok := strings.CutPrefix(candidate, word); ok {
			suffixes = append(suffixes, []rune(rest))
		}
	}
	return suffixes, utf8.RuneCountInString(word)
}

// translate turns the editor library's errors into ours.
func translate(err error) error {
	if errors.Is(err, readline.ErrInterrupt) {
		return ErrInterrupt
	}
	return err
}
