package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type shown map[string]struct{ Value, Source string }

// configShow runs `gero config show --json` with a user file, env and extra
// flags, and decodes the result. GERO_* and the user file are isolated per test.
func configShow(t *testing.T, file string, env map[string]string, flags ...string) shown {
	t.Helper()
	for _, k := range []string{"GERO_CONFIG", "GERO_CENTRAL_CONFIG_DIR", "GERO_UPSTREAM", "GERO_OUTPUT", "CI", "GITHUB_ACTIONS"} {
		t.Setenv(k, "")
	}
	if file != "" {
		p := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(p, []byte(file), 0o600))
		t.Setenv("GERO_CONFIG", p)
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	args := append([]string{"config", "show", "--json"}, flags...)
	stdout, _, err := execRoot(t, args...)
	require.NoError(t, err)
	var got shown
	require.NoError(t, json.Unmarshal([]byte(stdout), &got), stdout)
	return got
}

func TestConfigShowUserFile(t *testing.T) {
	got := configShow(t, "upstream: rancher\nfork: origin\nbranchPrefix: dan/\n", nil)

	assert.Equal(t, "rancher", got["upstream"].Value)
	assert.Equal(t, "file", got["upstream"].Source)
	assert.Equal(t, "origin", got["fork"].Value)
	assert.Equal(t, "dan/", got["branchPrefix"].Value)
}

func TestConfigShowPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		env        map[string]string
		flags      []string
		wantValue  string
		wantSource string
	}{
		{"file only", "upstream: f\n", nil, nil, "f", "file"},
		{"env beats file", "upstream: f\n", map[string]string{"GERO_UPSTREAM": "e"}, nil, "e", "env"},
		{"flag beats env", "upstream: f\n", map[string]string{"GERO_UPSTREAM": "e"}, []string{"--upstream", "x"}, "x", "flag"},
		{"flag beats file", "upstream: f\n", nil, []string{"--upstream=x"}, "x", "flag"},
		{"nothing set", "", nil, nil, "", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := configShow(t, tt.file, tt.env, tt.flags...)

			assert.Equal(t, tt.wantValue, got["upstream"].Value)
			assert.Equal(t, tt.wantSource, got["upstream"].Source)
		})
	}
}

func TestConfigShowGlobalFlags(t *testing.T) {
	got := configShow(t, "", nil, "--central-config-dir", "/cfg")

	assert.Equal(t, "/cfg", got["centralConfigDir"].Value)
	assert.Equal(t, "flag", got["centralConfigDir"].Source)
	// --json stands in for --output json.
	assert.Equal(t, "json", got["output"].Value)
	assert.Equal(t, "flag", got["output"].Source)
}

func TestConfigShowTextByDefault(t *testing.T) {
	t.Setenv("GERO_CONFIG", "")
	stdout, _, err := execRoot(t, "config", "show")

	require.NoError(t, err)
	assert.Regexp(t, `(?m)^output\s+text\s+default$`, stdout)
}

func TestConfigShowBadUserFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte("pushTo: forks\n"), 0o600))
	t.Setenv("GERO_CONFIG", p)

	stdout, _, err := execRoot(t, "config", "show")

	require.Error(t, err)
	assert.Empty(t, stdout)
}

// inCentralRepo changes into a temp directory that looks like a checkout of
// the central repo (config/ with a file of gero's) and returns its config directory.
func inCentralRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config", "rancher.yaml"), []byte("apiVersion: gero/v1alpha2\nkind: Project\nname: rancher\nspec:\n  repo: {name: rancher/rancher}\n"), 0o600))
	t.Chdir(dir)
	wd, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Join(wd, "config")
}

func TestConfigShowConfigDirFromWorkingDirectory(t *testing.T) {
	want := inCentralRepo(t)

	got := configShow(t, "", nil)

	assert.Equal(t, want, got["centralConfigDir"].Value)
	assert.Equal(t, "cwd", got["centralConfigDir"].Source)
}

func TestConfigShowConfigDirExplicitBeatsWorkingDirectory(t *testing.T) {
	inCentralRepo(t)

	t.Run("flag", func(t *testing.T) {
		got := configShow(t, "", nil, "--central-config-dir", "/flag")

		assert.Equal(t, "/flag", got["centralConfigDir"].Value)
		assert.Equal(t, "flag", got["centralConfigDir"].Source)
	})
	t.Run("env", func(t *testing.T) {
		got := configShow(t, "", map[string]string{"GERO_CENTRAL_CONFIG_DIR": "/env"})

		assert.Equal(t, "/env", got["centralConfigDir"].Value)
		assert.Equal(t, "env", got["centralConfigDir"].Source)
	})
	t.Run("user file", func(t *testing.T) {
		got := configShow(t, "centralConfigDir: /file\n", nil)

		assert.Equal(t, "/file", got["centralConfigDir"].Value)
		assert.Equal(t, "file", got["centralConfigDir"].Source)
	})
}

func TestConfigShowConfigDirUnsetOutsideCentralRepo(t *testing.T) {
	t.Chdir(t.TempDir())

	got := configShow(t, "", nil)

	assert.Empty(t, got["centralConfigDir"].Value)
	assert.Equal(t, "default", got["centralConfigDir"].Source)
}

// isolateGero clears the settings a developer's environment could leak in.
func isolateGero(t *testing.T) {
	t.Helper()
	for _, k := range []string{"GERO_CONFIG", "GERO_CENTRAL_CONFIG_DIR", "GERO_UPSTREAM", "GERO_OUTPUT", "CI", "GITHUB_ACTIONS"} {
		t.Setenv(k, "")
	}
}

func TestConfigValidateAcceptsTheCheckedInConfig(t *testing.T) {
	isolateGero(t)
	dir, err := filepath.Abs("../config")
	require.NoError(t, err)

	stdout, _, err := execRoot(t, "config", "validate", "--central-config-dir", dir)

	require.NoError(t, err)
	assert.Contains(t, stdout, "config OK")
	assert.Contains(t, stdout, dir)
}

func TestConfigValidateReportsProblems(t *testing.T) {
	isolateGero(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(`apiVersion: gero/v1alpha2
kind: Project
name: a
spec:
  repo: {name: not-a-slug}
`), 0o600))

	_, _, err := execRoot(t, "config", "validate", "--central-config-dir", dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "repo.name")
}

func TestConfigValidateWithoutConfigDirectory(t *testing.T) {
	isolateGero(t)
	t.Chdir(t.TempDir())

	_, _, err := execRoot(t, "config", "validate")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--central-config-dir")
}

// centralWith writes one Project per slug into a new config directory and
// returns it.
func centralWith(t *testing.T, slugs ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, slug := range slugs {
		_, name, _ := strings.Cut(slug, "/")
		require.NoError(t, os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(`apiVersion: gero/v1alpha2
kind: Project
name: `+name+`
spec:
  repo: {name: `+slug+`}
`), 0o600))
	}
	return dir
}

// withRancherPath points GERO_CONFIG at a new user file that maps
// rancher/rancher to /src/rancher in repos.paths.
func withRancherPath(t *testing.T) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte("repos:\n  paths:\n    rancher/rancher: /src/rancher\n"), 0o600))
	t.Setenv("GERO_CONFIG", p)
}

// Wiring only: the logic is tested in internal/cli/configresolve and
// internal/workspace.
func TestConfigResolveReposWiring(t *testing.T) {
	t.Run("a repos.paths entry resolves", func(t *testing.T) {
		isolateGero(t)
		t.Chdir(t.TempDir())
		central := centralWith(t, "rancher/rancher")
		withRancherPath(t)

		stdout, _, err := execRoot(t, "config", "resolve", "repos", "--central-config-dir", central)

		require.NoError(t, err)
		assert.Regexp(t, `(?m)^rancher/rancher\s+/src/rancher\s+repos\.paths$`, stdout)
	})

	t.Run("an unresolved repo prints the table and fails", func(t *testing.T) {
		isolateGero(t)
		t.Chdir(t.TempDir())
		central := centralWith(t, "rancher/rancher", "rancher/charts")
		withRancherPath(t)

		stdout, _, err := execRoot(t, "config", "resolve", "repos", "--central-config-dir", central)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no unique local clone")
		assert.Contains(t, stdout, "/src/rancher")
		assert.Contains(t, stdout, "rancher/charts")
		assert.Contains(t, stdout, "not found")
	})

	t.Run("json", func(t *testing.T) {
		isolateGero(t)
		t.Chdir(t.TempDir())
		central := centralWith(t, "rancher/rancher")
		withRancherPath(t)

		stdout, _, err := execRoot(t, "config", "resolve", "repos", "--json", "--central-config-dir", central)

		require.NoError(t, err)
		var got []struct{ Slug, Path, Source string }
		require.NoError(t, json.Unmarshal([]byte(stdout), &got), stdout)
		assert.Equal(t, []struct{ Slug, Path, Source string }{{"rancher/rancher", "/src/rancher", "repos.paths"}}, got)
	})

	t.Run("one slug", func(t *testing.T) {
		isolateGero(t)
		t.Chdir(t.TempDir())
		central := centralWith(t, "rancher/rancher", "rancher/charts")
		withRancherPath(t)

		stdout, _, err := execRoot(t, "config", "resolve", "repos", "rancher/rancher", "--central-config-dir", central)

		require.NoError(t, err)
		assert.Contains(t, stdout, "/src/rancher")
		assert.NotContains(t, stdout, "charts")
	})

	t.Run("at most one slug", func(t *testing.T) {
		isolateGero(t)

		_, _, err := execRoot(t, "config", "resolve", "repos", "a/b", "c/d")

		assert.Error(t, err)
	})

	t.Run("without a config directory", func(t *testing.T) {
		isolateGero(t)
		t.Chdir(t.TempDir())

		_, _, err := execRoot(t, "config", "resolve", "repos")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "--central-config-dir")
	})
}
