package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	tests := []struct {
		tag  string
		want Version
		ok   bool
	}{
		{"v2.15.3", Version{2, 15, 3, ""}, true},
		{"2.15.3", Version{2, 15, 3, ""}, true},
		{"v2.15.0-rc.1", Version{2, 15, 0, "rc.1"}, true},
		{"v2.15.0-alpha10", Version{2, 15, 0, "alpha10"}, true},
		{"v1.0.0-rc.1+build.5", Version{1, 0, 0, "rc.1"}, true},
		{"v1.0.0+build.5", Version{1, 0, 0, ""}, true},
		{"v1.0.0-x-y.z", Version{1, 0, 0, "x-y.z"}, true},
		{"v01.2.3", Version{}, false},
		{"v1.2", Version{}, false},
		{"v1.2.3.4", Version{}, false},
		{"v1.2.3-", Version{}, false},
		{"v1.2.3-rc..1", Version{}, false},
		{"v1.2.3-rc_1", Version{}, false},
		{"V1.2.3", Version{}, false},
		{"vv1.2.3", Version{}, false},
		{"main-abc1234", Version{}, false},
		{"v1.2.3-head-0123456", Version{1, 2, 3, "head-0123456"}, true},
		{"", Version{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got, ok := Parse(tt.tag)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVersionIsPrerelease(t *testing.T) {
	assert.True(t, Version{1, 0, 0, "rc.1"}.IsPrerelease())
	assert.False(t, Version{1, 0, 0, ""}.IsPrerelease())
}

func TestCompare(t *testing.T) {
	// Each row is in strictly ascending order.
	ordered := []string{
		"v1.0.0-alpha",
		"v1.0.0-alpha.1",
		"v1.0.0-alpha.beta",
		"v1.0.0-beta",
		"v1.0.0-beta.2",
		"v1.0.0-beta.11",
		"v1.0.0-rc.1",
		"v1.0.0",
		"v1.0.1",
		"v1.2.0",
		"v2.0.0",
	}
	for i := 1; i < len(ordered); i++ {
		a, _ := Parse(ordered[i-1])
		b, _ := Parse(ordered[i])
		assert.Equal(t, -1, Compare(a, b), "%s < %s", ordered[i-1], ordered[i])
		assert.Equal(t, 1, Compare(b, a), "%s > %s", ordered[i], ordered[i-1])
	}
}

func TestCompareNumbersAreNotTextWithinOneIdentifier(t *testing.T) {
	lower, _ := Parse("v2.8.0-rc2")
	higher, _ := Parse("v2.8.0-rc10")
	assert.Equal(t, -1, Compare(lower, higher), "rc2 < rc10")

	alpha, _ := Parse("v2.8.0-alpha10")
	rc, _ := Parse("v2.8.0-rc1")
	assert.Equal(t, -1, Compare(alpha, rc), "the text prefix decides first")
}

func TestCompareEqualIgnoresBuildMetadata(t *testing.T) {
	a, _ := Parse("v1.0.0+one")
	b, _ := Parse("v1.0.0+two")
	assert.Equal(t, 0, Compare(a, b))
}

func TestCompareNumbersAreNumeric(t *testing.T) {
	a, _ := Parse("v2.9.0")
	b, _ := Parse("v2.15.0")
	assert.Equal(t, -1, Compare(a, b), "9 < 15, not text order")
}
