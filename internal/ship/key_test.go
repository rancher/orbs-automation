package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/git"

	"github.com/stretchr/testify/assert"
)

func TestBranchKey(t *testing.T) {
	tests := []struct {
		name, source, tag, target, want string
	}{
		{"a release branch loses release/", "scc-operator", "v0.5.3", "release/v2.15", "scc-operator-v0.5.3-v2.15"},
		{"a default branch is kept", "scc-operator", "v0.5.3", "main", "scc-operator-v0.5.3-main"},
		{"a prerelease tag is kept whole", "scc-operator", "v0.6.0-rc.1", "release/v2.15", "scc-operator-v0.6.0-rc.1-v2.15"},
		{"only a leading release/ is stripped", "shell", "v0.9.0", "dev/release/v2", "shell-v0.9.0-dev/release/v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, BranchKey(tt.source, tt.tag, tt.target))
		})
	}
}

func TestBranchTailIsWhatEveryNameOfTheBumpEndsIn(t *testing.T) {
	assert.Equal(t, "v0.5.3-v2.15", BranchTail("v0.5.3", "release/v2.15"))
	assert.Equal(t, "v0.5.3-main", BranchTail("v0.5.3", "main"))
}

func TestBranchKeyEndsInTheTail(t *testing.T) {
	assert.Equal(t, "scc-operator-"+BranchTail("v0.5.3", "release/v2.15"), BranchKey("scc-operator", "v0.5.3", "release/v2.15"))
}

func TestSourceOfKeyIsWhatComesBeforeTheTail(t *testing.T) {
	assert.Equal(t, "scc-operator", sourceOfKey("scc-operator-v0.5.3-v2.15", "v0.5.3", "release/v2.15"))
	assert.Equal(t, "rancher-assets", sourceOfKey("rancher-assets-v0.1.0-rc.1-main", "v0.1.0-rc.1", "main"))
}

func names(bs []git.Branch) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Name
	}
	return out
}

func TestBumpBranchesAreThoseOfAnyVersionAndAnyPrefix(t *testing.T) {
	all := []git.Branch{
		{Name: "bot/scc-operator-v0.5.2-v2.15"},
		{Name: "alice/scc-operator-v0.5.3-v2.15"},
		{Name: "scc-operator-v0.5.4-rc.1-v2.15"},
		{Name: "dan-scc-operator-v0.5.3-v2.15"},
	}
	assert.Equal(t, names(all), bumpBranches(all, "scc-operator", "release/v2.15"))
}

func TestBumpBranchesLeaveOutWhatIsNotThisBump(t *testing.T) {
	branches := []git.Branch{
		{Name: "bot/scc-operator-v0.5.3-v2.14"},  // another target
		{Name: "bot/scc-operator-v0.5.3-main"},   // another target
		{Name: "bot/other-v0.5.3-v2.15"},         // another producer
		{Name: "bot/scc-operator-v0.5.3-v12.15"}, // a longer label that ends the same
		{Name: "release/v2.15"},                  // a branch of the repo
		{Name: "main"},                           // and the default
		{Name: "bot/scc-operator-v0.5.3-v2.15"},  // the one that is
	}
	assert.Equal(t, []string{"bot/scc-operator-v0.5.3-v2.15"}, bumpBranches(branches, "scc-operator", "release/v2.15"))
}

func TestBumpBranchesOfTheDefaultBranchUseItsName(t *testing.T) {
	branches := []git.Branch{{Name: "bot/scc-operator-v0.5.3-main"}, {Name: "bot/scc-operator-v0.5.3-v2.15"}}
	assert.Equal(t, []string{"bot/scc-operator-v0.5.3-main"}, bumpBranches(branches, "scc-operator", "main"))
}
