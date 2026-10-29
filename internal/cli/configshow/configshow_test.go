package configshow

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/cli/configdir"
	"github.com/rancher/orbs-automation/internal/clicfg"
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
			got := byKey(Entries(load(t, tt.file, tt.env, tt.flags), configdir.Result{}))

			for k, want := range tt.want {
				assert.Equal(t, want, got[k], k)
			}
		})
	}
}

func TestEntriesOrderIsStable(t *testing.T) {
	s := load(t, "repos:\n  paths:\n    b/b: /b\n    a/a: /a\n", nil, nil)

	first := Entries(s, configdir.Result{})

	assert.NotEmpty(t, first)
	for range 5 {
		assert.Equal(t, first, Entries(s, configdir.Result{}))
	}
}

func TestRenderText(t *testing.T) {
	s := load(t, "fork: origin\n", nil, map[string]string{"upstream": "rancher"})

	out, err := Render(s, configdir.Result{}, clicfg.OutputText)

	require.NoError(t, err)
	assert.Regexp(t, `(?m)^upstream\s+rancher\s+flag$`, out)
	assert.Regexp(t, `(?m)^fork\s+origin\s+file$`, out)
	assert.Regexp(t, `(?m)^output\s+text\s+default$`, out)
	assert.Regexp(t, `(?m)^configDir\s+\(unset\)\s+default$`, out)
}

func TestRenderJSON(t *testing.T) {
	s := load(t, "fork: origin\n", map[string]string{"GERO_OUTPUT": "json"}, nil)

	out, err := Render(s, configdir.Result{}, clicfg.OutputJSON)

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

	out, err := Render(s, configdir.Result{}, clicfg.OutputEnv)

	require.NoError(t, err)
	assert.Contains(t, out, "GERO_UPSTREAM=rancher\n")
	assert.Contains(t, out, "GERO_OUTPUT=text\n")
}

func TestRenderUnknownFormat(t *testing.T) {
	_, err := Render(clicfg.Settings{}, configdir.Result{}, "xml")

	assert.Error(t, err)
}
