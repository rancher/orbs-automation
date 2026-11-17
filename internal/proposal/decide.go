package proposal

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
)

// DecideInput is what is decided about one proposal: the PRs that may be of the same one.
type DecideInput struct {
	Repo    string  // the repository the PR is in, owner/name
	Subject Subject // the proposal: its branch, and its texts, whose key finds its other branches
	Marker  Marker  // who the proposal is
	// Compare orders two versions of a marker: negative when a is older than b, and false
	// when they cannot be ordered. Nil orders them as semver tags.
	Compare func(a, b string) (int, bool)
	// Branches are the branches of the repo as the host has them, in which the other branches
	// of this proposal are looked for by the end of their names. Nil looks for none.
	Branches []git.Branch
	// Noun is what the proposal is called in what is said about it: "bump" when empty.
	Noun string
	// Blocked says why a newer PR of the actor's own blocks the proposal, given that PR's
	// marker. Nil says it in general words.
	Blocked func(newer Marker, pr pulls.PR) string
}

// Decided is what was found out about one proposal before any work: the decision about its
// PR, what to warn about, and, for one that needs no work, its result.
type Decided struct {
	Decision Decision
	Warnings []string
	// Done is set when there is nothing to make: a newer PR blocks it, or the lookup failed.
	Done *TargetResult
}

// Decide looks at the open PRs of a proposal's base branch, when there is a publisher, and
// decides what to do about them. A PR of the actor's own at a newer version blocks it, and a
// failed lookup fails it, each before any work. It only reads.
func Decide(ctx context.Context, pub *Publisher, me string, in DecideInput, sink Sink) Decided {
	var st Decided
	if pub == nil {
		return st
	}
	noun := in.Noun
	if noun == "" {
		noun = "bump"
	}
	s := in.Subject
	open, err := pub.Remote.OpenForBase(ctx, in.Repo, s.Branch, 0)
	if err != nil {
		st.Done = &TargetResult{Status: ResultFailed, Branch: s.Preview.Branch,
			Error: fmt.Sprintf("look for the open PRs of %s: %v", in.Repo, err)}
		return st
	}
	prs := open.PRs
	addHead := func(head string) {
		byHead, err := pub.Remote.OpenForHead(ctx, in.Repo, head)
		if err != nil {
			st.Warnings = append(st.Warnings, fmt.Sprintf("could not look for the PR of %s: %v", head, err))
			return
		}
		for _, p := range byHead {
			if !slices.ContainsFunc(prs, func(q pulls.PR) bool { return q.Number == p.Number }) {
				prs = append(prs, p)
			}
		}
	}
	// The listing reads only the newest, so also ask for the one head this run
	// would push to, which is exact whatever the number of open PRs.
	ownHead := pub.HeadOwnerOf(in.Repo) + ":" + s.Preview.Branch
	addHead(ownHead)
	// And for the branches of this proposal that anyone left on the upstream, found by
	// the tail of their names in the refs the run already has: a PR of an older or
	// a newer version is found whatever the prefix and however many PRs are open.
	owner, _, _ := strings.Cut(in.Repo, "/")
	branches := BumpBranches(in.Branches, SourceOfKey(s.Preview.Key, in.Marker.Version, s.Branch), s.Branch)
	if len(branches) > MaxBumpBranches {
		st.Warnings = append(st.Warnings, fmt.Sprintf("%d branches of this %s are on %s, so only the first %d were asked for their PR", len(branches), noun, in.Repo, MaxBumpBranches))
		branches = branches[:MaxBumpBranches]
	}
	for _, name := range branches {
		if head := owner + ":" + name; head != ownHead {
			addHead(head)
		}
	}
	compare := in.Compare
	if compare == nil {
		compare = CompareVersions
	}
	st.Decision = DecidePublishBy(prs, me, in.Marker, compare)
	emit(sink, Event{Kind: EventDecided, Branch: s.Branch, Line: s.Line, Decision: &st.Decision})
	if open.Truncated {
		st.Warnings = append(st.Warnings, fmt.Sprintf("only the newest %d open PRs of %s were looked at, so an older PR of this %s may have been missed", len(open.PRs), in.Repo, noun))
	}
	for _, p := range prs {
		if found, ok := ParseMarker(p.Body); ok && found.SameBump(in.Marker) {
			st.Warnings = append(st.Warnings, MarkerWarnings(p, found)...)
		}
	}
	for _, a := range st.Decision.Also {
		behind, known := 0, false
		if a.HeadOwner != "" {
			if n, err := pub.Remote.BehindBy(ctx, in.Repo, s.Branch, a.HeadOwner+":"+a.HeadRef); err == nil {
				behind, known = n, true
			}
		}
		st.Warnings = append(st.Warnings, AlsoNote(a, s.Branch, behind, known))
	}
	for _, o := range st.Decision.OtherBase {
		st.Warnings = append(st.Warnings, OtherBaseNote(o, s.Branch))
	}
	if st.Decision.Action == PublishBlocked {
		newer, _ := ParseMarker(st.Decision.Newer.Body)
		reason := fmt.Sprintf("#%d already writes %s to %s, which is newer than %s", st.Decision.Newer.Number, newer.Version, s.Branch, in.Marker.Version)
		if in.Blocked != nil {
			reason = in.Blocked(newer, *st.Decision.Newer)
		}
		st.Done = &TargetResult{Status: ResultBlocked, Branch: s.Preview.Branch, Warnings: st.Warnings, Error: reason}
	}
	return st
}
