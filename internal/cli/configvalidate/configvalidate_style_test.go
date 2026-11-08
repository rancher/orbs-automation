package configvalidate

import (
	"regexp"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
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

func valid(t *testing.T) fakeWorkspace {
	t.Helper()
	return fakeWorkspace{
		dir:   workspace.CentralConfigDir{Dir: "/work/config"},
		graph: chaintest.Projects(t, "a", "acme/a", "b", "acme/b"),
	}
}

func TestStyledSummaryIsThePlainSummaryWithEscapesAdded(t *testing.T) {
	plain, err := run(valid(t), nil)
	require.NoError(t, err)

	styled, err := run(valid(t), colored())

	require.NoError(t, err)
	assert.Equal(t, plain, strip(styled))
	assert.NotEqual(t, plain, styled)
	assert.Contains(t, styled, colored().Style(style.OK, "config OK"))
}

func TestNoColorIsThePlainSummary(t *testing.T) {
	plain, err := run(valid(t), nil)
	require.NoError(t, err)

	for who, v := range map[string]*style.Renderer{"plain": style.Plain(), "none": style.New(clicfg.Terminal{})} {
		got, err := run(valid(t), v)

		require.NoError(t, err, who)
		assert.Equal(t, plain, got, who)
	}
	assert.NotContains(t, plain, "\x1b")
}
