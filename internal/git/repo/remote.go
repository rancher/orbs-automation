package repo

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/rancher/orbs-automation/internal/git"
)

const (
	tagRefPrefix    = "refs/tags/"
	branchRefPrefix = "refs/heads/"
	peeledSuffix    = "^{}"
)

// lister lists the references a remote advertises, as git ls-remote does.
type lister func(ctx context.Context, url string, auth transport.AuthMethod) ([]*plumbing.Reference, error)

// ListRemoteRefs lists the tags and branches of the repository at url without
// cloning it: each tag with the commit it points at (an annotated tag is
// resolved to its commit, so the result is the same for either kind) and the
// head commit of each branch. It is one request, and like git ls-remote it
// needs no token for a public repository. It asks anonymously first and sends
// token only when the host asks for credentials, so a token is never exposed to
// a repository that does not need it. An empty token means none is available.
func ListRemoteRefs(ctx context.Context, url, token string) (git.Refs, error) {
	return listRefs(ctx, goGitList, url, token)
}

func goGitList(ctx context.Context, url string, auth transport.AuthMethod) ([]*plumbing.Reference, error) {
	remote := gogit.NewRemote(memory.NewStorage(), &config.RemoteConfig{Name: "origin", URLs: []string{url}})
	// AppendPeeled is what makes an annotated tag report its commit: without it
	// the list holds only the tag object, whose hash is not a commit.
	return remote.ListContext(ctx, &gogit.ListOptions{Auth: auth, PeelingOption: gogit.AppendPeeled})
}

func listRefs(ctx context.Context, list lister, url, token string) (git.Refs, error) {
	refs, err := list(ctx, url, nil)
	if errors.Is(err, transport.ErrAuthenticationRequired) {
		if token == "" {
			return git.Refs{}, fmt.Errorf("list refs of %s: %w: it needs a token, set GH_TOKEN or sign in with gh", url, err)
		}
		refs, err = list(ctx, url, &http.BasicAuth{Username: "x-access-token", Password: token})
	}
	if err != nil {
		return git.Refs{}, fmt.Errorf("list refs of %s: %w", url, err)
	}
	return refsFromReferences(refs), nil
}

// refsFromReferences picks the tags and branches out of the advertised
// references. An annotated tag is advertised twice: the tag object under its
// name, and the commit it peels to under name^{}. The commit is the one that
// matters. A branch is its head commit.
func refsFromReferences(refs []*plumbing.Reference) git.Refs {
	commits := map[string]string{}
	peeled := map[string]string{}
	var out git.Refs
	for _, r := range refs {
		full := r.Name().String()
		if name, ok := strings.CutPrefix(full, branchRefPrefix); ok {
			out.Branches = append(out.Branches, git.Branch{Name: name, Commit: r.Hash().String()})
			continue
		}
		name, ok := strings.CutPrefix(full, tagRefPrefix)
		if !ok {
			continue
		}
		if base, isPeeled := strings.CutSuffix(name, peeledSuffix); isPeeled {
			peeled[base] = r.Hash().String()
		} else {
			commits[name] = r.Hash().String()
		}
	}
	for name, commit := range commits {
		if p, ok := peeled[name]; ok {
			commit = p
		}
		out.Tags = append(out.Tags, git.Tag{Name: name, Commit: commit})
	}
	slices.SortFunc(out.Tags, func(a, b git.Tag) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(out.Branches, func(a, b git.Branch) int { return strings.Compare(a.Name, b.Name) })
	return out
}
