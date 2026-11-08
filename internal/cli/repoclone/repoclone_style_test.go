package repoclone

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

func colored() *style.Renderer {
	return style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
}

func results() map[string]Result {
	return map[string]Result{
		"cloned":  {Project: "rancher/rancher", Outcome: Cloned, Path: "/w/rancher", Remotes: []string{"origin", "upstream"}},
		"noted":   {Project: "rancher/rancher", Outcome: Cloned, Path: "/w/rancher", Remotes: []string{"upstream"}, Note: "danpock has no fork of rancher/rancher: cloned the upstream alone"},
		"dry run": {Project: "rancher/rancher", Outcome: Cloned, Path: "/w/rancher", Remotes: []string{"upstream"}, DryRun: true},
		"skipped": {Project: "rancher/rancher", Outcome: Skipped, Path: "/w/rancher"},
		"failed":  {Project: "rancher/rancher", Outcome: Failed, Error: "permission denied"},
	}
}

func TestStyledReportIsThePlainReportWithEscapesAdded(t *testing.T) {
	all := results()
	require.NotEmpty(t, all)
	for name, r := range all {
		plain := render(r, nil)

		styled := render(r, colored())

		assert.Equal(t, plain, strip(styled), name)
		assert.NotEqual(t, plain, styled, name)
	}
}

func TestNoColorIsThePlainReport(t *testing.T) {
	for name, r := range results() {
		plain := render(r, nil)

		for who, v := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
			assert.Equal(t, plain, render(r, v), name+" "+who)
		}
		assert.NotContains(t, plain, "\x1b", name)
	}
}

func TestWhatWasDoneTakesTheLookOfIt(t *testing.T) {
	v := colored()
	all := results()

	assert.Contains(t, render(all["cloned"], v), v.Style(style.OK, "Cloned"))
	assert.Contains(t, render(all["dry run"], v), v.Style(style.Dim, "Would clone"))
	assert.Contains(t, render(all["failed"], v), v.Style(style.Error, "Failed"))
	assert.Contains(t, render(all["skipped"], v), v.Style(style.Dim, "Skipped rancher/rancher: already cloned at /w/rancher"))
}
