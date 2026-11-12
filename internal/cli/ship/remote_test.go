package ship

import (
	"context"
	"testing"

	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var creds = github.Credentials{Host: "github.com", Token: "tok"}

func TestNewRemoteInCIIsTheBotNamedByTheWorkflow(t *testing.T) {
	r, err := newRemote(creds, clicfg.Settings{CI: true, BotUser: "rancher-scc-bot[bot]"})

	require.NoError(t, err)
	me, err := r.Me(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "rancher-scc-bot[bot]", me)
}

func TestNewRemoteInCIWithNoBotUserSaysWhatToSet(t *testing.T) {
	_, err := newRemote(creds, clicfg.Settings{CI: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GERO_BOT_USER")
}

func TestNewRemoteOnAWorkstationNeedsNoBotUser(t *testing.T) {
	r, err := newRemote(creds, clicfg.Settings{})

	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewRemoteWithNoTokenIsAnError(t *testing.T) {
	_, err := newRemote(github.Credentials{Host: "github.com"}, clicfg.Settings{})

	require.ErrorIs(t, err, github.ErrNoToken)
}
