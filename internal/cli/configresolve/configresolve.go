// Package configresolve holds the logic of gero config resolve repos: it lists
// where each Project of the central config lives on this machine and how that
// was decided. It is framework-free: cmd/ owns the cobra wiring.
package configresolve

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrUnresolved is returned, with the table still written, when a Project has
// no local clone or several equally good ones.
var ErrUnresolved = errors.New("some repositories have no unique local clone")

// Workspace is what Repos needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	Config() (config.Config, error)
	RepoPaths(slugs []string) []workspace.RepoPath
}

// Repos resolves the local clone of each Project in the central config found
// from s and the working directory cwd, or of the one Project named by slugs,
// and renders the rows as s.Output (text or json). When any row is unresolved
// the output is still returned, with ErrUnresolved.
func Repos(s clicfg.Settings, cwd string, slugs []string) (string, error) {
	return repos(workspace.Open(s, cwd), s.Output, slugs)
}

func repos(ws Workspace, format string, slugs []string) (string, error) {
	if format != clicfg.OutputText && format != clicfg.OutputJSON {
		return "", fmt.Errorf("output format %q is not supported by config resolve repos: use text or json", format)
	}
	cfg, err := ws.Config()
	if err != nil {
		return "", err
	}
	asked, err := projectSlugs(cfg, slugs)
	if err != nil {
		return "", err
	}
	rows := ws.RepoPaths(asked)
	var out string
	if format == clicfg.OutputJSON {
		out, err = renderJSON(rows)
	} else {
		out = renderText(rows)
	}
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if r.Err != nil {
			return out, ErrUnresolved
		}
	}
	return out, nil
}

// projectSlugs returns the slug of every Project in cfg, or just the one named
// by want, spelled as the config spells it. Externals have no rows.
func projectSlugs(cfg config.Config, want []string) ([]string, error) {
	var all []string
	for _, p := range cfg.Projects {
		all = append(all, p.Repo.Name)
	}
	if len(want) == 0 {
		return all, nil
	}
	var out []string
	for _, w := range want {
		i := indexFold(all, w)
		if i < 0 {
			return nil, fmt.Errorf("%q is not a Project in the central config", w)
		}
		out = append(out, all[i])
	}
	return out, nil
}

func indexFold(list []string, s string) int {
	for i, v := range list {
		if strings.EqualFold(v, s) {
			return i
		}
	}
	return -1
}

func renderText(rows []workspace.RepoPath) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SLUG\tPATH\tSOURCE")
	for _, r := range rows {
		path, source := r.Path, r.Source
		switch {
		case errors.Is(r.Err, workspace.ErrRepoAmbiguous):
			path = fmt.Sprintf("(ambiguous: %s; set repos.paths.%s to choose one)", strings.Join(r.Candidates, ", "), r.Slug)
		case r.Err != nil:
			path = fmt.Sprintf("(not found: clone it under a repos.search directory or set repos.paths.%s)", r.Slug)
		}
		if source == "" {
			source = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.Slug, path, source)
	}
	_ = w.Flush() // writes to a strings.Builder, which cannot fail
	return b.String()
}

func renderJSON(rows []workspace.RepoPath) (string, error) {
	type row struct {
		Slug       string   `json:"slug"`
		Path       string   `json:"path,omitempty"`
		Source     string   `json:"source,omitempty"`
		Candidates []string `json:"candidates,omitempty"`
		Error      string   `json:"error,omitempty"`
	}
	out := make([]row, len(rows))
	for i, r := range rows {
		out[i] = row{Slug: r.Slug, Path: r.Path, Source: r.Source, Candidates: r.Candidates}
		if r.Err != nil {
			out[i].Error = r.Err.Error()
		}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
