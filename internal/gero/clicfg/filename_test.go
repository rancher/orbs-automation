package clicfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An error caused by a value in the user file names the file and the key, so a
// developer knows where to look. YAML errors already did; these are the errors
// found after the file parsed.
func TestLoadNamesTheFileForEveryRejectedValue(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		home string
		key  string
	}{
		{"invalid output", "output: xml\n", "", "output"},
		{"relative centralConfigDir", "centralConfigDir: rel/dir\n", "", "centralConfigDir"},
		{"tilde without a home", "centralConfigDir: ~/cfg\n", "", "centralConfigDir"},
		{"relative search path", "repos:\n  search: [rel]\n", "", "repos.search"},
		{"relative repo path", "repos:\n  paths:\n    rancher/rancher: rel\n", "", "repos.paths.rancher/rancher"},
		{"bad slug in paths", "repos:\n  paths:\n    rancher: /x\n", "", "repos.paths"},
		{"bad slug in overrides", "repos:\n  overrides:\n    rancher:\n      upstream: u\n", "", "repos.overrides"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(Inputs{UserFile: file(tt.yaml), UserFileName: "/home/dev/gero.yaml", Home: tt.home})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "/home/dev/gero.yaml", "the file")
			assert.Contains(t, err.Error(), tt.key, "the key")
		})
	}
}

func TestLoadNamesAnUnnamedFileAsTheUserConfig(t *testing.T) {
	_, err := Load(Inputs{UserFile: file("output: xml\n")})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "user config")
}

// A bad value from the environment or a flag is not the file's fault, even when
// a file was loaded, so the message must not send the developer to it.
func TestLoadDoesNotBlameTheFileForTheEnvironmentOrAFlag(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		_, err := Load(Inputs{
			UserFile:     file("upstream: rancher\n"),
			UserFileName: "/home/dev/gero.yaml",
			Getenv:       env(map[string]string{"GERO_OUTPUT": "xml"}),
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "output (from env)")
		assert.NotContains(t, err.Error(), "/home/dev/gero.yaml")
	})

	t.Run("flag", func(t *testing.T) {
		_, err := Load(Inputs{
			UserFile:     file("upstream: rancher\n"),
			UserFileName: "/home/dev/gero.yaml",
			Flags:        flags(map[string]string{"output": "xml"}),
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "output (from flag)")
		assert.NotContains(t, err.Error(), "/home/dev/gero.yaml")
	})

	t.Run("a relative path from the environment", func(t *testing.T) {
		_, err := Load(Inputs{
			UserFile:     file("upstream: rancher\n"),
			UserFileName: "/home/dev/gero.yaml",
			Getenv:       env(map[string]string{"GERO_CENTRAL_CONFIG_DIR": "~/x"}), // no home to expand
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "centralConfigDir (from env)")
		assert.NotContains(t, err.Error(), "/home/dev/gero.yaml")
	})
}
