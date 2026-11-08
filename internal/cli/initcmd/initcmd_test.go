package initcmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var full = clicfg.UserFileValues{GitHubUser: "alice", Fork: "origin", Upstream: "upstream", BranchPrefix: "alice/"}

// setup returns Options that write under a temporary XDG_CONFIG_HOME, with all
// four values given and no terminal, and the path of the user file.
func setup(t *testing.T) (Options, string) {
	t.Helper()
	xdg := t.TempDir()
	env := map[string]string{"XDG_CONFIG_HOME": xdg}
	return Options{
		GitHubUser: full.GitHubUser, Fork: full.Fork, Upstream: full.Upstream, BranchPrefix: full.BranchPrefix,
		In: strings.NewReader(""), Out: &bytes.Buffer{},
		Getenv: func(k string) string { return env[k] }, Home: t.TempDir(),
	}, filepath.Join(xdg, "gero", "config.yaml")
}

func TestRunWritesTheFile(t *testing.T) {
	o, path := setup(t)

	out, err := Run(o)

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, clicfg.RenderUserFile(full), string(got))
	assert.Contains(t, out, path)
}

func TestRunWritesWhereGeroConfigSays(t *testing.T) {
	o, _ := setup(t)
	path := filepath.Join(t.TempDir(), "elsewhere", "mine.yaml")
	o.Getenv = func(k string) string { return map[string]string{"GERO_CONFIG": path}[k] }

	_, err := Run(o)

	require.NoError(t, err)
	assert.FileExists(t, path)
}

func TestRunRefusesAnExistingFile(t *testing.T) {
	o, path := setup(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("output: json\n"), 0o644))

	_, err := Run(o)

	require.ErrorIs(t, err, ErrExists)
	assert.Contains(t, err.Error(), path)
	assert.Contains(t, err.Error(), "--force")
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "output: json\n", string(got))
}

func TestRunForceReplacesEvenAFileThatDoesNotLoad(t *testing.T) {
	o, path := setup(t)
	o.Force = true
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("nonsense: [\n"), 0o644))

	_, err := Run(o)

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, clicfg.RenderUserFile(full), string(got))
}

func TestRunDryRunShowsTheFileAndWritesNothing(t *testing.T) {
	o, path := setup(t)
	o.DryRun = true

	out, err := Run(o)

	require.NoError(t, err)
	assert.NoFileExists(t, path)
	assert.Contains(t, out, clicfg.RenderUserFile(full))
	assert.Contains(t, out, "Dry run")
}

func TestRunWithoutATerminalNeedsEveryValue(t *testing.T) {
	for name, clear := range map[string]func(*Options){
		"github user":   func(o *Options) { o.GitHubUser = "" },
		"fork":          func(o *Options) { o.Fork = "" },
		"upstream":      func(o *Options) { o.Upstream = "" },
		"branch prefix": func(o *Options) { o.BranchPrefix = "" },
	} {
		t.Run(name, func(t *testing.T) {
			o, path := setup(t)
			clear(&o)

			_, err := Run(o)

			require.ErrorIs(t, err, ErrNeedsValues)
			assert.NoFileExists(t, path)
		})
	}
}

func TestRunNamesTheFlagsThatAreMissing(t *testing.T) {
	o, _ := setup(t)
	o.GitHubUser, o.BranchPrefix = "", ""

	_, err := Run(o)

	require.ErrorIs(t, err, ErrNeedsValues)
	assert.Contains(t, err.Error(), "--github-user")
	assert.Contains(t, err.Error(), "--branch-prefix")
	assert.NotContains(t, err.Error(), "--fork")
}

func TestRunAsksForWhatIsMissingOnATerminal(t *testing.T) {
	o, path := setup(t)
	o.Interactive = true
	o.GitHubUser, o.Fork, o.Upstream, o.BranchPrefix = "", "", "", ""
	var asked bytes.Buffer
	o.Out = &asked
	// A login, then Enter for each default.
	o.In = strings.NewReader("alice\n\n\n\n")

	_, err := Run(o)

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, clicfg.RenderUserFile(clicfg.UserFileValues{
		GitHubUser: "alice", Fork: "origin", Upstream: "upstream", BranchPrefix: "alice/",
	}), string(got))
	assert.Contains(t, asked.String(), "[origin]")
	assert.Contains(t, asked.String(), "[upstream]")
	assert.Contains(t, asked.String(), "[alice/]")
}

func TestRunOnATerminalKeepsWhatFlagsGave(t *testing.T) {
	o, path := setup(t)
	o.Interactive = true
	o.BranchPrefix = ""
	o.In = strings.NewReader("mine/\n")

	_, err := Run(o)

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, clicfg.RenderUserFile(clicfg.UserFileValues{
		GitHubUser: "alice", Fork: "origin", Upstream: "upstream", BranchPrefix: "mine/",
	}), string(got))
}

func TestRunAnAnswerOfNothingForTheLoginIsAnError(t *testing.T) {
	o, path := setup(t)
	o.Interactive = true
	o.GitHubUser = ""
	o.In = strings.NewReader("\n")

	_, err := Run(o)

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrNeedsValues))
	assert.Contains(t, err.Error(), "GitHub")
	assert.NoFileExists(t, path)
}

func TestRunRefusesValuesThatBreakTheFile(t *testing.T) {
	for name, set := range map[string]func(*Options){
		"space in a remote": func(o *Options) { o.Fork = "my remote" },
		"newline":           func(o *Options) { o.GitHubUser = "a\nb" },
		"space in a login":  func(o *Options) { o.GitHubUser = "a b" },
	} {
		t.Run(name, func(t *testing.T) {
			o, path := setup(t)
			set(&o)

			_, err := Run(o)

			require.Error(t, err)
			assert.NoFileExists(t, path)
		})
	}
}

func TestRunInCIIsRefusedWithoutAnExplicitPath(t *testing.T) {
	o, path := setup(t)
	xdg := filepath.Dir(filepath.Dir(path))
	o.Getenv = func(k string) string { return map[string]string{"CI": "true", "XDG_CONFIG_HOME": xdg}[k] }

	_, err := Run(o)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "in CI")
	assert.NoFileExists(t, path)
}

func TestRunInCIWritesToAnExplicitPath(t *testing.T) {
	o, _ := setup(t)
	path := filepath.Join(t.TempDir(), "c.yaml")
	o.Getenv = func(k string) string { return map[string]string{"CI": "true", "GERO_CONFIG": path}[k] }

	_, err := Run(o)

	require.NoError(t, err)
	assert.FileExists(t, path)
}
