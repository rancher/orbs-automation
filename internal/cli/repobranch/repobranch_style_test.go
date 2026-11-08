package repobranch

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

func results() map[string]Result {
	base := Result{Project: "rancher/rancher", Branch: "my-fix", From: "upstream/release/v2.15", Base: "1b3c9e07deadbeef"}
	switched, dry, existing := base, base, base
	switched.Switched, dry.DryRun, existing.Existing = true, true, true
	return map[string]Result{"created": base, "switched": switched, "dry run": dry, "existing": existing}
}

func TestStyledReportIsThePlainReportWithEscapesAdded(t *testing.T) {
	all := results()
	require.NotEmpty(t, all)
	for name, r := range all {
		plain := render(r, 1, nil)

		styled := render(r, 1, colored())

		assert.Equal(t, plain, strip(styled), name)
		assert.NotEqual(t, plain, styled, name)
	}
}

func TestNoColorIsThePlainReport(t *testing.T) {
	for name, r := range results() {
		plain := render(r, 1, nil)

		for who, v := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
			assert.Equal(t, plain, render(r, 1, v), name+" "+who)
		}
		assert.NotContains(t, plain, "\x1b", name)
	}
}

func TestWhatWasDoneTakesTheLookOfIt(t *testing.T) {
	v := colored()
	all := results()

	assert.Contains(t, render(all["created"], 1, v), v.Style(style.OK, "Created"))
	assert.Contains(t, render(all["switched"], 1, v), v.Style(style.OK, "Created"))
	assert.Contains(t, render(all["dry run"], 1, v), v.Style(style.Dim, "Would create"))
	assert.Contains(t, render(all["existing"], 1, v),
		v.Style(style.Dim, "my-fix already exists in rancher/rancher with 1 commit of ours on upstream/release/v2.15: nothing to do"))
}

func TestTheHintToSwitchIsSecondaryText(t *testing.T) {
	v := colored()

	out := render(results()["created"], 1, v)

	assert.Contains(t, out, "  "+v.Style(style.Dim, "git switch my-fix"))
}

func TestTheJSONReportIsNeverStyled(t *testing.T) {
	o := opts()
	o.Style = colored()

	out, err := do(t, newWorkspace(), cleanGit(), clicfg.OutputJSON, false, o)

	require.NoError(t, err)
	assert.NotContains(t, out, "\x1b")
}
