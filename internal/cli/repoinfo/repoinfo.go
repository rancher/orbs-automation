// Package repoinfo holds the logic of gero repo info: the central-config entry
// of one repository, its latest releases as the host reports them, and where it
// is cloned on this machine with the state of that clone. It is
// framework-free: cmd/ owns the cobra wiring.
package repoinfo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what Info needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Project returns the Project with the given config name or owner/name
	// slug, or the one in the working directory when the key is empty.
	Project(key string) (config.Project, error)
	RepoPath(slug string) (workspace.RepoPath, error)
	// UpstreamRefs lists the tags and branch heads of slug as its host reports
	// them.
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
}

// Clone is what Info reads from a local clone.
type Clone interface {
	// Head returns the full ref HEAD points to and its abbreviated commit.
	Head() (ref, short string, err error)
	// Clean reports whether no tracked file is changed and no operation (merge,
	// rebase) is in progress.
	Clean() (bool, error)
}

// Opener opens the clone in dir.
type Opener func(dir string) (Clone, error)

// Options are the choices of one run.
type Options struct {
	// Offline skips everything that needs the network: the tags on the host.
	Offline bool
}

// Info is everything the command reports about one repository.
type Info struct {
	Slug          string        `json:"slug"`
	Owned         bool          `json:"owned"`
	DefaultBranch string        `json:"defaultBranch"`
	Release       ReleaseInfo   `json:"release"`
	Upstream      *UpstreamInfo `json:"upstream,omitempty"` // nil when offline
	Local         LocalInfo     `json:"local"`
}

// ReleaseInfo is the Project's release policy with defaults applied.
type ReleaseInfo struct {
	Branches           string     `json:"branches"`
	TagPattern         string     `json:"tagPattern"`
	RequiresPrerelease bool       `json:"requiresPrerelease"`
	Lines              []LineInfo `json:"lines,omitempty"`
}

// LineInfo maps one Rancher line to the Project's series and branch, with the
// latest tags of that series when they were read.
type LineInfo struct {
	Line   string `json:"line"`
	Series string `json:"series"`
	Branch string `json:"branch"`
	// IsDefaultBranch is set when the config names no branch, so Branch is the
	// Project's default branch.
	IsDefaultBranch bool `json:"isDefaultBranch,omitempty"`
	// Head is the commit at the head of Branch on the host. It is empty when
	// the host has no such branch, or when the host was not read.
	Head             string   `json:"head,omitempty"`
	LatestStable     *TagInfo `json:"latestStable,omitempty"`
	LatestPrerelease *TagInfo `json:"latestPrerelease,omitempty"`
}

// TagInfo is a tag and the commit it points at.
type TagInfo struct {
	Tag    string `json:"tag"`
	Commit string `json:"commit"`
}

// UpstreamInfo is what the host reports. The latest tags are the highest by
// version among those matching the Project's tag pattern; a prerelease is
// there only when it is newer than the latest stable tag. Heads is the head of
// each branch a release touches. Error is set instead when the host could not
// be read.
type UpstreamInfo struct {
	Heads            []BranchInfo `json:"heads,omitempty"`
	LatestStable     *TagInfo     `json:"latestStable,omitempty"`
	LatestPrerelease *TagInfo     `json:"latestPrerelease,omitempty"`
	Error            string       `json:"error,omitempty"`
}

// BranchInfo is a branch and the commit at its head on the host, empty when the
// host has no such branch.
type BranchInfo struct {
	Branch string `json:"branch"`
	Commit string `json:"commit,omitempty"`
}

// LocalInfo is the local clone and its state. Error is set instead of the
// state when the clone is missing, ambiguous or unreadable. It may be stale:
// it is what the clone had at its last fetch.
type LocalInfo struct {
	Path   string `json:"path,omitempty"`
	Source string `json:"source,omitempty"`
	Branch string `json:"branch,omitempty"`
	Commit string `json:"commit,omitempty"`
	// Dirty is set when a tracked file is changed or an operation is in
	// progress.
	Dirty bool   `json:"dirty,omitempty"`
	Error string `json:"error,omitempty"`
}

// Run reports on the Project named by slugs, or on the one whose repository
// contains the working directory cwd, and renders it as s.Output (text or
// json).
func Run(ctx context.Context, s clicfg.Settings, cwd string, slugs []string, opts Options) (string, error) {
	if s.Output != clicfg.OutputText && s.Output != clicfg.OutputJSON {
		return "", fmt.Errorf("output format %q is not supported by repo info: use text or json", s.Output)
	}
	ws := workspace.Open(s, cwd)
	i, err := gather(ctx, ws, func(dir string) (Clone, error) {
		r, err := ws.OpenAt(dir)
		if err != nil {
			return nil, err
		}
		return r, nil
	}, slugs, opts)
	if err != nil {
		return "", err
	}
	if s.Output == clicfg.OutputJSON {
		return renderJSON(i)
	}
	return renderText(i), nil
}

func gather(ctx context.Context, ws Workspace, open Opener, slugs []string, opts Options) (Info, error) {
	var slug string
	if len(slugs) > 0 {
		slug = slugs[0]
	}
	p, err := ws.Project(slug)
	if err != nil {
		return Info{}, err
	}
	i := Info{
		Slug:          p.Repo.Name,
		Owned:         p.Repo.Owned,
		DefaultBranch: p.Repo.DefaultBranch,
		Release: ReleaseInfo{
			Branches:           p.Release.Branches,
			TagPattern:         p.Release.TagPattern,
			RequiresPrerelease: p.Release.RequiresPrerelease,
		},
	}
	for _, l := range p.Release.Lines {
		branch, isDefault := p.Branch(l)
		i.Release.Lines = append(i.Release.Lines, LineInfo{
			Line: l.Line, Series: l.Series(), Branch: branch, IsDefaultBranch: isDefault,
		})
	}
	if !opts.Offline {
		i.Upstream = upstream(ctx, ws, p, &i.Release.Lines)
	}
	i.Local = local(ws, open, p.Repo.Name)
	return i, nil
}

// upstream reads the tags of the Project's repository from its host, and fills
// in the latest tags of each line. A failure is recorded and not returned, so
// the rest of the report is still shown.
func upstream(ctx context.Context, ws Workspace, p config.Project, lines *[]LineInfo) *UpstreamInfo {
	refs, err := ws.UpstreamRefs(ctx, p.Repo.Name)
	if err != nil {
		return &UpstreamInfo{Error: err.Error()}
	}
	tags := refs.Tags
	heads := map[string]string{}
	for _, b := range refs.Branches {
		heads[b.Name] = b.Commit
	}
	pattern := p.Release.TagPattern
	latest := git.LatestReleases(tags, pattern)
	for n := range *lines {
		l := &(*lines)[n]
		var inSeries []git.Tag
		for _, t := range tags {
			if git.InSeries(t.Name, l.Series) {
				inSeries = append(inSeries, t)
			}
		}
		perLine := git.LatestReleases(inSeries, pattern)
		l.LatestStable, l.LatestPrerelease = tagInfo(perLine.Stable), tagInfo(perLine.Prerelease)
		l.Head = heads[l.Branch]
	}
	u := &UpstreamInfo{LatestStable: tagInfo(latest.Stable), LatestPrerelease: tagInfo(latest.Prerelease)}
	for _, branch := range p.ReleaseBranches() {
		u.Heads = append(u.Heads, BranchInfo{Branch: branch, Commit: heads[branch]})
	}
	return u
}

func tagInfo(t *git.Tag) *TagInfo {
	if t == nil {
		return nil
	}
	return &TagInfo{Tag: t.Name, Commit: t.Commit}
}

// local reports on the clone of slug. Problems are recorded in Error rather
// than failing the command, so the rest is still shown.
func local(ws Workspace, open Opener, slug string) LocalInfo {
	rp, err := ws.RepoPath(slug)
	if err != nil {
		return LocalInfo{Error: err.Error()}
	}
	l := LocalInfo{Path: rp.Path, Source: rp.Source}
	c, err := open(rp.Path)
	if err != nil {
		l.Error = err.Error()
		return l
	}
	ref, short, err := c.Head()
	switch {
	case errors.Is(err, workspace.ErrDetachedHead):
		l.Branch = "(detached)"
	case err != nil:
		l.Error = err.Error()
	default:
		l.Branch, l.Commit = strings.TrimPrefix(ref, "refs/heads/"), short
	}
	return l
}

func renderText(i Info) string {
	var b strings.Builder
	section(&b, i.Slug, func(row func(label, value string)) {
		row("owned", yesNo(i.Owned))
		row("default branch", i.DefaultBranch)
		row("release branches", i.Release.Branches)
		row("tag pattern", i.Release.TagPattern)
		row("requires prerelease", yesNo(i.Release.RequiresPrerelease))
	})
	if len(i.Release.Lines) > 0 {
		renderLines(&b, i)
	}
	renderUpstream(&b, i)
	renderLocal(&b, i.Local)
	return b.String()
}

// section writes a heading and, under it, rows of aligned label and value.
func section(b *strings.Builder, heading string, rows func(row func(label, value string))) {
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString(heading + "\n")
	w := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	rows(func(label, value string) { fmt.Fprintf(w, "  %s\t%s\n", label, value) })
	_ = w.Flush() // writes to a strings.Builder, which cannot fail
}

func renderLines(b *strings.Builder, i Info) {
	b.WriteString("\nRelease lines\n")
	w := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	withTags := i.Upstream != nil && i.Upstream.Error == ""
	if withTags {
		fmt.Fprintln(w, "  LINE\tBRANCH\tHEAD\tLATEST STABLE\tLATEST PRE-RELEASE")
	} else {
		fmt.Fprintln(w, "  LINE\tBRANCH")
	}
	for _, l := range i.Release.Lines {
		branch := l.Branch
		if l.IsDefaultBranch {
			branch += " (default branch)"
		}
		if withTags {
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", l.Line, branch, headText(l.Head),
				tagText(l.LatestStable), tagText(l.LatestPrerelease))
		} else {
			fmt.Fprintf(w, "  %s\t%s\n", l.Line, branch)
		}
	}
	_ = w.Flush()
}

func renderUpstream(b *strings.Builder, i Info) {
	switch u := i.Upstream; {
	case u == nil:
		section(b, "Upstream", func(row func(label, value string)) {
			row("tags", "not read (--offline)")
		})
	case u.Error != "":
		section(b, "Upstream", func(row func(label, value string)) {
			row("tags", "unavailable: "+u.Error)
		})
	default:
		section(b, "Upstream", func(row func(label, value string)) {
			if i.Release.Branches != config.BranchesPerLine {
				// A project with a branch per line has its heads in the table.
				for _, h := range u.Heads {
					row("head of "+h.Branch, headText(h.Commit))
				}
			}
			row("latest stable", tagText(u.LatestStable))
			row("latest pre-release", tagText(u.LatestPrerelease))
		})
	}
}

func renderLocal(b *strings.Builder, l LocalInfo) {
	section(b, "Local clone", func(row func(label, value string)) {
		if l.Path != "" {
			row("path", fmt.Sprintf("%s (%s)", l.Path, l.Source))
		}
		if l.Branch != "" {
			row("branch", l.Branch)
		}
		if l.Commit != "" {
			row("commit", l.Commit)
		}
		if l.Error != "" {
			row("state", "unavailable: "+l.Error)
		}
	})
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

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func renderJSON(i Info) (string, error) {
	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
