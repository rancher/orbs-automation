package configfetch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/bundlestore"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
)

func returning(l bundlestore.Loaded, err error) Fetcher {
	return func(context.Context, clicfg.Settings) (bundlestore.Loaded, error) { return l, err }
}

func TestRunSaysWhichBundleWasFetched(t *testing.T) {
	l := bundlestore.Loaded{Source: bundlestore.Source{Tag: "v0.6.0", FetchedAt: time.Now()}}

	out, err := Run(context.Background(), clicfg.Settings{}, returning(l, nil))

	require.NoError(t, err)
	assert.Contains(t, out, "fetched")
	assert.Contains(t, out, "bundle v0.6.0")
}

func TestRunSaysWhenTheBundleInTheBinaryIsTheNewestAndWhy(t *testing.T) {
	l := bundlestore.Loaded{Source: bundlestore.Source{Embedded: true, Tag: "v0.5.0", Note: "the newest release on this channel, v0.5.0, is not newer than this gero's own, v0.5.0"}}

	out, err := Run(context.Background(), clicfg.Settings{}, returning(l, nil))

	require.NoError(t, err)
	assert.Contains(t, out, "up to date")
	assert.Contains(t, out, "embedded bundle v0.5.0")
	assert.Contains(t, out, "is not newer than this gero's own")
}

func TestRunPrintsTheNoteOfAFetchedBundleToo(t *testing.T) {
	l := bundlestore.Loaded{Source: bundlestore.Source{Tag: "v0.5.1", Note: "the newest bundle, v0.7.0-rc.1, is one this gero cannot read (x); using the bundle of v0.5.1"}}

	out, err := Run(context.Background(), clicfg.Settings{}, returning(l, nil))

	require.NoError(t, err)
	assert.Contains(t, out, "bundle v0.5.1")
	assert.Contains(t, out, "cannot read")
}

func TestRunReturnsTheErrorOfTheFetch(t *testing.T) {
	boom := errors.New("the GitHub rate limit is used up")

	out, err := Run(context.Background(), clicfg.Settings{}, returning(bundlestore.Loaded{}, boom))

	require.ErrorIs(t, err, boom)
	assert.Empty(t, out)
}

func TestRunHandsTheSettingsToTheFetch(t *testing.T) {
	var got clicfg.Settings
	fetch := func(_ context.Context, s clicfg.Settings) (bundlestore.Loaded, error) {
		got = s
		return bundlestore.Loaded{Source: bundlestore.Source{Embedded: true}}, nil
	}

	_, err := Run(context.Background(), clicfg.Settings{GeroVersion: "0.5.0", Offline: true}, fetch)

	require.NoError(t, err)
	assert.Equal(t, "0.5.0", got.GeroVersion)
	assert.True(t, got.Offline)
}
