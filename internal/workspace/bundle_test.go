package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/rancher/orbs-automation/internal/bundle/bundletest"
	"github.com/rancher/orbs-automation/internal/bundlestore"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
)

// fakeBundle is a published release of gero, as the resolver would hand it over.
type fakeBundle struct {
	loaded bundlestore.Loaded
	err    error
	calls  int
}

func (f *fakeBundle) Load(context.Context) (bundlestore.Loaded, error) {
	f.calls++
	return f.loaded, f.err
}

// publishedBundle is the bundle of the release tag with a project for each name.
func publishedBundle(t *testing.T, tag string, names ...string) *fakeBundle {
	t.Helper()
	h, fsys, err := bundle.Parse(bundletest.Compile(t, tag, bundletest.Config(names...)))
	require.NoError(t, err)
	return &fakeBundle{loaded: bundlestore.Loaded{
		Header: h, FS: fsys,
		Source: bundlestore.Source{Tag: tag, FetchedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Age: 3 * time.Hour, FromCache: true},
	}}
}

func TestWithNoLocalConfigTheConfigIsReadFromTheBundle(t *testing.T) {
	b := publishedBundle(t, "v0.5.0", "alpha", "bravo")

	w := Open(settings(t, "", nil, nil), t.TempDir()).WithBundle(b)
	g, err := w.Chain()

	require.NoError(t, err)
	assert.Len(t, g.Projects(), 2)
	src, err := w.ConfigSource()
	require.NoError(t, err)
	assert.Equal(t, CentralConfigDir{}, src.Dir)
	require.NotNil(t, src.Bundle)
	assert.Equal(t, "v0.5.0", src.Bundle.Tag)
	assert.Equal(t, 3*time.Hour, src.Bundle.Age)
}

func TestTheBundleIsLoadedOncePerInvocation(t *testing.T) {
	b := publishedBundle(t, "v0.5.0", "alpha")
	w := Open(settings(t, "", nil, nil), t.TempDir()).WithBundle(b)

	_, err := w.Chain()
	require.NoError(t, err)
	_, err = w.Chain()
	require.NoError(t, err)
	_, err = w.ConfigSource()
	require.NoError(t, err)

	assert.Equal(t, 1, b.calls)
}

func TestALocalDirectoryWinsAndTheBundleIsNeverTouched(t *testing.T) {
	b := publishedBundle(t, "v0.9.0", "from-the-bundle", "another")
	dir := chainConfig(t, map[string]string{"a.yaml": projectDocV2("a", "o/a")})

	w := Open(settings(t, "", nil, map[string]string{"central-config-dir": dir}), t.TempDir()).WithBundle(b)
	g, err := w.Chain()

	require.NoError(t, err)
	assert.Len(t, g.Projects(), 1)
	assert.NotNil(t, g.Project("a"))
	src, err := w.ConfigSource()
	require.NoError(t, err)
	assert.Equal(t, CentralConfigDir{Dir: dir, Source: "flag"}, src.Dir)
	assert.Nil(t, src.Bundle)
	assert.Zero(t, b.calls, "no cache read and no request when local config is used")
}

func TestTheCentralCheckoutInTheWorkingDirectoryWinsOverTheBundle(t *testing.T) {
	b := publishedBundle(t, "v0.9.0", "from-the-bundle", "another")
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "config"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "config", "a.yaml"), []byte(projectDocV2("a", "o/a")), 0o600))

	w := Open(settings(t, "", nil, nil), root).WithBundle(b)
	g, err := w.Chain()

	require.NoError(t, err)
	assert.Len(t, g.Projects(), 1)
	assert.Zero(t, b.calls)
}

func TestALocalDirectoryThatIsSetButBrokenFailsAndNeverFallsBackToTheBundle(t *testing.T) {
	for name, dir := range map[string]string{
		"invalid config": chainConfig(t, map[string]string{"bad.yaml": projectDocV2("p", "nope")}),
		"empty":          t.TempDir(),
		"missing":        filepath.Join(t.TempDir(), "typo"),
	} {
		b := publishedBundle(t, "v0.9.0", "from-the-bundle")

		w := Open(settings(t, "", nil, map[string]string{"central-config-dir": dir}), t.TempDir()).WithBundle(b)
		g, err := w.Chain()

		require.Error(t, err, name)
		assert.Nil(t, g, name)
		assert.NotErrorIs(t, err, ErrNotFound, name)
		assert.NotContains(t, err.Error(), "bundle", name)
		assert.Zero(t, b.calls, name)
	}
}

func TestWhenTheBundleCannotBeLoadedTheConfigIsNotFoundAndSaysWhy(t *testing.T) {
	b := &fakeBundle{err: errors.New("no config bundle is available: the GitHub rate limit is used up")}
	w := Open(settings(t, "", nil, nil), t.TempDir()).WithBundle(b)

	_, err := w.Chain()

	require.ErrorIs(t, err, ErrNotFound)
	assert.Contains(t, err.Error(), "rate limit")
	_, err = w.ConfigSource()
	require.ErrorIs(t, err, ErrNotFound)
}

func TestABundleWithNoConfigInItIsEmptyConfigAndNamesTheBundle(t *testing.T) {
	// Compile refuses an empty config, so this is a bundle made by hand.
	h, fsys, err := bundle.Parse([]byte("apiVersion: gero-bundle/v1\nkind: Bundle\nformatVersion: 1\ngeroVersion: v0.5.0\nsourceCommit: abc\ncompiledAt: 2026-01-02T03:04:05Z\n"))
	require.NoError(t, err)
	b := &fakeBundle{loaded: bundlestore.Loaded{Header: h, FS: fsys, Source: bundlestore.Source{Tag: "v0.5.0"}}}

	_, err = Open(settings(t, "", nil, nil), t.TempDir()).WithBundle(b).Chain()

	require.ErrorIs(t, err, ErrEmptyConfig)
	assert.Contains(t, err.Error(), "v0.5.0")
}

func TestWithNoBundleSourceTheConfigIsNotFoundAsBefore(t *testing.T) {
	w := Open(settings(t, "", nil, nil), t.TempDir()).WithBundle(nil)

	_, err := w.Chain()

	require.ErrorIs(t, err, ErrNotFound)
	_, err = w.ConfigSource()
	require.ErrorIs(t, err, ErrNotFound)
}

func TestTheDefaultBundleSourceNeedsAReleaseContextAndIsNotUsedInCI(t *testing.T) {
	assert.Nil(t, bundleFor(clicfg.Settings{}), "no gero version: no release context")
	assert.Nil(t, bundleFor(clicfg.Settings{GeroVersion: "v0.5.0", CI: true}), "CI has a checkout")
	assert.NotNil(t, bundleFor(clicfg.Settings{GeroVersion: "v0.5.0"}))
	assert.NotNil(t, bundleFor(clicfg.Settings{GeroVersion: "dev", Offline: true}))
}

// A local directory and a bundle that are both good, so the mode alone decides.
func bothSources(t *testing.T) (dir string, b *fakeBundle) {
	t.Helper()
	return chainConfig(t, map[string]string{"a.yaml": projectDocV2("a", "o/a")}), publishedBundle(t, "v0.9.0", "from-the-bundle", "another")
}

func TestConfigSourceBundleIgnoresALocalDirectory(t *testing.T) {
	dir, b := bothSources(t)
	flags := map[string]string{"central-config-dir": dir, "config-source": "bundle"}

	w := Open(settings(t, "", nil, flags), t.TempDir()).WithBundle(b)
	g, err := w.Chain()

	require.NoError(t, err)
	assert.Len(t, g.Projects(), 2, "the bundle's projects, not the directory's")
	src, err := w.ConfigSource()
	require.NoError(t, err)
	assert.Equal(t, CentralConfigDir{}, src.Dir)
	require.NotNil(t, src.Bundle)
	assert.Equal(t, "v0.9.0", src.Bundle.Tag)
}

func TestConfigSourceBundleIgnoresTheCheckoutInTheWorkingDirectory(t *testing.T) {
	_, b := bothSources(t)
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "config"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "config", "a.yaml"), []byte(projectDocV2("a", "o/a")), 0o600))

	w := Open(settings(t, "", nil, map[string]string{"config-source": "bundle"}), root).WithBundle(b)
	src, err := w.ConfigSource()

	require.NoError(t, err)
	assert.NotNil(t, src.Bundle)
}

func TestConfigSourceBundleThatCannotBeLoadedFailsAndNeverFallsBackToTheDirectory(t *testing.T) {
	dir, _ := bothSources(t)
	b := &fakeBundle{err: errors.New("no config bundle is available: offline and nothing cached")}
	flags := map[string]string{"central-config-dir": dir, "config-source": "bundle"}

	w := Open(settings(t, "", nil, flags), t.TempDir()).WithBundle(b)
	_, err := w.Chain()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "offline and nothing cached")
	assert.Contains(t, err.Error(), "config-source")
	_, err = w.ConfigSource()
	require.Error(t, err)
}

func TestConfigSourceBundleWithNoBundleSourceSaysSo(t *testing.T) {
	w := Open(settings(t, "", nil, map[string]string{"config-source": "bundle"}), t.TempDir()).WithBundle(nil)

	_, err := w.ConfigSource()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "config-source")
}

func TestConfigSourceDirNeverTouchesTheBundle(t *testing.T) {
	b := publishedBundle(t, "v0.9.0", "from-the-bundle")
	w := Open(settings(t, "", nil, map[string]string{"config-source": "dir"}), t.TempDir()).WithBundle(b)

	_, err := w.Chain()

	require.ErrorIs(t, err, ErrNotFound)
	assert.Zero(t, b.calls)
}

func TestConfigSourceDirUsesALocalDirectory(t *testing.T) {
	dir, b := bothSources(t)
	flags := map[string]string{"central-config-dir": dir, "config-source": "dir"}

	g, err := Open(settings(t, "", nil, flags), t.TempDir()).WithBundle(b).Chain()

	require.NoError(t, err)
	assert.Len(t, g.Projects(), 1)
	assert.Zero(t, b.calls)
}

func TestTheDefaultBundleSourceFollowsTheConfigSource(t *testing.T) {
	assert.NotNil(t, bundleFor(clicfg.Settings{GeroVersion: "v0.5.0", ConfigSource: clicfg.ConfigSourceBundle, CI: true}), "an explicit bundle is used in CI")
	assert.Nil(t, bundleFor(clicfg.Settings{GeroVersion: "v0.5.0", ConfigSource: clicfg.ConfigSourceAuto, CI: true}))
	assert.Nil(t, bundleFor(clicfg.Settings{GeroVersion: "v0.5.0", ConfigSource: clicfg.ConfigSourceDir}))
	assert.Nil(t, bundleFor(clicfg.Settings{ConfigSource: clicfg.ConfigSourceBundle}), "still no release context, so none")
}
