package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/git/repo"
)

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
// repos.paths entry, a scan of the repos.search directories and the clone root, matched by
// remote URL (never by directory name), then a sibling of the working
// directory's repository named like the repo. The scan is built once, and only
// when an earlier rule misses. A slug that no rule resolves, or that several
// clones match equally, is a row with Err set, not a failure of the call.
func (w *Workspace) RepoPaths(slugs []string) []RepoPath {
	out := make([]RepoPath, len(slugs))
	if len(slugs) == 0 {
		return out
	}
	here := w.cwdClone()
	var scanned []clone
	scan := func() []clone {
		if scanned == nil {
			scanned = scanClones(w.scanDirs())
		}
		return scanned
	}
	for i, slug := range slugs {
		out[i] = w.repoPath(slug, here, scan)
	}
	return out
}

// RepoPath resolves one slug as RepoPaths does and returns its Err as the
// error.
func (w *Workspace) RepoPath(slug string) (RepoPath, error) {
	rp := w.RepoPaths([]string{slug})[0]
	return rp, rp.Err
}

func (w *Workspace) repoPath(slug string, here *clone, scan func() []clone) RepoPath {
	if here != nil && here.matches(slug) == strong {
		return RepoPath{Slug: slug, Path: here.path, Source: PathCwd}
	}
	if rp, ok := w.configuredPath(slug, scan); ok {
		return rp
	}
	if here != nil {
		_, name, _ := strings.Cut(slug, "/")
		sibling := filepath.Join(filepath.Dir(here.path), name)
		if info, err := os.Stat(sibling); name != "" && err == nil && info.IsDir() {
			return RepoPath{Slug: slug, Path: sibling, Source: PathSibling}
		}
	}
	return RepoPath{Slug: slug, Err: ErrRepoNotFound}
}

// scanDirs are the directories scanned for clones: the repos.search entries and
// the clone root, since a clone that gero repo clone made there is found again
// without being listed twice.
func (w *Workspace) scanDirs() []string {
	dirs := slices.Clone(w.settings.Repos.Search)
	if root := w.settings.Repos.CloneRoot; root != "" {
		dirs = append(dirs, root)
	}
	return dirs
}

// ConfiguredRepoPath resolves one slug by the user's own config alone: a
// repos.paths entry, else a clone found under a repos.search directory or the clone root. The
// working directory plays no part, so the answer is the same wherever it is
// asked. A slug that neither finds, or that several clones match equally, is
// ErrRepoNotFound or ErrRepoAmbiguous.
func (w *Workspace) ConfiguredRepoPath(slug string) (RepoPath, error) {
	rp, ok := w.configuredPath(slug, func() []clone { return scanClones(w.scanDirs()) })
	if !ok {
		rp = RepoPath{Slug: slug, Err: ErrRepoNotFound}
	}
	return rp, rp.Err
}

// configuredPath applies the two rules that come from the user file.
func (w *Workspace) configuredPath(slug string, scan func() []clone) (RepoPath, bool) {
	for key, p := range w.settings.Repos.Paths {
		if strings.EqualFold(key, slug) {
			return RepoPath{Slug: slug, Path: p, Source: PathSettings}, true
		}
	}
	return fromScan(slug, scan())
}

// fromScan picks the clone for slug: the only strong match, else the only weak
// one. Several of the best strength are ambiguous. ok is false when none match.
func fromScan(slug string, clones []clone) (RepoPath, bool) {
	var strongs, weaks []string
	for _, c := range clones {
		switch c.matches(slug) {
		case strong:
			strongs = append(strongs, c.path)
		case weak:
			weaks = append(weaks, c.path)
		}
	}
	found := strongs
	if len(found) == 0 {
		found = weaks
	}
	switch len(found) {
	case 0:
		return RepoPath{}, false
	case 1:
		return RepoPath{Slug: slug, Path: found[0], Source: PathScan}, true
	}
	slices.Sort(found)
	return RepoPath{Slug: slug, Candidates: found, Err: ErrRepoAmbiguous}, true
}

// strength is how well a clone's remotes match a slug.
type strength int

const (
	none   strength = iota
	weak            // only a same-name remote under another owner: a fork
	strong          // a remote that is the slug itself
)

// clone is a local repository and the slugs its remote URLs name.
type clone struct {
	path  string
	slugs []string
}

func (c clone) matches(slug string) strength {
	owner, name, _ := strings.Cut(slug, "/")
	best := none
	for _, s := range c.slugs {
		o, n, _ := strings.Cut(s, "/")
		switch {
		case strings.EqualFold(s, slug):
			return strong
		case strings.EqualFold(n, name) && !strings.EqualFold(o, owner):
			best = weak
		}
	}
	return best
}

// cwdClone returns the repository containing the working directory, or nil
// when there is none or its remotes cannot be read.
func (w *Workspace) cwdClone() *clone {
	r, err := w.Current()
	if err != nil {
		return nil
	}
	return clonePath(r.Root(), r)
}

// clonePath returns the clone at path for the opened repository r, or nil when
// its remotes cannot be read.
func clonePath(path string, r *repo.Repo) *clone {
	remotes, err := r.Remotes()
	if err != nil {
		return nil
	}
	c := &clone{path: path}
	for _, urls := range remotes {
		for _, u := range urls {
			if slug, err := git.SlugFromURL(u); err == nil {
				c.slugs = append(c.slugs, slug)
			}
		}
	}
	return c
}

// maxScanDepth is how many directory levels below a search directory the scan
// goes, so a team directory, or a group inside one, can hold clones without being
// listed itself. It bounds the walk, which stops at every clone and skips hidden
// directories, when a search directory is much larger than a projects directory.
const maxScanDepth = 5

// scanClones finds the clones below each dir, to maxScanDepth levels, without
// looking inside a clone. A directory that cannot be read is skipped, hidden
// directories are not entered, and a clone found twice is listed once.
func scanClones(dirs []string) []clone {
	var out []clone
	seen := map[string]bool{}
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		for _, child := range subdirs(dir) {
			if r, err := repo.Open(child); err == nil {
				if !seen[child] {
					seen[child] = true
					if c := clonePath(child, r); c != nil {
						out = append(out, *c)
					}
				}
				continue
			}
			if depth < maxScanDepth {
				walk(child, depth+1)
			}
		}
	}
	for _, dir := range dirs {
		walk(dir, 1)
	}
	return out
}

// subdirs lists the directories directly inside dir, or nothing when dir
// cannot be read.
func subdirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}
