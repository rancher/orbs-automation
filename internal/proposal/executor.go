package proposal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git/gitcli"
)

// Executor is what a run needs to make commits.
type Executor struct {
	Git     Git
	Runner  bump.StepRunner
	TempDir string // where worktrees go
	// Sign says whether git signs every commit with the engineer's own key
	// (signCommits). Nil leaves it to git's own configuration (commit.gpgSign).
	Sign *bool
	// Identify says who the commits are made as, in the clone: git's own answer on
	// a workstation, the bot's in CI. Not set means it is not asked.
	Identify func(ctx context.Context, clone string) (Identity, error)
	// Publish, when set, finds the open PRs of each target before making anything,
	// and with it a run that is not --no-push or --dry-run pushes and opens the
	// PRs. Without it a run stops at the commits.
	Publish *Publisher
	// Confirm, when set, is asked once before anything is written, if the run is
	// going to publish something: it is given what will be done and says whether
	// to go on. It is not asked for a dry run, a --no-push run, or when nothing
	// would be written. Not set means go on.
	Confirm func(ctx context.Context, c Confirmation) (bool, error)
	// Now is the time, for the line a refresh records. Not set is time.Now.
	Now func() time.Time
}

// Identity is the author and committer of the commits of a run.
type Identity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Actor is who a run acts as, shown before anything is written: the identity its
// commits carry, the GitHub account its PRs are opened by (when it opens any), and
// whether the commits are signed.
type Actor struct {
	Identity
	Login   string `json:"login,omitempty"`
	Signing bool   `json:"signing"`
}

// Confirmation is what a run is about to do, for the one question before it
// writes: the targets that will get a new PR, the older PRs that will be closed,
// and the targets that will be left alone.
type Confirmation struct {
	Consumer string     // the repository the PRs are in
	Release  string     // the tag being shipped
	Push     PushTarget // where the branches are pushed
	Opens    []string   // the targets that will get a new PR, by line (TargetPlan.Key)
	Closes   []PRRef    // the older PRs that will be closed, for every target
	Already  []string   // the targets whose PR is already open
	Blocked  []string   // the targets a newer PR of the actor's blocks
	// Refreshes are the target branches whose open PR will be rebuilt, and its
	// branch replaced if it differs: a force push, with a lease.
	Refreshes []string
	// MayReplace says a branch an earlier attempt left on the remote, if it is
	// ours, will be replaced to open the new PRs: --refresh was given.
	MayReplace bool
}

// PublisherFor makes the publisher of a run for the consumer's clone: where its branches
// are pushed, decided from the flag, the settings and the project, and the login whose PRs
// are the actor's own. It returns the target the branches go to.
func PublisherFor(ctx context.Context, site Site, ex *Executor, consumer *chain.Project, clone, pushTo string, ci, refresh bool) (*Publisher, string, *PushTarget, error) {
	p := *ex.Publish
	p.Clone = clone
	settings := site.PushSettings(consumer.Slug())
	target, err := ResolvePushTarget(
		PushChoice{Flag: pushTo, UserRepo: settings.UserRepo, Project: consumer.PullRequests().PushTo, UserDefault: settings.UserDefault},
		PushRemotes{Fork: settings.Fork, Upstream: settings.Upstream}, ci)
	if err != nil {
		return nil, "", nil, err
	}
	if target.Role == RoleFork && settings.GitHubUser == "" {
		return nil, "", nil, errors.New("pushing to the fork needs githubUser in the user file: the PR's head is the fork's owner and the branch")
	}
	p.Push, p.HeadOwner = target, settings.GitHubUser
	p.IsBot, p.BotLabels, p.Refresh = ci, consumer.PullRequests().BotLabels, refresh
	me, err := p.Remote.Me(ctx)
	if err != nil {
		return nil, "", nil, fmt.Errorf("%w: %v", ErrNoRemote, err)
	}
	return &p, me, &target, nil
}

// ActorFor says who a run acts as: the identity its commits carry, the login its PRs
// are opened by, and whether the commits are signed.
func ActorFor(ctx context.Context, ex *Executor, clone, login string) (*Actor, error) {
	id, err := ex.Identify(ctx, clone)
	if err != nil {
		return nil, fmt.Errorf("commit identity: %w", err)
	}
	return &Actor{Identity: id, Login: login, Signing: signs(ctx, ex.Sign, clone)}, nil
}

// signs is whether the commits are signed: what signCommits says, and when it says
// nothing what git does in the clone. A clone git cannot be asked about is said to
// sign nothing, since this only reports.
func signs(ctx context.Context, set *bool, clone string) bool {
	if set != nil {
		return *set
	}
	on, err := gitcli.ConfigBool(ctx, clone, "commit.gpgSign")
	return err == nil && on
}
