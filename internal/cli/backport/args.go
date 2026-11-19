// Package backport holds the logic of gero repo backport: what the command asks of the backport
// domain, the questions it asks a person at a conflict, and how it reports what came of it. It is
// framework-free: cmd/ owns the cobra wiring.
package backport

// Args is what the positional arguments of gero repo backport say.
type Args struct {
	Project string   // the repo, by config name or slug; empty for the one the working directory is in
	Lines   []string // the targets, as typed
	PR      int      // the number of the merged PR
	// Repo is the owner/name a PR URL named, or empty when the PR was a number. It must be the
	// repo of the project.
	Repo string
}

// ParseArgs reads [project] <line>[,<line>...] <pr>. The PR is always the last argument: a
// number, #number or the URL of the PR. The targets are the one before it, separated by commas.
func ParseArgs(args []string) (Args, error) {
	return Args{}, nil
}
