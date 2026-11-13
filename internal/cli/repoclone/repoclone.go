// Package repoclone holds the logic of gero repo clone: putting the team's
// projects on a workstation, each under the clone root. It is framework-free:
// cmd/ owns the cobra wiring (docs/specs/CLI_FRAMEWORK.md).
package repoclone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Options is what the user asked for; the command fills it from its arguments
// and flags.
type Options struct {
	Projects []string // by config name or slug; none with All
	All      bool     // every Project the active team owns, and that is not external
	Into     string   // --into: the clone root, which beats the environment and the user file
	// Style colors the text report. Nil is plain, and so is a renderer built for a
	// terminal without color; the JSON report is never styled.
	Style *style.Renderer
}

// Outcome says what became of one Project.
const (
	Cloned  = "cloned"  // a new clone was made
	Skipped = "skipped" // there was a clone already
	Failed  = "failed"  // nothing was made, and Error says why
)

// Result is what became of one Project, as --json prints it.
type Result struct {
	Project string `json:"project"` // the repository, owner/name
	Outcome string `json:"outcome"` // Cloned, Skipped or Failed
	Path    string `json:"path,omitempty"`
	// Remotes are the remotes the clone has, in the order they were made: the
	// fork then the upstream, or the upstream alone.
	Remotes []string `json:"remotes,omitempty"`
	// Note says what the outcome does not, such as that the fork was not found and
	// the upstream alone was cloned.
	Note   string `json:"note,omitempty"`
	Error  string `json:"error,omitempty"`
	DryRun bool   `json:"dryRun,omitempty"`
}

// Workspace is what the command needs from the invocation's workspace. It is
// defined here, where it is consumed.
type Workspace interface {
	UpstreamFor(slug string) string
	ConfiguredRepoPath(slug string) (workspace.RepoPath, error)
}

// ProjectSource is where the Projects to clone come from: the central config,
// which the workspace reads from a local directory or, with none named, from the
// central bundle (docs/specs/CENTRAL_BUNDLE.md).
type ProjectSource interface {
	// Project is the Project named by its config name or slug.
	Project(key string) (*chain.Project, error)
	// Projects are all of them.
	Projects() ([]*chain.Project, error)
}

// Git is what the command needs of git. It is defined here, where it is consumed.
type Git interface {
	Clone(ctx context.Context, url, dir, remote string) error
	AddRemote(ctx context.Context, dir, name, url string) error
	RemoteURL(ctx context.Context, dir, name string) (string, error)
}

// Forks says whether the host has a fork.
type Forks interface {
	// ForkExists is whether user has a fork of the repository slug.
	ForkExists(ctx context.Context, user, slug string) (bool, error)
}

// Deps are what the command runs on, so a test can stand in for each.
type Deps struct {
	Workspace Workspace
	Source    ProjectSource
	Git       Git
	Forks     Forks
}

// Run clones what opts asks for and writes the report to w in the format the
// settings ask for. It returns an error after the report when any Project failed.
func Run(ctx context.Context, w io.Writer, s clicfg.Settings, cwd string, opts Options) error {
	ws := workspace.Open(s, cwd)
	opts.Style = style.New(s.Terminal)
	return run(ctx, w, Deps{Workspace: ws, Source: chainSource{ws}, Git: gitCLI{}, Forks: hostForks{}}, s, cwd, opts)
}

// target is where one selected Project goes, and what is already known of it.
type target struct {
	slug     string
	upstream string // the name of the upstream remote
	path     string // where the clone is, or goes
	found    bool   // a clone was found by the scan, so nothing is to be made
	err      error  // the Project cannot be cloned, and why
}

func run(ctx context.Context, w io.Writer, d Deps, s clicfg.Settings, cwd string, opts Options) error {
	if s.Output != clicfg.OutputText && s.Output != clicfg.OutputJSON {
		return fmt.Errorf("output format %q is not supported by repo clone: use text or json", s.Output)
	}
	selected, err := selectProjects(d.Source, s.Team, opts)
	if err != nil {
		return err
	}
	targets, err := plan(d.Workspace, s, cwd, opts.Into, selected)
	if err != nil {
		return err
	}
	results := make([]Result, len(targets))
	failed := 0
	for i, t := range targets {
		results[i] = cloneOne(ctx, d, s, t)
		if results[i].Outcome == Failed {
			failed++
		}
	}
	if err := report(w, s.Output, results, opts.Style); err != nil {
		return err
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d projects failed", failed, len(results))
	}
	return nil
}

// selectProjects is the Projects the user named, or with All the ones the team
// owns and that are not external, each once and in the order given.
func selectProjects(src ProjectSource, team string, opts Options) ([]*chain.Project, error) {
	switch {
	case opts.All && len(opts.Projects) > 0:
		return nil, errors.New("name projects or pass --all, not both")
	case !opts.All && len(opts.Projects) == 0:
		return nil, errors.New("name the projects to clone, or pass --all for every project your team owns")
	}
	var all []*chain.Project
	if opts.All {
		ps, err := src.Projects()
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			if p.Owned(team) && !p.External() {
				all = append(all, p)
			}
		}
		return all, nil
	}
	for _, key := range opts.Projects {
		p, err := src.Project(key)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(all, p) {
			all = append(all, p)
		}
	}
	return all, nil
}

// plan finds where each Project goes: where the user's config says, where a clone
// already is, else under the clone root. It fails, before anything is cloned, when
// a Project has no upstream remote name, or when one needs the root and none is set.
func plan(ws Workspace, s clicfg.Settings, cwd, into string, projects []*chain.Project) ([]target, error) {
	root := s.Repos.CloneRoot
	if into != "" {
		root = into
		if !filepath.IsAbs(root) {
			root = filepath.Join(cwd, root)
		}
	}
	out := make([]target, 0, len(projects))
	for _, p := range projects {
		slug := p.Slug()
		upstream := ws.UpstreamFor(slug)
		if upstream == "" {
			return nil, fmt.Errorf("no upstream remote is set for %s: set upstream in the user file, or repos.overrides.%s.upstream", slug, slug)
		}
		t := target{slug: slug, upstream: upstream}
		rp, err := ws.ConfiguredRepoPath(slug)
		switch {
		case err == nil:
			t.path, t.found = rp.Path, rp.Source == workspace.PathScan
		case errors.Is(err, workspace.ErrRepoNotFound):
			if root == "" {
				return nil, errors.New("no clone root is set for " + slug + ": pass --into, set GERO_REPOS_CLONE_ROOT, or set repos.cloneRoot in the user file")
			}
			t.path = filepath.Join(root, repoName(slug))
		default:
			t.err = err
			if len(rp.Candidates) > 0 {
				t.err = fmt.Errorf("%w: %s", err, strings.Join(rp.Candidates, ", "))
			}
		}
		out = append(out, t)
	}
	return out, nil
}

// repoName is the name of a repository, the directory its clone goes in.
func repoName(slug string) string {
	_, name, _ := strings.Cut(slug, "/")
	return name
}

// sshURL is where the repository slug is cloned from.
// TODO(host): github.com only; the host of a GitHub Enterprise Server is not read.
func sshURL(slug string) string { return "git@github.com:" + slug + ".git" }

// cloneOne makes the clone of one Project, or says why it is not made.
func cloneOne(ctx context.Context, d Deps, s clicfg.Settings, t target) Result {
	res := Result{Project: t.slug, Path: t.path, DryRun: s.DryRun}
	fail := func(err error) Result {
		res.Outcome, res.Error, res.DryRun = Failed, err.Error(), false
		return res
	}
	if t.err != nil {
		return fail(t.err)
	}
	if t.found {
		res.Outcome, res.DryRun = Skipped, false
		return res
	}
	present, err := inspect(ctx, d.Git, s, t)
	if err != nil {
		return fail(err)
	}
	if present {
		res.Outcome, res.DryRun = Skipped, false
		return res
	}
	forkRemote, forkURL, note, err := fork(ctx, d.Forks, s, t.slug)
	if err != nil {
		return fail(err)
	}
	res.Note = note
	res.Outcome = Cloned
	if forkURL == "" {
		res.Remotes = []string{t.upstream}
	} else {
		res.Remotes = []string{forkRemote, t.upstream}
	}
	if s.DryRun {
		return res
	}
	if forkURL == "" {
		if err := d.Git.Clone(ctx, sshURL(t.slug), t.path, t.upstream); err != nil {
			return fail(err)
		}
		return res
	}
	if err := d.Git.Clone(ctx, forkURL, t.path, forkRemote); err != nil {
		return fail(err)
	}
	if err := d.Git.AddRemote(ctx, t.path, t.upstream, sshURL(t.slug)); err != nil {
		return fail(fmt.Errorf("the fork is cloned into %s, but adding the %s remote failed: %w", t.path, t.upstream, err))
	}
	return res
}

// fork says whether to clone the user's fork: it is wanted when the user file has
// a fork remote and a githubUser, and it is cloned when the host has it. forkURL
// is "" for the upstream alone, and note says why when a fork was wanted and is
// not there.
func fork(ctx context.Context, forks Forks, s clicfg.Settings, slug string) (remote, forkURL, note string, err error) {
	if s.Fork == "" || s.GitHubUser == "" {
		return "", "", "", nil
	}
	ok, err := forks.ForkExists(ctx, s.GitHubUser, slug)
	if err != nil {
		return "", "", "", err
	}
	if !ok {
		verb := "cloned"
		if s.DryRun {
			verb = "would clone"
		}
		return "", "", fmt.Sprintf("%s has no fork of %s: %s the upstream alone", s.GitHubUser, slug, verb), nil
	}
	return s.Fork, sshURL(s.GitHubUser + "/" + repoName(slug)), "", nil
}

// inspect looks at where the clone goes. It is true when a clone of the Project is
// already there, false when the place is free (it is not there, or is empty), and
// an error when something else is.
func inspect(ctx context.Context, g Git, s clicfg.Settings, t target) (bool, error) {
	entries, err := os.ReadDir(t.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("%s: %w", t.path, err)
	case len(entries) == 0:
		return false, nil
	}
	forkSlug := s.GitHubUser + "/" + repoName(t.slug)
	var other string
	for _, remote := range []string{t.upstream, s.Fork} {
		if remote == "" {
			continue
		}
		url, err := g.RemoteURL(ctx, t.path, remote)
		if err != nil {
			return false, err
		}
		if url == "" {
			continue
		}
		got, err := git.SlugFromURL(url)
		if err != nil {
			got = url
		}
		if strings.EqualFold(got, t.slug) || (s.GitHubUser != "" && strings.EqualFold(got, forkSlug)) {
			return true, nil
		}
		if other == "" {
			other = got
		}
	}
	if other != "" {
		return false, fmt.Errorf("%s holds a clone of %s, not of %s: clone elsewhere with --into, or set repos.paths.%s", t.path, other, t.slug, t.slug)
	}
	return false, fmt.Errorf("%s is not empty and is not a clone of %s: leave it, or clone elsewhere with --into", t.path, t.slug)
}

// report writes the results in the format asked for.
func report(w io.Writer, format string, results []Result, view *style.Renderer) error {
	if format == clicfg.OutputJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Repos []Result `json:"repos"`
		}{results})
	}
	var b strings.Builder
	for _, r := range results {
		b.WriteString(render(r, view))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// render is one result in text. What was made is green, what only would be and
// what was already there is secondary text, and a failure is an error.
func render(r Result, view *style.Renderer) string {
	remotes := " (" + strings.Join(r.Remotes, ", ") + ")"
	var line string
	switch {
	case r.Outcome == Failed:
		return fmt.Sprintf("%s %s: %s\n", view.Style(style.Error, "Failed"), r.Project, r.Error)
	case r.Outcome == Skipped:
		return view.Style(style.Dim, fmt.Sprintf("Skipped %s: already cloned at %s", r.Project, r.Path)) + "\n"
	case r.DryRun:
		line = fmt.Sprintf("%s %s into %s%s\n", view.Style(style.Dim, "Would clone"), r.Project, r.Path, remotes)
	default:
		line = fmt.Sprintf("%s %s into %s%s\n", view.Style(style.OK, "Cloned"), r.Project, r.Path, remotes)
	}
	if r.Note != "" {
		line += "  " + view.Style(style.Warn, r.Note) + "\n"
	}
	return line
}
