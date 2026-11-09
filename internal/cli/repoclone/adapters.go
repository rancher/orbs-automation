package repoclone

import (
	"context"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git/gitcli"
	"github.com/rancher/orbs-automation/internal/platform/github"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// chainSource is the ProjectSource that reads the central config through the
// workspace: a local directory when one is named, else the central bundle.
type chainSource struct{ ws *workspace.Workspace }

func (c chainSource) Project(key string) (*chain.Project, error) {
	return c.ws.Project(key)
}

func (c chainSource) Projects() ([]*chain.Project, error) {
	g, err := c.ws.Chain()
	if err != nil {
		return nil, err
	}
	return g.Projects(), nil
}

// gitCLI is Git on the engineer's own git binary.
type gitCLI struct{}

func (gitCLI) Clone(ctx context.Context, url, dir, remote string) error {
	return gitcli.Clone(ctx, url, dir, remote)
}

func (gitCLI) AddRemote(ctx context.Context, dir, name, url string) error {
	return gitcli.AddRemote(ctx, dir, name, url)
}

func (gitCLI) RemoteURL(ctx context.Context, dir, name string) (string, error) {
	return gitcli.RemoteURL(ctx, dir, name)
}

// hostForks is Forks on the GitHub API, with the token gero finds, and with none
// for a public repository.
type hostForks struct{}

func (hostForks) ForkExists(ctx context.Context, user, slug string) (bool, error) {
	auth := github.GoGh{}
	host, _ := auth.DefaultHost()
	creds, err := github.Resolve(auth, host)
	if err != nil {
		creds = github.Credentials{Host: host}
	}
	return github.ForkExists(ctx, creds, user, slug)
}
