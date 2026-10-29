package workspace

import "errors"

// How a RepoPath was found: where the winning rule looked.
const (
	PathCwd      = "cwd"               // the repository containing the working directory
	PathSettings = "repos.paths"       // a repos.paths entry in the user file
	PathScan     = "repos.search scan" // a clone found under a repos.search directory
	PathSibling  = "sibling"           // a directory next to the working directory's repository
)

var (
	// ErrRepoNotFound is returned when no rule finds a local clone for a slug.
	ErrRepoNotFound = errors.New("no local clone found")
	// ErrRepoAmbiguous is returned when several clones match a slug equally
	// well. RepoPath.Candidates lists them.
	ErrRepoAmbiguous = errors.New("several local clones match")
)

// RepoPath is where one repository lives on this machine.
type RepoPath struct {
	Slug       string   // owner/name, as asked for
	Path       string   // empty unless the clone was found
	Source     string   // one of the Path constants; empty unless found
	Candidates []string // sorted; set only with ErrRepoAmbiguous
	Err        error    // nil, ErrRepoNotFound or ErrRepoAmbiguous
}

// RepoPaths resolves the local clone of each slug, in the order given. The
// rules, first match wins: the repository containing the working directory, a
// repos.paths entry, a depth-2 scan of the repos.search directories matched by
// remote URL (never by directory name), then a sibling of the working
// directory's repository named like the repo. The scan is built once, and only
// when an earlier rule misses. A slug that no rule resolves, or that several
// clones match equally, is a row with Err set, not a failure of the call.
func (w *Workspace) RepoPaths(slugs []string) []RepoPath {
	out := make([]RepoPath, len(slugs))
	for i, slug := range slugs {
		out[i] = RepoPath{Slug: slug, Err: errors.New("not implemented")}
	}
	return out
}

// RepoPath resolves one slug as RepoPaths does and returns its Err as the
// error.
func (w *Workspace) RepoPath(slug string) (RepoPath, error) {
	rp := w.RepoPaths([]string{slug})[0]
	return rp, rp.Err
}
