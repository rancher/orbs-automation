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
func (s *Service) OpenForBase(ctx context.Context, slug, base string, max int) (Open, error) {
	owner, name, err := split(slug)
	if err != nil {
		return Open{}, err
	}
	if max <= 0 {
		max = DefaultMax
	}
	opts := &gogithub.PullRequestListOptions{
		State: "open", Base: base, Sort: "created", Direction: "desc",
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}
	var out Open
	for {
		page, resp, err := s.client.PullRequests.List(ctx, owner, name, opts)
		if err != nil {
			return Open{}, fmt.Errorf("list the open pull requests of %s for %s: %w", slug, base, err)
		}
		for _, p := range page {
			out.PRs = append(out.PRs, convert(p))
		}
		if len(out.PRs) >= max {
			out.Truncated = len(out.PRs) > max || resp.NextPage != 0
			out.PRs = out.PRs[:max]
			return out, nil
		}
		if resp.NextPage == 0 {
			return out, nil
		}
		opts.Page = resp.NextPage
	}
}

// OpenForHead lists the open pull requests from one head, owner:branch, with
// one request. It is how to find the PR of a branch whose name is known.
func (s *Service) OpenForHead(ctx context.Context, slug, head string) ([]PR, error) {
	owner, name, err := split(slug)
	if err != nil {
		return nil, err
	}
	page, _, err := s.client.PullRequests.List(ctx, owner, name, &gogithub.PullRequestListOptions{
		State: "open", Head: head, ListOptions: gogithub.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, fmt.Errorf("list the open pull requests of %s from %s: %w", slug, head, err)
	}
	out := make([]PR, len(page))
	for i, p := range page {
		out[i] = convert(p)
	}
	return out, nil
}

// Create opens a pull request.
func (s *Service) Create(ctx context.Context, slug string, in NewPR) (PR, error) {
	owner, name, err := split(slug)
	if err != nil {
		return PR{}, err
	}
	created, _, err := s.client.PullRequests.Create(ctx, owner, name, gogithub.CreatePullRequest{
		Title: gogithub.Ptr(in.Title), Body: gogithub.Ptr(in.Body), Head: in.Head,
		Base: in.Base, Draft: gogithub.Ptr(in.Draft),
	})
	if err != nil {
		return PR{}, fmt.Errorf("open a pull request in %s from %s to %s: %w", slug, in.Head, in.Base, err)
	}
	return convert(created), nil
}

// Comment adds a comment to a pull request or an issue.
func (s *Service) Comment(ctx context.Context, slug string, number int, body string) error {
	owner, name, err := split(slug)
	if err != nil {
		return err
	}
	if _, _, err := s.client.Issues.CreateComment(ctx, owner, name, number, gogithub.IssueCommentRequest{Body: body}); err != nil {
		return fmt.Errorf("comment on %s#%d: %w", slug, number, err)
	}
	return nil
}

// Close closes a pull request or an issue, without merging. The issues endpoint
// does it for both.
func (s *Service) Close(ctx context.Context, slug string, number int) error {
	owner, name, err := split(slug)
	if err != nil {
		return err
	}
	if _, _, err := s.client.Issues.Update(ctx, owner, name, number, gogithub.UpdateIssueRequest{State: gogithub.Ptr("closed")}); err != nil {
		return fmt.Errorf("close %s#%d: %w", slug, number, err)
	}
	return nil
}

// RepoLabels lists the names of the labels of a repository, every page of them.
func (s *Service) RepoLabels(ctx context.Context, slug string) ([]string, error) {
	owner, name, err := split(slug)
	if err != nil {
		return nil, err
	}
	opts := &gogithub.ListOptions{PerPage: 100}
	var names []string
	for {
		page, resp, err := s.client.Issues.ListLabels(ctx, owner, name, opts)
		if err != nil {
			return nil, fmt.Errorf("list the labels of %s: %w", slug, err)
		}
		for _, l := range page {
			names = append(names, l.GetName())
		}
		if resp.NextPage == 0 {
			return names, nil
		}
		opts.Page = resp.NextPage
	}
}

// AddLabels adds labels, which must exist, to a pull request or an issue. No
// labels is no request.
func (s *Service) AddLabels(ctx context.Context, slug string, number int, labels []string) error {
	owner, name, err := split(slug)
	if err != nil {
		return err
	}
	if len(labels) == 0 {
		return nil
	}
	if _, _, err := s.client.Issues.AddLabelsToIssue(ctx, owner, name, number, labels); err != nil {
		return fmt.Errorf("add labels to %s#%d: %w", slug, number, err)
	}
	return nil
}

// convert turns go-github's pull request into ours.
func convert(p *gogithub.PullRequest) PR {
	labels := make([]string, len(p.Labels))
	for i, l := range p.Labels {
		labels[i] = l.GetName()
	}
	return PR{
		Number: p.GetNumber(), URL: p.GetHTMLURL(), Title: p.GetTitle(), Body: p.GetBody(),
		Author: p.GetUser().GetLogin(), Base: p.GetBase().GetRef(),
		HeadRef: p.GetHead().GetRef(), HeadOwner: p.GetHead().GetUser().GetLogin(),
		Labels: labels, Draft: p.GetDraft(),
	}
}
