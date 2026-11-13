package authstatus

import (
	"regexp"
	"testing"

	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

func colored() *style.Renderer {
	return style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
}

func statuses() map[string]Status {
	return map[string]Status{
		"valid":        {Host: "github.com", HostSource: "default", TokenSource: "GH_TOKEN", Valid: true, Login: "octocat"},
		"rejected":     {Host: "github.com", HostSource: "default", TokenSource: "GH_TOKEN"},
		"installation": {Host: "github.com", HostSource: "default", TokenSource: "GH_TOKEN", Valid: true, Installation: true},
	}
}

func render(t *testing.T, st Status, format string, view *style.Renderer) string {
	t.Helper()
	out, err := Render(st, format, view)
	require.NoError(t, err)
	return out
}

func TestStyledReportIsThePlainReportWithEscapesAdded(t *testing.T) {
	for name, st := range statuses() {
		plain := render(t, st, clicfg.OutputText, nil)

		styled := render(t, st, clicfg.OutputText, colored())

		assert.Equal(t, plain, strip(styled), name)
		assert.NotEqual(t, plain, styled, name)
	}
}

func TestNoColorIsThePlainReport(t *testing.T) {
	for name, st := range statuses() {
		plain := render(t, st, clicfg.OutputText, nil)

		for who, v := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
			assert.Equal(t, plain, render(t, st, clicfg.OutputText, v), name+" "+who)
		}
		assert.NotContains(t, plain, "\x1b", name)
	}
}

func TestTheLabelsAreSecondaryAndTheLoginTakesTheLookOfWhatItSays(t *testing.T) {
	v := colored()
	all := statuses()

	valid := render(t, all["valid"], clicfg.OutputText, v)
	rejected := render(t, all["rejected"], clicfg.OutputText, v)
	installation := render(t, all["installation"], clicfg.OutputText, v)

	for _, label := range []string{"Host:", "Token:", "Login:"} {
		assert.Contains(t, valid, v.Style(style.Dim, label), label)
	}
	assert.Contains(t, valid, v.Style(style.OK, "octocat"))
	assert.Contains(t, rejected, v.Style(style.Error, "token rejected by GitHub"))
	assert.Contains(t, installation, v.Style(style.OK, "installation token"))
}

func TestJSONIsNeverStyled(t *testing.T) {
	for name, st := range statuses() {
		plain := render(t, st, clicfg.OutputJSON, nil)

		got := render(t, st, clicfg.OutputJSON, colored())

		assert.Equal(t, plain, got, name)
		assert.NotContains(t, got, "\x1b", name)
	}
}
