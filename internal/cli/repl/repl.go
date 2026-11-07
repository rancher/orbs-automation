// Package repl is the core of gero repl: the working directory as session
// state, the line splitting, the REPL-only commands and the loop that runs a
// line. It has no terminal code and no cobra: a front end gives it lines and
// a runner for the commands of the tree (docs/specs/REPL.md).
package repl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/editor"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

// Runner runs one line of the command tree, already split into words and
// without the leading gero, in the working directory cwd.
type Runner func(ctx context.Context, cwd string, args []string) error

// ProjectDir returns the clone root of the project named by its config name or
// owner/name slug, for cd and ls. It is gero cd's own lookup and refuses what
// gero cd refuses.
type ProjectDir func(key string) (string, error)

// Session is what the REPL carries between lines: the working directory. The
// other fields are how it reaches the rest of gero, and are injected so a test
// needs neither.
type Session struct {
	Dir        string
	Run        Runner
	ProjectDir ProjectDir
	// Env is the environment the REPL started with, which is also the
	// environment of the shell it starts, and StartShell starts it.
	Env        []string
	StartShell ShellFunc
	// Style is how the prompt looks. Nil is plain.
	Style *style.Renderer
}

// ErrGeroPrefix is returned for a line that starts with gero.
var ErrGeroPrefix = errors.New("the REPL takes the command alone: type " +
	"the command without the leading gero")

// Split cuts a line into words at blanks. Single or double quotes keep blanks
// inside one word. There is no escaping, expansion or any other shell syntax.
func Split(line string) ([]string, error) {
	words := []string{}
	var word strings.Builder
	var quote rune
	inWord := false
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote", quote)
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}

// Prompt is the text shown before each line.
func Prompt() string { return "gero> " }

// Exec runs one line. It reports true when the line leaves the REPL. An error
// ends the line, not the session.
func (s *Session) Exec(ctx context.Context, out io.Writer, line string) (bool, error) {
	args, err := Split(line)
	if err != nil || len(args) == 0 {
		return false, err
	}
	switch args[0] {
	case "gero":
		return false, ErrGeroPrefix
	case "exit":
		return true, nil
	case "pwd":
		_, err = fmt.Fprintln(out, s.Dir)
		return false, err
	case "ls":
		return false, s.ls(out, args[1:])
	case "cd":
		return false, s.cd(args[1:])
	case "shell":
		return false, s.shell(ctx, args[1:])
	}
	return false, s.Run(ctx, s.Dir, args)
}

// cd moves the REPL into the clone root of the project named by its one
// argument, and nothing else: no paths, no @, no argument.
func (s *Session) cd(args []string) error {
	if len(args) != 1 {
		return errors.New("cd takes one project, by its config name or owner/name slug")
	}
	// The word goes as typed, "." and "@name" included: it is the project
	// lookup of gero cd that refuses them.
	dir, err := s.projectDir(args[0])
	if err != nil {
		return err
	}
	// The REPL owns the working directory, so it moves itself, and every command
	// run after this reads the new one from the process.
	if err := os.Chdir(dir); err != nil {
		return err
	}
	s.Dir = dir
	return nil
}

// shell runs the user's $SHELL as an interactive child, with GERO_REPL set, and
// returns when it exits. The word shadows the tree's gero shell.
func (s *Session) shell(ctx context.Context, args []string) error {
	if len(args) > 0 {
		return errors.New("shell takes no arguments: the gero shell commands are for your own shell")
	}
	sh, err := shellPath(s.Env)
	if err != nil {
		return err
	}
	return s.StartShell(ctx, sh, s.Dir, markedEnv(s.Env))
}

// ls lists the working directory, a directory under it, or a project's clone
// root. A bare word is a directory when one exists, else a project; a leading @
// is always a project.
func (s *Session) ls(out io.Writer, args []string) error {
	if len(args) > 1 {
		return errors.New("ls takes one target")
	}
	target := "."
	if len(args) == 1 {
		target = args[0]
	}
	dir, err := s.lsDir(target)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries { // ReadDir sorts by name
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		if _, err := fmt.Fprintln(out, name); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) lsDir(target string) (string, error) {
	if key, ok := strings.CutPrefix(target, "@"); ok {
		return s.projectDir(key)
	}
	dir, err := s.under(target)
	if err != nil {
		return "", err
	}
	info, statErr := os.Stat(dir)
	switch {
	case statErr == nil && info.IsDir():
		return dir, nil
	case statErr == nil:
		return "", fmt.Errorf("%s is not a directory", target)
	case !errors.Is(statErr, fs.ErrNotExist) || filepath.IsAbs(target) || strings.HasPrefix(target, "."):
		return "", statErr
	}
	return s.projectDir(target)
}

// under resolves target against the working directory and refuses a result
// outside it.
func (s *Session) under(target string) (string, error) {
	if filepath.IsAbs(target) {
		return "", fmt.Errorf("%s is outside the working directory", target)
	}
	dir := filepath.Join(s.Dir, target)
	rel, err := filepath.Rel(s.Dir, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the working directory", target)
	}
	return dir, nil
}

func (s *Session) projectDir(key string) (string, error) {
	if s.ProjectDir == nil {
		return "", fmt.Errorf("%s is not a directory here, and no projects are configured", key)
	}
	return s.ProjectDir(key)
}

// Loop reads lines from r until it ends or a line leaves the REPL, showing the
// results on out and the errors of lines on errOut. The prompt is shown by r.
func Loop(ctx context.Context, r editor.Reader, out, errOut io.Writer, s *Session) error {
	for {
		line, err := r.ReadLine(s.prompt())
		switch {
		case errors.Is(err, io.EOF):
			_, werr := io.WriteString(out, "\n")
			return werr
		case errors.Is(err, editor.ErrInterrupt):
			continue // Ctrl-C clears the line, and is never the way out
		case err != nil:
			return err
		}
		exit, err := s.Exec(ctx, out, line)
		if err != nil {
			if _, werr := fmt.Fprintln(errOut, "Error:", err); werr != nil {
				return werr
			}
		}
		if exit {
			return nil
		}
	}
}

// UseTerminal sets how the prompt looks from what the terminal can do.
func (s *Session) UseTerminal(t clicfg.Terminal) { s.Style = style.New(t) }

// prompt is what is shown before each line: the default prompt, in the look of
// a prompt when the terminal has color. The space after it is not styled, so
// nothing of the style reaches what is typed, and it stays one line of plain
// text and complete SGR escapes, which is all the editor measures
// (docs/specs/REPL.md).
func (s *Session) prompt() string {
	text := strings.TrimSuffix(Prompt(), " ")
	return s.Style.Style(style.Prompt, text) + " "
}

// NewReader is the line reader for gero repl on a terminal: line editing, history
// kept in historyFile (none when empty) and completion with complete (none when
// nil).
func NewReader(in io.Reader, out io.Writer, historyFile string, complete editor.Complete) (editor.Reader, error) {
	return editor.New(editor.Options{In: in, Out: out, HistoryFile: historyFile, Complete: complete})
}
