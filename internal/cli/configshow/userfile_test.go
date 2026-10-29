package configshow

import (
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const userFilePath = "/home/dev/.config/gero/config.yaml"

// loadFrom loads a user file named userFilePath, with env as the environment.
func loadFrom(t *testing.T, content string, env map[string]string) clicfg.Settings {
	t.Helper()
	in := clicfg.Inputs{
		Getenv:       func(k string) string { return env[k] },
		UserFileName: userFilePath,
		Home:         "/home/dev",
	}
	if content != "" {
		in.UserFile = strings.NewReader(content)
	}
	s, err := clicfg.Load(in)
	require.NoError(t, err)
	return s
}

func TestEntriesStartWithTheUserFile(t *testing.T) {
	tests := []struct {
		name string
		s    clicfg.Settings
		want Entry
	}{
		{
			name: "loaded from the default location",
			s:    loadFrom(t, "output: json\n", nil),
			want: Entry{"userFile", userFilePath, "default"},
		},
		{
			name: "named by GERO_CONFIG",
			s:    loadFrom(t, "output: json\n", map[string]string{"GERO_CONFIG": userFilePath}),
			want: Entry{"userFile", userFilePath, "env"},
		},
		{
			name: "none applies",
			s:    loadFrom(t, "", nil),
			want: Entry{"userFile", "", "default"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := Entries(tt.s, workspace.ConfigDir{})

			require.NotEmpty(t, es)
			assert.Equal(t, tt.want, es[0], "it comes first, because the file sources below refer to it")
		})
	}
}

func TestRenderTextShowsTheUserFileOrNone(t *testing.T) {
	loaded, err := Render(loadFrom(t, "output: json\n", nil), workspace.ConfigDir{}, clicfg.OutputText)
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^userFile\s+`+userFilePath+`\s+default$`, loaded)

	none, err := Render(loadFrom(t, "", nil), workspace.ConfigDir{}, clicfg.OutputText)
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^userFile\s+\(none\)\s+default$`, none)
}

func TestRenderJSONHasTheUserFile(t *testing.T) {
	out, err := Render(loadFrom(t, "output: json\n", nil), workspace.ConfigDir{}, clicfg.OutputJSON)

	require.NoError(t, err)
	assert.Contains(t, out, `"userFile": {`)
	assert.Contains(t, out, `"value": "`+userFilePath+`"`)
}

func TestRenderEnvLeavesTheUserFileOut(t *testing.T) {
	out, err := Render(loadFrom(t, "output: json\n", nil), workspace.ConfigDir{}, clicfg.OutputEnv)

	require.NoError(t, err)
	assert.NotContains(t, out, userFilePath, "GERO_CONFIG is how the file is named, not a setting to export")
	assert.NotContains(t, strings.ToLower(out), "userfile")
}
