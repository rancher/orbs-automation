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
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/git/releasetag"
	"github.com/rancher/orbs-automation/internal/semver"
	"github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrIncomplete is returned by Run, after the report, when the host could not
// be read, so the report lacks what only the host knows.
var ErrIncomplete = errors.New("the host could not be read: the report is incomplete")

// Workspace is what Status needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Reader is the projects and update methods of the config and what the host
	// has: the project asked about, the tags and branch heads of slug as its host
	// reports them, and the files of a consumer, which tracing who has picked a
	// release up reads with the same pieces gero ship uses.
	ship.Reader
	RepoPath(slug string) (workspace.RepoPath, error)
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
	// Consumers is what each project that this one ships to holds of its releases,
	// and Sources what this one takes from each project that ships to it. They are
	// left out for a project with none, or when the options skip them.
	Consumers []ship.ConsumerHoldings `json:"consumers,omitempty"`
	Sources   []ship.SourceHoldings   `json:"sources,omitempty"`
	// TraceError is why the consumers or the sources could not be worked out at all,
	// such as an update method that is not usable.
	TraceError string `json:"traceError,omitempty"`
}

// Options says which sections beyond the releases and the local clone to leave
// out.
type Options struct {
	NoConsumers bool // do not trace who has picked up what this project ships
	NoSources   bool // do not trace what this project takes from others
}

// Target is one release branch and where its releases stand.
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
	// State is where the line stands, from its newest tag and its head, as
	// releasetag.State names it. It needs the host alone. Empty when the host has no
	// such branch.
	State string `json:"state,omitempty"`
	// Next are the tags the line could have next, as gero repo tag would make them.
	Next []Next `json:"next,omitempty"`
}

// Next is a tag a line could have next.
type Next struct {
	Tag string `json:"tag"`
	// Kind is prerelease, stable or promotion.
	Kind string `json:"kind"`
	// Promotes is the prerelease a promotion is the stable version of.
	Promotes string `json:"promotes,omitempty"`
	// Available is false when the tag may not be made now, and Why says why.
	Available bool   `json:"available"`
	Why       string `json:"why,omitempty"`
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
func Run(ctx context.Context, s clicfg.Settings, cwd string, slugs []string, opts Options) (string, error) {
	if s.Output != clicfg.OutputText && s.Output != clicfg.OutputJSON {
		return "", fmt.Errorf("output format %q is not supported by repo status: use text or json", s.Output)
	}
	ws := workspace.Open(s, cwd)
	st, err := gatherWith(ctx, ws, func(dir string) (Clone, error) {
		r, err := ws.OpenAt(dir)
		if err != nil {
			return nil, err
		}
		return r, nil
	}, slugs, opts)
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
	return gatherWith(ctx, ws, open, slugs, Options{})
}

// gatherWith is gather with the options of the command.
func gatherWith(ctx context.Context, ws Workspace, open Opener, slugs []string, opts Options) (Status, error) {
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
	trace(ctx, ws, p, opts, &st)
	return st, nil
}

// trace adds the consumers and the sources of p to the status, unless the options
// skip them. What it reads is read by ship.Holdings and ship.Sources, which are the
// pieces a run of gero ship reads a consumer with.
func trace(ctx context.Context, ws ship.Reader, p config.Project, opts Options, st *Status) {
	var problems []string
	note := func(err error) {
		if err != nil && !slices.Contains(problems, err.Error()) {
			problems = append(problems, err.Error())
		}
	}
	if !opts.NoConsumers {
		var err error
		st.Consumers, err = ship.Holdings(ctx, ws, p)
		note(err)
	}
	if !opts.NoSources {
		var err error
		st.Sources, err = ship.Sources(ctx, ws, p)
		note(err)
	}
	st.TraceError = strings.Join(problems, "; ")
}

// holdStatus says in a few words how what a consumer branch holds stands against
// the releases that go to it: the latest stable, and when a newer prerelease
// exists the newest. A Relation of newer means the release is newer than what the
// branch holds, so the branch is behind.
func holdStatus(h ship.Holding) string {
	if h.Error != "" {
		return "unreadable: " + h.Error
	}
	if h.Stable == nil {
		return "-"
	}
	stable := h.Stable
	if h.Newest != nil && h.Newest.Tag != stable.Tag {
		newest := h.Newest
		switch {
		case newest.UpToDate:
			return "current"
		case stable.UpToDate:
			return "on stable, " + newest.Tag + " is newer"
		case stable.Relation == updatedispatcher.RelationNewer:
			return "behind " + stable.Tag
		case newest.Relation == updatedispatcher.RelationNewer:
			return "on a prerelease, " + newest.Tag + " is newer"
		case newest.Relation == updatedispatcher.RelationOlder:
			return "ahead of " + newest.Tag
		}
		return "unknown"
	}
	switch {
	case stable.UpToDate:
		return "current"
	case stable.Relation == updatedispatcher.RelationNewer:
		return "behind " + stable.Tag
	case stable.Relation == updatedispatcher.RelationOlder:
		return "ahead of " + stable.Tag
	}
	return "unknown"
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

// fill reads the latest tags and the head of t from refs, works out its state and
// the tags it could have next from them (internal/git/releasetag, which is what
// gero repo tag asks), and compares them with the clone, which may be nil.
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
	if t.Head != "" {
		c := releasetag.Choice{
			Tags: refs.Tags, Pattern: p.Release.TagPattern, Series: t.Series, Commit: t.Head,
			RequiresPrerelease: p.Release.RequiresPrerelease,
		}
		t.State, t.Next = string(releasetag.StateOf(c)), nextOf(c)
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

// nextOf lists the tags a line could have next, as gero repo tag would make them.
// When the newest tag is a prerelease both candidates are listed, the next one and
// the promotion, each with the reason it is not available now when it is not, so
// the engineer sees why and not a missing option. Otherwise it is the next
// prerelease when that may be made now, and none when it may not.
func nextOf(c releasetag.Choice) []Next {
	newest := releasetag.Newest(releasetag.Input{Tags: c.Tags, Pattern: c.Pattern, Series: c.Series})
	open := false
	if newest != nil {
		v, _ := semver.Parse(newest.Name)
		open = v.IsPrerelease()
	}
	var out []Next
	for i, cand := range releasetag.Candidates(c) {
		switch {
		case errors.Is(cand.Err, releasetag.ErrNoBase):
			continue
		case !open && (i > 0 || cand.Err != nil):
			continue
		}
		n := Next{Tag: cand.Tag, Kind: string(cand.Kind), Available: cand.Err == nil, Why: why(cand.Err)}
		if cand.Promotes != nil {
			n.Promotes = cand.Promotes.Name
		}
		out = append(out, n)
	}
	return out
}

// why says in a few words why a tag may not be made now.
func why(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, releasetag.ErrNothingNew):
		return "no new commits"
	case errors.Is(err, releasetag.ErrMovedSincePrerelease):
		return "the branch has moved since the rc"
	case errors.Is(err, releasetag.ErrNeedsPrerelease):
		return "needs an rc first"
	}
	return err.Error()
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

// renderText is the report as sections that each answer one question: what the
// lines of the project have and what they could have next (the host's facts, and
// the meaning made of them), and how the local clone stands (the clone's facts).
func renderText(st Status) string {
	var b strings.Builder
	b.WriteString(st.Slug + "\n")
	if st.Error != "" {
		fmt.Fprintf(&b, "  host: unavailable: %s\n", st.Error)
		return b.String()
	}
	perLine := len(st.Targets) > 0 && st.Targets[0].Line != ""
	b.WriteString("\nReleases\n")
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	header := "BRANCH\tHEAD\tLATEST STABLE\tLATEST PRE-RELEASE\tSTATE\tNEXT"
	if perLine {
		header = "LINE\t" + header
	}
	fmt.Fprintln(w, "  "+header)
	for _, t := range st.Targets {
		cells := []string{branchText(t, perLine), headText(t.Head), tagText(t.LatestStable), tagText(t.LatestPrerelease), stateText(t), nextText(t.Next)}
		if perLine {
			cells = append([]string{t.Line}, cells...)
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(cells, "\t"))
	}
	_ = w.Flush() // writes to a strings.Builder, which cannot fail
	if st.Clone != "" {
		fmt.Fprintf(&b, "\nLocal clone: unavailable: %s\n", st.Clone)
	} else {
		b.WriteString("\nLocal clone\n")
		w = tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		header = "BRANCH\tUNRELEASED\tLOCAL"
		if perLine {
			header = "LINE\t" + header
		}
		fmt.Fprintln(w, "  "+header)
		for _, t := range st.Targets {
			cells := []string{branchText(t, perLine), countText(t.Unreleased), localText(t.Local)}
			if perLine {
				cells = append([]string{t.Line}, cells...)
			}
			fmt.Fprintf(w, "  %s\n", strings.Join(cells, "\t"))
		}
		_ = w.Flush()
	}
	for _, src := range st.Sources {
		renderHoldings(&b, fmt.Sprintf("Sources: %s (%s), %s", src.Name, src.Producer, src.Method), src.Error, src.Targets)
	}
	for _, c := range st.Consumers {
		renderHoldings(&b, fmt.Sprintf("Consumers: %s (%s), %s", c.Name, c.Consumer, c.Method), c.Error, c.Targets)
	}
	if st.TraceError != "" {
		fmt.Fprintf(&b, "\ncould not trace: %s\n", st.TraceError)
	}
	return b.String()
}

// renderHoldings is one consumer's or one source's section: what each branch holds
// against the latest stable release, and how that stands.
func renderHoldings(b *strings.Builder, heading, errText string, targets []ship.Holding) {
	b.WriteString("\n" + heading + "\n")
	if errText != "" {
		fmt.Fprintf(b, "  unavailable: %s\n", errText)
		return
	}
	perLine := slices.ContainsFunc(targets, func(h ship.Holding) bool { return h.Line != "" })
	w := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	header := "BRANCH\tHEAD\tHOLDS\tLATEST STABLE\tSTATUS"
	if perLine {
		header = "LINE\t" + header
	}
	fmt.Fprintln(w, "  "+header)
	for _, h := range targets {
		latest := "-"
		if h.Stable != nil {
			latest = h.Stable.Tag
		}
		holds := h.Current
		if holds == "" {
			holds = "-"
		}
		cells := []string{h.Branch, headText(h.Head), holds, latest, holdStatus(h)}
		if perLine {
			cells = append([]string{h.Line}, cells...)
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(cells, "\t"))
	}
	_ = w.Flush() // writes to a strings.Builder, which cannot fail
}

// branchText is the branch, with a note when it is the default branch because the
// line has none of its own.
func branchText(t Target, perLine bool) string {
	if perLine && t.IsDefaultBranch {
		return t.Branch + " (default branch)"
	}
	return t.Branch
}

// stateText says where a line stands in words, with the commits since the tag when
// the clone could count them.
func stateText(t Target) string {
	since := ""
	if t.Unreleased != nil && *t.Unreleased > 0 {
		since = fmt.Sprintf(" (%d commits)", *t.Unreleased)
	}
	switch releasetag.State(t.State) {
	case releasetag.StateNoRelease:
		return "no release"
	case releasetag.StateReleased:
		return "released"
	case releasetag.StateWorkSinceRelease:
		return "work since release" + since
	case releasetag.StateRCOpen:
		return "rc open, head on the rc"
	case releasetag.StateRCOpenWork:
		return "rc open, work since the rc" + since
	}
	return "-"
}

// nextText lists the candidates: one that may be made as its tag, a promotion with
// the prerelease it promotes, and one that may not be made now in parentheses with
// the reason.
func nextText(next []Next) string {
	if len(next) == 0 {
		return "-"
	}
	parts := make([]string, len(next))
	for i, n := range next {
		switch {
		case n.Available && n.Promotes != "":
			parts[i] = fmt.Sprintf("%s (promotes %s)", n.Tag, n.Promotes)
		case n.Available:
			parts[i] = n.Tag
		case n.Promotes != "":
			parts[i] = fmt.Sprintf("(%s promotes %s: %s)", n.Tag, n.Promotes, n.Why)
		default:
			parts[i] = fmt.Sprintf("(%s: %s)", n.Tag, n.Why)
		}
	}
	return strings.Join(parts, " or ")
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
