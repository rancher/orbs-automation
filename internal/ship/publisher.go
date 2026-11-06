package ship

import (
	"context"

	"github.com/rancher/orbs-automation/internal/github/pulls"
)

// Remote is the GitHub side of publishing, as ship needs it. It is defined here,
// where it is used, and the PR service (internal/github/pulls) is what satisfies
// it, with Me from the token's login.
type Remote interface {
	// Me is the login that is running: whose PRs are the actor's own.
	Me(ctx context.Context) (string, error)
	OpenForBase(ctx context.Context, slug, base string, limit int) (pulls.Open, error)
	Create(ctx context.Context, slug string, in pulls.NewPR) (pulls.PR, error)
	Comment(ctx context.Context, slug string, number int, body string) error
	Close(ctx context.Context, slug string, number int) error
	RepoLabels(ctx context.Context, slug string) ([]string, error)
	AddLabels(ctx context.Context, slug string, number int, labels []string) error
}

// Publisher pushes a committed branch, opens its PR and closes the PRs it
// supersedes.
type Publisher struct {
	Git    Git
	Remote Remote
	Clone  string     // the clone the branch is in
	Push   PushTarget // where the branch is pushed
	// HeadOwner is the owner of the fork, for the PR's head when the branch is
	// pushed there: the head is HeadOwner:branch. It is not used for the upstream.
	HeadOwner string
	// IsBot says the actor is the bot, which adds BotLabels to the PR's labels.
	IsBot     bool
	BotLabels []string
}

// publish pushes the branch of a target that was committed, opens its PR with
// the labels that exist, and then comments on and closes each superseded PR, in
// that order, so a failure leaves the older PRs as they were. res is updated: a
// failure to push or to open the PR fails the target, and one to label or to close
// an older PR is a warning.
//
// STUB: does nothing until a later commit.
func (p Publisher) publish(ctx context.Context, consumer string, tp TargetPlan, m Marker, dec Publish, res *TargetResult) {
}
