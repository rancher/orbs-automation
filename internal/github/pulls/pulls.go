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
type Service struct {
	client *gogithub.Client
	creds  github.Credentials
	opts   []gogithub.ClientOptionsFunc
	bot    string // the login to answer Me with, when the token is an installation's
}

// New returns a Service for the resolved credentials. Extra options (a transport
// for tests) are applied last.
func New(c github.Credentials, opts ...gogithub.ClientOptionsFunc) (*Service, error) {
	client, err := github.NewClient(c, opts...)
	if err != nil {
		return nil, err
	}
	return &Service{client: client, creds: c, opts: opts}, nil
}

// AsBot returns the service for a token that belongs to an installation and not
// a user, such as the bot's in CI: Me is then login, because there is no user to
// ask GitHub about.
func (s *Service) AsBot(login string) *Service {
	bot := *s
	bot.bot = login
	return &bot
}

// Me is the login whose PRs are the actor's own: the one the token belongs to,
// asked of GitHub, or the bot's when the service is AsBot.
func (s *Service) Me(ctx context.Context) (string, error) {
	if s.bot != "" {
		return s.bot, nil
	}
	return github.Login(ctx, s.creds, s.opts...)
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
// limit of them (DefaultMax when it is 0). It does not read them all: more than
// limit is Truncated.
func (s *Service) OpenForBase(ctx context.Context, slug, base string, limit int) (Open, error) {
	owner, name, err := split(slug)
	if err != nil {
		return Open{}, err
	}
	if limit <= 0 {
		limit = DefaultMax
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
		if len(out.PRs) >= limit {
			out.Truncated = len(out.PRs) > limit || resp.NextPage != 0
			out.PRs = out.PRs[:limit]
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
	title, body, draft := in.Title, in.Body, in.Draft
	created, _, err := s.client.PullRequests.Create(ctx, owner, name, gogithub.CreatePullRequest{
		Title: &title, Body: &body, Head: in.Head, Base: in.Base, Draft: &draft,
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
	closed := "closed"
	if _, _, err := s.client.Issues.Update(ctx, owner, name, number, gogithub.UpdateIssueRequest{State: &closed}); err != nil {
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

// Comment is a comment on a pull request or an issue.
type Comment struct {
	ID     int64
	Body   string
	Author string
}

// BehindBy says how many commits base has that head does not: how far a PR's
// branch is behind the branch it targets. head is owner:branch for a branch in a
// fork.
func (s *Service) BehindBy(ctx context.Context, slug, base, head string) (int, error) {
	owner, name, err := split(slug)
	if err != nil {
		return 0, err
	}
	cmp, _, err := s.client.Repositories.CompareCommits(ctx, owner, name, base, head, &gogithub.ListOptions{PerPage: 1})
	if err != nil {
		return 0, fmt.Errorf("compare %s with %s in %s: %w", head, base, slug, err)
	}
	return cmp.GetBehindBy(), nil
}

// UpdateBody replaces the title and the body of a pull request.
func (s *Service) UpdateBody(ctx context.Context, slug string, number int, title, body string) error {
	owner, name, err := split(slug)
	if err != nil {
		return err
	}
	if _, _, err := s.client.PullRequests.Edit(ctx, owner, name, number, &gogithub.PullRequest{Title: &title, Body: &body}); err != nil {
		return fmt.Errorf("update %s#%d: %w", slug, number, err)
	}
	return nil
}

// Comments lists the comments of a pull request or an issue, oldest first, up to
// the first limit of them (DefaultMax when it is 0).
func (s *Service) Comments(ctx context.Context, slug string, number, limit int) ([]Comment, error) {
	owner, name, err := split(slug)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = DefaultMax
	}
	opts := &gogithub.IssueListCommentsOptions{ListOptions: gogithub.ListOptions{PerPage: 100}}
	var out []Comment
	for {
		page, resp, err := s.client.Issues.ListComments(ctx, owner, name, number, opts)
		if err != nil {
			return nil, fmt.Errorf("list the comments of %s#%d: %w", slug, number, err)
		}
		for _, c := range page {
			out = append(out, Comment{ID: c.GetID(), Body: c.GetBody(), Author: c.GetUser().GetLogin()})
		}
		if len(out) >= limit {
			return out[:limit], nil
		}
		if resp.NextPage == 0 {
			return out, nil
		}
		opts.Page = resp.NextPage
	}
}

// EditComment replaces the body of a comment.
func (s *Service) EditComment(ctx context.Context, slug string, id int64, body string) error {
	owner, name, err := split(slug)
	if err != nil {
		return err
	}
	if _, _, err := s.client.Issues.UpdateComment(ctx, owner, name, id, gogithub.IssueCommentRequest{Body: body}); err != nil {
		return fmt.Errorf("edit comment %d of %s: %w", id, slug, err)
	}
	return nil
}
