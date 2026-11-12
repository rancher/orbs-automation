package cigate

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/gero/clicfg"
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

func TestRequireWorkstation(t *testing.T) {
	assert.NoError(t, RequireWorkstation("gero doctor self-update", clicfg.Settings{}, false))

	err := RequireWorkstation("gero doctor self-update", clicfg.Settings{CI: true}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gero doctor self-update")
	assert.Contains(t, err.Error(), "not in CI")

	err = RequireWorkstation("gero doctor self-update", clicfg.Settings{}, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gero doctor self-update")
	assert.Contains(t, err.Error(), "not in the repl")
}
