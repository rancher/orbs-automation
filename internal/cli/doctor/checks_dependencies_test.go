package doctor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	assert.Equal(t, []string{"version", "gh", "auth-token", "auth-api", "rate-limit"}, checkNames(checks))
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
