package repl

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestRefuse(t *testing.T) {
	rows := []struct {
		name    string
		env     map[string]string
		tty, ci bool
		want    string // empty: it starts
	}{
		{"a terminal", map[string]string{"TERM": "xterm-256color"}, true, false, ""},
		{"no terminal", map[string]string{"TERM": "xterm"}, false, false, "terminal"},
		{"CI", map[string]string{"TERM": "xterm"}, true, true, "CI"},
		{"inside a REPL", map[string]string{"TERM": "xterm", "GERO_REPL": "1"}, true, false, "GERO_REPL"},
		{"TERM unset", map[string]string{}, true, false, "TERM"},
		{"TERM dumb", map[string]string{"TERM": "dumb"}, true, false, "TERM"},
	}
	require.NotEmpty(t, rows)
	for _, r := range rows {
		err := Refuse(env(r.env), clicfg.Terminal{StdinTTY: r.tty, StdoutTTY: r.tty}, r.ci)

		if r.want == "" {
			assert.NoError(t, err, r.name)
			continue
		}
		require.Error(t, err, r.name)
		assert.Contains(t, err.Error(), r.want, r.name)
	}
}

// waitForCancel is a line that runs until it is cancelled, and gives up after a
// while so a missing cancel is a failure and not a hang.
func waitForCancel(ctx context.Context, _ string, _ []string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		return errors.New("not cancelled")
	}
}

// send gives sigs a signal, and fails the test when nothing takes it.
func send(t *testing.T, sigs chan<- os.Signal) {
	t.Helper()
	select {
	case sigs <- os.Interrupt:
	case <-time.After(time.Second):
		t.Fatal("nothing took the signal")
	}
}

func TestInterruptCancelsTheRunningLine(t *testing.T) {
	sigs := make(chan os.Signal)
	run := Interruptible(sigs, waitForCancel)
	go func() { time.Sleep(50 * time.Millisecond); sigs <- os.Interrupt }()

	err := run(context.Background(), "/", nil)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestInterruptAtThePromptDoesNotCancelTheNextLine(t *testing.T) {
	sigs := make(chan os.Signal)
	run := Interruptible(sigs, func(ctx context.Context, _ string, _ []string) error { return ctx.Err() })
	// The second send is taken only after the first has been handled.
	send(t, sigs)
	send(t, sigs)

	err := run(context.Background(), "/", nil)

	assert.NoError(t, err)
}

func TestTheLineSeesTheParentContext(t *testing.T) {
	sigs := make(chan os.Signal)
	run := Interruptible(sigs, waitForCancel)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := run(ctx, "/", nil)

	assert.ErrorIs(t, err, context.Canceled)
}
