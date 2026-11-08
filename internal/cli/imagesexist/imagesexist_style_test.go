package imagesexist

import (
	"regexp"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/ociregistry"

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

func report(t *testing.T, tag string, results map[string]ociregistry.Result) Report {
	t.Helper()
	r, err := check(ctx, &fakeWorkspace{project: sccProject(t)}, &fakeChecker{results: results}, "", tag)
	require.NoError(t, err)
	return r
}

func reports(t *testing.T) map[string]Report {
	return map[string]Report{
		"all found": report(t, "v0.5.3", nil),
		"not found": report(t, "v0.5.3-rc.1", map[string]ociregistry.Result{
			"stgregistry.suse.com rancher/scc-operator:v0.5.3-rc.1": {Status: ociregistry.NotFound}}),
		"unreachable": report(t, "v1", map[string]ociregistry.Result{
			"registry-1.docker.io rancher/scc-operator:v1": {Status: ociregistry.Unreachable, Detail: "429 Too Many Requests"}}),
		"no images": {Project: "rancher/x", Tag: "v1", Stable: true, Warnings: []string{"nothing to check"}},
	}
}

func TestStyledReportIsThePlainReportWithEscapesAdded(t *testing.T) {
	for name, r := range reports(t) {
		plain := renderText(r, nil)

		styled := renderText(r, colored())

		assert.Equal(t, plain, strip(styled), name)
		assert.NotEqual(t, plain, styled, name)
	}
}

func TestNoColorIsThePlainReport(t *testing.T) {
	for name, r := range reports(t) {
		plain := renderText(r, nil)

		for who, v := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
			assert.Equal(t, plain, renderText(r, v), name+" "+who)
		}
		assert.NotContains(t, plain, "\x1b", name)
	}
}

func TestStatusesTakeTheLookOfWhatTheySay(t *testing.T) {
	v := colored()
	all := reports(t)

	notFound := renderText(all["not found"], v)
	unreachable := renderText(all["unreachable"], v)

	assert.Contains(t, notFound, v.Style(style.OK, "found"))
	assert.Contains(t, notFound, v.Style(style.Error, "not found"))
	assert.Contains(t, notFound, v.Style(style.Dim, "skipped"), "a prerelease skips the registry that has only stable")
	assert.Contains(t, unreachable, v.Style(style.Error, "unreachable")+" (429 Too Many Requests)",
		"the status is styled and the reason after it is not")
	for _, head := range []string{"IMAGE", "REGISTRY", "STATUS", "REFERENCE"} {
		assert.Contains(t, notFound, v.Style(style.Dim, head), head)
	}
}

func TestTheTitleAndTheClosingLineTakeTheirLook(t *testing.T) {
	v := colored()
	all := reports(t)

	assert.Contains(t, renderText(all["all found"], v), v.Style(style.Heading, "rancher/scc-operator v0.5.3 (stable)"))
	assert.Contains(t, renderText(all["all found"], v), v.Style(style.OK, "All required images were found."))
	assert.Contains(t, renderText(all["not found"], v), v.Style(style.Error, "1 not found: the release is not fully published."))
	assert.Contains(t, renderText(all["unreachable"], v), v.Style(style.Error, "1 unreachable: the release is not fully published."))
}

func TestWarningsAndTheEmptyReportAreStyled(t *testing.T) {
	v := colored()

	out := renderText(reports(t)["no images"], v)

	assert.Contains(t, out, v.Style(style.Warn, "Warning: nothing to check"))
	assert.Contains(t, out, v.Style(style.Dim, "No image is expected for this tag."))
}
