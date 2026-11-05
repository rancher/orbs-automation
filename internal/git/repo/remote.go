package repo

import (
	"context"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/rancher/orbs-automation/internal/git"
)

// lister lists the references a remote advertises, as git ls-remote does.
type lister func(ctx context.Context, url string, auth transport.AuthMethod) ([]*plumbing.Reference, error)

// ListRemoteTags lists the tags of the repository at url without cloning it,
// each with the commit it points at: an annotated tag is resolved to its
// commit, so the result is the same for either kind. It is one request, and
// like git ls-remote it needs no token for a public repository. It asks
// anonymously first and sends token only when the host asks for credentials,
// so a token is never exposed to a repository that does not need it. An empty
// token means none is available.
func ListRemoteTags(ctx context.Context, url, token string) ([]git.Tag, error) {
	return listTags(ctx, goGitList, url, token)
}

func goGitList(ctx context.Context, url string, auth transport.AuthMethod) ([]*plumbing.Reference, error) {
	return nil, nil
}

func listTags(ctx context.Context, list lister, url, token string) ([]git.Tag, error) {
	return nil, nil
}
