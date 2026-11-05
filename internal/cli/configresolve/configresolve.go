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

// The advice for each kind of unresolved row, shown under the table and by
// gero doctor.
const (
	HintNotFound  = "clone it under a repos.search directory, or set repos.paths.<slug>"
	HintAmbiguous = "set repos.paths.<slug> to choose one"
)

// Workspace is what Repos needs from the invocation's workspace. It is defined
// here, where it is consumed, so tests can use a fake.
type Workspace interface {
	Config() (config.Config, error)
	// Project returns the Project with the given config name or owner/name
	// slug, or the one in the working directory for "" or ".".
	Project(key string) (config.Project, error)
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
	asked, err := projectSlugs(ws, cfg, slugs)
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

// projectSlugs returns the slug of every Project in cfg, or just the ones named
// by want, each a config name, a slug or "." for the working directory's
// Project, spelled as the config spells the slug. Externals have no rows.
func projectSlugs(ws Workspace, cfg config.Config, want []string) ([]string, error) {
	if len(want) == 0 {
		all := make([]string, len(cfg.Projects))
		for i, p := range cfg.Projects {
			all[i] = p.Repo.Name
		}
		return all, nil
	}
	out := make([]string, 0, len(want))
	for _, key := range want {
		p, err := ws.Project(key)
		if err != nil {
			return nil, err
		}
		out = append(out, p.Repo.Name)
	}
	return out, nil
}

func renderText(rows []workspace.RepoPath) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SLUG\tPATH\tSOURCE")
	var notFound, ambiguous bool
	for _, r := range rows {
		path, source := r.Path, r.Source
		switch {
		case errors.Is(r.Err, workspace.ErrRepoAmbiguous):
			path = fmt.Sprintf("(ambiguous: %s)", strings.Join(r.Candidates, ", "))
			ambiguous = true
		case r.Err != nil:
			path = "(not found)"
			notFound = true
		}
		if source == "" {
			source = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.Slug, path, source)
	}
	_ = w.Flush() // writes to a strings.Builder, which cannot fail
	if notFound || ambiguous {
		b.WriteString("\n")
	}
	if notFound {
		b.WriteString("not found: " + HintNotFound + "\n")
	}
	if ambiguous {
		b.WriteString("ambiguous: " + HintAmbiguous + "\n")
	}
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
