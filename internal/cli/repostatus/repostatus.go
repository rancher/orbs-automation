// Package repostatus holds the logic of gero repo status: where the releases of
// one repository stand, read from its host and compared with the local clone.
// Unlike repo info it reads the network. It is framework-free: cmd/ owns the
// cobra wiring.
package repostatus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrIncomplete is returned by Run, after the report, when the host could not
// be read, so the report lacks what only the host knows.
var ErrIncomplete = errors.New("the host could not be read: the report is incomplete")

// Workspace is what Status needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Project returns the Project with the given config name or owner/name
	// slug, or the one in the working directory when the key is empty.
	Project(key string) (config.Project, error)
	RepoPath(slug string) (workspace.RepoPath, error)
	// UpstreamRefs lists the tags and branch heads of slug as its host reports
	// them.
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
	// TODO(release-state): consumer tracing adds UpstreamFile(ctx, slug, ref,
	// path) here (workspace.Workspace has it) and Updates(), the engine of the
	// updates config. Do not read a pin here by hand: gero ship reads what a
	// consumer holds with the same pieces, so use them (see the TODO on fill).
	// Keep it a separate interface if the fakes in the tests grow past a few
	// methods.
}

// Clone is what Status reads from a local clone.
type Clone interface {
	// BranchCommit returns the commit a local branch is at, and false when the
	// clone has no such branch.
	BranchCommit(branch string) (commit string, found bool, err error)
	// Count returns how many commits are reachable from to and not from from,
	// and false when the clone lacks either commit.
	Count(from, to string) (n int, known bool, err error)
}

// Opener opens the clone in dir.
type Opener func(dir string) (Clone, error)

// Status is everything the command reports about one repository.
type Status struct {
	Slug          string `json:"slug"`
	DefaultBranch string `json:"defaultBranch"`
	// Error is set when the host could not be read; Targets then have no tags
	// and no heads.
	Error string `json:"error,omitempty"`
	// Targets is a row per release line, or the one row of a project with a
	// single release branch.
	Targets []Target `json:"targets"`
	// Clone is why the comparisons with the local clone are missing, when
	// they are.
	Clone string `json:"clone,omitempty"`
}

// Target is one release branch and where its releases stand.
//
// TODO(release-state): Target is a flat row of facts. Per-line state
// resolution wants a derived, named state on top of them (see the TODOs on
// fill), and a Consumers field for the trace of who has picked the tags up.
// Expect Target to split into the facts read (host, clone) and a State computed
// from them by a function with no I/O, so the rules are table-tested.
type Target struct {
	// Line and Series are set for a project with a branch per line.
	Line   string `json:"line,omitempty"`
	Series string `json:"series,omitempty"`
	Branch string `json:"branch"`
	// IsDefaultBranch is set when the config names no branch for the line, so
	// Branch is the Project's default branch.
	IsDefaultBranch bool `json:"isDefaultBranch,omitempty"`
	// Head is the commit at the head of Branch on the host, empty when the
	// host has no such branch.
	Head             string   `json:"head,omitempty"`
	LatestStable     *TagInfo `json:"latestStable,omitempty"`
	LatestPrerelease *TagInfo `json:"latestPrerelease,omitempty"`
	// Unreleased is the number of commits on Head since the latest tag (the
	// pre-release when there is a newer one), counted in the local clone. It is
	// nil when unknown: no tag, no head, or the clone lacks a commit.
	Unreleased *int `json:"unreleased,omitempty"`
	// Local is how the clone's branch compares with Head.
	Local string `json:"local,omitempty"`
}

// TagInfo is a tag and the commit it points at.
type TagInfo struct {
	Tag    string `json:"tag"`
	Commit string `json:"commit"`
}

// Run reports on the Project named by slugs, or on the one whose repository
// contains the working directory cwd, and renders it as s.Output (text or
// json). When the host could not be read it returns the report together with
// ErrIncomplete.
func Run(ctx context.Context, s clicfg.Settings, cwd string, slugs []string) (string, error) {
	if s.Output != clicfg.OutputText && s.Output != clicfg.OutputJSON {
		return "", fmt.Errorf("output format %q is not supported by repo status: use text or json", s.Output)
	}
	ws := workspace.Open(s, cwd)
	st, err := gather(ctx, ws, func(dir string) (Clone, error) {
		r, err := ws.OpenAt(dir)
		if err != nil {
			return nil, err
		}
		return r, nil
	}, slugs)
	if err != nil {
		return "", err
	}
	out := renderText(st)
	if s.Output == clicfg.OutputJSON {
		if out, err = renderJSON(st); err != nil {
			return "", err
		}
	}
	if st.Error != "" {
		return out, ErrIncomplete
	}
	return out, nil
}

func gather(ctx context.Context, ws Workspace, open Opener, slugs []string) (Status, error) {
	var key string
	if len(slugs) > 0 {
		key = slugs[0]
	}
	p, err := ws.Project(key)
	if err != nil {
		return Status{}, err
	}
	st := Status{Slug: p.Repo.Name, DefaultBranch: p.Repo.DefaultBranch, Targets: targets(p)}
	refs, err := ws.UpstreamRefs(ctx, p.Repo.Name)
	if err != nil {
		st.Error = err.Error()
		return st, nil
	}
	clone, why := openClone(ws, open, p.Repo.Name)
	st.Clone = why
	for n := range st.Targets {
		fill(&st.Targets[n], p, refs, clone)
	}
	return st, nil
}

// targets lists the release branches of p, before anything is read.
func targets(p config.Project) []Target {
	if p.Release.Branching != config.BranchingPerLine {
		return []Target{{Branch: p.Repo.DefaultBranch, IsDefaultBranch: true}}
	}
	var ts []Target
	for _, l := range p.Release.Lines {
		branch, isDefault := p.Branch(l)
		ts = append(ts, Target{Line: l.Line, Series: l.Series(), Branch: branch, IsDefaultBranch: isDefault})
	}
	return ts
}

// fill reads the latest tags and the head of t from refs, and compares them
// with the clone, which may be nil.
//
// TODO(release-state): three things to build on this, in this order.
//
//  1. Release in progress. A line has one when it has a pre-release newer than
//     the latest stable tag (what LatestPrerelease already means), or commits
//     past the latest tag (Unreleased > 0). Name the states (nothing
//     released, released and quiet, work since release, pre-release open) and
//     compute them from the facts here, not in the renderer.
//  2. What comes next, from SemVer alone. The newest tag decides, through
//     semver.Version.NextStable and NextPrerelease (internal/semver), which
//     answer for a stable or a prerelease tag alike: after a stable tag the
//     next is a prerelease of a higher version (patch, minor or major is the
//     engineer's call, so list the candidates); after a prerelease it is
//     another iteration (rc.N+1) or the stable of that version. Those know
//     nothing of projects. What is allowed is decided here, with the project's
//     requiresPrerelease: a project that requires a pre-release before every
//     stable has no direct stable successor to a stable tag.
//  3. Consumers. For each downstream the project ships to (Engine.RoutesFrom
//     of the updates engine, internal/updatedispatcher), take the consumer
//     branches a line goes to (Route.BranchesFor), look up each head commit
//     (git.Refs.Branch of the consumer's UpstreamRefs) and read the pinned
//     value there with the route's Invocation.Check over
//     updatedispatcher.HostTree, which is exactly what gero ship does. Check
//     with the latest stable as the release says up to date, behind or ahead
//     (Check.Relation), and with the latest pre-release when it differs; a
//     strategy that finds no pin is an error to report as "no pin found". A consumer on a pre-release of a version that is now stable is
//     behind. This is the costly tier (a request per consumer and branch), so
//     it runs only when asked for (see the spec: tiers behind flags).
func fill(t *Target, p config.Project, refs git.Refs, clone Clone) {
	tags := refs.Tags
	if t.Series != "" {
		tags = nil
		for _, tag := range refs.Tags {
			if git.InSeries(tag.Name, t.Series) {
				tags = append(tags, tag)
			}
		}
	}
	latest := git.LatestReleases(tags, p.Release.TagPattern)
	t.LatestStable, t.LatestPrerelease = tagInfo(latest.Stable), tagInfo(latest.Prerelease)
	for _, b := range refs.Branches {
		if b.Name == t.Branch {
			t.Head = b.Commit
		}
	}
	if clone == nil || t.Head == "" {
		return
	}
	if base := newest(t); base != nil {
		if n, known, err := clone.Count(base.Commit, t.Head); err == nil && known {
			t.Unreleased = &n
		}
	}
	t.Local = compare(clone, t.Branch, t.Head)
}

// newest is the tag releases are counted from: a pre-release is only there when
// it is newer than the latest stable tag.
func newest(t *Target) *TagInfo {
	if t.LatestPrerelease != nil {
		return t.LatestPrerelease
	}
	return t.LatestStable
}

// compare says how the clone's branch stands against the host's head.
func compare(c Clone, branch, head string) string {
	local, found, err := c.BranchCommit(branch)
	switch {
	case err != nil:
		return "unreadable: " + err.Error()
	case !found:
		return "no local branch"
	case local == head:
		return "in sync"
	}
	behind, okBehind, err1 := c.Count(local, head)
	ahead, _, err2 := c.Count(head, local)
	switch {
	case err1 != nil || err2 != nil || !okBehind:
		// The host's head is not in the clone: it has not been fetched, so how
		// far behind is unknown, and the clone cannot tell if it is ahead.
		return "behind (fetch to see by how much)"
	case ahead == 0:
		return fmt.Sprintf("behind %d", behind)
	case behind == 0:
		return fmt.Sprintf("ahead %d", ahead)
	default:
		return fmt.Sprintf("diverged (ahead %d, behind %d)", ahead, behind)
	}
}

// openClone opens the clone of slug. The reason is set instead when there is
// none to compare with.
func openClone(ws Workspace, open Opener, slug string) (Clone, string) {
	rp, err := ws.RepoPath(slug)
	if err != nil {
		return nil, err.Error()
	}
	c, err := open(rp.Path)
	if err != nil {
		return nil, err.Error()
	}
	return c, ""
}

func tagInfo(t *git.Tag) *TagInfo {
	if t == nil {
		return nil
	}
	return &TagInfo{Tag: t.Name, Commit: t.Commit}
}

func renderText(st Status) string {
	var b strings.Builder
	b.WriteString(st.Slug + "\n")
	if st.Error != "" {
		fmt.Fprintf(&b, "  host: unavailable: %s\n", st.Error)
		return b.String()
	}
	b.WriteString("\n")
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	perLine := len(st.Targets) > 0 && st.Targets[0].Line != ""
	if perLine {
		fmt.Fprintln(w, "  LINE\tBRANCH\tHEAD\tLATEST STABLE\tLATEST PRE-RELEASE\tUNRELEASED\tLOCAL")
	} else {
		fmt.Fprintln(w, "  BRANCH\tHEAD\tLATEST STABLE\tLATEST PRE-RELEASE\tUNRELEASED\tLOCAL")
	}
	for _, t := range st.Targets {
		branch := t.Branch
		if perLine && t.IsDefaultBranch {
			branch += " (default branch)"
		}
		cells := []string{branch, headText(t.Head), tagText(t.LatestStable), tagText(t.LatestPrerelease),
			countText(t.Unreleased), localText(t.Local)}
		if perLine {
			cells = append([]string{t.Line}, cells...)
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(cells, "\t"))
	}
	_ = w.Flush() // writes to a strings.Builder, which cannot fail
	if st.Clone != "" {
		fmt.Fprintf(&b, "\nLocal clone: unavailable: %s\n", st.Clone)
	}
	return b.String()
}

// headText renders the commit at a branch head, or says the host has no such
// branch.
func headText(commit string) string {
	if commit == "" {
		return "(not found)"
	}
	return git.Short(commit)
}

// tagText renders a tag as "v2.15.3 (c4246d8)", or "-" for none.
func tagText(t *TagInfo) string {
	if t == nil {
		return "-"
	}
	return t.Tag + " (" + git.Short(t.Commit) + ")"
}

func countText(n *int) string {
	if n == nil {
		return "-"
	}
	return strconv.Itoa(*n)
}

func localText(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func renderJSON(st Status) (string, error) {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
