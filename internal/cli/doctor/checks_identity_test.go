package doctor

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/gero/actor"
	"github.com/stretchr/testify/assert"
)

type fakeResolver struct {
	id    actor.Identity
	err   error
	calls int
}

func (f *fakeResolver) Resolve(context.Context) (actor.Identity, error) {
	f.calls++
	return f.id, f.err
}

func TestCommitIdentityCheckMetadata(t *testing.T) {
	ws := NewCommitIdentityCheck(&fakeResolver{}, false)
	assert.Equal(t, "commit-identity", ws.Name())
	assert.Equal(t, "authentication", ws.Category())
	assert.NotEmpty(t, ws.Description())
	assert.False(t, ws.Online(), "a workstation reads git config")
	assert.Empty(t, ws.Needs())

	ci := NewCommitIdentityCheck(&fakeResolver{}, true)
	assert.True(t, ci.Online(), "the bot's id is looked up on GitHub")
	assert.Equal(t, []string{"auth-api"}, ci.Needs())
}

func TestCommitIdentityCheckReportsTheIdentity(t *testing.T) {
	r := &fakeResolver{id: actor.Identity{Name: "Dan Pock", Email: "dan@example.com"}}

	got := NewCommitIdentityCheck(r, false).Run(context.Background())

	assert.Equal(t, Result{Status: StatusOK, Message: "commits as Dan Pock <dan@example.com>"}, got)
}

func TestCommitIdentityCheckWorkstationWithoutGitIdentityWarns(t *testing.T) {
	r := &fakeResolver{err: errors.New(`git has no user.name or user.email: set it with "git config --global user.name <value>"`)}

	got := NewCommitIdentityCheck(r, false).Run(context.Background())

	assert.Equal(t, StatusWarn, got.Status, "read-only commands do not need an identity")
	assert.Contains(t, got.Message, "user.name")
	assert.Contains(t, got.Remediation, "git config")
}

func TestCommitIdentityCheckCIWithoutBotUserWarns(t *testing.T) {
	r := &fakeResolver{err: errors.New("GERO_BOT_USER is required")}

	got := NewCommitIdentityCheck(r, true).Run(context.Background())

	assert.Equal(t, StatusWarn, got.Status)
	assert.Contains(t, got.Message, "GERO_BOT_USER")
	assert.Contains(t, got.Remediation, "GERO_BOT_USER")
}
