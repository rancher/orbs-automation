package repolist

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

func colored() *style.Renderer {
	return style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
}

func mixed(t *testing.T) *fakeWorkspace {
	t.Helper()
	return &fakeWorkspace{graph: newGraph(t), rows: map[string]workspace.RepoPath{
		"rancher/rancher": found("rancher/rancher", "/src/rancher"),
		"rancher/op":      {Slug: "rancher/op", Candidates: []string{"/src/a", "/src/b"}, Err: workspace.ErrRepoAmbiguous},
	}}
}

func render(t *testing.T, format string, view *style.Renderer) string {
	t.Helper()
	out, err := list(mixed(t), everyoneExists, format, view)
	require.NoError(t, err)
	return out
}

func TestStyledListIsThePlainListWithEscapesAdded(t *testing.T) {
	plain := render(t, clicfg.OutputText, nil)

	styled := render(t, clicfg.OutputText, colored())

	assert.Equal(t, plain, strip(styled))
	assert.NotEqual(t, plain, styled)
}

func TestNoColorIsThePlainList(t *testing.T) {
	plain := render(t, clicfg.OutputText, nil)

	for who, v := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
		assert.Equal(t, plain, render(t, clicfg.OutputText, v), who)
	}
	assert.NotContains(t, plain, "\x1b")
}

func TestTheHeadingsAndTheSummaryTakeTheirLook(t *testing.T) {
	v := colored()

	out := render(t, clicfg.OutputText, v)

	assert.Contains(t, out, v.Style(style.Heading, "Cloned (1)"))
	assert.Contains(t, out, v.Style(style.Heading, "Not cloned (2)"))
	assert.Contains(t, out, v.Style(style.Warn, "Ambiguous (1)"))
	assert.Contains(t, out, v.Style(style.Dim, "1 of 4 projects cloned"))
}

func TestJSONIsNeverStyled(t *testing.T) {
	plain := render(t, clicfg.OutputJSON, nil)

	got := render(t, clicfg.OutputJSON, colored())

	assert.Equal(t, plain, got)
	assert.NotContains(t, got, "\x1b")
}
