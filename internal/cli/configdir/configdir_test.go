package configdir

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const cwd = "/work/orbs-automation"

var centralRepo = fstest.MapFS{"config/rancher-rancher.project.yaml": {}}

func settings(t *testing.T, file string, env, flags map[string]string) clicfg.Settings {
	t.Helper()
	in := clicfg.Inputs{
		Getenv: func(k string) string { return env[k] },
		Flags: func(n string) (string, bool) {
			v, ok := flags[n]
			return v, ok
		},
	}
	if file != "" {
		in.UserFile = strings.NewReader(file)
	}
	s, err := clicfg.Load(in)
	require.NoError(t, err)
	return s
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		env   map[string]string
		flags map[string]string
		fsys  fstest.MapFS
		want  Result
	}{
		{"flag", "", nil, map[string]string{"config-dir": "/flag"}, nil, Result{"/flag", "flag"}},
		{"env", "", map[string]string{"GERO_CONFIG_DIR": "/env"}, nil, nil, Result{"/env", "env"}},
		{"user file", "configDir: /file\n", nil, nil, nil, Result{"/file", "file"}},
		{
			"flag beats env and file",
			"configDir: /file\n", map[string]string{"GERO_CONFIG_DIR": "/env"}, map[string]string{"config-dir": "/flag"}, nil,
			Result{"/flag", "flag"},
		},
		{"explicit beats cwd", "configDir: /file\n", nil, nil, centralRepo, Result{"/file", "file"}},
		{"cwd checkout of the central repo", "", nil, nil, centralRepo, Result{cwd + "/config", SourceCwd}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(settings(t, tt.file, tt.env, tt.flags), cwd, tt.fsys)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveNotFound(t *testing.T) {
	tests := []struct {
		name string
		fsys fstest.MapFS
	}{
		{"empty working directory", fstest.MapFS{}},
		{"config dir without project files", fstest.MapFS{"config/rancher.lines.yaml": {}}},
		{"project file outside config", fstest.MapFS{"rancher-rancher.project.yaml": {}}},
		{"nested too deep", fstest.MapFS{"config/sub/rancher-rancher.project.yaml": {}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(settings(t, "", nil, nil), cwd, tt.fsys)

			require.ErrorIs(t, err, ErrNotFound)
			assert.Equal(t, Result{}, got)
			for _, place := range []string{"--config-dir", "GERO_CONFIG_DIR", "configDir", "./config"} {
				assert.Contains(t, err.Error(), place)
			}
		})
	}
}

func TestResolveNilFS(t *testing.T) {
	_, err := Resolve(settings(t, "", nil, nil), cwd, nil)

	assert.ErrorIs(t, err, ErrNotFound)
}
