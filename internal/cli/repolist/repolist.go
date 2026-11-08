// Package repolist holds the logic of gero repo list: which Projects of the
// central config have a clone on this machine, as three lists: cloned, not cloned
// and ambiguous. It reports what is on disk. gero config resolve repos reports how
// the config resolves each repository, and from which source. It is
// framework-free: cmd/ owns the cobra wiring.
package repolist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Clone is a Project that has a clone, and where it is.
type Clone struct {
	Slug string `json:"slug"`
	Path string `json:"path"`
}

// Ambiguous is a Project that has several equally good clones.
type Ambiguous struct {
	Slug       string   `json:"slug"`
	Candidates []string `json:"candidates"`
}

// Report is the three lists, each sorted by slug.
type Report struct {
	Cloned    []Clone     `json:"cloned"`
	NotCloned []string    `json:"notCloned"`
	Ambiguous []Ambiguous `json:"ambiguous"`
}

// Workspace is what the command needs from the invocation's workspace. It is
// defined here, where it is consumed, so tests can use a fake.
type Workspace interface {
	Chain() (*chain.Graph, error)
	RepoPaths(slugs []string) []workspace.RepoPath
}

// List finds which Projects of the central config found from s and the working
// directory cwd have a clone, and renders the lists as s.Output (text or json).
func List(s clicfg.Settings, cwd string) (string, error) {
	return list(workspace.Open(s, cwd), isDir, s.Output, style.New(s.Terminal))
}

// isDir is whether path is a directory that exists: a repos.paths entry is taken
// as the user wrote it, so it can name a directory that is not there.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func list(ws Workspace, isDir func(string) bool, format string, view *style.Renderer) (string, error) {
	if format != clicfg.OutputText && format != clicfg.OutputJSON {
		return "", fmt.Errorf("output format %q is not supported by repo list: use text or json", format)
	}
	g, err := ws.Chain()
	if err != nil {
		return "", err
	}
	external := map[string]bool{}
	slugs := make([]string, 0, len(g.Projects()))
	expected := 0
	for _, p := range g.Projects() {
		slugs = append(slugs, p.Slug())
		external[p.Slug()] = p.External()
		if !p.External() {
			expected++
		}
	}

	report := Report{Cloned: []Clone{}, NotCloned: []string{}, Ambiguous: []Ambiguous{}}
	clonedExpected := 0
	for _, r := range ws.RepoPaths(slugs) {
		switch {
		case errors.Is(r.Err, workspace.ErrRepoAmbiguous):
			report.Ambiguous = append(report.Ambiguous, Ambiguous{Slug: r.Slug, Candidates: r.Candidates})
		case r.Err != nil || !isDir(r.Path):
			// A Project outside the organization is not expected to be cloned.
			if !external[r.Slug] {
				report.NotCloned = append(report.NotCloned, r.Slug)
			}
		default:
			report.Cloned = append(report.Cloned, Clone{Slug: r.Slug, Path: r.Path})
			if !external[r.Slug] {
				clonedExpected++
			}
		}
	}
	slices.SortFunc(report.Cloned, func(a, b Clone) int { return compareSlugs(a.Slug, b.Slug) })
	slices.SortFunc(report.Ambiguous, func(a, b Ambiguous) int { return compareSlugs(a.Slug, b.Slug) })
	slices.SortFunc(report.NotCloned, compareSlugs)

	if format == clicfg.OutputJSON {
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", err
		}
		return string(b) + "\n", nil
	}
	if len(slugs) == 0 {
		return "No projects in the central config.\n", nil
	}
	return renderText(report, clonedExpected, expected, view), nil
}

func compareSlugs(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }

// renderText is the three lists, the empty ones left out, then how many of the
// projects that are expected to be cloned are.
func renderText(r Report, cloned, expected int, view *style.Renderer) string {
	var sections []string
	if len(r.Cloned) > 0 {
		rows := make([][]string, len(r.Cloned))
		for i, c := range r.Cloned {
			rows[i] = []string{"  " + c.Slug, c.Path}
		}
		sections = append(sections, view.Style(style.Heading, fmt.Sprintf("Cloned (%d)", len(r.Cloned)))+"\n"+ui.Align(rows, view.Width))
	}
	if len(r.NotCloned) > 0 {
		var b strings.Builder
		b.WriteString(view.Style(style.Heading, fmt.Sprintf("Not cloned (%d)", len(r.NotCloned))) + "\n")
		for _, slug := range r.NotCloned {
			b.WriteString("  " + slug + "\n")
		}
		sections = append(sections, b.String())
	}
	if len(r.Ambiguous) > 0 {
		rows := make([][]string, len(r.Ambiguous))
		for i, a := range r.Ambiguous {
			rows[i] = []string{"  " + a.Slug, strings.Join(a.Candidates, ", ")}
		}
		sections = append(sections, view.Style(style.Warn, fmt.Sprintf("Ambiguous (%d)", len(r.Ambiguous)))+"\n"+ui.Align(rows, view.Width))
	}
	sections = append(sections, view.Style(style.Dim, fmt.Sprintf("%d of %d projects cloned", cloned, expected))+"\n")
	return strings.Join(sections, "\n")
}
