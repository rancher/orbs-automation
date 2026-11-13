package ship

import (
	"context"
	"testing"

	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/git/gitcli"
	"github.com/rancher/orbs-automation/internal/platform/github"
	"github.com/stretchr/testify/assert"
)

func TestPushContextCarriesTheTokenInCIOnly(t *testing.T) {
	creds := github.Credentials{Host: "github.com", Token: "t0ken"}

	assert.True(t, gitcli.HasToken(pushContext(context.Background(), clicfg.Settings{CI: true}, creds)))
	assert.False(t, gitcli.HasToken(pushContext(context.Background(), clicfg.Settings{}, creds)), "the engineer's own credentials push")
}

func TestPushContextWithoutATokenCarriesNone(t *testing.T) {
	assert.False(t, gitcli.HasToken(pushContext(context.Background(), clicfg.Settings{CI: true}, github.Credentials{Host: "github.com"})))
}
