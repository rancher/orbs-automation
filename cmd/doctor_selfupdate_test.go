package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/rancher/orbs-automation/internal/cli/repl"
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

func TestDoctorSelfUpdateRuns(t *testing.T) {
	t.Run("on a workstation it prints its text", func(t *testing.T) {
		workstationEnv(t, nil)

		stdout, _, err := execRoot(t, "doctor", "self-update")

		require.NoError(t, err)
		assert.Contains(t, stdout, "self-update")
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
