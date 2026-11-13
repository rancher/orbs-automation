package ciship

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/cli/ship"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRun replaces run for a test and records the options it was called with.
func fakeRun(t *testing.T, err error) *ship.Options {
	t.Helper()
	var got ship.Options
	old := run
	run = func(_ context.Context, _ io.Writer, _ *slog.Logger, _ clicfg.Settings, _ string, opts ship.Options) error {
		got = opts
		return err
	}
	t.Cleanup(func() { run = old })
	return &got
}

func TestRunShipsWithoutAsking(t *testing.T) {
	got := fakeRun(t, nil)

	err := Run(context.Background(), io.Discard, nil, clicfg.Settings{CI: true}, "/work",
		ship.Options{Project: "shell", Arg: "v0.9.1", NoPush: true, Interactive: true, In: strings.NewReader("y\n")})

	require.NoError(t, err)
	assert.Equal(t, "shell", got.Project)
	assert.Equal(t, "v0.9.1", got.Arg)
	assert.True(t, got.Yes, "a workflow cannot be asked")
	assert.False(t, got.Interactive)
	assert.Nil(t, got.In)
	assert.False(t, got.NoPush, "a workflow that does not push has no reason to run")
}

func TestRunRefusesTheWorkflowsOwnToken(t *testing.T) {
	got := fakeRun(t, nil)
	var out bytes.Buffer

	err := Run(context.Background(), &out, nil, clicfg.Settings{CI: true, DefaultToken: true}, "/work", ship.Options{Project: "shell"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gero ci:ship")
	assert.Contains(t, err.Error(), "GH_TOKEN")
	assert.Equal(t, ship.Options{}, *got, "nothing was handed on")
}

func TestRunReturnsWhatShipReturns(t *testing.T) {
	want := errors.New("a push was rejected")
	fakeRun(t, want)

	err := Run(context.Background(), io.Discard, nil, clicfg.Settings{CI: true}, "/work", ship.Options{})

	assert.ErrorIs(t, err, want)
}
