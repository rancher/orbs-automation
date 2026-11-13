package ship

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunShip replaces runShip for a test and records the options it was called with.
func fakeRunShip(t *testing.T, err error) *Options {
	t.Helper()
	var got Options
	old := runShip
	runShip = func(_ context.Context, _ io.Writer, _ *slog.Logger, _ clicfg.Settings, _ string, opts Options) error {
		got = opts
		return err
	}
	t.Cleanup(func() { runShip = old })
	return &got
}

func TestRunCIShipsWithoutAsking(t *testing.T) {
	got := fakeRunShip(t, nil)

	err := RunCI(context.Background(), io.Discard, nil, clicfg.Settings{CI: true}, "/work",
		Options{Project: "shell", Arg: "v0.9.1", NoPush: true, Interactive: true, In: strings.NewReader("y\n")})

	require.NoError(t, err)
	assert.Equal(t, "shell", got.Project)
	assert.Equal(t, "v0.9.1", got.Arg)
	assert.True(t, got.Yes, "a workflow cannot be asked")
	assert.False(t, got.Interactive)
	assert.Nil(t, got.In)
	assert.False(t, got.NoPush, "a workflow that does not push has no reason to run")
}

func TestRunCIRefusesTheWorkflowsOwnToken(t *testing.T) {
	got := fakeRunShip(t, nil)
	var out bytes.Buffer

	err := RunCI(context.Background(), &out, nil, clicfg.Settings{CI: true, DefaultToken: true}, "/work", Options{Project: "shell"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gero ci:ship")
	assert.Contains(t, err.Error(), "GH_TOKEN")
	assert.Equal(t, Options{}, *got, "nothing was handed on")
}

func TestRunCIReturnsWhatShipReturns(t *testing.T) {
	want := errors.New("a push was rejected")
	fakeRunShip(t, want)

	err := RunCI(context.Background(), io.Discard, nil, clicfg.Settings{CI: true}, "/work", Options{})

	assert.ErrorIs(t, err, want)
}
