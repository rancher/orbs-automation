package clicfg

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRecordsTheUserFileItRead(t *testing.T) {
	s, err := Load(Inputs{
		UserFile:     strings.NewReader("output: json\n"),
		UserFileName: "/home/dev/.config/gero/config.yaml",
		Home:         "/home/dev",
	})

	require.NoError(t, err)
	assert.Equal(t, "/home/dev/.config/gero/config.yaml", s.UserFile)
	assert.Equal(t, SourceDefault, s.Source("userFile"), "found at the default location")
}

func TestLoadUserFileNamedByGeroConfigComesFromTheEnvironment(t *testing.T) {
	env := map[string]string{"GERO_CONFIG": "/elsewhere/gero.yaml"}

	s, err := Load(Inputs{
		Getenv:       func(k string) string { return env[k] },
		UserFile:     strings.NewReader("output: json\n"),
		UserFileName: "/elsewhere/gero.yaml",
	})

	require.NoError(t, err)
	assert.Equal(t, "/elsewhere/gero.yaml", s.UserFile)
	assert.Equal(t, SourceEnv, s.Source("userFile"))
}

func TestLoadWithNoUserFileRecordsNone(t *testing.T) {
	s, err := Load(Inputs{UserFileName: "/home/dev/.config/gero/config.yaml"}) // looked at, not there

	require.NoError(t, err)
	assert.Empty(t, s.UserFile, "the path that was looked at is not a file that was loaded")
	assert.Equal(t, SourceDefault, s.Source("userFile"))
}
