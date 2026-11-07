package repl

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplit(t *testing.T) {
	rows := []struct {
		line string
		want []string
	}{
		{"", []string{}},
		{"   ", []string{}},
		{"repo status", []string{"repo", "status"}},
		{"  repo \t status  ", []string{"repo", "status"}},
		{`pr create --title "a b"`, []string{"pr", "create", "--title", "a b"}},
		{`say 'it is "fine"'`, []string{"say", `it is "fine"`}},
		{`a"b c"d`, []string{"ab cd"}},
	}
	require.NotEmpty(t, rows)
	for _, r := range rows {
		got, err := Split(r.line)

		require.NoError(t, err, r.line)
		assert.Equal(t, r.want, got, r.line)
	}
}

func TestSplitUnterminatedQuote(t *testing.T) {
	_, err := Split(`pr create --title "oops`)

	assert.Error(t, err)
}

func TestPromptIsPlain(t *testing.T) {
	assert.Equal(t, "gero> ", Prompt())
}

// tree makes dir/{docs/,test/integration/,README.md,a.txt}.
func tree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs", "specs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "test", "integration"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), nil, 0o600))
	return dir
}

func exec(t *testing.T, s *Session, line string) (string, bool, error) {
	t.Helper()
	var out bytes.Buffer
	exit, err := s.Exec(context.Background(), &out, line)
	return out.String(), exit, err
}

func TestPwd(t *testing.T) {
	s := &Session{Dir: "/work/here"}

	out, exit, err := exec(t, s, "pwd")

	require.NoError(t, err)
	assert.False(t, exit)
	assert.Equal(t, "/work/here\n", out)
}

func TestExit(t *testing.T) {
	s := &Session{Dir: "/"}

	_, exit, err := exec(t, s, "exit")

	require.NoError(t, err)
	assert.True(t, exit)
}

func TestLsListsTheWorkingDirectorySortedWithSlashOnDirectories(t *testing.T) {
	s := &Session{Dir: tree(t)}

	out, _, err := exec(t, s, "ls")

	require.NoError(t, err)
	assert.Equal(t, "README.md\na.txt\ndocs/\ntest/\n", out)
}

func TestLsTargetsUnderTheWorkingDirectory(t *testing.T) {
	s := &Session{Dir: tree(t)}
	for _, target := range []string{"docs", "./docs", "docs/", "./docs/../docs"} {
		out, _, err := exec(t, s, "ls "+target)

		require.NoError(t, err, target)
		assert.Equal(t, "specs/\n", out, target)
	}
	out, _, err := exec(t, s, "ls .")
	require.NoError(t, err)
	assert.Equal(t, "README.md\na.txt\ndocs/\ntest/\n", out)
}

func TestLsRefusesWhatLeavesTheWorkingDirectoryOrIsNotADirectory(t *testing.T) {
	s := &Session{Dir: filepath.Join(tree(t), "docs")}
	other := t.TempDir()
	for _, target := range []string{"..", "../test", other, "specs/../.."} {
		out, _, err := exec(t, s, "ls "+target)

		require.Error(t, err, target)
		assert.Empty(t, out, target)
	}
	s = &Session{Dir: tree(t)}
	_, _, err := exec(t, s, "ls a.txt")
	require.Error(t, err)
	_, _, err = exec(t, s, "ls nowhere")
	require.Error(t, err)
}

func TestLsTakesOneTarget(t *testing.T) {
	s := &Session{Dir: tree(t)}

	_, _, err := exec(t, s, "ls docs test")

	assert.Error(t, err)
}

func projects(keys map[string]string) ProjectDir {
	return func(key string) (string, error) {
		if dir, ok := keys[strings.TrimPrefix(key, "@")]; ok {
			return dir, nil
		}
		return "", errors.New("unknown project " + key)
	}
}

func TestLsProject(t *testing.T) {
	clone := tree(t)
	s := &Session{Dir: t.TempDir(), ProjectDir: projects(map[string]string{
		"scc-operator": clone, "rancher/scc-operator": clone,
	})}
	for _, target := range []string{"scc-operator", "@scc-operator", "rancher/scc-operator", "@rancher/scc-operator"} {
		out, _, err := exec(t, s, "ls "+target)

		require.NoError(t, err, target)
		assert.Equal(t, "README.md\na.txt\ndocs/\ntest/\n", out, target)
	}
}

func TestLsBareWordIsADirectoryBeforeAProject(t *testing.T) {
	clone := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(clone, "from-project"), nil, 0o600))
	s := &Session{Dir: tree(t), ProjectDir: projects(map[string]string{"docs": clone})}

	out, _, err := exec(t, s, "ls docs")
	require.NoError(t, err)
	assert.Equal(t, "specs/\n", out)

	out, _, err = exec(t, s, "ls @docs")
	require.NoError(t, err)
	assert.Equal(t, "from-project\n", out)
}

func TestLsUnknownProject(t *testing.T) {
	s := &Session{Dir: tree(t), ProjectDir: projects(nil)}

	_, _, err := exec(t, s, "ls nothing")
	require.Error(t, err)
	_, _, err = exec(t, s, "ls @nothing")
	require.Error(t, err)
}

func TestLsAtDirectoryNamedWithAnAt(t *testing.T) {
	dir := tree(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "@odd"), 0o755))
	s := &Session{Dir: dir, ProjectDir: projects(nil)}

	_, _, err := exec(t, s, "ls ./@odd")

	assert.NoError(t, err)
}

func TestLeadingGeroIsRefused(t *testing.T) {
	var ran bool
	s := &Session{Dir: "/", Run: func(context.Context, string, []string) error { ran = true; return nil }}

	_, _, err := exec(t, s, "gero repo status")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "REPL takes the command alone")
	assert.False(t, ran)
}

func TestOtherLinesGoToTheRunnerWithTheDirectory(t *testing.T) {
	var gotDir string
	var gotArgs []string
	s := &Session{Dir: "/work", Run: func(_ context.Context, cwd string, args []string) error {
		gotDir, gotArgs = cwd, args
		return nil
	}}

	_, exit, err := exec(t, s, `repo status --json`)

	require.NoError(t, err)
	assert.False(t, exit)
	assert.Equal(t, "/work", gotDir)
	assert.Equal(t, []string{"repo", "status", "--json"}, gotArgs)
}

func TestARunnerErrorIsReturned(t *testing.T) {
	boom := errors.New("boom")
	s := &Session{Dir: "/", Run: func(context.Context, string, []string) error { return boom }}

	_, exit, err := exec(t, s, "repo status")

	assert.ErrorIs(t, err, boom)
	assert.False(t, exit)
}

func TestEmptyLineDoesNothing(t *testing.T) {
	s := &Session{Dir: "/", Run: func(context.Context, string, []string) error { return errors.New("ran") }}

	out, exit, err := exec(t, s, "   ")

	require.NoError(t, err)
	assert.False(t, exit)
	assert.Empty(t, out)
}

func TestLoopShowsThePromptRunsLinesAndKeepsGoingAfterAnError(t *testing.T) {
	s := &Session{Dir: "/work", Run: func(context.Context, string, []string) error { return errors.New("bad line") }}
	var out, errOut bytes.Buffer

	err := Loop(context.Background(), strings.NewReader("pwd\nrepo status\n\npwd\nexit\npwd\n"), &out, &errOut, s)

	require.NoError(t, err)
	assert.Equal(t, "gero> /work\ngero> gero> gero> /work\ngero> ", out.String())
	assert.Equal(t, "Error: bad line\n", errOut.String())
}

func TestLoopEndsAtEndOfInput(t *testing.T) {
	s := &Session{Dir: "/work"}
	var out, errOut bytes.Buffer

	err := Loop(context.Background(), strings.NewReader("pwd\n"), &out, &errOut, s)

	require.NoError(t, err)
	assert.Equal(t, "gero> /work\ngero> \n", out.String())
}

func TestLoopReportsASplitErrorAndGoesOn(t *testing.T) {
	s := &Session{Dir: "/work"}
	var out, errOut bytes.Buffer

	err := Loop(context.Background(), strings.NewReader("ls \"oops\npwd\n"), &out, &errOut, s)

	require.NoError(t, err)
	assert.NotEmpty(t, errOut.String())
	assert.Contains(t, out.String(), "/work\n")
}
