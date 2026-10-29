package configshow

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func load(t *testing.T, file string, env map[string]string, flags map[string]string) clicfg.Settings {
	t.Helper()
	in := clicfg.Inputs{
		Getenv: func(k string) string { return env[k] },
		Flags: func(n string) (string, bool) {
			v, ok := flags[n]
			return v, ok
		},
		Home: "/home/dev",
	}
	if file != "" {
		in.UserFile = strings.NewReader(file)
	}
	s, err := clicfg.Load(in)
	require.NoError(t, err)
	return s
}

func byKey(es []Entry) map[string]Entry {
	m := map[string]Entry{}
	for _, e := range es {
		m[e.Key] = e
	}
	return m
}

func TestEntries(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		env   map[string]string
		flags map[string]string
		want  map[string]Entry
	}{
		{
			name: "defaults only",
			want: map[string]Entry{
				"output":   {"output", "text", "default"},
				"upstream": {"upstream", "", "default"},
				"fork":     {"fork", "", "default"},
			},
		},
		{
			name: "user file values",
			file: "upstream: rancher\nfork: origin\nbranchPrefix: dan/\nconfigDir: ~/cfg\n",
			want: map[string]Entry{
				"upstream":     {"upstream", "rancher", "file"},
				"fork":         {"fork", "origin", "file"},
				"branchPrefix": {"branchPrefix", "dan/", "file"},
				"configDir":    {"configDir", "/home/dev/cfg", "file"},
				"output":       {"output", "text", "default"},
			},
		},
		{
			name:  "env beats file, flag beats env",
			file:  "upstream: fromfile\noutput: text\n",
			env:   map[string]string{"GERO_UPSTREAM": "fromenv", "GERO_OUTPUT": "json"},
			flags: map[string]string{"upstream": "fromflag"},
			want: map[string]Entry{
				"upstream": {"upstream", "fromflag", "flag"},
				"output":   {"output", "json", "env"},
			},
		},
		{
			name: "repos sections",
			file: "repos:\n  search: [/src]\n  paths:\n    rancher/rancher: /r\n  overrides:\n    rancher/scc-operator:\n      upstream: rancher\n",
			want: map[string]Entry{
				"repos.search":                                  {"repos.search", "/src", "file"},
				"repos.paths.rancher/rancher":                   {"repos.paths.rancher/rancher", "/r", "file"},
				"repos.overrides.rancher/scc-operator.upstream": {"repos.overrides.rancher/scc-operator.upstream", "rancher", "file"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := byKey(Entries(load(t, tt.file, tt.env, tt.flags), workspace.ConfigDir{}))

			for k, want := range tt.want {
				assert.Equal(t, want, got[k], k)
			}
		})
	}
}

func TestEntriesOrderIsStable(t *testing.T) {
	s := load(t, "repos:\n  paths:\n    b/b: /b\n    a/a: /a\n", nil, nil)

	first := Entries(s, workspace.ConfigDir{})

	assert.NotEmpty(t, first)
	for range 5 {
		assert.Equal(t, first, Entries(s, workspace.ConfigDir{}))
	}
}

func TestRenderText(t *testing.T) {
	s := load(t, "fork: origin\n", nil, map[string]string{"upstream": "rancher"})

	out, err := Render(s, workspace.ConfigDir{}, clicfg.OutputText)

	require.NoError(t, err)
	assert.Regexp(t, `(?m)^upstream\s+rancher\s+flag$`, out)
	assert.Regexp(t, `(?m)^fork\s+origin\s+file$`, out)
	assert.Regexp(t, `(?m)^output\s+text\s+default$`, out)
	assert.Regexp(t, `(?m)^configDir\s+\(none; \./config in a central checkout\)\s+default$`, out)
}

func TestRenderJSON(t *testing.T) {
	s := load(t, "fork: origin\n", map[string]string{"GERO_OUTPUT": "json"}, nil)

	out, err := Render(s, workspace.ConfigDir{}, clicfg.OutputJSON)

	require.NoError(t, err)
	var got map[string]struct{ Value, Source string }
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	assert.Equal(t, "origin", got["fork"].Value)
	assert.Equal(t, "file", got["fork"].Source)
	assert.Equal(t, "json", got["output"].Value)
	assert.Equal(t, "env", got["output"].Source)
}

func TestRenderEnv(t *testing.T) {
	s := load(t, "upstream: rancher\n", nil, nil)

	out, err := Render(s, workspace.ConfigDir{}, clicfg.OutputEnv)

	require.NoError(t, err)
	assert.Contains(t, out, "GERO_UPSTREAM=rancher\n")
	assert.Contains(t, out, "GERO_OUTPUT=text\n")
}

func TestRenderUnknownFormat(t *testing.T) {
	_, err := Render(clicfg.Settings{}, workspace.ConfigDir{}, "xml")

	assert.Error(t, err)
}

func TestEntriesResolvedConfigDir(t *testing.T) {
	resolved := workspace.ConfigDir{Dir: "/work/config", Source: workspace.SourceCwd}

	t.Run("resolved directory replaces the unset setting", func(t *testing.T) {
		got := byKey(Entries(load(t, "", nil, nil), resolved))

		assert.Equal(t, Entry{"configDir", "/work/config", "cwd"}, got["configDir"])
	})

	t.Run("zero result keeps the setting as loaded", func(t *testing.T) {
		got := byKey(Entries(load(t, "configDir: /file\n", nil, nil), workspace.ConfigDir{}))

		assert.Equal(t, Entry{"configDir", "/file", "file"}, got["configDir"])
	})

	t.Run("other entries are untouched", func(t *testing.T) {
		got := byKey(Entries(load(t, "upstream: rancher\n", nil, nil), resolved))

		assert.Equal(t, Entry{"upstream", "rancher", "file"}, got["upstream"])
	})
}

func TestRenderResolvedConfigDir(t *testing.T) {
	resolved := workspace.ConfigDir{Dir: "/work/config", Source: workspace.SourceCwd}
	s := load(t, "", nil, nil)

	text, err := Render(s, resolved, clicfg.OutputText)
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^configDir\s+/work/config\s+cwd$`, text)

	env, err := Render(s, resolved, clicfg.OutputEnv)
	require.NoError(t, err)
	assert.Contains(t, env, "GERO_CONFIG_DIR=/work/config\n")
}

func TestRenderTextHintsForUnsetKeys(t *testing.T) {
	out, err := Render(load(t, "", nil, nil), workspace.ConfigDir{}, clicfg.OutputText)

	require.NoError(t, err)
	assert.Regexp(t, `(?m)^upstream\s+\(inferred from remotes\)\s+default$`, out)
	assert.Regexp(t, `(?m)^fork\s+\(inferred from remotes\)\s+default$`, out)
	assert.Regexp(t, `(?m)^githubUser\s+\(inferred from the fork remote\)\s+default$`, out)
	assert.Regexp(t, `(?m)^branchPrefix\s+\(none\)\s+default$`, out)
	assert.NotContains(t, out, "(unset)")
}

func TestRenderTextHintsOnlyWhereNothingIsSet(t *testing.T) {
	s := load(t, "upstream: rancher\n", nil, map[string]string{"config-dir": "/flag"})

	out, err := Render(s, workspace.ConfigDir{}, clicfg.OutputText)

	require.NoError(t, err)
	assert.Regexp(t, `(?m)^upstream\s+rancher\s+file$`, out)
	assert.Regexp(t, `(?m)^configDir\s+/flag\s+flag$`, out)
}

func TestRenderHintsStayOutOfMachineFormats(t *testing.T) {
	s := load(t, "", nil, nil)

	js, err := Render(s, workspace.ConfigDir{}, clicfg.OutputJSON)
	require.NoError(t, err)
	var got map[string]struct{ Value, Source string }
	require.NoError(t, json.Unmarshal([]byte(js), &got), js)
	assert.Empty(t, got["upstream"].Value)
	assert.Equal(t, "default", got["upstream"].Source)

	env, err := Render(s, workspace.ConfigDir{}, clicfg.OutputEnv)
	require.NoError(t, err)
	assert.Contains(t, env, "GERO_UPSTREAM=\n")
	assert.NotContains(t, env, "inferred")
}

func TestRenderTextPointsAtResolveWhenReposSearchIsSet(t *testing.T) {
	s := load(t, "repos:\n  search:\n    - /work/suse\n", nil, nil)

	out, err := Render(s, workspace.ConfigDir{}, clicfg.OutputText)

	require.NoError(t, err)
	assert.Regexp(t, `(?m)^repos\.search\s+/work/suse\s+file$`, out)
	assert.True(t, strings.HasSuffix(out, "\nrun `gero config resolve repos` to see where each repo resolves\n"), out)
}

func TestRenderTextHasNoResolveHintWithoutReposSearch(t *testing.T) {
	out, err := Render(load(t, "repos:\n  paths:\n    rancher/rancher: /src/rancher\n", nil, nil), workspace.ConfigDir{}, clicfg.OutputText)

	require.NoError(t, err)
	assert.NotContains(t, out, "config resolve")
}

func TestRenderResolveHintStaysOutOfMachineFormats(t *testing.T) {
	s := load(t, "repos:\n  search:\n    - /work/suse\n", nil, nil)

	for _, format := range []string{clicfg.OutputJSON, clicfg.OutputEnv} {
		out, err := Render(s, workspace.ConfigDir{}, format)

		require.NoError(t, err)
		assert.NotContains(t, out, "config resolve", format)
	}
}
