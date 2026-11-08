// Package createpr is the task that opens a pull request for a branch that is
// already pushed, and labels it: one action, run alone or as a step of a longer
// process (docs/specs/PROJECT_LAYOUT.md, Tasks).
package createpr

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
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

// Run opens the PR and then applies the labels the repository has. A failure to
// open it is the error. A label the repository lacks, and any failure to read or
// apply the labels, is a warning, and the repository's labels are read only when
// there are labels to apply.
func Run(ctx context.Context, r Remote, in Input) (Output, error) {
	created, err := r.Create(ctx, in.Repo, pulls.NewPR{Title: in.Title, Body: in.Body, Head: in.Head, Base: in.Base})
	if err != nil {
		return Output{}, err
	}
	out := Output{Number: created.Number, URL: created.URL}
	out.Labels, out.Warnings = label(ctx, r, in.Repo, created.Number, in.Labels)
	return out, nil
}

// label applies the wanted labels that the repository has, and says which were
// applied, in the repository's spelling, and what went wrong.
func label(ctx context.Context, r Remote, repo string, number int, wanted []string) (applied, warnings []string) {
	if len(wanted) == 0 {
		return nil, nil
	}
	have, err := r.RepoLabels(ctx, repo)
	if err != nil {
		return nil, []string{fmt.Sprintf("could not read the labels of %s, so none were applied: %v", repo, err)}
	}
	spelling := make(map[string]string, len(have))
	for _, l := range have {
		spelling[strings.ToLower(l)] = l
	}
	var apply []string
	for _, l := range wanted {
		actual, ok := spelling[strings.ToLower(l)]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("label %s is not in %s, so it was not applied", l, repo))
			continue
		}
		if !slices.Contains(apply, actual) {
			apply = append(apply, actual)
		}
	}
	if err := r.AddLabels(ctx, repo, number, apply); err != nil {
		return nil, append(warnings, fmt.Sprintf("could not apply the labels: %v", err))
	}
	return apply, warnings
}
