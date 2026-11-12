package style

import (
	"regexp"
	"testing"

	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var roles = []Role{Heading, OK, Warn, Error, Dim, Dir, Prompt}

var profiles = []clicfg.Profile{clicfg.Profile16, clicfg.Profile256, clicfg.ProfileTrueColor}

// sgr is one complete SGR escape: ESC [ digits and semicolons m.
var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

func term(p clicfg.Profile) clicfg.Terminal { return clicfg.Terminal{StdoutTTY: true, Profile: p} }

func TestPlainNeverStyles(t *testing.T) {
	for _, r := range []*Renderer{Plain(), New(term(clicfg.ProfileNone))} {
		for _, role := range roles {
			assert.Equal(t, "ok 世界", r.Style(role, "ok 世界"), "role %d", role)
		}
	}
}

func TestEveryRoleIsStyledAtEveryProfile(t *testing.T) {
	for _, p := range profiles {
		r := New(term(p))
		for _, role := range roles {
			got := r.Style(role, "text")

			assert.NotEqual(t, "text", got, "profile %d role %d is styled", p, role)
			assert.Equal(t, "text", strip(got), "profile %d role %d keeps the text and adds only SGR escapes", p, role)
			assert.Regexp(t, `^\x1b\[`, got, "it starts with an escape")
		}
	}
}

func TestStyleKeepsEveryEscapeComplete(t *testing.T) {
	for _, p := range profiles {
		r := New(term(p))
		for _, role := range roles {
			got := r.Style(role, "a")

			assert.NotContains(t, sgr.ReplaceAllString(got, ""), "\x1b", "profile %d role %d has a lone escape", p, role)
		}
	}
}

func TestDimUsesTheRichGrayOnlyWhereTheTerminalHasIt(t *testing.T) {
	d16 := New(term(clicfg.Profile16)).Style(Dim, "x")
	d256 := New(term(clicfg.Profile256)).Style(Dim, "x")
	dTrue := New(term(clicfg.ProfileTrueColor)).Style(Dim, "x")

	assert.NotEqual(t, d16, d256, "256 colors have a gray that 16 do not")
	assert.NotContains(t, d16, "38;5;", "16 colors never send a 256-color code")
	assert.Contains(t, d256, "38;5;")
	assert.Contains(t, dTrue, "38;5;", "truecolor uses the same gray as 256")
}

func TestRolesLookDifferent(t *testing.T) {
	r := New(term(clicfg.Profile16))
	seen := map[string]Role{}
	for _, role := range roles {
		got := r.Style(role, "x")

		other, dup := seen[got]
		assert.False(t, dup, "role %d looks the same as role %d", role, other)
		seen[got] = role
	}
	require.Len(t, seen, len(roles))
}

func TestWidthCountsCellsAndNotEscapes(t *testing.T) {
	r := New(term(clicfg.Profile256))
	rows := []struct {
		text string
		want int
	}{
		{"", 0},
		{"gero> ", 6},
		{"世界", 4},
		{"a世b", 4},
	}
	for _, row := range rows {
		assert.Equal(t, row.want, r.Width(row.text), row.text)
		assert.Equal(t, row.want, r.Width(r.Style(OK, row.text)), "styled "+row.text)
	}
}

// A command that was given no renderer shows plain text, so none has to make one.
func TestNilRendererIsPlain(t *testing.T) {
	var r *Renderer

	for _, role := range roles {
		assert.Equal(t, "text", r.Style(role, "text"))
	}
	assert.Equal(t, 4, r.Width("世界"))
}
