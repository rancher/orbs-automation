package configresolve

import (
	"regexp"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

func colored() *style.Renderer {
	return style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
}

func run(t *testing.T, format string, view *style.Renderer) string {
	t.Helper()
	out, err := repos(mixed(), format, nil, view)
	require.ErrorIs(t, err, ErrUnresolved)
	return out
}

func TestStyledTableIsThePlainTableWithEscapesAdded(t *testing.T) {
	plain := run(t, clicfg.OutputText, nil)

	styled := run(t, clicfg.OutputText, colored())

	assert.Equal(t, plain, strip(styled))
	assert.NotEqual(t, plain, styled)
}

func TestNoColorIsThePlainTable(t *testing.T) {
	plain := run(t, clicfg.OutputText, nil)

	for who, v := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
		assert.Equal(t, plain, run(t, clicfg.OutputText, v), who)
	}
	assert.NotContains(t, plain, "\x1b")
}

func TestEachOutcomeTakesItsLook(t *testing.T) {
	v := colored()

	out := run(t, clicfg.OutputText, v)

	for _, head := range []string{"SLUG", "PATH", "SOURCE"} {
		assert.Contains(t, out, v.Style(style.Dim, head), head)
	}
	assert.Contains(t, out, v.Style(style.Error, "(not found)"))
	assert.Contains(t, out, v.Style(style.Warn, "(ambiguous: /src/op-a, /src/op-b)"))
	assert.Contains(t, out, "/src/rancher", "a found path is not styled")
	assert.NotContains(t, out, v.Style(style.Dim, "/src/rancher"))
}

func TestTheHintsAreSecondaryText(t *testing.T) {
	v := colored()

	out := run(t, clicfg.OutputText, v)

	assert.Contains(t, out, v.Style(style.Dim, "not found: "+HintNotFound))
	assert.Contains(t, out, v.Style(style.Dim, "ambiguous: "+HintAmbiguous))
}

func TestJSONIsNeverStyled(t *testing.T) {
	plain := run(t, clicfg.OutputJSON, nil)

	got := run(t, clicfg.OutputJSON, colored())

	assert.Equal(t, plain, got)
	assert.NotContains(t, got, "\x1b")
}
