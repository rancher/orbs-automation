package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
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

func TestConfigDir(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		env   map[string]string
		flags map[string]string
		fsys  fstest.MapFS
		want  ConfigDir
	}{
		{"flag", "", nil, map[string]string{"config-dir": "/flag"}, nil, ConfigDir{"/flag", "flag"}},
		{"env", "", map[string]string{"GERO_CONFIG_DIR": "/env"}, nil, nil, ConfigDir{"/env", "env"}},
		{"user file", "configDir: /file\n", nil, nil, nil, ConfigDir{"/file", "file"}},
		{
			"flag beats env and file",
			"configDir: /file\n", map[string]string{"GERO_CONFIG_DIR": "/env"}, map[string]string{"config-dir": "/flag"}, nil,
			ConfigDir{"/flag", "flag"},
		},
		{"explicit beats cwd", "configDir: /file\n", nil, nil, centralRepo, ConfigDir{"/file", "file"}},
		{"cwd checkout of the central repo", "", nil, nil, centralRepo, ConfigDir{cwd + "/config", SourceCwd}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := open(settings(t, tt.file, tt.env, tt.flags), cwd, tt.fsys)

			got, err := w.ConfigDir()

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfigDirNotFound(t *testing.T) {
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
			w := open(settings(t, "", nil, nil), cwd, tt.fsys)

			got, err := w.ConfigDir()

			require.ErrorIs(t, err, ErrNotFound)
			assert.Equal(t, ConfigDir{}, got)
			for _, place := range []string{"--config-dir", "GERO_CONFIG_DIR", "configDir", "./config"} {
				assert.Contains(t, err.Error(), place)
			}
		})
	}
}

func TestConfigDirNilFS(t *testing.T) {
	_, err := open(settings(t, "", nil, nil), cwd, nil).ConfigDir()

	assert.ErrorIs(t, err, ErrNotFound)
}

// Open must not touch the file system: a command that never asks for the
// config directory must not fail because the working directory is odd.
func TestOpenDoesNoIO(t *testing.T) {
	w := Open(settings(t, "", nil, nil), "/does/not/exist")

	require.NotNil(t, w)
}

func TestOpenReadsWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, mkProjectFile(dir))

	got, err := Open(settings(t, "", nil, nil), dir).ConfigDir()

	require.NoError(t, err)
	assert.Equal(t, ConfigDir{Dir: filepath.Join(dir, "config"), Source: SourceCwd}, got)
}

func mkProjectFile(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config", "rancher-rancher.project.yaml"), nil, 0o600)
}
