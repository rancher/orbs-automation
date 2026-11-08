package cmd

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/rancher/orbs-automation/internal/cli/repl"
	"github.com/rancher/orbs-automation/internal/cli/selfupdate"
	platform "github.com/rancher/orbs-automation/internal/platform/selfupdate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// workstationEnv puts the process on a workstation, outside the REPL, and then
// applies env.
func workstationEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range []string{"GITHUB_ACTIONS", "CI", "GERO_CONTEXT", repl.EnvRepl} {
		t.Setenv(k, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestHideWorkstationOnly(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"workstation", nil, false},
		{"CI=true", map[string]string{"CI": "true"}, true},
		{"GITHUB_ACTIONS=true", map[string]string{"GITHUB_ACTIONS": "true"}, true},
		{"GERO_CONTEXT=ci", map[string]string{"GERO_CONTEXT": "ci"}, true},
		{"GERO_CONTEXT=workstation on a runner", map[string]string{"GITHUB_ACTIONS": "true", "GERO_CONTEXT": "workstation"}, false},
		{"inside the REPL", map[string]string{repl.EnvRepl: "1"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workstationEnv(t, tt.env)

			assert.Equal(t, tt.want, hideWorkstationOnly(os.Getenv))
		})
	}
}

// helpOf returns the help of gero doctor from a tree built now.
func helpOf(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	require.NoError(t, root.Execute())
	return out.String()
}

func TestDoctorSelfUpdateIsListedOnlyOnAWorkstation(t *testing.T) {
	t.Run("workstation", func(t *testing.T) {
		workstationEnv(t, nil)

		assert.Contains(t, helpOf(t, "doctor", "--help"), "self-update")
	})
	t.Run("CI", func(t *testing.T) {
		workstationEnv(t, map[string]string{"CI": "true"})

		assert.NotContains(t, helpOf(t, "doctor", "--help"), "self-update")
	})
	t.Run("REPL", func(t *testing.T) {
		workstationEnv(t, map[string]string{repl.EnvRepl: "1"})

		assert.NotContains(t, helpOf(t, "doctor", "--help"), "self-update")
	})
}

type stubUpdater struct {
	available bool
	applied   []platform.Status
}

func (u *stubUpdater) Check(context.Context) (platform.Status, error) {
	return platform.Status{Current: "1.0.0", Latest: platform.Release{Version: "v9.9.9"}, Available: u.available}, nil
}
func (*stubUpdater) Blocked() string { return "" }
func (u *stubUpdater) Apply(_ context.Context, s platform.Status) (string, error) {
	u.applied = append(u.applied, s)
	return s.Latest.Version, nil
}

// fakeUpdater makes gero doctor self-update use an updater that reaches no one.
func fakeUpdater(t *testing.T, available bool) *stubUpdater {
	t.Helper()
	u := &stubUpdater{available: available}
	old := buildUpdater
	buildUpdater = func(string) (selfupdate.Updater, error) { return u, nil }
	t.Cleanup(func() { buildUpdater = old })
	return u
}

// runTree runs args on a tree built now, so no flag is left from another test.
func runTree(t *testing.T, args ...string) (string, error) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestDoctorSelfUpdateRuns(t *testing.T) {
	t.Run("on a workstation it checks, and with --check changes nothing", func(t *testing.T) {
		workstationEnv(t, nil)
		u := fakeUpdater(t, true)

		stdout, err := runTree(t, "doctor", "self-update", "--check")

		require.NoError(t, err)
		assert.Contains(t, stdout, "v9.9.9 is available")
		assert.Empty(t, u.applied)
	})
	t.Run("on a workstation --yes installs without asking", func(t *testing.T) {
		workstationEnv(t, nil)
		u := fakeUpdater(t, true)

		stdout, err := runTree(t, "doctor", "self-update", "--yes")

		require.NoError(t, err)
		assert.Contains(t, stdout, "Updated gero to v9.9.9")
		assert.Len(t, u.applied, 1)
	})
	t.Run("--dry-run installs nothing", func(t *testing.T) {
		workstationEnv(t, nil)
		u := fakeUpdater(t, true)

		stdout, err := runTree(t, "doctor", "self-update", "--dry-run", "--yes")

		require.NoError(t, err)
		assert.Contains(t, stdout, "Would update")
		assert.Empty(t, u.applied)
	})
	t.Run("in CI it is refused, and prints nothing", func(t *testing.T) {
		workstationEnv(t, map[string]string{"GITHUB_ACTIONS": "true"})

		stdout, _, err := execRoot(t, "doctor", "self-update")

		require.Error(t, err)
		assert.ErrorContains(t, err, "gero doctor self-update")
		assert.ErrorContains(t, err, "not in CI")
		assert.Empty(t, stdout)
	})
	t.Run("in the REPL it is refused, and prints nothing", func(t *testing.T) {
		workstationEnv(t, map[string]string{repl.EnvRepl: "1"})

		stdout, _, err := execRoot(t, "doctor", "self-update")

		require.Error(t, err)
		assert.ErrorContains(t, err, "gero doctor self-update")
		assert.ErrorContains(t, err, "not in the repl")
		assert.Empty(t, stdout)
	})
}
