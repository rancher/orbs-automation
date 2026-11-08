// Package releasetag is the policy of the tags of a release line: which tag a
// line can have next, which tags it may not have, and what its newest is. It is
// pure, with no I/O and no knowledge of how a tag is made, and it knows the
// project's settings only as arguments (the tag pattern, the line's series and
// requiresPrerelease). Version arithmetic is in internal/platform/semver, which knows
// nothing of lines; the tag task, gero repo tag and gero repo status all ask
// here, so what the status says is what the tag command does
// (docs/specs/TASK_COMMANDS.md).
package releasetag

import (
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/semver"
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
	Repo   string // owner/name, to say which repository a message is about
	Tag    string
	Commit string // the commit to tag: the head of the line's branch, as the host has it
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
	if in.Series != "" && !semver.InSeries(in.Tag, in.Series) {
		return Decision{}, fmt.Errorf("%w: %s is not in %s", ErrNotInSeries, in.Tag, in.Series)
	}
	if in.RequiresPrerelease && !v.IsPrerelease() && !hasPrereleaseOf(v, in) {
		return Decision{}, fmt.Errorf("%w: %s has none of %s on %s", ErrNeedsPrerelease, in.Tag, base(v), short(in.Commit))
	}
	newest := Newest(in)
	if newest != nil {
		if nv, _ := semver.Parse(newest.Name); semver.Compare(v, nv) <= 0 {
			return Decision{}, fmt.Errorf("%w: %s is not newer than %s", ErrNotNewer, in.Tag, newest.Name)
		}
	}
	return Decision{Action: Create, Newest: newest}, nil
}

// Newest is the newest tag of the line, of any kind: of the series when there
// is one, else of every tag the pattern matches. A tag that is not a version, such
// as the rolling main-<sha> ones, does not count.
func Newest(in Input) *git.Tag {
	var best *git.Tag
	var bestV semver.Version
	for i := range in.Tags {
		t := in.Tags[i]
		if in.Pattern != "" {
			if ok, err := path.Match(in.Pattern, t.Name); err != nil || !ok {
				continue
			}
		}
		if in.Series != "" && !semver.InSeries(t.Name, in.Series) {
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

// Kind says what a candidate tag is.
type Kind string

// The kinds of tag a line can have next.
const (
	// KindPrerelease: the next iteration of the prerelease under way, or after a
	// stable tag the first prerelease of the next version.
	KindPrerelease Kind = "prerelease"
	// KindStable: the next stable tag, after a stable one.
	KindStable Kind = "stable"
	// KindPromotion: the stable version of the prerelease that is newest.
	KindPromotion Kind = "promotion"
)

// Candidate is a tag the line could have next, and whether it may be made now.
type Candidate struct {
	Tag  string
	Kind Kind
	// Promotes is the prerelease a promotion is the stable version of.
	Promotes *git.Tag
	// Err is why the tag may not be made now (ErrNothingNew, ErrNeedsPrerelease,
	// ErrMovedSincePrerelease, ErrNoBase), or nil when it may. Tag is still the tag it
	// would be, except for ErrNoBase, which has none.
	Err error
}

// Candidates are the two tags a line can have next, a prerelease and a stable or
// promotion, each with the reason it may not be made now when it may not. It is
// the one set of rules that Choose picks from and that gero repo status shows, so
// what the status says is what the tag command does. Stable is not used.
func Candidates(c Choice) []Candidate {
	return []Candidate{prereleaseCandidate(c), stableCandidate(c)}
}

// Choose picks the tag to make from the newest tag of the line, with
// internal/platform/semver doing the arithmetic. It is the policy that semver does not
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
	cand := prereleaseCandidate(c)
	if c.Stable {
		cand = stableCandidate(c)
	}
	if cand.Err != nil {
		return Chosen{}, cand.Err
	}
	return Chosen{Tag: cand.Tag, Promotes: cand.Promotes}, nil
}

// vPrefix is "v" when the line's newest tag has one, else when the pattern says.
func vPrefix(c Choice, newest *git.Tag) string {
	name := c.Pattern
	if newest != nil {
		name = newest.Name
	}
	if strings.HasPrefix(name, "v") {
		return "v"
	}
	return ""
}

func prereleaseCandidate(c Choice) Candidate {
	cand := Candidate{Kind: KindPrerelease}
	newest := Newest(Input{Tags: c.Tags, Pattern: c.Pattern, Series: c.Series})
	if newest == nil {
		start, ok := seriesStart(c.Series)
		if !ok {
			cand.Err = fmt.Errorf("%w: name the tag", ErrNoBase)
			return cand
		}
		start.Prerelease = "rc.1"
		cand.Tag = vPrefix(c, nil) + start.String()
		return cand
	}
	v, _ := semver.Parse(newest.Name)
	label := "rc"
	if v.IsPrerelease() {
		label = "" // the one under way keeps its name
	}
	next, err := v.NextPrerelease(c.Bump, label)
	if err != nil {
		cand.Err = err
		return cand
	}
	cand.Tag = vPrefix(c, newest) + next.String()
	if newest.Commit == c.Commit {
		cand.Err = fmt.Errorf("%w: %s is on %s", ErrNothingNew, newest.Name, short(c.Commit))
	}
	return cand
}

func stableCandidate(c Choice) Candidate {
	cand := Candidate{Kind: KindStable}
	newest := Newest(Input{Tags: c.Tags, Pattern: c.Pattern, Series: c.Series})
	if newest == nil {
		start, ok := seriesStart(c.Series)
		if !ok {
			cand.Err = fmt.Errorf("%w: name the tag", ErrNoBase)
			return cand
		}
		cand.Tag = vPrefix(c, nil) + start.String()
		if c.RequiresPrerelease {
			cand.Err = fmt.Errorf("%w: the project requires a prerelease before every stable tag, so make one first", ErrNeedsPrerelease)
		}
		return cand
	}
	v, _ := semver.Parse(newest.Name)
	cand.Tag = vPrefix(c, newest) + v.NextStable(c.Bump).String()
	switch {
	case v.IsPrerelease():
		cand.Kind, cand.Promotes = KindPromotion, newest
		if newest.Commit != c.Commit {
			cand.Err = fmt.Errorf("%w: %s is on %s and the branch is at %s, so the commits after it have no prerelease: make another",
				ErrMovedSincePrerelease, newest.Name, short(newest.Commit), short(c.Commit))
		}
	case newest.Commit == c.Commit:
		cand.Err = fmt.Errorf("%w: %s is on %s", ErrNothingNew, newest.Name, short(c.Commit))
	case c.RequiresPrerelease:
		cand.Err = fmt.Errorf("%w: the project requires a prerelease before every stable tag, so make one first", ErrNeedsPrerelease)
	}
	return cand
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

// State is where a release line stands, from its newest tag and its head.
type State string

// The states.
const (
	// StateNoRelease: the line has no tag.
	StateNoRelease State = "no-release"
	// StateReleased: the newest tag is stable and the head is on it.
	StateReleased State = "released"
	// StateWorkSinceRelease: the newest tag is stable and the head is past it.
	StateWorkSinceRelease State = "work-since-release"
	// StateRCOpen: the newest tag is a prerelease and the head is on it, so it can
	// be promoted.
	StateRCOpen State = "rc-open"
	// StateRCOpenWork: the newest tag is a prerelease and the head is past it.
	StateRCOpenWork State = "rc-open-work"
)

// StateOf says where the line stands: c has the tags of the line and the head of
// its branch, and the rest of it is not used. "On the tag" is the head's commit
// being the tag's, which the host alone can say, with no clone. It is "" when
// there is no head, since there is then no branch to be in a state.
func StateOf(c Choice) State {
	if c.Commit == "" {
		return ""
	}
	newest := Newest(Input{Tags: c.Tags, Pattern: c.Pattern, Series: c.Series})
	if newest == nil {
		return StateNoRelease
	}
	v, _ := semver.Parse(newest.Name)
	onTag := newest.Commit == c.Commit
	switch {
	case v.IsPrerelease() && onTag:
		return StateRCOpen
	case v.IsPrerelease():
		return StateRCOpenWork
	case onTag:
		return StateReleased
	}
	return StateWorkSinceRelease
}
