package clicfg

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogNames(t *testing.T) {
	assert.Equal(t, "GERO_LOG", EnvName("log"))
	assert.Equal(t, "log", FlagName("log"))
}

func TestLoadLogDefault(t *testing.T) {
	s, err := Load(Inputs{})

	require.NoError(t, err)
	assert.Equal(t, LogWarn, s.LogLevel)
	assert.Equal(t, SourceDefault, s.Source("log"))
}

func TestLoadLogPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		flags      map[string]string
		want       string
		wantSource Source
	}{
		{"env", map[string]string{"GERO_LOG": "debug"}, nil, LogDebug, SourceEnv},
		{"flag", nil, map[string]string{"log": "info"}, LogInfo, SourceFlag},
		{"flag beats env", map[string]string{"GERO_LOG": "error"}, map[string]string{"log": "debug"}, LogDebug, SourceFlag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Load(Inputs{Getenv: env(tt.env), Flags: flags(tt.flags)})

			require.NoError(t, err)
			assert.Equal(t, tt.want, s.LogLevel)
			assert.Equal(t, tt.wantSource, s.Source("log"))
		})
	}
}

func TestLoadLogIgnoresUserFile(t *testing.T) {
	s, err := Load(Inputs{UserFile: strings.NewReader("upstream: origin\n")})

	require.NoError(t, err)
	assert.Equal(t, LogWarn, s.LogLevel)
}

func TestLoadLogRejectsUnknownLevel(t *testing.T) {
	_, err := Load(Inputs{Getenv: env(map[string]string{"GERO_LOG": "loud"})})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "log (from env)")
	assert.Contains(t, err.Error(), `"loud"`)
}
