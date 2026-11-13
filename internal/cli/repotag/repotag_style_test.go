package repotag

import (
	"bytes"
	"regexp"
	"strings"
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

func actions() map[string]Result {
	base := Result{Project: "rancher/shell", Branch: "release/v2.15", Commit: "abcdef0123456789", Tag: "v0.8.4-rc.1", Remote: "upstream"}
	out := map[string]Result{}
	for _, a := range []string{"tagged", "tagged and pushed", "pushed", "in your clone", "would tag", "would tag and push", "would push", "already tagged"} {
		r := base
		r.Action = a
		out[a] = r
	}
	return out
}

func reported(t *testing.T, r Result, format string, view *style.Renderer) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, report(&out, format, r, view))
	return out.String()
}

func TestStyledReportIsThePlainReportWithEscapesAdded(t *testing.T) {
	all := actions()
	require.NotEmpty(t, all)
	for name, r := range all {
		plain := reported(t, r, "text", nil)

		styled := reported(t, r, "text", colored())

		assert.Equal(t, plain, strip(styled), name)
		assert.NotEqual(t, plain, styled, name)
	}
}

func TestNoColorIsThePlainReport(t *testing.T) {
	for name, r := range actions() {
		plain := reported(t, r, "text", nil)

		for who, v := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
			assert.Equal(t, plain, reported(t, r, "text", v), name+" "+who)
		}
		assert.NotContains(t, plain, "\x1b", name)
	}
}

func TestWhatWasDoneTakesTheLookOfIt(t *testing.T) {
	v := colored()
	all := actions()

	assert.Contains(t, reported(t, all["tagged"], "text", v), v.Style(style.OK, "Tagged"))
	assert.Contains(t, reported(t, all["tagged and pushed"], "text", v), v.Style(style.OK, "Tagged and pushed"))
	assert.Contains(t, reported(t, all["pushed"], "text", v), v.Style(style.OK, "Pushed"))
	for _, a := range []string{"would tag", "would tag and push", "would push", "already tagged", "in your clone"} {
		first, _, _ := strings.Cut(strip(reported(t, all[a], "text", nil)), "\n")
		assert.Contains(t, reported(t, all[a], "text", v), v.Style(style.Dim, first), a+": the line is secondary")
	}
}

func TestTheHintToPushIsSecondaryText(t *testing.T) {
	v := colored()

	out := reported(t, actions()["tagged"], "text", v)

	assert.Contains(t, out, "  "+v.Style(style.Dim, "to send it: git push upstream v0.8.4-rc.1"))
}

func TestTheJSONReportIsNeverStyled(t *testing.T) {
	for name, r := range actions() {
		plain := reported(t, r, "json", nil)

		got := reported(t, r, "json", colored())

		assert.Equal(t, plain, got, name)
		assert.NotContains(t, got, "\x1b", name)
	}
}

func TestTheQuestionIsStyledAndItsPlainTextIsTheSame(t *testing.T) {
	v := colored()
	r := actions()["would tag and push"]
	ask := func(view *style.Renderer) string {
		var out bytes.Buffer
		opts := Options{Interactive: true, In: strings.NewReader("y\n"), Style: view}
		require.NoError(t, approve(&out, "text", r, opts))
		return out.String()
	}

	styled := ask(v)

	text := "Push the tag " + r.what() + " to upstream?"
	assert.Contains(t, styled, v.Style(style.Heading, text))
	assert.Contains(t, styled, v.Style(style.Dim, "[y/N]"))
	assert.Equal(t, ask(nil), strip(styled))
	assert.NotContains(t, ask(nil), "\x1b")
}
