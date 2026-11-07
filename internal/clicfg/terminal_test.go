package clicfg

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColorNamesAndDefault(t *testing.T) {
	assert.Equal(t, "GERO_COLOR", EnvName("color"))
	assert.Equal(t, "color", FlagName("color"))

	s, err := Load(Inputs{})

	require.NoError(t, err)
	assert.Equal(t, ColorAuto, s.Color, "color is on by default: auto")
	assert.Equal(t, SourceDefault, s.Source("color"))
}

func TestColorPrecedence(t *testing.T) {
	file := "color: never\n"
	tests := []struct {
		name  string
		env   map[string]string
		flags map[string]string
		file  string
		want  string
		src   Source
	}{
		{"file", nil, nil, file, ColorNever, SourceFile},
		{"env beats file", map[string]string{"GERO_COLOR": "always"}, nil, file, ColorAlways, SourceEnv},
		{"flag beats env", map[string]string{"GERO_COLOR": "always"}, map[string]string{"color": "never"}, "", ColorNever, SourceFlag},
		{"flag beats file", nil, map[string]string{"color": "auto"}, file, ColorAuto, SourceFlag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := Inputs{Getenv: env(tt.env), Flags: flags(tt.flags)}
			if tt.file != "" {
				in.UserFile = strings.NewReader(tt.file)
			}

			s, err := Load(in)

			require.NoError(t, err)
			assert.Equal(t, tt.want, s.Color)
			assert.Equal(t, tt.src, s.Source("color"))
		})
	}
}

func TestColorMustBeAutoAlwaysOrNever(t *testing.T) {
	for _, bad := range []string{"yes", "true", "AUTO", "256"} {
		_, err := Load(Inputs{Getenv: env(map[string]string{"GERO_COLOR": bad})})

		require.Error(t, err, bad)
		assert.Contains(t, err.Error(), "color", bad)
	}
}

func TestResolveTerminal(t *testing.T) {
	tty := Facts{StdinTTY: true, StdoutTTY: true, Width: 100}
	pipe := Facts{StdinTTY: true, StdoutTTY: false}
	xterm := map[string]string{"TERM": "xterm"}
	rows := []struct {
		name  string
		color string
		env   map[string]string
		facts Facts
		want  Profile
	}{
		// never and always
		{"never on a truecolor terminal", ColorNever, map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor"}, tty, ProfileNone},
		{"always to a pipe, 256", ColorAlways, map[string]string{"TERM": "xterm-256color"}, pipe, Profile256},
		{"always with no TERM is 16, not none", ColorAlways, nil, pipe, Profile16},
		{"always with a dumb TERM is 16", ColorAlways, map[string]string{"TERM": "dumb"}, tty, Profile16},
		{"always beats NO_COLOR", ColorAlways, map[string]string{"TERM": "xterm", "NO_COLOR": "1"}, tty, Profile16},
		// auto, on by default, by depth
		{"auto truecolor via COLORTERM", ColorAuto, map[string]string{"TERM": "xterm", "COLORTERM": "truecolor"}, tty, ProfileTrueColor},
		{"auto truecolor via 24bit", ColorAuto, map[string]string{"TERM": "xterm", "COLORTERM": "24bit"}, tty, ProfileTrueColor},
		{"auto 256", ColorAuto, map[string]string{"TERM": "xterm-256color"}, tty, Profile256},
		{"auto 16", ColorAuto, xterm, tty, Profile16},
		{"auto 16 for screen", ColorAuto, map[string]string{"TERM": "screen"}, tty, Profile16},
		{"auto dumb is none", ColorAuto, map[string]string{"TERM": "dumb"}, tty, ProfileNone},
		{"auto no TERM is none", ColorAuto, nil, tty, ProfileNone},
		// auto, the ways it goes down
		{"auto to a pipe is none", ColorAuto, map[string]string{"TERM": "xterm-256color"}, pipe, ProfileNone},
		{"auto with NO_COLOR is none", ColorAuto, map[string]string{"TERM": "xterm-256color", "NO_COLOR": "1"}, tty, ProfileNone},
		{"auto with an empty NO_COLOR still colors", ColorAuto, map[string]string{"TERM": "xterm", "NO_COLOR": ""}, tty, Profile16},
		// auto and CLICOLOR_FORCE
		{"CLICOLOR_FORCE colors a pipe", ColorAuto, map[string]string{"TERM": "xterm-256color", "CLICOLOR_FORCE": "1"}, pipe, Profile256},
		{"CLICOLOR_FORCE=0 does not", ColorAuto, map[string]string{"TERM": "xterm", "CLICOLOR_FORCE": "0"}, pipe, ProfileNone},
		{"NO_COLOR beats CLICOLOR_FORCE", ColorAuto, map[string]string{"TERM": "xterm", "NO_COLOR": "1", "CLICOLOR_FORCE": "1"}, pipe, ProfileNone},
	}
	require.NotEmpty(t, rows)
	for _, r := range rows {
		got := ResolveTerminal(r.color, env(r.env), r.facts)

		assert.Equal(t, r.want, got.Profile, r.name)
	}
}

func TestResolveTerminalCarriesTheFactsAndCursorControl(t *testing.T) {
	tty := Facts{StdinTTY: true, StdoutTTY: true, Width: 132}

	got := ResolveTerminal(ColorAuto, env(map[string]string{"TERM": "xterm"}), tty)

	assert.True(t, got.StdinTTY)
	assert.True(t, got.StdoutTTY)
	assert.Equal(t, 132, got.Width)
	assert.True(t, got.Cursor)

	for _, term := range []map[string]string{nil, {"TERM": "dumb"}} {
		got = ResolveTerminal(ColorAuto, env(term), tty)

		assert.False(t, got.Cursor, "TERM %v cannot do cursor control", term)
	}
}

func TestLoadResolvesTheTerminalFromTheSettings(t *testing.T) {
	facts := Facts{StdinTTY: true, StdoutTTY: true, Width: 90}
	getenv := env(map[string]string{"TERM": "xterm-256color"})

	s, err := Load(Inputs{Getenv: getenv, Facts: facts})
	require.NoError(t, err)
	assert.Equal(t, Profile256, s.Terminal.Profile)
	assert.Equal(t, 90, s.Terminal.Width)

	s, err = Load(Inputs{Getenv: getenv, Facts: facts, Flags: flags(map[string]string{"color": "never"})})
	require.NoError(t, err)
	assert.Equal(t, ProfileNone, s.Terminal.Profile)
}
