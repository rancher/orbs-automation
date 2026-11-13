package releasesign

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPinsTheReleaseWorkflowOfThisRepository(t *testing.T) {
	_, err := New(nil)
	require.NoError(t, err)

	re := regexp.MustCompile(SignerPattern)
	assert.True(t, re.MatchString("https://github.com/rancher/orbs-automation/.github/workflows/release.yaml@refs/tags/v0.5.0"))
	assert.True(t, re.MatchString("https://github.com/rancher/orbs-automation/.github/workflows/release.yaml@refs/tags/v0.0.1-rc.2"))
	assert.False(t, re.MatchString("https://github.com/rancher/orbs-automation/.github/workflows/release.yaml@refs/heads/main"))
	assert.False(t, re.MatchString("https://github.com/someone/orbs-automation/.github/workflows/release.yaml@refs/tags/v0.5.0"))
}
