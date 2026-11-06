package ship

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
	Role   PushRole
	Remote string // the remote's name
	Source string // the layer that decided, for the report
}

// resolvePushTarget decides where to push. In CI the answer is always the
// upstream. Otherwise the first layer that says is used, from the flag outward,
// and with none the fork when one is set and the upstream when it is not. A role
// with no remote set, and a value that is neither role, are errors that say what
// to set.
//
// STUB: always the upstream until a later commit.
func resolvePushTarget(c PushChoice, r PushRemotes, ci bool) (PushTarget, error) {
	return PushTarget{Role: RoleUpstream, Remote: r.Upstream}, nil
}
