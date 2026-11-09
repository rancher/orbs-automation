package initcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var full = clicfg.UserFileValues{
	GitHubUser: "alice", Fork: "origin", Upstream: "upstream", BranchPrefix: "alice/",
	CentralConfigDir: "~/orbs/config", Search: []string{"~/src", "/work"}, CloneRoot: "~/new",
}

// setup returns Options that write under a temporary XDG_CONFIG_HOME, with all
// four values given and no terminal, and the path of the user file.
func setup(t *testing.T) (Options, string) {
	t.Helper()
	xdg := t.TempDir()
	env := map[string]string{"XDG_CONFIG_HOME": xdg}
	return Options{
		GitHubUser: full.GitHubUser, Fork: full.Fork, Upstream: full.Upstream, BranchPrefix: full.BranchPrefix,
		CentralConfigDir: full.CentralConfigDir, Search: full.Search, CloneRoot: full.CloneRoot,
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

func TestRunWithoutATerminalSkipsWhatIsNotGiven(t *testing.T) {
	for _, tc := range []struct {
		name  string
		clear func(*Options, *clicfg.UserFileValues)
	}{
		{"github user", func(o *Options, v *clicfg.UserFileValues) { o.GitHubUser, v.GitHubUser = "", "" }},
		{"fork", func(o *Options, v *clicfg.UserFileValues) { o.Fork, v.Fork = "", "" }},
		{"upstream", func(o *Options, v *clicfg.UserFileValues) { o.Upstream, v.Upstream = "", "" }},
		{"branch prefix", func(o *Options, v *clicfg.UserFileValues) { o.BranchPrefix, v.BranchPrefix = "", "" }},
		{"config dir", func(o *Options, v *clicfg.UserFileValues) { o.CentralConfigDir, v.CentralConfigDir = "", "" }},
		{"search", func(o *Options, v *clicfg.UserFileValues) { o.Search, v.Search = nil, nil }},
		{"clone root", func(o *Options, v *clicfg.UserFileValues) { o.CloneRoot, v.CloneRoot = "", "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, path := setup(t)
			want := full
			tc.clear(&o, &want)

			_, err := Run(o)

			require.NoError(t, err)
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, clicfg.RenderUserFile(want), string(got))
		})
	}
}

func TestRunWithNothingGivenWritesTheSchemaCommentOnly(t *testing.T) {
	o, path := setup(t)
	o.GitHubUser, o.Fork, o.Upstream, o.BranchPrefix, o.CentralConfigDir, o.Search, o.CloneRoot = "", "", "", "", "", nil, ""

	_, err := Run(o)

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, clicfg.UserFileSchemaComment+"\n", string(got))
}

func blank(o *Options) {
	o.GitHubUser, o.Fork, o.Upstream, o.BranchPrefix, o.CentralConfigDir, o.Search, o.CloneRoot = "", "", "", "", "", nil, ""
}

func TestRunAsksEveryQuestionOnATerminal(t *testing.T) {
	o, path := setup(t)
	blank(&o)
	o.Interactive = true
	var asked bytes.Buffer
	o.Out = &asked
	// In the order asked: login, fork, upstream, branch prefix, config dir, search, clone root.
	o.In = strings.NewReader("alice\norigin\nupstream\nalice/\n~/orbs/config\n~/src, /work\n~/new\n")

	_, err := Run(o)

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, clicfg.RenderUserFile(full), string(got))
	assert.Contains(t, asked.String(), "Enter to skip")
}

func TestRunAnEmptyAnswerSkipsTheQuestion(t *testing.T) {
	o, path := setup(t)
	blank(&o)
	o.Interactive = true
	// A login, then Enter on the rest but the clone root.
	o.In = strings.NewReader("alice\n\n\n\n\n\n~/new\n")

	_, err := Run(o)

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, clicfg.RenderUserFile(clicfg.UserFileValues{GitHubUser: "alice", CloneRoot: "~/new"}), string(got))
}

func TestRunSkipsEverythingWhenTheInputEnds(t *testing.T) {
	o, path := setup(t)
	blank(&o)
	o.Interactive = true
	o.In = strings.NewReader("")

	_, err := Run(o)

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, clicfg.UserFileSchemaComment+"\n", string(got))
}

func TestRunOnATerminalAsksOnlyWhatFlagsLeftOut(t *testing.T) {
	o, path := setup(t)
	o.Interactive = true
	o.BranchPrefix = ""
	var asked bytes.Buffer
	o.Out = &asked
	o.In = strings.NewReader("mine/\n")

	_, err := Run(o)

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	want := full
	want.BranchPrefix = "mine/"
	assert.Equal(t, clicfg.RenderUserFile(want), string(got))
	assert.NotContains(t, asked.String(), "GitHub login")
}

func TestRunSearchAnswerIsCommaSeparatedAndBlanksAreDropped(t *testing.T) {
	o, path := setup(t)
	blank(&o)
	o.Interactive = true
	o.In = strings.NewReader("\n\n\n\n\n ~/a ,, /b ,\n\n")

	_, err := Run(o)

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, clicfg.RenderUserFile(clicfg.UserFileValues{Search: []string{"~/a", "/b"}}), string(got))
}

// The two path questions look alike, so the prompt says what each is for: search
// finds clones that exist, the clone root is where new ones go.
func TestRunExplainsHowSearchDiffersFromTheCloneRoot(t *testing.T) {
	o, _ := setup(t)
	blank(&o)
	o.Interactive = true
	var asked bytes.Buffer
	o.Out = &asked
	o.In = strings.NewReader("")

	_, err := Run(o)

	require.NoError(t, err)
	out := asked.String()
	assert.Contains(t, out, "repos.search")
	assert.Contains(t, out, "repos.cloneRoot")
	assert.Contains(t, out, "already have")
	assert.Contains(t, out, "new clones")
	assert.Contains(t, out, "scanned as well")
	assert.Less(t, strings.Index(out, "already have"), strings.Index(out, "Directories to scan"), "the note comes before the first of the two questions")
}

func TestRunDoesNotExplainWhenNeitherPathIsAsked(t *testing.T) {
	o, _ := setup(t)
	o.Interactive = true
	o.GitHubUser = ""
	var asked bytes.Buffer
	o.Out = &asked
	o.In = strings.NewReader("alice\n")

	_, err := Run(o)

	require.NoError(t, err)
	assert.NotContains(t, asked.String(), "already have")
}

func TestRunRefusesPathsThatCannotBeRead(t *testing.T) {
	for name, set := range map[string]func(*Options){
		"relative config dir": func(o *Options) { o.CentralConfigDir = "rel/cfg" },
		"relative search":     func(o *Options) { o.Search = []string{"/ok", "rel"} },
		"relative clone root": func(o *Options) { o.CloneRoot = "rel" },
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

func TestRunNamesTheKeyOfAPathThatCannotBeRead(t *testing.T) {
	o, _ := setup(t)
	o.CloneRoot = "rel"

	_, err := Run(o)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "repos.cloneRoot")
}
