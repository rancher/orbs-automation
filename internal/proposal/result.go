package proposal

import (
	"errors"

	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
)

// ResultStatus is what a run did for one target.
type ResultStatus string

// The results.
const (
	// ResultCommitted: the commit is on the new branch in the clone.
	ResultCommitted ResultStatus = "committed"
	// ResultWouldCommit: a dry run made the commit in a worktree and removed it.
	ResultWouldCommit ResultStatus = "would commit"
	// ResultUpToDate: the edit and the steps changed nothing.
	ResultUpToDate ResultStatus = "already up to date"
	// ResultFailed: preparing the branch, a step or the commit failed.
	ResultFailed ResultStatus = "failed"
	// ResultOpened: the branch was pushed and the PR opened.
	ResultOpened ResultStatus = "opened"
	// ResultAlreadyOpen: the actor's own PR at this version is open, so nothing
	// was made.
	ResultAlreadyOpen ResultStatus = "already open"
	// ResultBlocked: the actor's own PR at a newer version is open.
	ResultBlocked ResultStatus = "blocked"
	// ResultRefreshed: the actor's own open PR was rebuilt and its branch replaced.
	ResultRefreshed ResultStatus = "refreshed"
	// ResultWouldRefresh: a dry run found that a refresh would replace the branch.
	ResultWouldRefresh ResultStatus = "would refresh"
)

// TargetResult is what a run did for one target that needed a PR.
type TargetResult struct {
	Status  ResultStatus `json:"status"`
	Branch  string       `json:"branch"`            // the new branch
	Commits []CommitInfo `json:"commits,omitempty"` // the commits made, oldest first
	Place   Place        `json:"place,omitempty"`   // where it was made
	Reason  string       `json:"reason,omitempty"`  // why that was a worktree
	Changes []string     `json:"changes,omitempty"` // what the strategy changed, in words
	Error   string       `json:"error,omitempty"`   // why it failed

	// What publishing did, set only by a run that publishes.
	Pushed     string   `json:"pushed,omitempty"`     // the remote the branch was pushed to
	PR         *PRRef   `json:"pr,omitempty"`         // the PR that was opened
	Superseded []PRRef  `json:"superseded,omitempty"` // the older PRs that were closed
	Warnings   []string `json:"warnings,omitempty"`   // what went wrong that did not fail the target
	// WouldSupersede is what a dry run would have closed.
	WouldSupersede []PRRef `json:"wouldSupersede,omitempty"`
	// BehindBy is how many commits the target has that an already open PR's branch
	// does not, when it is behind.
	BehindBy int `json:"behindBy,omitempty"`
	// Note says more about the result than its status does: that an already open
	// PR is identical to a fresh build, or why it was not refreshed.
	Note string `json:"note,omitempty"`
}

// PRRef is a PR, by number and where to see it.
type PRRef struct {
	Number int    `json:"number"`
	URL    string `json:"url,omitempty"`
}

// ErrDeclined is returned by a run, with its plan, when the confirmation was
// answered no. Nothing was written.
var ErrDeclined = errors.New("declined: nothing was written")

// Tip is the newest commit of the result, or "" when there is none.
func (r TargetResult) Tip() string { return Tip(r.Commits) }

// ErrNoRemote is returned by a run when it is to publish and cannot tell who is
// running, so it does not know whose PRs are its own.
var ErrNoRemote = errors.New("could not tell who is running on GitHub")

// ErrNoClone is returned by a run when there are commits to make and the
// engineer has no clone of the consumer to make them in.
var ErrNoClone = errors.New("no local clone of the consumer")

// Refs turns PRs into references.
func Refs(prs []pulls.PR) []PRRef {
	out := make([]PRRef, len(prs))
	for i, p := range prs {
		out[i] = PRRef{Number: p.Number, URL: p.URL}
	}
	return out
}

// Finish records a failure to end the work, which is an error even when the
// commit was made.
func Finish(result *TargetResult, err error) *TargetResult {
	if err != nil {
		result.Error = "the work was done, but ending it failed: " + err.Error()
		result.Status = ResultFailed
	}
	return result
}
