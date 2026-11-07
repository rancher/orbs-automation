// Package createtag is the task that tags a commit of a project's release line
// in the engineer's clone: one action, run alone or as a step of a longer process
// (docs/specs/PROJECT_LAYOUT.md, Tasks). Which tag, and on which commit, is the
// caller's to say; the task checks that it is a tag the line can have, and never
// moves or replaces one.
package createtag

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/semver"
)

var (
	// ErrNotAVersion is returned for a tag that is not a SemVer version.
	ErrNotAVersion = errors.New("the tag is not a version")
	// ErrPattern is returned for a tag the project's tagPattern does not match.
	ErrPattern = errors.New("the tag does not match the project's tag pattern")
	// ErrNotInSeries is returned for a version outside the series of the line.
	ErrNotInSeries = errors.New("the tag is not in the line's series")
	// ErrNotNewer is returned for a version that is not newer than the newest tag
	// the line already has.
	ErrNotNewer = errors.New("the tag is not newer than the line's newest")
	// ErrNeedsPrerelease is returned for a stable tag of a project that requires a
	// prerelease first, when the commit has no prerelease of that version.
	ErrNeedsPrerelease = errors.New("a stable tag needs a prerelease of that version on the same commit first")
	// ErrNothingNew is returned when the commit already has the line's newest tag,
	// so another tag of the line would say nothing new.
	ErrNothingNew = errors.New("the commit already has the line's newest tag")
	// ErrMovedSincePrerelease is returned when a prerelease is to be promoted to
	// stable and the head of the branch is no longer the commit it was made on.
	ErrMovedSincePrerelease = errors.New("the branch has moved since the prerelease")
	// ErrNoBase is returned when there is no tag to follow and no series to start
	// one from.
	ErrNoBase = errors.New("the line has no tag to follow and no series to start from")
	// ErrTagExists is returned when the tag is already there, on another commit. A
	// tag is never moved.
	ErrTagExists = errors.New("the tag already exists on another commit")
)

// Input says which tag to make, on what, and what the repository holds already.
type Input struct {
	Repo     string // owner/name, to say which repository a message is about
	CloneDir string // the clone the tag is made in
	Tag      string
	Commit   string // the commit to tag: the head of the line's branch, as the host has it
	// Tags are the repository's tags as the host has them, and the local tag of the
	// name if there is one, with the commit each is on (an annotated tag at the
	// commit it peels to).
	Tags    []git.Tag
	Pattern string // the project's tagPattern, such as "v*"; "" matches every tag
	Series  string // the line's series, such as "0.8.x" or "v2.15"; "" for no series
	// RequiresPrerelease is the project's release.requiresPrerelease: a stable tag
	// is only made on a commit that already has a prerelease of its version.
	RequiresPrerelease bool
}

// Action is what making the tag comes to.
type Action string

// The actions.
const (
	// Create: the tag does not exist and will be made.
	Create Action = "create"
	// Already: the tag exists on this very commit, so there is nothing to do.
	Already Action = "already"
)

// Decision is what Check found.
type Decision struct {
	Action Action
	// Newest is the tag the new one follows in the line, when there is one.
	Newest *git.Tag
}

// Git is what making the tag needs from git. It is defined here, where it is
// used.
type Git interface {
	// CreateTag makes the lightweight tag name at commit in the clone in dir.
	CreateTag(ctx context.Context, dir, name, commit string) error
}

// Check says what making the tag would do, or why it cannot be made, and reads
// nothing and writes nothing. A tag that exists is settled first: on this commit
// it is already done, whatever else is true of it, and on another it is an
// error. Then the tag must match the pattern, be a version, be in the series and
// be newer than the newest tag of the line, so a tag made by hand in between, or
// an older one asked for by mistake, is caught.
func Check(in Input) (Decision, error) {
	for _, t := range in.Tags {
		if t.Name != in.Tag {
			continue
		}
		if t.Commit == in.Commit {
			return Decision{Action: Already}, nil
		}
		return Decision{}, fmt.Errorf("%w: %s is on %s in %s, and a tag is never moved", ErrTagExists, in.Tag, short(t.Commit), in.Repo)
	}
	if in.Pattern != "" {
		if ok, err := path.Match(in.Pattern, in.Tag); err != nil || !ok {
			return Decision{}, fmt.Errorf("%w: %s is not %s", ErrPattern, in.Tag, in.Pattern)
		}
	}
	v, ok := semver.Parse(in.Tag)
	if !ok {
		return Decision{}, fmt.Errorf("%w: %s", ErrNotAVersion, in.Tag)
	}
	if in.Series != "" && !git.InSeries(in.Tag, in.Series) {
		return Decision{}, fmt.Errorf("%w: %s is not in %s", ErrNotInSeries, in.Tag, in.Series)
	}
	if in.RequiresPrerelease && !v.IsPrerelease() && !hasPrereleaseOf(v, in) {
		return Decision{}, fmt.Errorf("%w: %s has none of %s on %s", ErrNeedsPrerelease, in.Tag, base(v), short(in.Commit))
	}
	newest := newestOf(in)
	if newest != nil {
		if nv, _ := semver.Parse(newest.Name); semver.Compare(v, nv) <= 0 {
			return Decision{}, fmt.Errorf("%w: %s is not newer than %s", ErrNotNewer, in.Tag, newest.Name)
		}
	}
	return Decision{Action: Create, Newest: newest}, nil
}

// newestOf is the newest tag of the line, of any kind: of the series when there
// is one, else of every tag the pattern matches. A tag that is not a version, such
// as the rolling main-<sha> ones, does not count.
func newestOf(in Input) *git.Tag {
	var best *git.Tag
	var bestV semver.Version
	for i := range in.Tags {
		t := in.Tags[i]
		if in.Pattern != "" {
			if ok, err := path.Match(in.Pattern, t.Name); err != nil || !ok {
				continue
			}
		}
		if in.Series != "" && !git.InSeries(t.Name, in.Series) {
			continue
		}
		v, ok := semver.Parse(t.Name)
		if !ok {
			continue
		}
		if best == nil || semver.Compare(v, bestV) > 0 {
			best, bestV = &t, v
		}
	}
	return best
}

// Run checks the tag and makes it. A tag that is already on the commit is not
// made again, and is not an error.
func Run(ctx context.Context, g Git, in Input) (Decision, error) {
	d, err := Check(in)
	if err != nil || d.Action == Already {
		return d, err
	}
	if err := g.CreateTag(ctx, in.CloneDir, in.Tag, in.Commit); err != nil {
		return Decision{}, err
	}
	return d, nil
}

// short abbreviates a commit for a message.
func short(commit string) string { return commit[:min(len(commit), 8)] }

// Choice is what Choose needs to pick the tag to make when none was named.
type Choice struct {
	Tags    []git.Tag
	Pattern string
	Series  string
	Commit  string // the head of the line's branch
	// Stable asks for a stable tag: the promotion of the newest prerelease, or after
	// a stable tag the next stable one. Without it the tag is a prerelease.
	Stable bool
	// Bump says how much a new version rises after a stable tag. It does not
	// apply to the iteration or the promotion of a prerelease.
	Bump               semver.Bump
	RequiresPrerelease bool
}

// Chosen is the tag Choose picked.
type Chosen struct {
	Tag string
	// Promotes is the prerelease that the tag is the stable version of, when it is.
	Promotes *git.Tag
}

// Choose picks the tag to make from the newest tag of the line, with
// internal/semver doing the arithmetic. It is the policy that semver does not
// have: which tag may follow which.
//
// Without Stable it is a prerelease: the next iteration of the one under way, or
// after a stable tag the first of the next version. With Stable after a
// prerelease it is the promotion of that prerelease, which only a head that is
// still on the prerelease's commit may have, and after a stable tag it is the
// next stable, unless the project requires a prerelease first. A commit that
// already has the line's newest tag gets no new one. The tag is written with the
// "v" the line's tags have, and with the project's pattern when there are none.
func Choose(c Choice) (Chosen, error) {
	newest := newestOf(Input{Tags: c.Tags, Pattern: c.Pattern, Series: c.Series})
	if newest == nil {
		return first(c)
	}
	prefix := ""
	if strings.HasPrefix(newest.Name, "v") {
		prefix = "v"
	}
	v, _ := semver.Parse(newest.Name)
	switch {
	case !c.Stable:
		if newest.Commit == c.Commit {
			return Chosen{}, fmt.Errorf("%w: %s is on %s", ErrNothingNew, newest.Name, short(c.Commit))
		}
		label := "rc"
		if v.IsPrerelease() {
			label = "" // the one under way keeps its name
		}
		next, err := v.NextPrerelease(c.Bump, label)
		if err != nil {
			return Chosen{}, err
		}
		return Chosen{Tag: prefix + next.String()}, nil
	case v.IsPrerelease():
		if newest.Commit != c.Commit {
			return Chosen{}, fmt.Errorf("%w: %s is on %s and the branch is at %s, so the commits after it have no prerelease: make another",
				ErrMovedSincePrerelease, newest.Name, short(newest.Commit), short(c.Commit))
		}
		return Chosen{Tag: prefix + v.NextStable(c.Bump).String(), Promotes: newest}, nil
	case newest.Commit == c.Commit:
		return Chosen{}, fmt.Errorf("%w: %s is on %s", ErrNothingNew, newest.Name, short(c.Commit))
	case c.RequiresPrerelease:
		return Chosen{}, fmt.Errorf("%w: the project requires a prerelease before every stable tag, so make one first", ErrNeedsPrerelease)
	}
	return Chosen{Tag: prefix + v.NextStable(c.Bump).String()}, nil
}

// first is the first tag of a line that has none, from the start of its series.
func first(c Choice) (Chosen, error) {
	start, ok := seriesStart(c.Series)
	if !ok {
		return Chosen{}, fmt.Errorf("%w: name the tag", ErrNoBase)
	}
	prefix := ""
	if strings.HasPrefix(c.Pattern, "v") {
		prefix = "v"
	}
	if !c.Stable {
		start.Prerelease = "rc.1"
		return Chosen{Tag: prefix + start.String()}, nil
	}
	if c.RequiresPrerelease {
		return Chosen{}, fmt.Errorf("%w: the project requires a prerelease before every stable tag, so make one first", ErrNeedsPrerelease)
	}
	return Chosen{Tag: prefix + start.String()}, nil
}

// seriesStart is the lowest version of a series: 0.8.x is 0.8.0, 9.x is 9.0.0 and
// v2.15 is 2.15.0. It is false for no series, or one that is not numbers and x.
func seriesStart(series string) (semver.Version, bool) {
	if series == "" {
		return semver.Version{}, false
	}
	parts := strings.Split(strings.TrimPrefix(series, "v"), ".")
	if len(parts) > 3 {
		return semver.Version{}, false
	}
	var n [3]uint64
	for i, part := range parts {
		if part == "x" {
			continue
		}
		num, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return semver.Version{}, false
		}
		n[i] = num
	}
	return semver.Version{Major: n[0], Minor: n[1], Patch: n[2]}, true
}

// hasPrereleaseOf reports whether the commit has a prerelease tag of the version
// v, which is stable: the tag that the stable one is the promotion of.
func hasPrereleaseOf(v semver.Version, in Input) bool {
	for _, t := range in.Tags {
		if t.Commit != in.Commit {
			continue
		}
		pv, ok := semver.Parse(t.Name)
		if ok && pv.IsPrerelease() && pv.Major == v.Major && pv.Minor == v.Minor && pv.Patch == v.Patch {
			return true
		}
	}
	return false
}

// base is a version without its prerelease, as a prerelease tag of it reads.
func base(v semver.Version) string { return v.String() + "-*" }
