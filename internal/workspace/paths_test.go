package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/git/repo/repotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	rancherSlug = "rancher/rancher"
	upstreamURL = "https://github.com/rancher/rancher.git"
	forkURL     = "git@github.com:danpock/rancher.git"
)

// userFile renders a user file with the given repos.search and repos.paths.
func userFile(search []string, paths map[string]string) string {
	var b strings.Builder
	b.WriteString("repos:\n")
	if len(search) > 0 {
		b.WriteString("  search:\n")
		for _, s := range search {
			fmt.Fprintf(&b, "    - %s\n", s)
		}
	}
	if len(paths) > 0 {
		b.WriteString("  paths:\n")
		for slug, p := range paths {
			fmt.Fprintf(&b, "    %s: %s\n", slug, p)
		}
	}
	return b.String()
}

// clones creates a repository with the given remotes (name to URL) at each
// relative directory below root.
func clones(t *testing.T, root string, layout map[string]map[string]string) {
	t.Helper()
	for rel, remotes := range layout {
		repotest.NewRepoAt(t, filepath.Join(root, rel), remotes)
	}
}

// workspaceFor opens a Workspace at cwd with the given repos.search and
// repos.paths settings.
func workspaceFor(t *testing.T, cwd string, search []string, paths map[string]string) *Workspace {
	t.Helper()
	return Open(settings(t, userFile(search, paths), nil, nil), cwd)
}

func TestRepoPathsCwd(t *testing.T) {
	t.Run("at the repository root", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{"origin": forkURL, "upstream": upstreamURL})

		got := workspaceFor(t, dir, nil, nil).RepoPaths([]string{rancherSlug})

		require.Len(t, got, 1)
		assert.Equal(t, RepoPath{Slug: rancherSlug, Path: dir, Source: PathCwd}, got[0])
	})

	t.Run("from a subdirectory, the path is the repository root", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{"origin": upstreamURL})

		got := workspaceFor(t, subdir(t, dir), nil, nil).RepoPaths([]string{rancherSlug})

		assert.Equal(t, RepoPath{Slug: rancherSlug, Path: dir, Source: PathCwd}, got[0])
	})

	t.Run("a repository for another slug is not the match", func(t *testing.T) {
		dir := t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{"origin": "https://github.com/rancher/scc-operator.git"})

		got := workspaceFor(t, dir, nil, nil).RepoPaths([]string{rancherSlug})

		assert.ErrorIs(t, got[0].Err, ErrRepoNotFound)
	})

	t.Run("beats repos.paths and the scan", func(t *testing.T) {
		dir, search := t.TempDir(), t.TempDir()
		repotest.NewRepoAt(t, dir, map[string]string{"origin": upstreamURL})
		clones(t, search, map[string]map[string]string{"rancher": {"origin": upstreamURL}})

		got := workspaceFor(t, dir, []string{search}, map[string]string{rancherSlug: "/elsewhere"}).RepoPaths([]string{rancherSlug})

		assert.Equal(t, PathCwd, got[0].Source)
		assert.Equal(t, dir, got[0].Path)
	})

	t.Run("outside any repository", func(t *testing.T) {
		got := workspaceFor(t, t.TempDir(), nil, nil).RepoPaths([]string{rancherSlug})

		assert.ErrorIs(t, got[0].Err, ErrRepoNotFound)
	})
}

func TestRepoPathsSettings(t *testing.T) {
	t.Run("a repos.paths entry", func(t *testing.T) {
		got := workspaceFor(t, t.TempDir(), nil, map[string]string{rancherSlug: "/src/rancher-src"}).RepoPaths([]string{rancherSlug})

		assert.Equal(t, RepoPath{Slug: rancherSlug, Path: "/src/rancher-src", Source: PathSettings}, got[0])
	})

	t.Run("beats the scan", func(t *testing.T) {
		search := t.TempDir()
		clones(t, search, map[string]map[string]string{"rancher": {"origin": upstreamURL}})

		got := workspaceFor(t, t.TempDir(), []string{search}, map[string]string{rancherSlug: "/src/rancher-src"}).RepoPaths([]string{rancherSlug})

		assert.Equal(t, PathSettings, got[0].Source)
		assert.Equal(t, "/src/rancher-src", got[0].Path)
	})

	t.Run("only for its own slug", func(t *testing.T) {
		got := workspaceFor(t, t.TempDir(), nil, map[string]string{"rancher/scc-operator": "/src/scc"}).RepoPaths([]string{rancherSlug})

		assert.ErrorIs(t, got[0].Err, ErrRepoNotFound)
	})
}

func TestRepoPathsScan(t *testing.T) {
	forkOnly := map[string]string{"origin": forkURL}
	strong := map[string]string{"origin": forkURL, "upstream": upstreamURL}

	tests := []struct {
		name       string
		layout     map[string]map[string]string
		want       string   // relative path of the clone, when found
		candidates []string // relative paths, when ambiguous
		wantErr    error
	}{
		{"matched by remote, not by directory name",
			map[string]map[string]string{"rancher-src": strong}, "rancher-src", nil, nil},
		{"upstream remote url",
			map[string]map[string]string{"x": {"origin": upstreamURL}}, "x", nil, nil},
		{"found at depth 2",
			map[string]map[string]string{"suse/rancher": strong}, "suse/rancher", nil, nil},
		{"not found at depth 3",
			map[string]map[string]string{"a/b/c": strong}, "", nil, ErrRepoNotFound},
		{"does not look inside a clone",
			map[string]map[string]string{"outer": {"origin": "https://github.com/x/y.git"}, "outer/vendor": strong},
			"", nil, ErrRepoNotFound},
		{"a fork alone is a weak match",
			map[string]map[string]string{"my-fork": forkOnly}, "my-fork", nil, nil},
		{"a strong match beats a weak one",
			map[string]map[string]string{"my-fork": forkOnly, "rancher": strong}, "rancher", nil, nil},
		{"two strong matches are ambiguous",
			map[string]map[string]string{"a": strong, "b": {"upstream": upstreamURL}}, "", []string{"a", "b"}, ErrRepoAmbiguous},
		{"two weak matches are ambiguous",
			map[string]map[string]string{"a": forkOnly, "b": {"origin": "https://github.com/other/rancher"}}, "", []string{"a", "b"}, ErrRepoAmbiguous},
		{"slugs compare case-insensitively",
			map[string]map[string]string{"r": {"origin": "https://github.com/Rancher/Rancher.git"}}, "r", nil, nil},
		{"another repo name is no match, even from the same owner",
			map[string]map[string]string{"s": {"origin": "https://github.com/rancher/scc-operator.git"}}, "", nil, ErrRepoNotFound},
		{"a remote that is a local path is ignored",
			map[string]map[string]string{"r": {"backup": "/mnt/backup/rancher/rancher"}}, "", nil, ErrRepoNotFound},
		{"a clone without remotes is ignored",
			map[string]map[string]string{"r": nil}, "", nil, ErrRepoNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			search := t.TempDir()
			clones(t, search, tt.layout)

			got := workspaceFor(t, t.TempDir(), []string{search}, nil).RepoPaths([]string{rancherSlug})[0]

			assert.ErrorIs(t, got.Err, tt.wantErr)
			if tt.wantErr != nil {
				assert.Empty(t, got.Path)
				assert.Empty(t, got.Source)
			} else {
				assert.Equal(t, filepath.Join(search, tt.want), got.Path)
				assert.Equal(t, PathScan, got.Source)
			}
			var wantCandidates []string
			for _, c := range tt.candidates {
				wantCandidates = append(wantCandidates, filepath.Join(search, c))
			}
			assert.Equal(t, wantCandidates, got.Candidates)
		})
	}

	t.Run("directories that are not clones are ignored", func(t *testing.T) {
		search := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(search, "docs", "notes"), 0o750))

		got := workspaceFor(t, t.TempDir(), []string{search}, nil).RepoPaths([]string{rancherSlug})

		assert.ErrorIs(t, got[0].Err, ErrRepoNotFound)
	})

	t.Run("a missing search directory is skipped", func(t *testing.T) {
		search := t.TempDir()
		clones(t, search, map[string]map[string]string{"rancher": strong})

		got := workspaceFor(t, t.TempDir(), []string{filepath.Join(search, "missing"), search}, nil).RepoPaths([]string{rancherSlug})

		require.NoError(t, got[0].Err)
		assert.Equal(t, filepath.Join(search, "rancher"), got[0].Path)
	})

	t.Run("matches across several search directories", func(t *testing.T) {
		first, second := t.TempDir(), t.TempDir()
		clones(t, second, map[string]map[string]string{"rancher": strong})

		got := workspaceFor(t, t.TempDir(), []string{first, second}, nil).RepoPaths([]string{rancherSlug})

		require.NoError(t, got[0].Err)
		assert.Equal(t, filepath.Join(second, "rancher"), got[0].Path)
	})

	t.Run("the same clone reached twice is not ambiguous", func(t *testing.T) {
		search := t.TempDir()
		clones(t, search, map[string]map[string]string{"suse/rancher": strong})

		got := workspaceFor(t, t.TempDir(), []string{search, filepath.Join(search, "suse")}, nil).RepoPaths([]string{rancherSlug})

		require.NoError(t, got[0].Err)
		assert.Equal(t, filepath.Join(search, "suse", "rancher"), got[0].Path)
	})

	t.Run("beats the sibling", func(t *testing.T) {
		parent, search := t.TempDir(), t.TempDir()
		repotest.NewRepoAt(t, filepath.Join(parent, "orbs-automation"), map[string]string{"origin": "https://github.com/rancher/orbs-automation.git"})
		require.NoError(t, os.MkdirAll(filepath.Join(parent, "rancher"), 0o750))
		clones(t, search, map[string]map[string]string{"rancher-src": strong})

		got := workspaceFor(t, filepath.Join(parent, "orbs-automation"), []string{search}, nil).RepoPaths([]string{rancherSlug})

		assert.Equal(t, PathScan, got[0].Source)
	})
}

func TestRepoPathsSibling(t *testing.T) {
	other := map[string]string{"origin": "https://github.com/rancher/orbs-automation.git"}

	t.Run("a directory next to the working directory's repository", func(t *testing.T) {
		parent := t.TempDir()
		repotest.NewRepoAt(t, filepath.Join(parent, "orbs-automation"), other)
		require.NoError(t, os.MkdirAll(filepath.Join(parent, "scc-operator"), 0o750))

		got := workspaceFor(t, filepath.Join(parent, "orbs-automation"), nil, nil).RepoPaths([]string{"rancher/scc-operator"})

		assert.Equal(t, RepoPath{Slug: "rancher/scc-operator", Path: filepath.Join(parent, "scc-operator"), Source: PathSibling}, got[0])
	})

	t.Run("from a subdirectory of the repository", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "orbs-automation")
		repotest.NewRepoAt(t, root, other)
		require.NoError(t, os.MkdirAll(filepath.Join(parent, "scc-operator"), 0o750))

		got := workspaceFor(t, subdir(t, root), nil, nil).RepoPaths([]string{"rancher/scc-operator"})

		assert.Equal(t, filepath.Join(parent, "scc-operator"), got[0].Path)
	})

	t.Run("no such directory", func(t *testing.T) {
		parent := t.TempDir()
		repotest.NewRepoAt(t, filepath.Join(parent, "orbs-automation"), other)

		got := workspaceFor(t, filepath.Join(parent, "orbs-automation"), nil, nil).RepoPaths([]string{"rancher/scc-operator"})

		assert.ErrorIs(t, got[0].Err, ErrRepoNotFound)
	})

	t.Run("not used outside a repository", func(t *testing.T) {
		parent := t.TempDir()
		cwd := filepath.Join(parent, "work")
		require.NoError(t, os.MkdirAll(cwd, 0o750))
		require.NoError(t, os.MkdirAll(filepath.Join(parent, "scc-operator"), 0o750))

		got := workspaceFor(t, cwd, nil, nil).RepoPaths([]string{"rancher/scc-operator"})

		assert.ErrorIs(t, got[0].Err, ErrRepoNotFound)
	})
}

func TestRepoPathsRows(t *testing.T) {
	search := t.TempDir()
	clones(t, search, map[string]map[string]string{
		"rancher":   {"upstream": upstreamURL},
		"op-a":      {"origin": "https://github.com/rancher/scc-operator.git"},
		"op-b":      {"origin": "https://github.com/rancher/scc-operator.git"},
		"unrelated": {"origin": "https://github.com/x/y.git"},
	})
	ws := workspaceFor(t, t.TempDir(), []string{search}, map[string]string{"rancher/charts": "/src/charts"})

	got := ws.RepoPaths([]string{"rancher/scc-operator", "rancher/missing", rancherSlug, "rancher/charts"})

	require.Len(t, got, 4)
	assert.Equal(t, "rancher/scc-operator", got[0].Slug)
	assert.ErrorIs(t, got[0].Err, ErrRepoAmbiguous)
	assert.Equal(t, []string{filepath.Join(search, "op-a"), filepath.Join(search, "op-b")}, got[0].Candidates)
	assert.Equal(t, "rancher/missing", got[1].Slug)
	assert.ErrorIs(t, got[1].Err, ErrRepoNotFound)
	assert.Equal(t, RepoPath{Slug: rancherSlug, Path: filepath.Join(search, "rancher"), Source: PathScan}, got[2])
	assert.Equal(t, RepoPath{Slug: "rancher/charts", Path: "/src/charts", Source: PathSettings}, got[3])
}

func TestRepoPathsNoSlugs(t *testing.T) {
	assert.Empty(t, workspaceFor(t, t.TempDir(), nil, nil).RepoPaths(nil))
}

func TestRepoPath(t *testing.T) {
	search := t.TempDir()
	clones(t, search, map[string]map[string]string{"rancher": {"upstream": upstreamURL}})
	ws := workspaceFor(t, t.TempDir(), []string{search}, nil)

	t.Run("found", func(t *testing.T) {
		got, err := ws.RepoPath(rancherSlug)

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(search, "rancher"), got.Path)
	})

	t.Run("not found returns the error and the row", func(t *testing.T) {
		got, err := ws.RepoPath("rancher/missing")

		assert.ErrorIs(t, err, ErrRepoNotFound)
		assert.Equal(t, "rancher/missing", got.Slug)
	})
}
