package configshow

import (
	"regexp"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

func withColor(s clicfg.Settings) clicfg.Settings {
	s.Terminal = clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256}
	return s
}

func render(t *testing.T, s clicfg.Settings, format string) string {
	t.Helper()
	out, err := Render(s, workspace.ConfigDir{}, format)
	require.NoError(t, err)
	return out
}

func settingsSet(t *testing.T) map[string]clicfg.Settings {
	t.Helper()
	return map[string]clicfg.Settings{
		"defaults":     load(t, "", nil, nil),
		"from a file":  load(t, "output: text\nupstream: up\nfork: origin\n", nil, nil),
		"env and flag": load(t, "", map[string]string{"GERO_LOG": "info"}, map[string]string{"upstream": "rancher"}),
		"repos search": load(t, "repos:\n  search:\n    - /src\n  paths:\n    a/a: /a\n", nil, nil),
	}
}

func TestStyledTextIsThePlainTextWithEscapesAdded(t *testing.T) {
	for name, s := range settingsSet(t) {
		plain := render(t, s, clicfg.OutputText)

		styled := render(t, withColor(s), clicfg.OutputText)

		assert.Equal(t, plain, strip(styled), name)
		assert.NotEqual(t, plain, styled, name)
	}
}

func TestNoColorIsThePlainTable(t *testing.T) {
	for name, s := range settingsSet(t) {
		plain := render(t, s, clicfg.OutputText)
		s.Terminal = clicfg.Terminal{StdoutTTY: true, Profile: clicfg.ProfileNone}

		assert.Equal(t, plain, render(t, s, clicfg.OutputText), name)
		assert.NotContains(t, plain, "\x1b", name)
	}
}

func TestTheColumnHeadsUnsetValuesAndDefaultsAreSecondaryText(t *testing.T) {
	st := style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
	s := load(t, "output: json\n", nil, map[string]string{"upstream": "rancher"})

	out := render(t, withColor(s), clicfg.OutputText)

	for _, head := range []string{"KEY", "VALUE", "SOURCE"} {
		assert.Contains(t, out, st.Style(style.Dim, head), head)
	}
	assert.Contains(t, out, st.Style(style.Dim, "(none; ./config in a central checkout)"), "a hint is secondary text")
	assert.Contains(t, out, st.Style(style.Dim, "default"), "a default is secondary text")
	assert.Contains(t, out, "flag", "a source that was chosen is not")
	assert.NotContains(t, out, st.Style(style.Dim, "flag"))
	assert.NotContains(t, out, st.Style(style.Dim, "file"))
}

func TestTheResolveHintIsSecondaryText(t *testing.T) {
	st := style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
	s := load(t, "repos:\n  search:\n    - /src\n", nil, nil)

	out := render(t, withColor(s), clicfg.OutputText)

	assert.Contains(t, out, st.Style(style.Dim, "run `gero config resolve repos` to see where each repo resolves"))
}

func TestJSONAndEnvAreNeverStyled(t *testing.T) {
	for name, s := range settingsSet(t) {
		for _, format := range []string{clicfg.OutputJSON, clicfg.OutputEnv} {
			plain := render(t, s, format)

			got := render(t, withColor(s), format)

			assert.Equal(t, plain, got, name+" "+format)
			assert.NotContains(t, got, "\x1b", name+" "+format)
		}
	}
}
