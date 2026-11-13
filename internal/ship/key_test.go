package ship

import (
	"testing"

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
