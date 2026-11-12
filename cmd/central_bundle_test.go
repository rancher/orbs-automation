package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/cmd/version"
	"github.com/rancher/orbs-automation/internal/bundle/bundletest"
	"github.com/rancher/orbs-automation/internal/bundlestore"
)

// asRelease makes the running gero the release, as a build of GoReleaser is, and
// lets the bundle be read from wherever there is one: a local directory, if any,
// then the cache, then the bundle compiled into the binary.
func asRelease(t *testing.T, v string) {
	t.Helper()
	old := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = old })
	t.Setenv("GERO_CONFIG_SOURCE", "auto")
}

// cacheBundle puts a bundle of the release in the cache, as a run that fetched it
// would have.
func cacheBundle(t *testing.T, tag string, names ...string) {
	t.Helper()
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	_, err := bundlestore.Store{Dir: filepath.Join(cache, "gero")}.Put(bundlestore.Entry{
		Tag: tag, Data: bundletest.Compile(t, tag, bundletest.Config(names...)), FetchedAt: time.Now(),
	})
	require.NoError(t, err)
}

func TestWithNoLocalConfigACommandReadsACachedBundleNewerThanItself(t *testing.T) {
	asRelease(t, "0.4.0")
	cacheBundle(t, "v0.5.0", "alpha", "bravo")
	t.Chdir(t.TempDir())

	stdout, _, err := execRoot(t, "repo", "clone", "--all", "--dry-run", "--upstream", "upstream", "--into", t.TempDir())

	require.NoError(t, err)
	assert.Contains(t, stdout, "Would clone rancher/alpha")
	assert.Contains(t, stdout, "Would clone rancher/bravo")
}

func TestALocalConfigDirectoryBeatsTheCachedBundle(t *testing.T) {
	asRelease(t, "0.4.0")
	cacheBundle(t, "v0.5.0", "from-the-bundle")
	t.Chdir(t.TempDir())
	local := t.TempDir()
	cfg := bundletest.Config("local-one")
	for name, f := range cfg {
		require.NoError(t, writeFile(filepath.Join(local, name), f.Data))
	}

	stdout, _, err := execRoot(t, "repo", "clone", "--all", "--dry-run", "--upstream", "upstream",
		"--central-config-dir", local, "--into", t.TempDir())

	require.NoError(t, err)
	assert.Contains(t, stdout, "Would clone rancher/local-one")
	assert.NotContains(t, stdout, "from-the-bundle")
}

// This is the CI job that has only the binary: no checkout, no cache, no network.
func TestWithNoLocalConfigAndNothingCachedACommandReadsTheBundleInTheBinary(t *testing.T) {
	asRelease(t, "0.5.0")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("CI", "true")
	t.Chdir(t.TempDir())

	stdout, _, err := execRoot(t, "repo", "clone", "--all", "--dry-run", "--upstream", "upstream", "--into", t.TempDir())

	require.NoError(t, err)
	assert.Contains(t, stdout, "Would clone rancher/backup-restore-operator", "a project of the real config/, embedded")
}

func TestWithNoLocalConfigAndTheDirectoryAskedForSaysWhereConfigIsLookedFor(t *testing.T) {
	asRelease(t, "0.5.0")
	t.Setenv("GERO_CONFIG_SOURCE", "dir")
	t.Chdir(t.TempDir())

	_, _, err := execRoot(t, "repo", "clone", "--all", "--dry-run", "--upstream", "upstream", "--into", t.TempDir())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "central config")
}
