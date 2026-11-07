package clicfg

import "strings"

// The values of the color setting.
const (
	ColorAuto   = "auto"
	ColorAlways = "always"
	ColorNever  = "never"
)

// Profile is how many colors the terminal is given: none, the 16 of the basic
// palette, 256, or truecolor.
type Profile int

// The profiles, from least to most.
const (
	ProfileNone Profile = iota
	Profile16
	Profile256
	ProfileTrueColor
)

// Facts are what only the process knows about its terminal, handed to Load so
// that it does no I/O.
type Facts struct {
	StdinTTY, StdoutTTY bool
	Width               int // columns of stdout when it is a TTY, else 0
}

// Terminal is what the terminal can do, decided once and handed to everything
// that writes (docs/specs/CLI_OUTPUT.md).
type Terminal struct {
	StdinTTY, StdoutTTY bool
	// Profile is the color to use: ProfileNone when color is off.
	Profile Profile
	// Cursor is whether the terminal can do cursor control: TERM set and not
	// dumb.
	Cursor bool
	Width  int
}

// ResolveTerminal works out the Terminal from the color setting, the
// environment and the facts of the process. Color is on by default: auto means
// as much as the terminal supports. The order is in
// docs/specs/CLI_OUTPUT.md#terminal-capabilities.
func ResolveTerminal(color string, getenv func(string) string, f Facts) Terminal {
	term := getenv("TERM")
	t := Terminal{
		StdinTTY: f.StdinTTY, StdoutTTY: f.StdoutTTY, Width: f.Width,
		Cursor: term != "" && term != "dumb",
	}
	depth := colorDepth(getenv)
	switch color {
	case ColorNever:
	case ColorAlways:
		t.Profile = forced(depth)
	default:
		switch force := getenv("CLICOLOR_FORCE"); {
		case getenv("NO_COLOR") != "":
		case force != "" && force != "0":
			t.Profile = forced(depth)
		case f.StdoutTTY:
			t.Profile = depth
		}
	}
	return t
}

// colorDepth is the color the environment says the terminal has: truecolor from
// COLORTERM, 256 or 16 colors from TERM, and none for TERM unset or dumb.
func colorDepth(getenv func(string) string) Profile {
	if ct := getenv("COLORTERM"); ct == "truecolor" || ct == "24bit" {
		return ProfileTrueColor
	}
	switch term := getenv("TERM"); {
	case strings.Contains(term, "256color"):
		return Profile256
	case term == "" || term == "dumb":
		return ProfileNone
	}
	return Profile16
}

// forced is the depth to use when color was asked for: what the environment
// says, and at least the 16 basic colors, since the terminal's silence is not a
// refusal.
func forced(depth Profile) Profile {
	if depth == ProfileNone {
		return Profile16
	}
	return depth
}
