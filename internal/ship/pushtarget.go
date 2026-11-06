package ship

import (
	"errors"
	"fmt"
)

// PushRole is where a branch is pushed, by role and not by remote name.
type PushRole string

// The roles.
const (
	// RoleFork is the engineer's own copy of the repository.
	RoleFork PushRole = "fork"
	// RoleUpstream is the real repository.
	RoleUpstream PushRole = "upstream"
)

// PushChoice is what each layer says about where to push: "", "fork" or
// "upstream". The nearest to the run wins.
type PushChoice struct {
	Flag        string // --push-to
	UserRepo    string // repos.overrides.<slug>.pushTo in the user file
	Project     string // the consumer's pullRequests.pushTo
	UserDefault string // pushTo in the user file
}

// PushRemotes are the remote names the roles stand for, as the settings give
// them; empty when not set.
type PushRemotes struct {
	Fork     string
	Upstream string
}

// PushTarget is where a branch will be pushed, and what decided it.
type PushTarget struct {
	Role   PushRole `json:"role"`
	Remote string   `json:"remote"` // the remote's name
	Source string   `json:"source"` // the layer that decided, for the report
}

// resolvePushTarget decides where to push. In CI the answer is always the
// upstream. Otherwise the first layer that says is used, from the flag outward,
// and with none the fork when one is set and the upstream when it is not. A role
// with no remote set, and a value that is neither role, are errors that say what
// to set. Only the layer that decides is read, so a bad value in one that is
// overridden is not an error.
func resolvePushTarget(c PushChoice, r PushRemotes, ci bool) (PushTarget, error) {
	if ci {
		return target(RoleUpstream, "CI always pushes to the upstream", r)
	}
	layers := []struct{ value, name string }{
		{c.Flag, "--push-to"},
		{c.UserRepo, "repos.overrides.<repo>.pushTo"},
		{c.Project, "the project's pullRequests.pushTo"},
		{c.UserDefault, "pushTo in the user file"},
	}
	for _, l := range layers {
		if l.value == "" {
			continue
		}
		role := PushRole(l.value)
		if role != RoleFork && role != RoleUpstream {
			return PushTarget{}, fmt.Errorf("%q is not a push target (%s): use fork or upstream", l.value, l.name)
		}
		return target(role, l.name, r)
	}
	if r.Fork != "" {
		return target(RoleFork, "a fork is set", r)
	}
	return target(RoleUpstream, "no fork is set", r)
}

// target finds the remote for a role, or says what to set.
func target(role PushRole, source string, r PushRemotes) (PushTarget, error) {
	switch {
	case role == RoleFork && r.Fork != "":
		return PushTarget{Role: role, Remote: r.Fork, Source: source}, nil
	case role == RoleUpstream && r.Upstream != "":
		return PushTarget{Role: role, Remote: r.Upstream, Source: source}, nil
	case role == RoleFork:
		return PushTarget{}, errors.New("pushing to the fork needs a fork remote: set fork in the user file, or push with --push-to upstream")
	}
	return PushTarget{}, errors.New("pushing to the upstream needs an upstream remote: set upstream in the user file (or repos.overrides.<repo>.upstream), or push with --push-to fork")
}
