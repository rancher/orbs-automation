// Package tags makes tags on GitHub. A tag is made on the host, as a ref at a
// commit, so no clone is involved and a person and a workflow do it the same
// way. go-github's types stay inside this package.
package tags

import (
	"context"

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
//
// STUB: makes nothing until a later commit.
func (s *Service) Create(ctx context.Context, slug, name, commit string) error { return nil }
