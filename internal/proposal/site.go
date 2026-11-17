package proposal

// PushSettings are the user's settings that decide where a branch is pushed, as
// the workspace has them.
type PushSettings struct {
	UserRepo    string // repos.overrides.<slug>.pushTo
	UserDefault string // pushTo
	Fork        string // the remote that holds the engineer's fork
	Upstream    string // the remote that is the real repository
	GitHubUser  string // the owner of the fork, for a PR's head
}

// Site is what a run that proposes changes needs from the invocation's workspace about the
// engineer's clone of the repo it proposes to. It is defined here, where it is consumed, so
// a fake can stand in for it.
type Site interface {
	// CloneDir is the directory of the engineer's clone of a repository, or an
	// error saying how to get one.
	CloneDir(slug string) (string, error)
	// Remote is the name of the remote of a repository's clone that is its
	// upstream.
	Remote(slug string) (string, error)
	// PushSettings are the user's settings that decide where a branch for a
	// repository is pushed.
	PushSettings(slug string) PushSettings
}
