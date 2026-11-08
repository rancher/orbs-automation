package cigate

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireCrossRepoToken(t *testing.T) {
	err := RequireCrossRepoToken("gero bump", clicfg.Settings{DefaultToken: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gero bump")
	assert.Contains(t, err.Error(), "GH_TOKEN")

	assert.NoError(t, RequireCrossRepoToken("gero bump", clicfg.Settings{}))
}
