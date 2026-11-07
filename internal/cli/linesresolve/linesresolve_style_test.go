package linesresolve

import (
	"regexp"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

func colored() *style.Renderer {
	return style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
}

func results(t *testing.T) map[string]Result {
	return map[string]Result{
		"all lines": resolve(t, Options{}),
		"one line":  resolve(t, Options{Lines: []string{"v2.15"}}),
	}
}

func TestStyledMapIsThePlainMapWithEscapesAdded(t *testing.T) {
	all := results(t)
	require.NotEmpty(t, all)
	for name, r := range all {
		plain := renderText(r, nil)

		styled := renderText(r, colored())

		assert.Equal(t, plain, strip(styled), name)
		assert.NotEqual(t, plain, styled, name)
	}
}

func TestNoColorIsThePlainMap(t *testing.T) {
	for name, r := range results(t) {
		plain := renderText(r, nil)

		for who, v := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
			assert.Equal(t, plain, renderText(r, v), name+" "+who)
		}
		assert.NotContains(t, plain, "\x1b", name)
	}
}

func TestTheTitleTheLineIdsAndTheLabelsTakeTheirLook(t *testing.T) {
	v := colored()

	out := renderText(results(t)["all lines"], v)

	assert.Contains(t, out, v.Style(style.Heading, "Release lines, through rancher/rancher"))
	for _, id := range []string{"v2.16", "v2.15", "v2.14", "v2.13"} {
		assert.Contains(t, out, v.Style(style.Heading, id), id)
	}
	for _, label := range []string{"takes from", "also on this line"} {
		assert.Contains(t, out, v.Style(style.Dim, label), label)
	}
}

func TestThePhaseTakesTheLookOfWhereTheLineIs(t *testing.T) {
	v := colored()

	out := renderText(results(t)["all lines"], v)

	assert.Contains(t, out, v.Style(style.Dim, "development"))
	assert.Contains(t, out, v.Style(style.OK, "ga"))
	assert.Contains(t, out, v.Style(style.Warn, "eom"))
	assert.Contains(t, out, v.Style(style.Error, "eol"))
}

func TestGapsStandOut(t *testing.T) {
	v := colored()

	out := renderText(results(t)["all lines"], v)

	assert.Contains(t, out, "  "+v.Style(style.Warn, "! shell has no line v2.13, so nothing of it ships to rancher v2.13"))
	assert.Contains(t, out, v.Style(style.Warn, "Gaps: 1"))
}

func TestNoGapsIsGood(t *testing.T) {
	v := colored()
	r := results(t)["all lines"]
	r.Gaps = nil
	for i := range r.Lines {
		r.Lines[i].Gaps = nil
	}

	out := renderText(r, v)

	assert.Contains(t, out, v.Style(style.OK, "No gaps."))
}

func TestARunWritesTheStyledTextAndNotTheJSON(t *testing.T) {
	text := func(format string, view *style.Renderer) string {
		var b strings.Builder
		require.NoError(t, run(&b, workspaceOf(t), format, Options{}, now, view))
		return b.String()
	}

	assert.Equal(t, text("text", nil), strip(text("text", colored())))
	assert.NotEqual(t, text("text", nil), text("text", colored()))
	assert.Equal(t, text("json", nil), text("json", colored()))
	assert.NotContains(t, text("json", colored()), "\x1b")
}
