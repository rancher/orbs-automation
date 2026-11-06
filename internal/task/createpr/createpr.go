// Package createpr is the task that opens a pull request for a branch that is
// already pushed, and labels it: one action, run alone or as a step of a longer
// process (docs/specs/PROJECT_LAYOUT.md, Tasks).
package createpr

import (
	"context"

	"github.com/rancher/orbs-automation/internal/github/pulls"
)

// Input is the PR to open.
type Input struct {
	Repo  string // the repository the PR is in, owner/name
	Title string
	Body  string
	// Head is the branch the PR is from, with the owner in front when it is in a
	// fork: owner:branch.
	Head string
	Base string // the branch the PR is into
	// Labels are the labels to apply. A label the repository lacks is a warning,
	// and so is any failure to apply them: a label never fails the PR.
	Labels []string
}

// Output is the PR that was opened.
type Output struct {
	Number int
	URL    string
	// Labels are the labels applied, in the repository's own spelling.
	Labels []string
	// Warnings are what went wrong that did not fail the PR.
	Warnings []string
}

// Remote is what opening the PR needs from GitHub. It is defined here, where it
// is used.
type Remote interface {
	Create(ctx context.Context, slug string, in pulls.NewPR) (pulls.PR, error)
	RepoLabels(ctx context.Context, slug string) ([]string, error)
	AddLabels(ctx context.Context, slug string, number int, labels []string) error
}

// Run opens the PR. TODO: the stub does nothing yet.
func Run(_ context.Context, _ Remote, _ Input) (Output, error) {
	return Output{}, nil
}
