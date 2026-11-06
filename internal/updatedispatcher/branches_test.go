package updatedispatcher

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// consumer has a development line on its default branch and two release branches.
func consumer() config.Project {
	return config.Project{
		Name: "rancher",
		Repo: config.Repo{Name: "rancher/rancher", DefaultBranch: "main"},
		Release: config.Release{Branching: config.BranchingPerLine, Lines: []config.ProjectLine{
			{Line: "v2.16"},
			{Line: "v2.15", Branch: "release/v2.15"},
			{Line: "v2.14", Branch: "release/v2.14"},
		}},
	}
}

func route(edgeLines string) Route {
	return Route{Edge: config.Edge{To: "rancher", Lines: edgeLines}, Consumer: consumer()}
}

// producerWithLines follows lines; producerWithout is in none.
var (
	producerWithLines = config.Project{Name: "bro", Release: config.Release{Lines: []config.ProjectLine{{Line: "v2.15"}}}}
	producerWithout   = config.Project{Name: "scc-operator"}
)

func TestBranchesForAUniversalEdgeIsEveryReleaseBranchWhateverTheLine(t *testing.T) {
	for _, line := range []string{"", "v2.15", "v9.9"} {
		got, err := route("").BranchesFor(producerWithout, line)

		require.NoError(t, err, line)
		assert.Equal(t, []string{"main", "release/v2.15", "release/v2.14"}, got, line)
	}
}

func TestBranchesForAnEdgeThatFollowsLinesIsTheConsumersBranchForTheLine(t *testing.T) {
	got, err := route("").BranchesFor(producerWithLines, "v2.15")

	require.NoError(t, err)
	assert.Equal(t, []string{"release/v2.15"}, got)

	got, err = route("").BranchesFor(producerWithLines, "v2.16")
	require.NoError(t, err)
	assert.Equal(t, []string{"main"}, got, "the development line is on the default branch")
}

func TestBranchesForTheEdgesOwnLinesBeatTheDefault(t *testing.T) {
	got, err := route(config.AcceptLinesUniversal).BranchesFor(producerWithLines, "v2.15")

	require.NoError(t, err)
	assert.Len(t, got, 3)
	got, err = route(config.AcceptLinesAll).BranchesFor(producerWithout, "v2.14")
	require.NoError(t, err)
	assert.Equal(t, []string{"release/v2.14"}, got)
}

func TestBranchesForAReleaseInNoLineHasNoBranchToFollowTo(t *testing.T) {
	got, err := route("").BranchesFor(producerWithLines, "")

	require.ErrorIs(t, err, ErrNoBranches)
	assert.Empty(t, got)
}

func TestBranchesForALineTheConsumerLacksIsAnError(t *testing.T) {
	_, err := route("").BranchesFor(producerWithLines, "v2.12")

	require.ErrorIs(t, err, ErrNoBranches)
	assert.Contains(t, err.Error(), "v2.12")
	assert.Contains(t, err.Error(), "rancher/rancher")
}

func TestBranchesForAUniversalEdgeToAConsumerWithNoReleaseBranchesIsAnError(t *testing.T) {
	r := route("")
	r.Consumer.Release = config.Release{Branching: config.BranchingPerLine}

	_, err := r.BranchesFor(producerWithout, "")

	require.ErrorIs(t, err, ErrNoBranches)
	assert.Contains(t, err.Error(), "rancher/rancher")
}
