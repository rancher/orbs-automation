package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var initFlags = []string{"--github-user", "alice", "--fork", "origin", "--upstream", "upstream", "--branch-prefix", "alice/"}

func TestInitWritesTheUserFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("GERO_CONFIG", path)

	stdout, _, err := execRoot(t, append([]string{"init"}, initFlags...)...)

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, clicfg.RenderUserFile(clicfg.UserFileValues{
		GitHubUser: "alice", Fork: "origin", Upstream: "upstream", BranchPrefix: "alice/",
	}), string(got))
	assert.Contains(t, stdout, path)
}

func TestInitRefusesAnExistingFileUnlessForced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("output: json\n"), 0o644))
	t.Setenv("GERO_CONFIG", path)

	_, _, err := execRoot(t, append([]string{"init"}, initFlags...)...)
	require.Error(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "output: json\n", string(got))

	_, _, err = execRoot(t, append([]string{"init", "--force"}, initFlags...)...)
	require.NoError(t, err)
	got, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(got), "githubUser: alice")
}

// A file the settings cannot load is the one init has to be able to replace.
func TestInitForceReplacesAFileThatDoesNotLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("pushTo: forks\n"), 0o644))
	t.Setenv("GERO_CONFIG", path)

	_, _, err := execRoot(t, append([]string{"init", "--force"}, initFlags...)...)

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(got), "githubUser: alice")
}

func TestInitWithoutATerminalSkipsWhatIsNotGiven(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("GERO_CONFIG", path)

	_, _, err := execRoot(t, "init", "--github-user", "alice")

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, clicfg.RenderUserFile(clicfg.UserFileValues{GitHubUser: "alice"}), string(got))
}

func TestInitTakesTheConfigDirSearchAndCloneRootFlags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("GERO_CONFIG", path)

	_, _, err := execRoot(t, "init", "--central-config-dir", "/cfg", "--search", "/a,/b", "--search", "/c", "--clone-root", "/new")

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, clicfg.RenderUserFile(clicfg.UserFileValues{
		CentralConfigDir: "/cfg", Search: []string{"/a", "/b", "/c"}, CloneRoot: "/new",
	}), string(got))
}

func TestInitDryRunWritesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("GERO_CONFIG", path)

	stdout, _, err := execRoot(t, append([]string{"init", "--dry-run"}, initFlags...)...)

	require.NoError(t, err)
	assert.NoFileExists(t, path)
	assert.Contains(t, stdout, "githubUser: alice")
}

func TestInitIsRefusedInTheREPL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("GERO_CONFIG", path)
	t.Setenv("GERO_REPL", "1")

	_, _, err := execRoot(t, append([]string{"init"}, initFlags...)...)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "REPL")
	assert.NoFileExists(t, path)
}
