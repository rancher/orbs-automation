package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lassoHolding = "bro-holds-wr+la-mod"

// pinStatusWorkspace is the pin world with bro's main branch holding wrangler wr and lasso la in go.mod,
// and wrangler v1.7.3 requiring lasso v0.2.4 as v1.7.4 requires v0.2.5. Lasso's newest is v0.2.6.
func pinStatusWorkspace(t *testing.T, wr, la string) pinWorkspace {
	t.Helper()
	w := newPinWorkspace(t)
	w.host.files["acme/wr@v1.7.3:go.mod"] = "module github.com/acme/wr\n\nrequire github.com/acme/la v0.2.4\n"
	w.host.files["acme/bro@sha-main:go.mod"] = "module acme/bro\n\nrequire (\n\tgithub.com/acme/la " + la + "\n\tgithub.com/acme/wr " + wr + "\n)\n"
	return w
}

// lassoOnMain is what bro's main branch holds of the lasso that wrangler's pin makes, as repo status reads it.
func lassoOnMain(t *testing.T, w pinWorkspace) HeldValue {
	t.Helper()
	srcs, err := Sources(ctx, w, w.g.Project("bro"))
	require.NoError(t, err)
	for _, src := range srcs {
		if src.Producer != "acme/la" {
			continue
		}
		require.Empty(t, src.Error)
		for _, tgt := range src.Targets {
			if tgt.Branch != "main" {
				continue
			}
			for _, held := range tgt.Held {
				if held.Name == lassoHolding {
					return held
				}
			}
		}
	}
	require.Fail(t, "no row", "%s on main", lassoHolding)
	return HeldValue{}
}

func TestStatusReadsAPinnedLassoAgainstWhatWranglerRequiresAndNotTheLatestLasso(t *testing.T) {
	got := lassoOnMain(t, pinStatusWorkspace(t, "v1.7.4", "v0.2.5")) // lasso v0.2.6 is out, and v1.7.4 needs v0.2.5

	require.NotNil(t, got.Pin)
	assert.Equal(t, "bro-holds-wr", got.Pin.Pinner)
	require.NotNil(t, got.Pin.Held)
	assert.Equal(t, "acme/wr v1.7.4", got.Pin.Held.RequiredBy)
	assert.Equal(t, &Verdict{Tag: "v0.2.5", Want: "v0.2.5", UpToDate: true, Relation: bump.RelationSame}, got.Pin.Held.Verdict)
	require.NotNil(t, got.Pin.Wanted)
	assert.Equal(t, &Verdict{Tag: "v0.2.5", Want: "v0.2.5", UpToDate: true, Relation: bump.RelationSame}, got.Pin.Wanted.Verdict)
	assert.Nil(t, got.Stable, "a pinned holding is not compared with the latest release of its stream")
}

func TestStatusSaysLassoMeetsWhatTheHeldWranglerRequiresButNotWhatTheNewestOne(t *testing.T) {
	got := lassoOnMain(t, pinStatusWorkspace(t, "v1.7.3", "v0.2.4")) // wrangler v1.7.4 is out and needs v0.2.5

	require.NotNil(t, got.Pin)
	assert.True(t, got.Pin.Held.Verdict.UpToDate, "consistent as it stands")
	assert.Equal(t, "acme/wr v1.7.3", got.Pin.Held.RequiredBy)
	assert.False(t, got.Pin.Wanted.Verdict.UpToDate, "but not for the wrangler sync would bring it to")
	assert.Equal(t, "v0.2.5", got.Pin.Wanted.Verdict.Tag)
	assert.Equal(t, "acme/wr v1.7.4", got.Pin.Wanted.RequiredBy)
	assert.Equal(t, bump.RelationNewer, got.Pin.Wanted.Verdict.Relation)
}

func TestStatusSaysLassoBelowWhatTheHeldWranglerRequiresIsNotConsistent(t *testing.T) {
	got := lassoOnMain(t, pinStatusWorkspace(t, "v1.7.4", "v0.2.3"))

	require.NotNil(t, got.Pin)
	assert.False(t, got.Pin.Held.Verdict.UpToDate)
	assert.Equal(t, "v0.2.5", got.Pin.Held.Verdict.Tag)
}

func TestStatusTakesALassoNewerThanThePinRequiresAsHeld(t *testing.T) {
	got := lassoOnMain(t, pinStatusWorkspace(t, "v1.7.4", "v0.2.6"))

	require.NotNil(t, got.Pin)
	assert.True(t, got.Pin.Held.Verdict.UpToDate, "a pin is a floor")
	assert.True(t, got.Pin.Wanted.Verdict.UpToDate)
}

func TestStatusSaysSoWhenTheHeldWranglerIsNoReleaseAndStillReadsTheWantedFloor(t *testing.T) {
	got := lassoOnMain(t, pinStatusWorkspace(t, "v1.7.1", "v0.2.3")) // v1.7.1 is not a tag of the stream

	require.NotNil(t, got.Pin)
	require.NotNil(t, got.Pin.Held)
	assert.Nil(t, got.Pin.Held.Verdict)
	assert.Contains(t, got.Pin.Held.Error, "no release")
	require.NotNil(t, got.Pin.Wanted)
	assert.Equal(t, "v0.2.5", got.Pin.Wanted.Verdict.Tag)
	assert.False(t, got.Pin.Wanted.Verdict.UpToDate)
}
