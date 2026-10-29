package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type shown map[string]struct{ Value, Source string }

// configShow runs `gero config show --json` with a user file, env and extra
// flags, and decodes the result. GERO_* and the user file are isolated per test.
func configShow(t *testing.T, file string, env map[string]string, flags ...string) shown {
	t.Helper()
	for _, k := range []string{"GERO_CONFIG", "GERO_CONFIG_DIR", "GERO_UPSTREAM", "GERO_OUTPUT", "CI", "GITHUB_ACTIONS"} {
		t.Setenv(k, "")
	}
	if file != "" {
		p := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(p, []byte(file), 0o600))
		t.Setenv("GERO_CONFIG", p)
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	args := append([]string{"config", "show", "--json"}, flags...)
	stdout, _, err := execRoot(t, args...)
	require.NoError(t, err)
	var got shown
	require.NoError(t, json.Unmarshal([]byte(stdout), &got), stdout)
	return got
}

func TestConfigShowUserFile(t *testing.T) {
	got := configShow(t, "upstream: rancher\nfork: origin\nbranchPrefix: dan/\n", nil)

	assert.Equal(t, "rancher", got["upstream"].Value)
	assert.Equal(t, "file", got["upstream"].Source)
	assert.Equal(t, "origin", got["fork"].Value)
	assert.Equal(t, "dan/", got["branchPrefix"].Value)
}

func TestConfigShowPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		env        map[string]string
		flags      []string
		wantValue  string
		wantSource string
	}{
		{"file only", "upstream: f\n", nil, nil, "f", "file"},
		{"env beats file", "upstream: f\n", map[string]string{"GERO_UPSTREAM": "e"}, nil, "e", "env"},
		{"flag beats env", "upstream: f\n", map[string]string{"GERO_UPSTREAM": "e"}, []string{"--upstream", "x"}, "x", "flag"},
		{"flag beats file", "upstream: f\n", nil, []string{"--upstream=x"}, "x", "flag"},
		{"nothing set", "", nil, nil, "", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := configShow(t, tt.file, tt.env, tt.flags...)

			assert.Equal(t, tt.wantValue, got["upstream"].Value)
			assert.Equal(t, tt.wantSource, got["upstream"].Source)
		})
	}
}

func TestConfigShowGlobalFlags(t *testing.T) {
	got := configShow(t, "", nil, "--config-dir", "/cfg")

	assert.Equal(t, "/cfg", got["configDir"].Value)
	assert.Equal(t, "flag", got["configDir"].Source)
	// --json stands in for --output json.
	assert.Equal(t, "json", got["output"].Value)
	assert.Equal(t, "flag", got["output"].Source)
}

func TestConfigShowTextByDefault(t *testing.T) {
	t.Setenv("GERO_CONFIG", "")
	stdout, _, err := execRoot(t, "config", "show")

	require.NoError(t, err)
	assert.Regexp(t, `(?m)^output\s+text\s+default$`, stdout)
}

func TestConfigShowBadUserFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte("upstrem: x\n"), 0o600))
	t.Setenv("GERO_CONFIG", p)

	stdout, _, err := execRoot(t, "config", "show")

	require.Error(t, err)
	assert.Empty(t, stdout)
}
