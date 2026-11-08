package clicfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderUserFile(t *testing.T) {
	got := RenderUserFile(UserFileValues{
		GitHubUser: "alice", Fork: "origin", Upstream: "upstream", BranchPrefix: "alice/",
		ConfigDir: "~/orbs/config", Search: []string{"~/src", "/work"}, CloneRoot: "~/src/new",
	})

	want := UserFileSchemaComment + "\n" +
		"githubUser: alice\n" +
		"fork: origin\n" +
		"upstream: upstream\n" +
		"branchPrefix: alice/\n" +
		"configDir: ~/orbs/config\n" +
		"repos:\n" +
		"  search:\n" +
		"    - ~/src\n" +
		"    - /work\n" +
		"  cloneRoot: ~/src/new\n"
	assert.Equal(t, want, got)
}

func TestRenderUserFileLeavesOutWhatIsEmpty(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    UserFileValues
		want string
	}{
		{"nothing", UserFileValues{}, UserFileSchemaComment + "\n"},
		{"one key", UserFileValues{Fork: "origin"}, UserFileSchemaComment + "\nfork: origin\n"},
		{"only a clone root", UserFileValues{CloneRoot: "/c"}, UserFileSchemaComment + "\nrepos:\n  cloneRoot: /c\n"},
		{"only search", UserFileValues{Search: []string{"/s"}}, UserFileSchemaComment + "\nrepos:\n  search:\n    - /s\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, RenderUserFile(tc.v))
		})
	}
}

func TestRenderUserFileQuotesPathsOnlyWhenNeeded(t *testing.T) {
	got := RenderUserFile(UserFileValues{ConfigDir: "~/my cfg", CloneRoot: "$HOME/src", Search: []string{"/a: b"}})

	assert.Contains(t, got, `configDir: "~/my cfg"`+"\n")
	assert.Contains(t, got, "  cloneRoot: $HOME/src\n")
	assert.Contains(t, got, `    - "/a: b"`+"\n")
}

func TestRenderUserFileQuotesWhatYAMLWouldRead(t *testing.T) {
	for _, tc := range []struct{ name, prefix, line string }{
		{"colon", "a: b", `branchPrefix: "a: b"`},
		{"hash", "a #b", `branchPrefix: "a #b"`},
		{"leading star", "*x", `branchPrefix: "*x"`},
		{"boolean word", "true", `branchPrefix: "true"`},
		{"number", "42", `branchPrefix: "42"`},
		{"quote", `a"b`, `branchPrefix: "a\"b"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderUserFile(UserFileValues{GitHubUser: "u", Fork: "f", Upstream: "up", BranchPrefix: tc.prefix})

			assert.Contains(t, got, tc.line+"\n")
		})
	}
}

// What gero init writes is what the next command reads, and strictly.
func TestRenderedUserFileLoads(t *testing.T) {
	in := UserFileValues{
		GitHubUser: "alice", Fork: "origin", Upstream: "upstream", BranchPrefix: "a: b/",
		ConfigDir: "~/my cfg", Search: []string{"~/src", "/w x"}, CloneRoot: "~/new",
	}

	s, err := Load(Inputs{
		Getenv:       func(string) string { return "" },
		UserFile:     strings.NewReader(RenderUserFile(in)),
		UserFileName: "config.yaml",
		Home:         "/home/alice",
	})

	require.NoError(t, err)
	assert.Empty(t, s.Warnings)
	assert.Equal(t, "alice", s.GitHubUser)
	assert.Equal(t, "origin", s.Fork)
	assert.Equal(t, "upstream", s.Upstream)
	assert.Equal(t, "a: b/", s.BranchPrefix)
	assert.Equal(t, "/home/alice/my cfg", s.ConfigDir)
	assert.Equal(t, []string{"/home/alice/src", "/w x"}, s.Repos.Search)
	assert.Equal(t, "/home/alice/new", s.Repos.CloneRoot)
}

// The comment names a schema that is in the repository, so the URL does not go
// stale when the file moves.
func TestUserFileSchemaCommentNamesACommittedSchema(t *testing.T) {
	const prefix = "# yaml-language-server: $schema=https://raw.githubusercontent.com/rancher/orbs-automation/main/"
	require.True(t, strings.HasPrefix(UserFileSchemaComment, prefix), UserFileSchemaComment)

	rel := strings.TrimPrefix(UserFileSchemaComment, prefix)

	_, err := os.Stat(filepath.Join("..", "..", "..", filepath.FromSlash(rel)))
	assert.NoError(t, err, "the schema the comment points at")
}
