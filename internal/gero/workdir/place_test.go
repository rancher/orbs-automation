package workdir

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChoosePlace(t *testing.T) {
	clean := CloneState{Branch: "main"}
	cases := []struct {
		name       string
		state      CloneState
		dryRun     bool
		want       Place
		wantReason string // a substring; empty means none
	}{
		{"a clean clone on a branch is worked in place", clean, false, InPlace, ""},
		{"a dry run never moves the checkout", clean, true, Worktree, "dry run"},
		{"a detached HEAD", CloneState{}, false, Worktree, "detached"},
		{"an operation in progress", CloneState{Branch: "main", InProgress: "rebase"}, false, Worktree, "rebase"},
		{"uncommitted changes", CloneState{Branch: "main", Tracked: []string{"a.go", "b.go"}}, false, Worktree, "uncommitted"},
		{"an untracked file in the way", CloneState{Branch: "main", Collisions: []string{"gen/x.go", "y.txt"}}, false, Worktree, "gen/x.go, y.txt"},
	}
	for _, c := range cases {
		got := ChoosePlace(c.state, c.dryRun)

		assert.Equal(t, c.want, got.Place, c.name)
		if c.wantReason == "" {
			assert.Empty(t, got.Reason, c.name)
		} else {
			assert.Contains(t, got.Reason, c.wantReason, c.name)
		}
	}
}

func TestChoosePlaceDryRunWinsOverTheStateOfTheClone(t *testing.T) {
	got := ChoosePlace(CloneState{Branch: "main", Tracked: []string{"a"}}, true)

	require.Equal(t, Worktree, got.Place)
	assert.Contains(t, got.Reason, "dry run", "the first reason is the one that would apply even to a clean clone")
}
