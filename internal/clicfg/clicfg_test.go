package clicfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func flags(m map[string]string) FlagLookup {
	return func(name string) (string, bool) {
		v, ok := m[name]
		return v, ok
	}
}

func TestEnvAndFlagNames(t *testing.T) {
	tests := []struct {
		key  string
		env  string
		flag string
	}{
		{"configDir", "GERO_CONFIG_DIR", "config-dir"},
		{"output", "GERO_OUTPUT", "output"},
		{"upstream", "GERO_UPSTREAM", "upstream"},
		{"githubUser", "GERO_GITHUB_USER", "github-user"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.env, EnvName(tt.key))
			assert.Equal(t, tt.flag, FlagName(tt.key))
		})
	}
}

func TestSourceString(t *testing.T) {
	assert.Equal(t, "default", SourceDefault.String())
	assert.Equal(t, "file", SourceFile.String())
	assert.Equal(t, "env", SourceEnv.String())
	assert.Equal(t, "flag", SourceFlag.String())
}

func TestLoadDefaults(t *testing.T) {
	s, err := Load(Inputs{})

	require.NoError(t, err)
	assert.Equal(t, OutputText, s.Output)
	assert.Empty(t, s.ConfigDir)
	assert.Empty(t, s.Upstream)
	assert.Equal(t, SourceDefault, s.Source("output"))
	assert.Equal(t, SourceDefault, s.Source("upstream"))
}

func TestLoadPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		flags      map[string]string
		wantRemote string
		wantSource Source
	}{
		{"nothing set", nil, nil, "", SourceDefault},
		{"env", map[string]string{"GERO_UPSTREAM": "rancher"}, nil, "rancher", SourceEnv},
		{"flag", nil, map[string]string{"upstream": "flagremote"}, "flagremote", SourceFlag},
		{"flag beats env", map[string]string{"GERO_UPSTREAM": "rancher"}, map[string]string{"upstream": "flagremote"}, "flagremote", SourceFlag},
		{"explicitly empty flag still counts as set", map[string]string{"GERO_UPSTREAM": "rancher"}, map[string]string{"upstream": ""}, "", SourceFlag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Load(Inputs{Getenv: env(tt.env), Flags: flags(tt.flags)})

			require.NoError(t, err)
			assert.Equal(t, tt.wantRemote, s.Upstream)
			assert.Equal(t, tt.wantSource, s.Source("upstream"))
		})
	}
}

func TestLoadOutputFromEnvAndFlag(t *testing.T) {
	s, err := Load(Inputs{Getenv: env(map[string]string{"GERO_OUTPUT": "json"})})
	require.NoError(t, err)
	assert.Equal(t, OutputJSON, s.Output)

	s, err = Load(Inputs{
		Getenv: env(map[string]string{"GERO_OUTPUT": "json"}),
		Flags:  flags(map[string]string{"output": "text"}),
	})
	require.NoError(t, err)
	assert.Equal(t, OutputText, s.Output)
}
