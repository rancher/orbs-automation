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

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Workspace is what Info needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Project returns the Project with the given config name or owner/name
	// slug, or the one in the working directory when the key is empty.
	Project(key string) (config.Project, error)
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
	Slug          string      `json:"slug"`
	Owned         bool        `json:"owned"`
	DefaultBranch string      `json:"defaultBranch"`
	Release       ReleaseInfo `json:"release"`
	Local         LocalInfo   `json:"local"`
}

// ReleaseInfo is the Project's release policy with defaults applied.
type ReleaseInfo struct {
	Branching          string     `json:"branching"`
	TagPattern         string     `json:"tagPattern"`
	RequiresPrerelease bool       `json:"requiresPrerelease"`
	Lines              []LineInfo `json:"lines,omitempty"`
}

// LineInfo maps one Rancher line to the Project's series and branch.
type LineInfo struct {
	Line   string `json:"line"`
	Series string `json:"series"`
	Branch string `json:"branch"`
	// IsDefaultBranch is set when the config names no branch, so Branch is the
	// Project's default branch.
	IsDefaultBranch bool `json:"isDefaultBranch,omitempty"`
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
		Slug:          p.Repo.Name,
		Owned:         p.Repo.Owned,
		DefaultBranch: p.Repo.DefaultBranch,
		Release: ReleaseInfo{
			Branching:          p.Release.Branching,
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
	i.Local = local(ws, open, p.Repo.Name)
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
		row("default branch", i.DefaultBranch)
		row("release branching", i.Release.Branching)
		row("tag pattern", i.Release.TagPattern)
		row("requires prerelease", yesNo(i.Release.RequiresPrerelease))
	})
	if len(i.Release.Lines) > 0 {
		renderLines(&b, view, i)
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

func renderLines(b *strings.Builder, view *style.Renderer, i Info) {
	b.WriteString("\n" + view.Style(style.Heading, "Release lines") + "\n")
	table := [][]string{{"  " + view.Style(style.Dim, "LINE"), view.Style(style.Dim, "SERIES"), view.Style(style.Dim, "BRANCH")}}
	for _, l := range i.Release.Lines {
		branch := l.Branch
		if l.IsDefaultBranch {
			branch += " (default branch)"
		}
		table = append(table, []string{"  " + l.Line, l.Series, branch})
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
