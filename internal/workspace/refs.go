package workspace

import (
	"context"

	"github.com/rancher/orbs-automation/internal/git"
)

// UpstreamRefs lists the tags and branches of the repository slug (owner/name)
// on its host: each tag with its commit and the head commit of each branch. It
// reads the host and not any local clone, so it is as fresh as the remote. It
// reads anonymously and sends a token only when the host asks for one, which a
// private repository does. It is one network call.
func (w *Workspace) UpstreamRefs(ctx context.Context, slug string) (git.Refs, error) {
	host, token := w.credentials()
	return w.remoteRefs(ctx, "https://"+host+"/"+slug+".git", token)
}
