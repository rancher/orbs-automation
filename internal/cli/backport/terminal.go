package backport

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/rancher/orbs-automation/internal/backport"
)

// Editor is a program that can open a directory, as gero knows it.
type Editor struct {
	Name    string // what it is called to a person
	Command string // what is run
}

// Editors are the ones offered when none is chosen, in the order they are listed. A command
// of the person's own can be typed instead.
var Editors = []Editor{
	{"GoLand", "goland"},
	{"IntelliJ IDEA", "idea"},
	{"VS Code", "code"},
	{"Cursor", "cursor"},
	{"Zed", "zed"},
	{"Sublime Text", "subl"},
}

// Terminal is a backport.Resolver that asks a person on a terminal. At a conflict it says where
// the files are, opens the editor on the work directory, and waits for the person to say that the
// conflicts are resolved or that the backport is to be abandoned. It is the only way a conflict is
// resolved: gero waits for it and runs nothing else until it has the answer.
type Terminal struct {
	In  io.Reader
	Out io.Writer
	// Editor is the command to open the work directory with. Empty asks, on the first conflict.
	Editor string
	// Remember keeps the editor that was chosen, for the next run. Nil forgets it.
	Remember func(command string) error
	// Installed says whether a command is found on the PATH.
	Installed func(command string) bool
	// Start opens a command on a directory and returns without waiting for it.
	Start func(ctx context.Context, command, dir string) error

	in *bufio.Reader
	// byHand says the person chose to edit the files themselves, so no editor is opened.
	byHand bool
}

var _ backport.Resolver = (*Terminal)(nil)

// Resolve tells the person about the conflict and waits for the answer.
func (t *Terminal) Resolve(ctx context.Context, c backport.Conflict) (backport.Choice, error) {
	if t.in == nil {
		t.in = bufio.NewReader(t.In)
	}
	t.say(conflictText(c))
	if t.Editor == "" && !t.byHand {
		if err := t.chooseEditor(); err != nil {
			return 0, err
		}
	}
	t.open(ctx, c.Dir)
	for {
		t.say("c = continue once the conflicts are resolved, a = abort this backport, o = open the editor again: ")
		answer, err := t.ask()
		if err != nil {
			return 0, err
		}
		switch strings.ToLower(answer) {
		case "c", "continue":
			return backport.Continue, nil
		case "a", "abort", "":
			if answer == "" && !t.atEnd() {
				continue
			}
			return backport.Abort, nil
		case "o", "open":
			t.open(ctx, c.Dir)
		}
	}
}

// conflictText says what stopped and where the files are.
func conflictText(c backport.Conflict) string {
	var b strings.Builder
	label := c.Line
	if label == "" {
		label = c.Branch
	}
	short := c.Commit[:min(7, len(c.Commit))]
	of := ""
	if c.PR != 0 {
		of = fmt.Sprintf(" of #%d", c.PR)
	}
	fmt.Fprintf(&b, "\nThe pick of %s %q%s conflicts on %s (%s).\nResolve these files in %s:\n", short, c.Subject, of, label, c.Branch, c.Dir)
	for _, f := range c.Files {
		fmt.Fprintf(&b, "  %s\n", f)
	}
	for _, f := range c.Left {
		fmt.Fprintf(&b, "  %s still has a conflict marker\n", f)
	}
	return b.String()
}

func (t *Terminal) say(text string) { _, _ = io.WriteString(t.Out, text) }

// atEnd reports whether the input has ended.
func (t *Terminal) atEnd() bool {
	_, err := t.in.Peek(1)
	return err != nil
}

// ask reads one answer, trimmed. The end of the input is an empty answer, and atEnd tells it
// from a blank line.
func (t *Terminal) ask() (string, error) {
	line, err := t.in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// open starts the editor on the work directory, and says so when it will not start, since the
// person can still open the directory themselves.
func (t *Terminal) open(ctx context.Context, dir string) {
	if t.Editor == "" || t.byHand {
		return
	}
	if err := t.Start(ctx, t.Editor, dir); err != nil {
		t.say(fmt.Sprintf("could not open %s: %v\n", t.Editor, err))
	}
}

// chooseEditor asks which editor to open, from the ones installed or a command typed in, and
// keeps a choice for the next run. Choosing none leaves the editing to the person.
func (t *Terminal) chooseEditor() error {
	var found []Editor
	for _, e := range Editors {
		if t.Installed(e.Command) {
			found = append(found, e)
		}
	}
	t.say("\nWhich editor should open the files?\n")
	for i, e := range found {
		t.say(fmt.Sprintf("  %d  %s (%s)\n", i+1, e.Name, e.Command))
	}
	t.say("  n  none, I will edit them myself\nChoose a number, n, or type a command: ")
	answer, err := t.ask()
	if err != nil {
		return err
	}
	if n, err := strconv.Atoi(answer); err == nil && n >= 1 && n <= len(found) {
		answer = found[n-1].Command
	}
	switch strings.ToLower(answer) {
	case "", "n", "none":
		t.byHand = true
		return nil
	}
	t.Editor = answer
	if t.Remember != nil {
		if err := t.Remember(answer); err != nil {
			t.say(fmt.Sprintf("could not keep %s as your editor: %v\n", answer, err))
		}
	}
	return nil
}
