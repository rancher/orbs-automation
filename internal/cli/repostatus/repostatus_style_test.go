package repostatus

import (
	"errors"
	"regexp"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

func colored() *style.Renderer {
	return style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
}

func status(t *testing.T, ws *fakeWorkspace, c Opener, slugs ...string) Status {
	t.Helper()
	st, err := gather(ctx, ws, c, slugs)
	require.NoError(t, err)
	return st
}

func statuses(t *testing.T) map[string]Status {
	noHost := fixture()
	noHost.refsErr = errors.New("network down")
	noClone := fixture()
	noClone.paths["rancher/rancher"] = workspace.RepoPath{Err: workspace.ErrRepoNotFound}
	return map[string]Status{
		"lines":     status(t, fixture(), opener(syncedClone(), nil)),
		"sources":   status(t, shipping(t), opener(syncedClone(), nil), "rancher/rancher"),
		"consumers": status(t, shipping(t), opener(syncedClone(), nil), "rancher/scc-operator"),
		"no host":   status(t, noHost, opener(syncedClone(), nil)),
		"no clone":  status(t, noClone, opener(syncedClone(), nil), "rancher/rancher"),
		"no trace": {Slug: "rancher/scc-operator", Consumers: []ship.ConsumerHoldings{
			{Consumer: "rancher/rancher", Name: "rancher", Method: "build-yaml-bump", Error: "network down"},
		}, TraceError: "method x is not usable"},
	}
}

func TestStyledReportIsThePlainReportWithEscapesAdded(t *testing.T) {
	all := statuses(t)
	require.NotEmpty(t, all)
	for name, st := range all {
		plain := renderText(st, nil)

		styled := renderText(st, colored())

		assert.Equal(t, plain, strip(styled), name)
		assert.NotEqual(t, plain, styled, name)
	}
}

func TestNoColorIsThePlainReport(t *testing.T) {
	for name, st := range statuses(t) {
		plain := renderText(st, nil)

		for who, v := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
			assert.Equal(t, plain, renderText(st, v), name+" "+who)
		}
		assert.NotContains(t, plain, "\x1b", name)
	}
}

func TestHeadingsAndColumnHeadsTakeTheirLook(t *testing.T) {
	v := colored()
	all := statuses(t)

	lines := renderText(all["lines"], v)
	sources := renderText(all["sources"], v)
	consumers := renderText(all["consumers"], v)

	for _, h := range []string{"rancher/rancher", "Releases", "Local clone"} {
		assert.Contains(t, lines, v.Style(style.Heading, h), h)
	}
	for _, head := range []string{"LINE", "BRANCH", "HEAD", "LATEST STABLE", "LATEST PRE-RELEASE", "STATE", "NEXT", "UNRELEASED", "LOCAL"} {
		assert.Contains(t, lines, v.Style(style.Dim, head), head)
	}
	assert.Contains(t, sources, v.Style(style.Heading, "Sources: scc-operator (rancher/scc-operator), build-yaml-bump"))
	assert.Contains(t, consumers, v.Style(style.Heading, "Consumers: rancher (rancher/rancher), build-yaml-bump"))
	for _, head := range []string{"HOLDS", "STATUS"} {
		assert.Contains(t, sources, v.Style(style.Dim, head), head)
	}
}

func TestWhereALineStandsTakesTheLookOfIt(t *testing.T) {
	v := colored()
	all := statuses(t)

	lines := renderText(all["lines"], v)
	sources := renderText(all["sources"], v)

	assert.Contains(t, lines, v.Style(style.OK, "in sync"))
	assert.Contains(t, lines, v.Style(style.Warn, "rc open, work since the rc (7 commits)"))
	onTheRC := fixture()
	onTheRC.branches[0].Commit = "sha-v2.13.0-rc.10"
	assert.Contains(t, renderText(status(t, onTheRC, opener(syncedClone(), nil)), v), v.Style(style.Dim, "rc open, head on the rc"))
	assert.Contains(t, sources, v.Style(style.Warn, "behind v0.5.3"))
	assert.Contains(t, sources, v.Style(style.Warn, "on stable, v0.6.0-rc.1 is newer"))
}

func TestWhatCouldNotBeReadStandsOut(t *testing.T) {
	v := colored()
	all := statuses(t)

	assert.Contains(t, renderText(all["no host"], v), "  "+v.Style(style.Error, "host: unavailable: network down"))
	assert.Contains(t, renderText(all["no clone"], v), v.Style(style.Warn, "Local clone: unavailable: "+workspace.ErrRepoNotFound.Error()))
	noTrace := renderText(all["no trace"], v)
	assert.Contains(t, noTrace, "  "+v.Style(style.Error, "unavailable: network down"))
	assert.Contains(t, noTrace, v.Style(style.Warn, "could not trace: method x is not usable"))
}
