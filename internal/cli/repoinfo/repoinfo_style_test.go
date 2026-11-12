package repoinfo

import (
	"errors"
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

func info(t *testing.T, c Clone, err error) Info {
	t.Helper()
	got, gerr := gather(fixture(t), "orbs", opener(c, err), nil)
	require.NoError(t, gerr)
	return got
}

func infos(t *testing.T) map[string]Info {
	dirty := cloneOnMain
	dirty.dirty = true
	return map[string]Info{
		"clean":      info(t, cloneOnMain, nil),
		"dirty":      info(t, dirty, nil),
		"unreadable": info(t, nil, errors.New("boom")),
		"no clone":   {Slug: "rancher/charts", Local: LocalInfo{Error: "no local clone found"}},
	}
}

func TestStyledReportIsThePlainReportWithEscapesAdded(t *testing.T) {
	all := infos(t)
	require.NotEmpty(t, all)
	for name, i := range all {
		plain := renderText(i, nil)

		styled := renderText(i, colored())

		assert.Equal(t, plain, strip(styled), name)
		assert.NotEqual(t, plain, styled, name)
	}
}

func TestNoColorIsThePlainReport(t *testing.T) {
	for name, i := range infos(t) {
		plain := renderText(i, nil)

		for who, v := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
			assert.Equal(t, plain, renderText(i, v), name+" "+who)
		}
		assert.NotContains(t, plain, "\x1b", name)
	}
}

func TestHeadingsAndLabelsAndColumnHeadsTakeTheirLook(t *testing.T) {
	v := colored()

	out := renderText(infos(t)["clean"], v)

	for _, h := range []string{"rancher/rancher", "Branches", "Stream rancher", "Local clone"} {
		assert.Contains(t, out, v.Style(style.Heading, h), h)
	}
	for _, label := range []string{"owned", "managed by", "default branch", "branch layout", "tag pattern", "path", "branch", "commit", "work tree"} {
		assert.Contains(t, out, v.Style(style.Dim, label), label)
	}
	for _, head := range []string{"LINE", "SERIES", "BRANCH"} {
		assert.Contains(t, out, v.Style(style.Dim, head), head)
	}
}

func TestTheStateOfTheWorkTreeTakesTheLookOfWhatItSays(t *testing.T) {
	v := colored()
	all := infos(t)

	assert.Contains(t, renderText(all["clean"], v), v.Style(style.OK, "clean"))
	assert.Contains(t, renderText(all["dirty"], v), v.Style(style.Warn, "changes or an operation in progress"))
	assert.Contains(t, renderText(all["unreadable"], v), v.Style(style.Error, "unavailable: boom"))
	assert.Contains(t, renderText(all["no clone"], v), v.Style(style.Error, "unavailable: no local clone found"))
}
