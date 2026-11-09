package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// userFileEnv returns an Environment whose environment is vars, with a new home
// directory.
func userFileEnv(t *testing.T, vars map[string]string) (Environment, string) {
	t.Helper()
	home := t.TempDir()
	return Environment{Home: home, Getenv: func(k string) string { return vars[k] }}, home
}

// writeUserFile writes content to a new file and returns its path.
func writeUserFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestUserFileCheckMetadata(t *testing.T) {
	c := NewUserFileCheck(Environment{})

	assert.Equal(t, "user-file", c.Name())
	assert.Equal(t, "configuration", c.Category())
	assert.False(t, c.Online())
	assert.Empty(t, c.Needs())
	assert.NotEmpty(t, c.Description())
}

func TestUserFileCheckWithNoFileIsFine(t *testing.T) {
	env, home := userFileEnv(t, nil)

	got := NewUserFileCheck(env).Run(context.Background())

	want := filepath.Join(home, ".config", "gero", "config.yaml")
	assert.Equal(t, Result{Status: StatusOK, Message: "no user file at " + want + " (optional)"}, got)
}

func TestUserFileCheckWithoutAnEnvironmentDoesNotPanic(t *testing.T) {
	got := NewUserFileCheck(Environment{Home: t.TempDir()}).Run(context.Background())

	assert.Equal(t, StatusOK, got.Status)
	assert.Contains(t, got.Message, "no user file at ")
}

func TestUserFileCheckLoadedFileReportsItsPath(t *testing.T) {
	path := writeUserFile(t, "output: json\nrepos:\n  search:\n    - "+t.TempDir()+"\n")
	env, _ := userFileEnv(t, map[string]string{"GERO_CONFIG": path})

	got := NewUserFileCheck(env).Run(context.Background())

	assert.Equal(t, Result{Status: StatusOK, Message: path}, got)
}

func TestUserFileCheckFileThatWouldStopACommandFails(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"a value that is not allowed", "pushTo: forks\n", "pushTo"},
		{"a path that is not absolute", "centralConfigDir: relative/dir\n", "absolute"},
		{"a file that is not YAML", "output: [unclosed\n", "yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeUserFile(t, tt.content)
			env, _ := userFileEnv(t, map[string]string{"GERO_CONFIG": path})

			got := NewUserFileCheck(env).Run(context.Background())

			assert.Equal(t, StatusFail, got.Status)
			assert.Contains(t, got.Message, path, "the message names the file")
			assert.Contains(t, got.Message, tt.want)
			assert.Contains(t, got.Remediation, "move the file aside")
			assert.Contains(t, got.Remediation, "docs/specs/CLI_CONFIG.md")
		})
	}
}

func TestUserFileCheckGeroConfigNamingAMissingFileFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.yaml")
	env, _ := userFileEnv(t, map[string]string{"GERO_CONFIG": missing})

	got := NewUserFileCheck(env).Run(context.Background())

	assert.Equal(t, StatusFail, got.Status)
	assert.Contains(t, got.Message, missing)
	assert.Contains(t, got.Remediation, "GERO_CONFIG")
}

func TestUserFileCheckJudgesTheFileNotTheEnvironment(t *testing.T) {
	path := writeUserFile(t, "output: json\n")
	// An invalid GERO_OUTPUT is a problem, but not the file's.
	env, _ := userFileEnv(t, map[string]string{"GERO_CONFIG": path, "GERO_OUTPUT": "bogus"})

	got := NewUserFileCheck(env).Run(context.Background())

	assert.Equal(t, Result{Status: StatusOK, Message: path}, got)
}

// A key that is not known does not stop a command, so the check passes with a
// warning that names it, and the keys that are valid are in effect.
func TestUserFileCheckUnknownKeyWarns(t *testing.T) {
	path := writeUserFile(t, "output: json\nrepos:\n  cloneDir: /x\n")
	env, _ := userFileEnv(t, map[string]string{"GERO_CONFIG": path})

	got := NewUserFileCheck(env).Run(context.Background())

	assert.Equal(t, StatusWarn, got.Status)
	assert.Contains(t, got.Message, path)
	assert.Contains(t, got.Message, "line 3")
	assert.Contains(t, got.Message, "cloneDir")
	assert.Contains(t, got.Remediation, "docs/specs/CLI_CONFIG.md")
}

// A search directory that is not there is skipped by the scan without a word, so
// a typo in it hides every clone under it. Doctor fails on it and names it.
func TestUserFileCheckSearchDirectoryThatIsNotThereFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "GitProject")
	present := t.TempDir()
	path := writeUserFile(t, "repos:\n  search:\n    - "+present+"\n    - "+missing+"\n")
	env, _ := userFileEnv(t, map[string]string{"GERO_CONFIG": path})

	got := NewUserFileCheck(env).Run(context.Background())

	assert.Equal(t, StatusFail, got.Status)
	assert.Contains(t, got.Message, "repos.search")
	assert.Contains(t, got.Message, missing)
	assert.NotContains(t, got.Message, present)
	assert.Contains(t, got.Remediation, "repos.search")
}

// The clone root is made by the first clone, so it is not expected to exist.
func TestUserFileCheckCloneRootThatIsNotThereIsFine(t *testing.T) {
	path := writeUserFile(t, "repos:\n  cloneRoot: "+filepath.Join(t.TempDir(), "later")+"\n")
	env, _ := userFileEnv(t, map[string]string{"GERO_CONFIG": path})

	got := NewUserFileCheck(env).Run(context.Background())

	assert.Equal(t, Result{Status: StatusOK, Message: path}, got)
}
