package repl

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTree is a command tree with a few commands, as cobra would answer: only the
// words that start with what is typed.
func fakeTree(asked *[][]string) func([]string, string) []string {
	return func(words []string, toComplete string) []string {
		if asked != nil {
			*asked = append(*asked, append(append([]string{}, words...), "|"+toComplete))
		}
		var all []string
		switch {
		case len(words) == 0:
			all = []string{"repo", "ship", "cd", "shell", "doctor"}
		case words[0] == "repo" && len(words) == 1:
			all = []string{"status", "info", "tag", "branch"}
		case words[0] == "repo" && words[1] == "status":
			all = []string{"--json", "--dry-run"}
		}
		var out []string
		for _, w := range all {
			if len(w) >= len(toComplete) && w[:len(toComplete)] == toComplete {
				out = append(out, w)
			}
		}
		return out
	}
}

func someProjects() []string { return []string{"scc-operator", "shell", "rancher"} }

func complete(t *testing.T, dir, line string) []string {
	t.Helper()
	c := Completion{Tree: fakeTree(nil), Projects: someProjects, Dir: func() string { return dir }}
	return c.Complete(line)
}

func TestTheFirstWordIsACommandOfTheTreeOrOneOnlyTheREPLHas(t *testing.T) {
	got := complete(t, t.TempDir(), "")

	assert.ElementsMatch(t, []string{"repo", "ship", "cd", "shell", "doctor", "pwd", "ls", "clear", "exit"}, got, "each once, shell too")
}

func TestTheFirstWordIsFilteredByWhatIsTyped(t *testing.T) {
	assert.ElementsMatch(t, []string{"shell", "ship"}, complete(t, t.TempDir(), "sh"))
	assert.ElementsMatch(t, []string{"pwd"}, complete(t, t.TempDir(), "p"))
	assert.ElementsMatch(t, []string{"exit"}, complete(t, t.TempDir(), "ex"))
	assert.Empty(t, complete(t, t.TempDir(), "zz"))
}

func TestLaterWordsAreTheTreesAndTheTreeIsToldWhatCameBefore(t *testing.T) {
	var asked [][]string
	c := Completion{Tree: fakeTree(&asked), Dir: func() string { return t.TempDir() }}

	assert.ElementsMatch(t, []string{"status", "info", "tag", "branch"}, c.Complete("repo "))
	assert.ElementsMatch(t, []string{"status"}, c.Complete("repo st"))
	assert.ElementsMatch(t, []string{"--json", "--dry-run"}, c.Complete("repo status --"))

	assert.Equal(t, [][]string{{"repo", "|"}, {"repo", "|st"}, {"repo", "status", "|--"}}, asked)
}

func TestCdCompletesProjectsAndNothingElse(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "docs"), 0o755))

	assert.ElementsMatch(t, []string{"scc-operator", "shell", "rancher"}, complete(t, dir, "cd "))
	assert.ElementsMatch(t, []string{"scc-operator", "shell"}, complete(t, dir, "cd s"))
	assert.Empty(t, complete(t, dir, "cd docs/"), "cd takes no paths")
	assert.Empty(t, complete(t, dir, "cd scc-operator "), "one project")
}

func TestLsCompletesDirectoriesAndProjectsWrittenWithAnAt(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs", "specs"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "test"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), nil, 0o600))

	got := complete(t, dir, "ls ")

	assert.ElementsMatch(t, []string{"docs/", "test/", "@scc-operator", "@shell", "@rancher"}, got, "directories, no files, and projects with the @")
	assert.ElementsMatch(t, []string{"docs/"}, complete(t, dir, "ls d"))
	assert.ElementsMatch(t, []string{"@scc-operator", "@shell"}, complete(t, dir, "ls @s"))
	assert.ElementsMatch(t, []string{"@scc-operator", "@shell", "@rancher"}, complete(t, dir, "ls @"))
}

func TestLsCompletesTheDirectoriesBelowOne(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs", "specs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs", "ai"), 0o755))

	assert.ElementsMatch(t, []string{"docs/specs/", "docs/ai/"}, complete(t, dir, "ls docs/"))
	assert.ElementsMatch(t, []string{"docs/specs/"}, complete(t, dir, "ls docs/sp"))
}

func TestLsNeverCompletesOutsideTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0o755))

	assert.Empty(t, complete(t, dir, "ls ../"))
	assert.Empty(t, complete(t, dir, "ls docs/../.."))
	assert.Empty(t, complete(t, dir, "ls /"))
}

func TestTheCommandsWithNothingToCompleteSayNothing(t *testing.T) {
	for _, line := range []string{"pwd ", "exit ", "shell ", "pwd x"} {
		assert.Empty(t, complete(t, t.TempDir(), line), line)
	}
}

func TestMissingPartsOfACompletionAreNotAProblem(t *testing.T) {
	var none Completion

	assert.ElementsMatch(t, []string{"pwd", "ls", "clear", "exit"}, none.Complete(""))
	assert.Empty(t, none.Complete("repo "))
	assert.Empty(t, none.Complete("cd "))
	assert.Empty(t, none.Complete("ls "))
}

func TestACompletionIsSortedSoTheListIsStable(t *testing.T) {
	got := complete(t, t.TempDir(), "")

	require.NotEmpty(t, got)
	assert.IsIncreasing(t, got)
}

type configOnly struct {
	cfg config.Config
	err error
}

func (c configOnly) Config() (config.Config, error) { return c.cfg, c.err }

func TestProjectNamesAreTheConfigNamesSorted(t *testing.T) {
	ws := configOnly{cfg: config.Config{Projects: []config.Project{
		{Name: "shell", Repo: config.Repo{Name: "rancher/shell"}},
		{Name: "rancher", Repo: config.Repo{Name: "rancher/rancher"}},
		{Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator"}},
	}}}

	got := projectNames(ws)

	assert.Equal(t, []string{"rancher", "scc-operator", "shell"}, got)
}

func TestProjectNamesWithoutAConfigAreNone(t *testing.T) {
	assert.Empty(t, projectNames(configOnly{err: errors.New("no config directory")}))
	assert.Empty(t, projectNames(configOnly{}))
}

type inDir struct {
	project config.Project
	err     error
	asked   []string
}

func (w *inDir) Project(key string) (config.Project, error) {
	w.asked = append(w.asked, key)
	return w.project, w.err
}

func TestTheProjectOfADirectoryIsTheNameOfTheOneItIsIn(t *testing.T) {
	ws := &inDir{project: config.Project{Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator"}}}

	assert.Equal(t, "scc-operator", projectNameOf(ws))
	assert.Equal(t, []string{"."}, ws.asked, "the one whose repository the directory is in")
}

func TestTheProjectOfADirectoryInNoneIsNothingAndSoIsAnyProblem(t *testing.T) {
	for _, err := range []error{workspace.ErrNotProject, workspace.ErrNotFound, errors.New("no git here")} {
		assert.Empty(t, projectNameOf(&inDir{err: err}), "%v", err)
	}
}
