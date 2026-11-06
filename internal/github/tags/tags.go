// Package tags makes tags on GitHub. A tag is made on the host, as a ref at a
// commit, so no clone is involved and a person and a workflow do it the same
// way. go-github's types stay inside this package.
package tags

import (
	"context"
	"fmt"
	"strings"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/rancher/orbs-automation/internal/github"
)

// Service talks to one GitHub host with one token.
type Service struct {
	client *gogithub.Client
}

// New returns a Service for the resolved credentials. Extra options (a transport
// for tests) are applied last.
func New(c github.Credentials, opts ...gogithub.ClientOptionsFunc) (*Service, error) {
	client, err := github.NewClient(c, opts...)
	if err != nil {
		return nil, err
	}
	return &Service{client: client}, nil
}

// Create makes the lightweight tag name at commit in the repository slug
// (owner/name): a ref, with no tag object and so no tagger and no message. It
// fails when the tag exists, and never moves one.
func (s *Service) Create(ctx context.Context, slug, name, commit string) error {
	owner, repo, err := split(slug)
	if err != nil {
		return err
	}
	ref := "refs/tags/" + name
	if _, _, err := s.client.Git.CreateRef(ctx, owner, repo, gogithub.CreateRef{Ref: ref, SHA: commit}); err != nil {
		return fmt.Errorf("make the tag %s at %s in %s: %w", name, commit, slug, err)
	}
	return nil
}

// split turns owner/name into its parts.
func split(slug string) (owner, name string, err error) {
	owner, name, ok := strings.Cut(slug, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("%q is not an owner/name repository", slug)
	}
	return owner, name, nil
}
