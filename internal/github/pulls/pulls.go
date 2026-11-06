// Package pulls creates, finds, comments on, labels and closes pull requests
// on GitHub. It is a service of its own, in plain types, because gero ship is not
// its only user: the reconciler's tracking issues need the same comment, label
// and close calls on issues, which GitHub treats as the same kind of item as a
// PR. go-github's types stay inside this package.
package pulls

import (
	"context"
	"fmt"
	"strings"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/rancher/orbs-automation/internal/github"
)

// PR is a pull request as the callers need it.
type PR struct {
	Number    int
	URL       string
	Title     string
	Body      string
	Author    string   // the login that opened it
	Base      string   // the branch it targets
	HeadRef   string   // the branch it is from
	HeadOwner string   // the owner of the repository the branch is in: the fork's, or the repository's own
	Labels    []string // the names of its labels
	Draft     bool
}

// NewPR is what to open.
type NewPR struct {
	Title string
	Body  string
	Head  string // the branch, or owner:branch for one in a fork
	Base  string
	Draft bool
}

// Open is the open pull requests that were looked at for a base branch.
type Open struct {
	PRs []PR
	// Truncated is set when there were more than were looked at, so the answer
	// may be missing the oldest.
	Truncated bool
}

// DefaultMax is how many open pull requests OpenForBase looks at when it is
// not told: one full page, the newest first. A bump's PR is recent, and the
// repositories it goes to can have hundreds open.
const DefaultMax = 100

// Service talks to one GitHub host with one token.
type Service struct{ client *gogithub.Client }

// New returns a Service for the resolved credentials. Extra options (a transport
// for tests) are applied last.
func New(c github.Credentials, opts ...gogithub.ClientOptionsFunc) (*Service, error) {
	client, err := github.NewClient(c, opts...)
	if err != nil {
		return nil, err
	}
	return &Service{client: client}, nil
}

// split turns owner/name into its parts.
func split(slug string) (owner, name string, err error) {
	owner, name, ok := strings.Cut(slug, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("%q is not an owner/name repository", slug)
	}
	return owner, name, nil
}

// OpenForBase lists the open pull requests that target base, newest first, up to
// max of them (DefaultMax when it is 0). It does not read them all: more than max
// is Truncated.
//
// STUB: finds nothing until a later commit.
func (s *Service) OpenForBase(ctx context.Context, slug, base string, max int) (Open, error) {
	return Open{}, nil
}

// OpenForHead lists the open pull requests from one head, owner:branch, with
// one request. It is how to find the PR of a branch whose name is known.
//
// STUB: finds nothing until a later commit.
func (s *Service) OpenForHead(ctx context.Context, slug, head string) ([]PR, error) {
	return nil, nil
}

// Create opens a pull request.
//
// STUB: opens nothing until a later commit.
func (s *Service) Create(ctx context.Context, slug string, in NewPR) (PR, error) {
	return PR{}, nil
}

// Comment adds a comment to a pull request or an issue.
//
// STUB: comments nothing until a later commit.
func (s *Service) Comment(ctx context.Context, slug string, number int, body string) error {
	return nil
}

// Close closes a pull request or an issue, without merging.
//
// STUB: closes nothing until a later commit.
func (s *Service) Close(ctx context.Context, slug string, number int) error { return nil }

// RepoLabels lists the names of the labels of a repository.
//
// STUB: lists none until a later commit.
func (s *Service) RepoLabels(ctx context.Context, slug string) ([]string, error) {
	return nil, nil
}

// AddLabels adds labels, which must exist, to a pull request or an issue.
//
// STUB: adds none until a later commit.
func (s *Service) AddLabels(ctx context.Context, slug string, number int, labels []string) error {
	return nil
}
