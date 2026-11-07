package clicfg

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
// environment and the facts of the process.
func ResolveTerminal(color string, getenv func(string) string, f Facts) Terminal {
	return Terminal{}
}
