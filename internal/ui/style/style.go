// Package style is how gero colors its output: named roles, and a Renderer
// built from the terminal that says what each looks like there. It is the one
// importer of lipgloss (docs/specs/DEPENDENCY_POLICY.md).
//
// Nothing outside it writes an escape, and nothing in it looks at the
// environment: the Terminal it is built from is the one place that was decided
// (docs/specs/CLI_OUTPUT.md#terminal-capabilities).
package style

import (
	"charm.land/lipgloss/v2"

	"github.com/rancher/orbs-automation/internal/clicfg"
)

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
	styles  map[Role]lipgloss.Style
}

// New returns the Renderer for t. With no color profile it is the plain one.
func New(t clicfg.Terminal) *Renderer {
	r := &Renderer{profile: t.Profile}
	if r.profile != clicfg.ProfileNone {
		r.styles = map[Role]lipgloss.Style{}
		for _, role := range []Role{Heading, OK, Warn, Error, Dim, Dir, Prompt} {
			r.styles[role] = styleFor(role, r.profile)
		}
	}
	return r
}

// Plain returns the Renderer that never styles: what machine formats, pipes and
// tests get.
func Plain() *Renderer { return &Renderer{} }

// Style returns text in the look of role. With no color it is text unchanged.
//
// The palette is the basic 16 colors with bold, which read on a light or a dark
// background, since the terminal's background is not known. lipgloss v2 always
// renders full escapes, so what is chosen here for the profile is all there is
// to downsampling: a color the terminal does not have is never asked for.
func (r *Renderer) Style(role Role, text string) string {
	st, ok := r.styles[role]
	if !ok {
		return text
	}
	return st.Render(text)
}

// Width is the number of terminal cells text takes, escapes not counted.
func (r *Renderer) Width(text string) int { return lipgloss.Width(text) }

func styleFor(role Role, p clicfg.Profile) lipgloss.Style {
	s := lipgloss.NewStyle()
	switch role {
	case Heading:
		return s.Bold(true)
	case OK:
		return s.Foreground(lipgloss.Color("2"))
	case Warn:
		return s.Foreground(lipgloss.Color("3"))
	case Error:
		return s.Bold(true).Foreground(lipgloss.Color("1"))
	case Dim:
		// Bright black on 16 colors; the mid gray where the terminal has it.
		if p == clicfg.Profile16 {
			return s.Foreground(lipgloss.Color("8"))
		}
		return s.Foreground(lipgloss.Color("245"))
	case Dir:
		return s.Bold(true).Foreground(lipgloss.Color("4"))
	case Prompt:
		return s.Bold(true).Foreground(lipgloss.Color("6"))
	}
	return s
}
