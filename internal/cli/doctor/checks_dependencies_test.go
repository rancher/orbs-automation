package doctor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/git/gitcli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGh answers Locate and Version from fixed values and records what Version
// was asked about.
type fakeGh struct {
	path       string
	locateErr  error
	version    string
	versionErr error
	versionFor []string
	versionRun func(ctx context.Context) (string, error)
}

func (f *fakeGh) Locate() (string, error) { return f.path, f.locateErr }

func (f *fakeGh) Version(ctx context.Context, path string) (string, error) {
	f.versionFor = append(f.versionFor, path)
	if f.versionRun != nil {
		return f.versionRun(ctx)
	}
	return f.version, f.versionErr
}

func TestGhCheckMetadata(t *testing.T) {
	c := NewGhCheck(&fakeGh{})

	assert.Equal(t, "gh", c.Name())
	assert.Equal(t, "dependencies", c.Category())
	assert.False(t, c.Online())
	assert.Empty(t, c.Needs())
	assert.NotEmpty(t, c.Description())
}

func TestGhCheckFound(t *testing.T) {
	g := &fakeGh{path: "/opt/homebrew/bin/gh", version: "gh version 2.100.0 (2026-09-03)"}

	got := NewGhCheck(g).Run(context.Background())

	assert.Equal(t, Result{Status: StatusOK, Message: "gh version 2.100.0 (2026-09-03) at /opt/homebrew/bin/gh"}, got)
	assert.Equal(t, []string{"/opt/homebrew/bin/gh"}, g.versionFor)
}

func TestGhCheckMissingIsAWarningThatSaysWhyItMatters(t *testing.T) {
	g := &fakeGh{locateErr: errors.New("gh not found on PATH")}

	got := NewGhCheck(g).Run(context.Background())

	assert.Equal(t, StatusWarn, got.Status)
	assert.Equal(t, "gh not found on PATH", got.Message)
	assert.Contains(t, got.Remediation, "keyring")
	assert.Contains(t, got.Remediation, "https://cli.github.com")
	assert.Contains(t, got.Remediation, "GH_TOKEN")
	assert.Contains(t, got.Remediation, "GH_PATH")
	assert.Empty(t, g.versionFor, "nothing to run when gh was not found")
}

func TestGhCheckThatCannotReportItsVersion(t *testing.T) {
	g := &fakeGh{path: "/usr/local/bin/gh", versionErr: errors.New("exit status 1")}

	got := NewGhCheck(g).Run(context.Background())

	assert.Equal(t, StatusWarn, got.Status)
	assert.Equal(t, "found at /usr/local/bin/gh, but running it failed: exit status 1", got.Message)
	assert.Contains(t, got.Remediation, "gh --version")
}

func TestGhCheckGivesUpOnAHungGh(t *testing.T) {
	g := &fakeGh{path: "/usr/local/bin/gh", versionRun: func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}}

	start := time.Now()
	got := newGhCheck(g, 20*time.Millisecond).Run(context.Background())

	assert.Equal(t, StatusWarn, got.Status)
	assert.Contains(t, got.Message, "context deadline exceeded")
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestDefaultChecksAreConsistent(t *testing.T) {
	checks := defaultChecks(testBuild)

	_, err := validate(checks)

	require.NoError(t, err, "every need must be registered before the check that has it")
	assert.Equal(t, []string{"version", "git", "gh", "auth-token", "auth-api", "rate-limit"}, checkNames(checks))
}

func checkNames(checks []Check) []string {
	out := make([]string, len(checks))
	for i, c := range checks {
		out[i] = c.Name()
	}
	return out
}

func TestGhBinaryUsesGhOnPath(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'gh version 9.9.9 (2030-01-01)'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755))
	t.Setenv("PATH", dir)
	t.Setenv("GH_PATH", "")

	path, err := ghBinary{}.Locate()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "gh"), path)

	version, err := ghBinary{}.Version(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, "gh version 9.9.9 (2030-01-01)", version)
}

// fakeGitBinary answers Version and Config from fixed values and records the
// config keys it was asked for.
type fakeGitBinary struct {
	version    string
	versionErr error
	versionRun func(ctx context.Context) (string, error)
	config     map[string]string
	configErr  map[string]error
	asked      []string
}

func (f *fakeGitBinary) Version(ctx context.Context) (string, error) {
	if f.versionRun != nil {
		return f.versionRun(ctx)
	}
	return f.version, f.versionErr
}

func (f *fakeGitBinary) Config(_ context.Context, key string) (string, error) {
	f.asked = append(f.asked, key)
	return f.config[key], f.configErr[key]
}

func TestGitCheckMetadata(t *testing.T) {
	c := NewGitCheck(&fakeGitBinary{})

	assert.Equal(t, "git", c.Name())
	assert.Equal(t, "dependencies", c.Category())
	assert.False(t, c.Online())
	assert.Empty(t, c.Needs())
	assert.NotEmpty(t, c.Description())
}

func TestGitCheckFoundWithAnIdentity(t *testing.T) {
	g := &fakeGitBinary{
		version: "git version 2.51.0",
		config:  map[string]string{"user.name": "Ada Lovelace", "user.email": "ada@example.com"},
	}

	got := NewGitCheck(g).Run(context.Background())

	assert.Equal(t, Result{Status: StatusOK, Message: "git version 2.51.0, user.name and user.email are set"}, got)
	assert.Equal(t, []string{"user.name", "user.email"}, g.asked)
}

func TestGitCheckWarnsAboutAMissingIdentity(t *testing.T) {
	tests := []struct {
		name            string
		config          map[string]string
		wantMessage     string
		wantRemediation string
	}{
		{
			name:            "no name",
			config:          map[string]string{"user.email": "ada@example.com"},
			wantMessage:     "git version 2.51.0, but user.name is not set",
			wantRemediation: `git config --global user.name "Your Name"`,
		},
		{
			name:            "no email",
			config:          map[string]string{"user.name": "Ada Lovelace"},
			wantMessage:     "git version 2.51.0, but user.email is not set",
			wantRemediation: `git config --global user.email "you@example.com"`,
		},
		{
			name:        "neither",
			config:      map[string]string{},
			wantMessage: "git version 2.51.0, but user.name and user.email are not set",
			wantRemediation: `git config --global user.name "Your Name" && ` +
				`git config --global user.email "you@example.com"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &fakeGitBinary{version: "git version 2.51.0", config: tt.config}

			got := NewGitCheck(g).Run(context.Background())

			assert.Equal(t, StatusWarn, got.Status)
			assert.Equal(t, tt.wantMessage, got.Message)
			assert.Equal(t, "set it so commits gero makes are yours: "+tt.wantRemediation, got.Remediation)
		})
	}
}

func TestGitCheckMissingFailsAndSaysWhyItMatters(t *testing.T) {
	g := &fakeGitBinary{versionErr: gitcli.ErrNotFound}

	got := NewGitCheck(g).Run(context.Background())

	assert.Equal(t, StatusFail, got.Status)
	assert.Equal(t, "git not found on PATH", got.Message)
	assert.Contains(t, got.Remediation, "https://git-scm.com")
	assert.Contains(t, got.Remediation, "gero repo branch-tags")
	assert.Empty(t, g.asked, "no point asking a missing git for its config")
}

func TestGitCheckThatCannotReportItsVersionFails(t *testing.T) {
	g := &fakeGitBinary{versionErr: errors.New("exit status 1")}

	got := NewGitCheck(g).Run(context.Background())

	assert.Equal(t, StatusFail, got.Status)
	assert.Equal(t, "git --version failed: exit status 1", got.Message)
	assert.Contains(t, got.Remediation, "git --version")
}

func TestGitCheckThatCannotReadItsConfigWarns(t *testing.T) {
	g := &fakeGitBinary{
		version:   "git version 2.51.0",
		config:    map[string]string{"user.email": "ada@example.com"},
		configErr: map[string]error{"user.name": errors.New("fatal: bad config")},
	}

	got := NewGitCheck(g).Run(context.Background())

	assert.Equal(t, StatusWarn, got.Status)
	assert.Equal(t, "could not read user.name: fatal: bad config", got.Message)
	assert.Contains(t, got.Remediation, "git config user.name")
}

func TestGitCheckGivesUpOnAHungGit(t *testing.T) {
	g := &fakeGitBinary{versionRun: func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}}

	start := time.Now()
	got := newGitCheck(g, 20*time.Millisecond).Run(context.Background())

	assert.Equal(t, StatusFail, got.Status)
	assert.Contains(t, got.Message, "context deadline exceeded")
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestGitBinaryUsesGitOnPath(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\ncase \"$1\" in\n--version) echo 'git version 9.9.9';;\nconfig) echo \"value-for-$3\";;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755))
	t.Setenv("PATH", dir)

	version, err := gitBinary{}.Version(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "git version 9.9.9", version)

	value, err := gitBinary{}.Config(context.Background(), "user.name")
	require.NoError(t, err)
	assert.Equal(t, "value-for-user.name", value)
}
