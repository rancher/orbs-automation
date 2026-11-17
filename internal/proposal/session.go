package proposal

import (
	"context"
	"fmt"

	"github.com/rancher/orbs-automation/internal/chain"
)

// Session is what a run that proposes changes to one repo sets up before it does anything:
// the engineer's clone and its upstream remote, the publisher when branches are pushed, who
// the run acts as, and, in CI, the identity the commits are made as.
type Session struct {
	Clone  string
	Remote string
	// Pub is the publisher of the run, and nil when nothing is pushed.
	Pub *Publisher
	// Me is the login whose PRs are the actor's own, when there is a publisher.
	Me    string
	Push  *PushTarget
	Actor *Actor
	// Committer is who the commits are made as when git's own configuration is not to
	// decide: the bot's, in CI. Empty leaves it to git.
	Committer Identity
}

// SessionInput says which repo a run proposes to and how it publishes.
type SessionInput struct {
	Repo    *chain.Project // the repo the PRs are in
	PushTo  string         // fork or upstream, or empty to let the settings decide
	CI      bool           // the actor is the bot
	Refresh bool           // let a branch of ours on the remote be replaced
	NoPush  bool           // stop after the commits
}

// Begin sets a run up. It is an error, ErrNoClone, when there is no clone to make the commits
// in, and it fails before anything is written when it is to publish and cannot tell who is
// running, or where to push.
func Begin(ctx context.Context, site Site, ex *Executor, in SessionInput) (*Session, error) {
	slug := in.Repo.Slug()
	clone, err := site.CloneDir(slug)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoClone, err)
	}
	remote, err := site.Remote(slug)
	if err != nil {
		return nil, err
	}
	s := &Session{Clone: clone, Remote: remote}
	if ex.Publish != nil && !in.NoPush {
		if s.Pub, s.Me, s.Push, err = PublisherFor(ctx, site, ex, in.Repo, clone, in.PushTo, in.CI, in.Refresh); err != nil {
			return nil, err
		}
	}
	if ex.Identify != nil {
		if s.Actor, err = ActorFor(ctx, ex, clone, s.Me); err != nil {
			return nil, err
		}
		if in.CI {
			// The bot has no git config in a fresh clone to read. A workstation's
			// commits take theirs from git, which is what was just read.
			s.Committer = s.Actor.Identity
		}
	}
	return s, nil
}

// Confirm asks the one question before anything is written, when the run is going to
// publish something and the executor can ask: c says what will be done and writes whether
// that is anything. It is ErrDeclined when the answer is no, with nothing written.
func (s *Session) Confirm(ctx context.Context, ex *Executor, dryRun bool, c Confirmation, writes bool) error {
	if s.Pub == nil || dryRun || ex.Confirm == nil || !writes {
		return nil
	}
	ok, err := ex.Confirm(ctx, c)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDeclined
	}
	return nil
}
