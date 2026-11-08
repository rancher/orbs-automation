// Package repoinfo holds the logic of gero repo info: the central-config entry
// of one repository, and where it is cloned on this machine with the state of
// that clone. It never reads the network (gero repo status does). It is
// framework-free: cmd/ owns the cobra wiring.
package repoinfo

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what Info needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Project returns the Project with the given config name or owner/name
	// slug, or the one in the working directory when the key is empty.
	Project(key string) (*chain.Project, error)
	RepoPath(slug string) (workspace.RepoPath, error)
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

// Info is everything the command reports about one repository.
type Info struct {
	Slug          string       `json:"slug"`
	Owned         bool         `json:"owned"`
	External      bool         `json:"external,omitempty"`
	ManagedBy     string       `json:"managedBy,omitempty"` // the team responsible; empty when an unowned project names none
	DefaultBranch string       `json:"defaultBranch"`
	Branches      BranchesInfo `json:"branches"`
	Streams       []StreamInfo `json:"streams,omitempty"`
	Local         LocalInfo    `json:"local"`
}

// BranchesInfo is how the Project's releases are laid out over branches.
type BranchesInfo struct {
	Layout string     `json:"layout"`
	Stages []string   `json:"stages,omitempty"`
	Lines  []LineInfo `json:"lines,omitempty"`
}

// LineInfo maps one Rancher line to the Project's branch.
type LineInfo struct {
	Line   string `json:"line"`
	Branch string `json:"branch,omitempty"`
	// IsDefaultBranch is set when the config names no branch, so Branch is the
	// Project's default branch.
	IsDefaultBranch bool `json:"isDefaultBranch,omitempty"`
	// Pending is set when the line has no branch yet.
	Pending bool `json:"pending,omitempty"`
	// Stages is the branch of each stage, for a line that has one for each.
	Stages map[string]string `json:"stages,omitempty"`
}

// StreamInfo is one release stream of the Project: what is released, which tags
// are releases, and the version series of each line.
type StreamInfo struct {
	Name               string           `json:"name"`
	Scheme             string           `json:"scheme"`
	TagPattern         string           `json:"tagPattern"`
	Universal          bool             `json:"universal,omitempty"`
	RequiresPrerelease bool             `json:"requiresPrerelease"`
	Lines              []StreamLineInfo `json:"lines,omitempty"`
}

// StreamLineInfo is the version series of a line in a stream.
type StreamLineInfo struct {
	Line   string `json:"line"`
	Series string `json:"series,omitempty"`
}

// LocalInfo is the local clone and its state. Error is set instead of the
// state when the clone is missing, ambiguous or unreadable. It is what the
// clone has now, which may not be what the host has.
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
func Run(s clicfg.Settings, cwd string, slugs []string) (string, error) {
	if s.Output != clicfg.OutputText && s.Output != clicfg.OutputJSON {
		return "", fmt.Errorf("output format %q is not supported by repo info: use text or json", s.Output)
	}
	ws := workspace.Open(s, cwd)
	i, err := gather(ws, func(dir string) (Clone, error) {
		r, err := ws.OpenAt(dir)
		if err != nil {
			return nil, err
		}
		return r, nil
	}, slugs)
	if err != nil {
		return "", err
	}
	if s.Output == clicfg.OutputJSON {
		return renderJSON(i)
	}
	return renderText(i, style.New(s.Terminal)), nil
}

func gather(ws Workspace, open Opener, slugs []string) (Info, error) {
	var slug string
	if len(slugs) > 0 {
		slug = slugs[0]
	}
	p, err := ws.Project(slug)
	if err != nil {
		return Info{}, err
	}
	i := Info{
		Slug:          p.Slug(),
		Owned:         p.Owned(),
		External:      p.External(),
		ManagedBy:     p.Steward(),
		DefaultBranch: p.DefaultBranch(),
		Branches:      BranchesInfo{Layout: p.Layout(), Stages: p.Stages()},
	}
	for _, lb := range p.LineBranches() {
		i.Branches.Lines = append(i.Branches.Lines, LineInfo{
			Line: lb.Line, Branch: lb.Branch, IsDefaultBranch: lb.IsDefault, Pending: lb.Pending, Stages: lb.Stages,
		})
	}
	for _, st := range p.Streams() {
		si := StreamInfo{
			Name: st.Name(), Scheme: st.Scheme(), TagPattern: st.TagPattern(),
			Universal: st.Universal(), RequiresPrerelease: st.RequiresPrerelease(),
		}
		for _, line := range st.Lines() {
			series, _ := st.Series(line)
			si.Lines = append(si.Lines, StreamLineInfo{Line: line, Series: series})
		}
		i.Streams = append(i.Streams, si)
	}
	i.Local = local(ws, open, p.Slug())
	return i, nil
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
	if l.Error == "" {
		clean, err := c.Clean()
		l.Dirty = !clean
		if err != nil {
			l.Dirty, l.Error = false, err.Error()
		}
	}
	return l
}

func renderText(i Info, view *style.Renderer) string {
	var b strings.Builder
	section(&b, view, i.Slug, func(row func(label, value string)) {
		row("owned", yesNo(i.Owned))
		if i.External {
			row("external", "yes")
		}
		row("managed by", orUnspecified(i.ManagedBy))
		row("default branch", i.DefaultBranch)
		row("branch layout", i.Branches.Layout)
		if len(i.Branches.Stages) > 0 {
			row("branch stages", strings.Join(i.Branches.Stages, ", "))
		}
	})
	if len(i.Branches.Lines) > 0 {
		renderBranches(&b, view, i)
	}
	for _, st := range i.Streams {
		renderStream(&b, view, st)
	}
	renderLocal(&b, view, i.Local)
	return b.String()
}

// section writes a heading and, under it, rows of aligned label and value. The
// labels are secondary text.
func section(b *strings.Builder, view *style.Renderer, heading string, rows func(row func(label, value string))) {
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString(view.Style(style.Heading, heading) + "\n")
	var table [][]string
	rows(func(label, value string) { table = append(table, []string{"  " + view.Style(style.Dim, label), value}) })
	b.WriteString(ui.Align(table, view.Width))
}

func renderBranches(b *strings.Builder, view *style.Renderer, i Info) {
	b.WriteString("\n" + view.Style(style.Heading, "Branches") + "\n")
	table := [][]string{{"  " + view.Style(style.Dim, "LINE"), view.Style(style.Dim, "BRANCH")}}
	for _, l := range i.Branches.Lines {
		table = append(table, []string{"  " + l.Line, branchText(l, i.Branches.Stages)})
	}
	b.WriteString(ui.Align(table, view.Width))
}

// branchText is the branch of a line as one cell: the branch, the default branch
// said so, a branch for each stage, or none yet.
func branchText(l LineInfo, stages []string) string {
	switch {
	case l.IsDefaultBranch:
		return l.Branch + " (default branch)"
	case l.Pending:
		return "(none yet)"
	case len(l.Stages) > 0:
		var parts []string
		for _, stage := range stages {
			if branch, ok := l.Stages[stage]; ok {
				parts = append(parts, stage+": "+branch)
			}
		}
		return strings.Join(parts, ", ")
	}
	return l.Branch
}

func renderStream(b *strings.Builder, view *style.Renderer, st StreamInfo) {
	section(b, view, "Stream "+st.Name, func(row func(label, value string)) {
		row("scheme", st.Scheme)
		row("tag pattern", st.TagPattern)
		row("requires prerelease", yesNo(st.RequiresPrerelease))
		if st.Universal {
			row("lines", "every line")
		}
	})
	if len(st.Lines) == 0 {
		return
	}
	table := [][]string{{"  " + view.Style(style.Dim, "LINE"), view.Style(style.Dim, "SERIES")}}
	for _, l := range st.Lines {
		table = append(table, []string{"  " + l.Line, l.Series})
	}
	b.WriteString(ui.Align(table, view.Width))
}

func renderLocal(b *strings.Builder, view *style.Renderer, l LocalInfo) {
	section(b, view, "Local clone", func(row func(label, value string)) {
		if l.Path != "" {
			row("path", fmt.Sprintf("%s (%s)", l.Path, l.Source))
		}
		if l.Branch != "" {
			row("branch", l.Branch)
		}
		if l.Commit != "" {
			row("commit", l.Commit)
		}
		if l.Branch != "" && l.Error == "" {
			if l.Dirty {
				row("work tree", view.Style(style.Warn, "changes or an operation in progress"))
			} else {
				row("work tree", view.Style(style.OK, "clean"))
			}
		}
		if l.Error != "" {
			row("state", view.Style(style.Error, "unavailable: "+l.Error))
		}
	})
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

// orUnspecified is how an unowned project that names no team is shown.
func orUnspecified(team string) string {
	if team == "" {
		return "unspecified"
	}
	return team
}
