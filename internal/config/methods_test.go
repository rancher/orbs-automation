package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func yes() *bool { b := true; return &b }
func no() *bool  { b := false; return &b }

func TestAllowedParamIsRequired(t *testing.T) {
	assert.True(t, AllowedParam{Key: "k"}.IsRequired(), "required unless there is a default")
	assert.False(t, AllowedParam{Key: "k", Default: "{tag}"}.IsRequired(), "a default makes it optional")
	assert.False(t, AllowedParam{Key: "k", Required: no()}.IsRequired(), "or saying so")
	assert.True(t, AllowedParam{Key: "k", Required: yes()}.IsRequired())
	assert.True(t, AllowedParam{Key: "k", Default: "x", Required: yes()}.IsRequired(), "an explicit required wins over a default")
}

func TestStepAppliesTo(t *testing.T) {
	tests := []struct {
		name         string
		since, until string
		line         string
		want         bool
	}{
		{"no range always runs", "", "", "v2.12", true},
		{"since: the line itself", "v2.16", "", "v2.16", true},
		{"since: a newer line", "v2.16", "", "v2.17", true},
		{"since: an older line", "v2.16", "", "v2.15", false},
		{"until: the line itself", "", "v2.15", "v2.15", true},
		{"until: an older line", "", "v2.15", "v2.12", true},
		{"until: a newer line", "", "v2.15", "v2.16", false},
		{"a range holds its ends", "v2.13", "v2.15", "v2.13", true},
		{"a range holds its other end", "v2.13", "v2.15", "v2.15", true},
		{"a range holds the middle", "v2.13", "v2.15", "v2.14", true},
		{"a range excludes below", "v2.13", "v2.15", "v2.12", false},
		{"a range excludes above", "v2.13", "v2.15", "v2.16", false},
		{"numbers, not text", "v2.9", "", "v2.10", true},
		{"a line that is not a number applies", "v2.16", "", "main", true},
		{"a bound that is not a number applies", "latest", "", "v2.12", true},
		{"no line at all applies", "v2.16", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Step{Since: tt.since, Until: tt.until}.AppliesTo(tt.line))
		})
	}
}

func TestEdgeAcceptLines(t *testing.T) {
	lined := Project{Release: Release{Lines: []ProjectLine{{Line: "v2.16"}}}}
	assert.Equal(t, AcceptLinesUniversal, Edge{}.AcceptLines(Project{}), "a producer with no lines is universal")
	assert.Equal(t, AcceptLinesAll, Edge{}.AcceptLines(lined), "a producer with lines follows them")
	assert.Equal(t, AcceptLinesAll, Edge{Lines: AcceptLinesAll}.AcceptLines(Project{}), "written wins")
	assert.Equal(t, AcceptLinesUniversal, Edge{Lines: AcceptLinesUniversal}.AcceptLines(lined))
}

func TestProjectLineOfBranch(t *testing.T) {
	p := Project{
		Repo: Repo{DefaultBranch: "main"},
		Release: Release{Lines: []ProjectLine{
			{Line: "v2.16"}, // the development line, on the default branch
			{Line: "v2.15", Branch: "release/v2.15"},
			{Line: "v2.14", Branch: "release/v2.14"},
		}},
	}
	assert.Equal(t, "v2.16", p.LineOfBranch("main"))
	assert.Equal(t, "v2.15", p.LineOfBranch("release/v2.15"))
	assert.Equal(t, "v2.14", p.LineOfBranch("release/v2.14"))
	assert.Equal(t, "", p.LineOfBranch("feature/x"))
	assert.Equal(t, "", Project{Repo: Repo{DefaultBranch: "main"}}.LineOfBranch("main"), "no lines")
}

func TestProjectMethod(t *testing.T) {
	p := Project{UpdateMethods: []UpdateMethod{{Name: "a"}, {Name: "b", Strategy: "yamlBump"}}}
	m, ok := p.Method("b")
	assert.True(t, ok)
	assert.Equal(t, "yamlBump", m.Strategy)
	_, ok = p.Method("nope")
	assert.False(t, ok)
}

func TestConfigLinks(t *testing.T) {
	rancher := Project{Name: "rancher", Repo: Repo{Name: "rancher/rancher"}, UpdateMethods: []UpdateMethod{{Name: "build-yaml", Strategy: "yamlBump"}}}
	charts := Project{Name: "charts", Repo: Repo{Name: "rancher/charts"}, UpdateMethods: []UpdateMethod{{Name: "pkg"}}}
	scc := Project{Name: "scc-operator", Downstream: []Edge{
		{To: "rancher", Method: "build-yaml", Params: map[string]string{"updateKey": "k"}},
		{To: "rancher/charts", Method: "pkg"}, // by slug
		{To: "nobody", Method: "x"},           // no such consumer: validation rejects it, Links skips it
		{To: "rancher", Method: "missing"},    // no such method
	}}
	cfg := Config{Projects: []Project{scc, rancher, charts}}

	got := cfg.Links(scc)

	assert.Len(t, got, 2)
	assert.Equal(t, "rancher", got[0].Consumer.Name)
	assert.Equal(t, "yamlBump", got[0].Method.Strategy)
	assert.Equal(t, "k", got[0].Edge.Params["updateKey"])
	assert.Equal(t, "charts", got[1].Consumer.Name, "an edge may name its consumer by slug")
}

func TestConfigLinksOfAProjectThatShipsNowhere(t *testing.T) {
	assert.Empty(t, Config{Projects: []Project{{Name: "a"}}}.Links(Project{Name: "a"}))
}
