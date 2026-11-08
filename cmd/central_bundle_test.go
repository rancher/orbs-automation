package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/bundle/bundletest"
	"github.com/rancher/orbs-automation/internal/bundlestore"
)

// cacheBundle puts a bundle of the release in the cache, as a run that fetched it
// would have.
func cacheBundle(t *testing.T, tag string, names ...string) {
	t.Helper()
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	require.NoError(t, bundlestore.Cache{Dir: filepath.Join(cache, "gero")}.Write(bundlestore.Entry{
		Tag: tag, Data: bundletest.Compile(t, tag, bundletest.Config(names...)), FetchedAt: time.Now(),
	}))
}

func TestWithNoLocalConfigACommandReadsTheCentralBundleFromTheCache(t *testing.T) {
	cacheBundle(t, "v0.5.0", "alpha", "bravo")
	t.Setenv("GERO_OFFLINE", "true")
	t.Chdir(t.TempDir())

	stdout, _, err := execRoot(t, "repo", "clone", "--all", "--dry-run", "--upstream", "upstream", "--into", t.TempDir())

	require.NoError(t, err)
	assert.Contains(t, stdout, "Would clone rancher/alpha")
	assert.Contains(t, stdout, "Would clone rancher/bravo")
}

func TestALocalConfigDirectoryBeatsTheCachedBundle(t *testing.T) {
	cacheBundle(t, "v0.5.0", "from-the-bundle")
	t.Setenv("GERO_OFFLINE", "true")
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

func TestWithNoLocalConfigAndNothingCachedOfflineSaysWhereConfigIsLookedFor(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("GERO_OFFLINE", "true")
	t.Chdir(t.TempDir())

	_, _, err := execRoot(t, "repo", "clone", "--all", "--dry-run", "--upstream", "upstream", "--into", t.TempDir())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "central config")
	assert.Contains(t, err.Error(), "bundle")
}
