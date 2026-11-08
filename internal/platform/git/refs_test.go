package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRefsBranch(t *testing.T) {
	refs := Refs{Branches: []Branch{{Name: "main", Commit: "aaa"}, {Name: "release/v2.15", Commit: "bbb"}}}

	got, ok := refs.Branch("release/v2.15")
	assert.True(t, ok)
	assert.Equal(t, Branch{Name: "release/v2.15", Commit: "bbb"}, got)

	_, ok = refs.Branch("release/v9")
	assert.False(t, ok)
	_, ok = Refs{}.Branch("main")
	assert.False(t, ok)
}
