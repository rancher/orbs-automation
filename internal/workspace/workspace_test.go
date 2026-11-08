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

// geroDoc is a file with a document of gero's.
var geroDoc = &fstest.MapFile{Data: []byte("apiVersion: gero/v1alpha2\nkind: Project\nname: a\n")}

var centralRepo = fstest.MapFS{"config/rancher.yaml": geroDoc}

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
		want  CentralConfigDir
	}{
		{"flag", "", nil, map[string]string{"central-config-dir": "/flag"}, nil, CentralConfigDir{"/flag", "flag"}},
		{"env", "", map[string]string{"GERO_CENTRAL_CONFIG_DIR": "/env"}, nil, nil, CentralConfigDir{"/env", "env"}},
		{"user file", "centralConfigDir: /file\n", nil, nil, nil, CentralConfigDir{"/file", "file"}},
		{
			"flag beats env and file",
			"centralConfigDir: /file\n", map[string]string{"GERO_CENTRAL_CONFIG_DIR": "/env"}, map[string]string{"central-config-dir": "/flag"}, nil,
			CentralConfigDir{"/flag", "flag"},
		},
		{"explicit beats cwd", "centralConfigDir: /file\n", nil, nil, centralRepo, CentralConfigDir{"/file", "file"}},
		{"cwd checkout of the central repo", "", nil, nil, centralRepo, CentralConfigDir{cwd + "/config", SourceCwd}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := open(settings(t, tt.file, tt.env, tt.flags), cwd, tt.fsys)

			got, err := w.CentralConfigDir()

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
		{"config dir without a file of gero's", fstest.MapFS{"config/app.yaml": {Data: []byte("port: 80\n")}}},
		{"a file of gero's outside config", fstest.MapFS{"rancher.yaml": geroDoc}},
		{"nested too deep", fstest.MapFS{"config/sub/rancher.yaml": geroDoc}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := open(settings(t, "", nil, nil), cwd, tt.fsys)

			got, err := w.CentralConfigDir()

			require.ErrorIs(t, err, ErrNotFound)
			assert.Equal(t, CentralConfigDir{}, got)
			for _, place := range []string{"--central-config-dir", "GERO_CENTRAL_CONFIG_DIR", "centralConfigDir", "./config"} {
				assert.Contains(t, err.Error(), place)
			}
		})
	}
}

func TestConfigDirNilFS(t *testing.T) {
	_, err := open(settings(t, "", nil, nil), cwd, nil).CentralConfigDir()

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

	got, err := Open(settings(t, "", nil, nil), dir).CentralConfigDir()

	require.NoError(t, err)
	assert.Equal(t, CentralConfigDir{Dir: filepath.Join(dir, "config"), Source: SourceCwd}, got)
}

func mkProjectFile(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config", "rancher.yaml"), geroDoc.Data, 0o600)
}
