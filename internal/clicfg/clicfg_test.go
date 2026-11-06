package clicfg

import (
	"io"
	"os"
	"path/filepath"
	"strings"
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

func file(s string) io.Reader { return strings.NewReader(s) }

func TestLoadFileLayer(t *testing.T) {
	const yml = "upstream: fromfile\noutput: json\nconfigDir: /etc/gero\n"

	s, err := Load(Inputs{UserFile: file(yml)})
	require.NoError(t, err)
	assert.Equal(t, "fromfile", s.Upstream)
	assert.Equal(t, OutputJSON, s.Output)
	assert.Equal(t, "/etc/gero", s.ConfigDir)
	assert.Equal(t, SourceFile, s.Source("upstream"))

	s, err = Load(Inputs{UserFile: file(yml), Getenv: env(map[string]string{"GERO_UPSTREAM": "fromenv"})})
	require.NoError(t, err)
	assert.Equal(t, "fromenv", s.Upstream)
	assert.Equal(t, SourceEnv, s.Source("upstream"))

	s, err = Load(Inputs{
		UserFile: file(yml),
		Getenv:   env(map[string]string{"GERO_UPSTREAM": "fromenv"}),
		Flags:    flags(map[string]string{"upstream": "fromflag"}),
	})
	require.NoError(t, err)
	assert.Equal(t, "fromflag", s.Upstream)
	assert.Equal(t, SourceFlag, s.Source("upstream"))
}

func TestLoadFileOnlyKeys(t *testing.T) {
	s, err := Load(Inputs{UserFile: file(`
fork: origin
githubUser: danpock
branchPrefix: dan/
pushTo: fork
repos:
  search:
    - /work/suse
  paths:
    rancher/rancher: /work/suse/rancher-src
  overrides:
    rancher/scc-operator:
      upstream: rancher
      pushTo: upstream
`)})

	require.NoError(t, err)
	assert.Equal(t, "origin", s.Fork)
	assert.Equal(t, "danpock", s.GitHubUser)
	assert.Equal(t, "dan/", s.BranchPrefix)
	assert.Equal(t, "fork", s.PushTo)
	assert.Equal(t, SourceFile, s.Source("pushTo"))
	assert.Equal(t, SourceFile, s.Source("fork"))
	assert.Equal(t, SourceDefault, s.Source("nonexistent"))
	assert.Equal(t, []string{"/work/suse"}, s.Repos.Search)
	assert.Equal(t, map[string]string{"rancher/rancher": "/work/suse/rancher-src"}, s.Repos.Paths)
	assert.Equal(t, map[string]RepoOverride{"rancher/scc-operator": {Upstream: "rancher", PushTo: "upstream"}}, s.Repos.Overrides)
}

func TestLoadEmptyFile(t *testing.T) {
	s, err := Load(Inputs{UserFile: file("")})

	require.NoError(t, err)
	assert.Equal(t, OutputText, s.Output)
}

func TestLoadFileErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{"unknown key", "upstrem: x\n", "upstrem"},
		{"unknown nested key", "repos:\n  root: /x\n", "root"},
		{"malformed", "upstream: [\n", "gero-test.yaml"},
		{"invalid output", "output: xml\n", "output"},
		{"relative configDir", "configDir: rel/dir\n", "absolute"},
		{"relative search path", "repos:\n  search: [rel]\n", "repos.search"},
		{"relative repo path", "repos:\n  paths:\n    rancher/rancher: rel\n", "repos.paths.rancher/rancher"},
		{"bad slug in paths", "repos:\n  paths:\n    rancher: /x\n", "owner/name"},
		{"bad slug in overrides", "repos:\n  overrides:\n    rancher:\n      upstream: u\n", "owner/name"},
		{"pushTo that is neither role", "pushTo: forks\n", "pushTo"},
		{"per-repo pushTo that is neither role", "repos:\n  overrides:\n    rancher/rancher:\n      pushTo: origin\n", "repos.overrides.rancher/rancher.pushTo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(Inputs{UserFile: file(tt.yaml), UserFileName: "gero-test.yaml"})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoadValidatesOutputFromEnvAndFlag(t *testing.T) {
	_, err := Load(Inputs{Getenv: env(map[string]string{"GERO_OUTPUT": "xml"})})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "env")

	_, err = Load(Inputs{Flags: flags(map[string]string{"output": "xml"})})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flag")
}

func TestLoadExpandsPaths(t *testing.T) {
	s, err := Load(Inputs{
		Home:     "/home/dan",
		Getenv:   env(map[string]string{"WORK": "/w"}),
		UserFile: file("configDir: ~/cfg\nrepos:\n  search: [\"$WORK/suse\", \"~\"]\n  paths:\n    rancher/rancher: ~/r\n"),
	})

	require.NoError(t, err)
	assert.Equal(t, "/home/dan/cfg", s.ConfigDir)
	assert.Equal(t, []string{"/w/suse", "/home/dan"}, s.Repos.Search)
	assert.Equal(t, "/home/dan/r", s.Repos.Paths["rancher/rancher"])
}

func TestLoadTildeWithoutHome(t *testing.T) {
	_, err := Load(Inputs{UserFile: file("configDir: ~/cfg\n")})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "home")
}

func TestLoadExpandsFlagAndEnvPathsButAllowsRelative(t *testing.T) {
	s, err := Load(Inputs{Home: "/home/dan", Flags: flags(map[string]string{"config-dir": "~/cfg"})})
	require.NoError(t, err)
	assert.Equal(t, "/home/dan/cfg", s.ConfigDir)

	s, err = Load(Inputs{Getenv: env(map[string]string{"GERO_CONFIG_DIR": "./config"})})
	require.NoError(t, err)
	assert.Equal(t, "./config", s.ConfigDir)
}

func TestInCI(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"empty environment", nil, false},
		{"CI=true", map[string]string{"CI": "true"}, true},
		{"GITHUB_ACTIONS=true", map[string]string{"GITHUB_ACTIONS": "true"}, true},
		{"CI=false", map[string]string{"CI": "false"}, false},
		{"GERO_CONTEXT=ci forces CI", map[string]string{"GERO_CONTEXT": "ci"}, true},
		{"GERO_CONTEXT=workstation beats CI=true", map[string]string{"GERO_CONTEXT": "workstation", "CI": "true"}, false},
		{"GERO_CONTEXT=workstation beats GITHUB_ACTIONS=true", map[string]string{"GERO_CONTEXT": "workstation", "GITHUB_ACTIONS": "true"}, false},
		{"GERO_CONTEXT=ci beats CI=false", map[string]string{"GERO_CONTEXT": "ci", "CI": "false"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InCI(env(tt.env))

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInCIRejectsUnknownContext(t *testing.T) {
	_, err := InCI(env(map[string]string{"GERO_CONTEXT": "laptop"}))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GERO_CONTEXT")
	assert.Contains(t, err.Error(), `"laptop"`)
	assert.Contains(t, err.Error(), ContextCI)
	assert.Contains(t, err.Error(), ContextWorkstation)
}

func TestLoadRecordsContext(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"workstation by default", nil, false},
		{"CI=true", map[string]string{"CI": "true"}, true},
		{"GERO_CONTEXT=ci", map[string]string{"GERO_CONTEXT": "ci"}, true},
		{"GERO_CONTEXT=workstation beats CI=true", map[string]string{"GERO_CONTEXT": "workstation", "CI": "true"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Load(Inputs{Getenv: env(tt.env)})

			require.NoError(t, err)
			assert.Equal(t, tt.want, s.CI)
		})
	}
}

func TestLoadRejectsUnknownContext(t *testing.T) {
	_, err := Load(Inputs{Getenv: env(map[string]string{"GERO_CONTEXT": "laptop"})})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GERO_CONTEXT")
}

func TestOpenUserFileFollowsForcedContext(t *testing.T) {
	t.Run("workstation forced in a CI environment loads the file", func(t *testing.T) {
		home := writeUserFile(t)

		r, _, err := OpenUserFile(env(map[string]string{"CI": "true", "GERO_CONTEXT": "workstation"}), home)

		require.NoError(t, err)
		require.NotNil(t, r)
		defer r.Close()
	})

	t.Run("CI forced on a workstation ignores the file", func(t *testing.T) {
		home := writeUserFile(t)

		r, _, err := OpenUserFile(env(map[string]string{"GERO_CONTEXT": "ci"}), home)

		require.NoError(t, err)
		assert.Nil(t, r)
	})

	t.Run("unknown context is an error", func(t *testing.T) {
		_, _, err := OpenUserFile(env(map[string]string{"GERO_CONTEXT": "laptop"}), t.TempDir())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "GERO_CONTEXT")
	})
}

func TestUserFilePath(t *testing.T) {
	tests := []struct {
		name         string
		env          map[string]string
		wantPath     string
		wantExplicit bool
	}{
		{"home default", nil, "/home/dan/.config/gero/config.yaml", false},
		{"xdg", map[string]string{"XDG_CONFIG_HOME": "/xdg"}, "/xdg/gero/config.yaml", false},
		{"explicit wins", map[string]string{"GERO_CONFIG": "/x/c.yaml", "XDG_CONFIG_HOME": "/xdg"}, "/x/c.yaml", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, explicit := UserFilePath(env(tt.env), "/home/dan")

			assert.Equal(t, tt.wantPath, path)
			assert.Equal(t, tt.wantExplicit, explicit)
		})
	}
}

// writeUserFile makes a home directory whose user file sets upstream.
func writeUserFile(t *testing.T) (home string) {
	t.Helper()
	home = t.TempDir()
	dir := filepath.Join(home, ".config", "gero")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("upstream: x\n"), 0o600))
	return home
}

func TestOpenUserFile(t *testing.T) {
	t.Run("present at default path", func(t *testing.T) {
		home := writeUserFile(t)

		r, path, err := OpenUserFile(env(nil), home)

		require.NoError(t, err)
		require.NotNil(t, r)
		defer r.Close()
		assert.Equal(t, filepath.Join(home, ".config", "gero", "config.yaml"), path)
		b, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, "upstream: x\n", string(b))
	})

	t.Run("absent at default path is fine", func(t *testing.T) {
		r, _, err := OpenUserFile(env(nil), t.TempDir())

		require.NoError(t, err)
		assert.Nil(t, r)
	})

	t.Run("CI ignores the default file", func(t *testing.T) {
		home := writeUserFile(t)

		r, _, err := OpenUserFile(env(map[string]string{"CI": "true"}), home)

		require.NoError(t, err)
		assert.Nil(t, r)
	})

	t.Run("CI honors explicit GERO_CONFIG", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "c.yaml")
		require.NoError(t, os.WriteFile(p, []byte("upstream: x\n"), 0o600))

		r, path, err := OpenUserFile(env(map[string]string{"CI": "true", "GERO_CONFIG": p}), t.TempDir())

		require.NoError(t, err)
		require.NotNil(t, r)
		defer r.Close()
		assert.Equal(t, p, path)
	})

	t.Run("explicit but missing is an error", func(t *testing.T) {
		_, _, err := OpenUserFile(env(map[string]string{"GERO_CONFIG": "/nope/c.yaml"}), t.TempDir())

		require.Error(t, err)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestLoadAcceptsEnvOutput(t *testing.T) {
	s, err := Load(Inputs{Getenv: env(map[string]string{"GERO_OUTPUT": "env"})})
	require.NoError(t, err)
	assert.Equal(t, OutputEnv, s.Output)

	s, err = Load(Inputs{UserFile: file("output: env\n")})
	require.NoError(t, err)
	assert.Equal(t, OutputEnv, s.Output)
}

func TestLoadBotUser(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		s, err := Load(Inputs{Getenv: env(nil)})

		require.NoError(t, err)
		assert.Empty(t, s.BotUser)
	})

	t.Run("GERO_BOT_USER is the bot login", func(t *testing.T) {
		s, err := Load(Inputs{Getenv: env(map[string]string{"GERO_BOT_USER": "rancher-scc-bot[bot]"})})

		require.NoError(t, err)
		assert.Equal(t, "rancher-scc-bot[bot]", s.BotUser)
	})

	t.Run("the user file cannot set it", func(t *testing.T) {
		_, err := Load(Inputs{Getenv: env(nil), UserFile: strings.NewReader("botUser: someone\n"), UserFileName: "config.yaml"})

		require.Error(t, err)
	})
}

func TestLoadDryRun(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		flags map[string]string
		want  bool
		src   Source
	}{
		{"off by default", nil, nil, false, SourceDefault},
		{"the flag", nil, map[string]string{"dry-run": "true"}, true, SourceFlag},
		{"the env, true", map[string]string{"GERO_DRY_RUN": "true"}, nil, true, SourceEnv},
		{"the env, 1", map[string]string{"GERO_DRY_RUN": "1"}, nil, true, SourceEnv},
		{"the env, false", map[string]string{"GERO_DRY_RUN": "false"}, nil, false, SourceEnv},
		{"the flag beats the env, on", map[string]string{"GERO_DRY_RUN": "false"}, map[string]string{"dry-run": "true"}, true, SourceFlag},
		{"the flag beats the env, off", map[string]string{"GERO_DRY_RUN": "true"}, map[string]string{"dry-run": "false"}, false, SourceFlag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Load(Inputs{Getenv: env(tt.env), Flags: flags(tt.flags)})
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.DryRun)
			assert.Equal(t, tt.src, s.Source("dryRun"))
		})
	}
}

func TestLoadDryRunRejectsAnythingElse(t *testing.T) {
	_, err := Load(Inputs{Getenv: env(map[string]string{"GERO_DRY_RUN": "maybe"})})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dryRun")
	assert.Contains(t, err.Error(), "maybe")

	_, err = Load(Inputs{Flags: flags(map[string]string{"dry-run": "sometimes"})})
	require.Error(t, err)
}

func TestLoadDryRunIsNotAUserFileKey(t *testing.T) {
	_, err := Load(Inputs{UserFile: strings.NewReader("dryRun: true\n"), UserFileName: "config.yaml"})
	require.Error(t, err, "a file that turned it on would turn it on for every run")
	assert.Contains(t, err.Error(), "dryRun")
}
