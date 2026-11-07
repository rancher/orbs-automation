// Package style is how gero colors its output: named roles, and a Renderer
// built from the terminal that says what each looks like there. It is the one
// importer of lipgloss (docs/specs/DEPENDENCY_POLICY.md).
//
// Nothing outside it writes an escape, and nothing in it looks at the
// environment: the Terminal it is built from is the one place that was decided
// (docs/specs/CLI_OUTPUT.md#terminal-capabilities).
package style

import "github.com/rancher/orbs-automation/internal/clicfg"

// Role is what a piece of text is, not how it looks.
type Role int

// The roles. A command says which it means, and the Renderer decides.
const (
	Heading Role = iota // a title or a column header
	OK                  // a thing that passed or succeeded
	Warn                // something to act on
	Error               // a failure
	Dim                 // secondary text: a hint, a source, a path
	Dir                 // a directory name
	Prompt              // the REPL prompt
)

// Renderer styles text for one terminal.
type Renderer struct {
	profile clicfg.Profile
}

// New returns the Renderer for t. With no color profile it is the plain one.
func New(t clicfg.Terminal) *Renderer { return &Renderer{profile: t.Profile} }

// Plain returns the Renderer that never styles: what machine formats, pipes and
// tests get.
func Plain() *Renderer { return &Renderer{} }

// Style returns text in the look of role. With no color it is text unchanged.
func (r *Renderer) Style(role Role, text string) string { return text }

// Width is the number of terminal cells text takes, escapes not counted.
func (r *Renderer) Width(text string) int { return len(text) }
