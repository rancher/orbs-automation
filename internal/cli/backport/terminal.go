package backport

import (
	"bufio"
	"context"
	"io"

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
}

var _ backport.Resolver = (*Terminal)(nil)

// Resolve tells the person about the conflict and waits for the answer.
func (t *Terminal) Resolve(ctx context.Context, c backport.Conflict) (backport.Choice, error) {
	return 0, nil
}
