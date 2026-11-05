package workspace

import (
	"context"

	"github.com/rancher/orbs-automation/internal/git"
)

// UpstreamTags lists the tags of the repository slug (owner/name) on its host,
// each with its commit, read from the host and not from any local clone, so it
// is as fresh as the remote. It reads anonymously and sends a token only when
// the host asks for one, which a private repository does. It is a network call.
func (w *Workspace) UpstreamTags(ctx context.Context, slug string) ([]git.Tag, error) {
	host, token := w.credentials()
	return w.remoteTags(ctx, "https://"+host+"/"+slug+".git", token)
}
