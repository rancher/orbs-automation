package updatedispatcher

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rancherProject() config.Project {
	return config.Project{
		Name: "rancher", Repo: config.Repo{Name: "rancher/rancher", DefaultBranch: "main"},
		Release: config.Release{Branching: "perLine", Lines: []config.ProjectLine{
			{Line: "v2.16"}, {Line: "v2.15", Branch: "release/v2.15"},
		}},
		UpdateMethods: []config.UpdateMethod{{
			Name: "build-yaml-bump", Strategy: "yamlBump",
			With:          map[string]string{"file": "build.yaml"},
			AllowedParams: []config.AllowedParam{{Key: "updateKey"}, {Key: "value", Default: "{tag}"}},
		}},
	}
}

func sccProject() config.Project {
	return config.Project{
		Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator", Owned: true},
		Downstream: []config.Edge{{
			To: "rancher", Method: "build-yaml-bump",
			Params: map[string]string{"updateKey": "defaultSccOperatorImage", "value": "{image}:{tag}"},
		}},
	}
}

func fixtureConfig() config.Config {
	return config.Config{Projects: []config.Project{sccProject(), rancherProject()}}
}

func TestNewEngine(t *testing.T) {
	e, err := NewEngine(Builtin(), fixtureConfig())
	require.NoError(t, err)

	m, ok := e.Method("rancher", "build-yaml-bump")
	require.True(t, ok)
	assert.Equal(t, "rancher", m.Project)
	assert.Equal(t, "yamlBump", m.StrategyName)
	assert.NotNil(t, m.Updater, "the strategy is configured when the engine is built")
	_, ok = e.Method("scc-operator", "build-yaml-bump")
	assert.False(t, ok, "methods are per project: a name is the consumer's")
}

func TestEngineMethodNamesArePerProject(t *testing.T) {
	cfg := fixtureConfig()
	other := rancherProject()
	other.Name, other.Repo.Name = "charts", "rancher/charts"
	cfg.Projects = append(cfg.Projects, other)
	e, err := NewEngine(Builtin(), cfg)
	require.NoError(t, err)
	_, a := e.Method("rancher", "build-yaml-bump")
	_, b := e.Method("charts", "build-yaml-bump")
	assert.True(t, a && b)
}

func TestNewEngineProblems(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*config.Config)
		want   []string
	}{
		{"unknown strategy", func(c *config.Config) { c.Projects[1].UpdateMethods[0].Strategy = "buildYamlBump" },
			[]string{`Project "rancher"`, `method "build-yaml-bump"`, "buildYamlBump", "yamlBump"}},
		{"a typo in with", func(c *config.Config) {
			c.Projects[1].UpdateMethods[0].With = map[string]string{"file": "build.yaml", "fiel": "x"}
		}, []string{`method "build-yaml-bump"`, `"fiel"`}},
		{"no file", func(c *config.Config) { c.Projects[1].UpdateMethods[0].With = nil }, []string{"with.file"}},
		{"a required input not offered", func(c *config.Config) {
			c.Projects[1].UpdateMethods[0].AllowedParams = []config.AllowedParam{{Key: "value"}}
			c.Projects[0].Downstream[0].Params = map[string]string{"value": "x"}
		}, []string{"yamlBump needs the input", "updateKey"}},
		{"an edge param the method does not allow", func(c *config.Config) {
			c.Projects[0].Downstream[0].Params["nope"] = "x"
		}, []string{"downstream[0]", `"nope"`, "allows"}},
		{"a required param the edge omits", func(c *config.Config) {
			c.Projects[0].Downstream[0].Params = map[string]string{}
		}, []string{"downstream[0]", `"updateKey"`, "required"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := fixtureConfig()
			tt.mutate(&cfg)
			_, err := NewEngine(Builtin(), cfg)
			require.Error(t, err)
			for _, w := range tt.want {
				assert.Contains(t, err.Error(), w)
			}
		})
	}
}

func TestNewEngineReportsEveryProblem(t *testing.T) {
	cfg := fixtureConfig()
	cfg.Projects[1].UpdateMethods = append(cfg.Projects[1].UpdateMethods, config.UpdateMethod{Name: "bad", Strategy: "nope"})
	cfg.Projects[0].Downstream[0].Params["extra"] = "x"
	e, err := NewEngine(Builtin(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nope"`)
	assert.Contains(t, err.Error(), `"extra"`)
	_, ok := e.Method("rancher", "build-yaml-bump")
	assert.True(t, ok, "the methods that were fine are still there")
}

func TestRoutesFrom(t *testing.T) {
	cfg := fixtureConfig()
	cfg.Projects[0].Downstream = append(cfg.Projects[0].Downstream,
		config.Edge{To: "nobody", Method: "x"}, config.Edge{To: "rancher", Method: "missing"})
	e, _ := NewEngine(Builtin(), cfg)

	routes := e.RoutesFrom(cfg.Projects[0])

	require.Len(t, routes, 1)
	assert.Equal(t, "rancher", routes[0].Consumer.Name)
	assert.Equal(t, "build-yaml-bump", routes[0].Method.Name)
	assert.Equal(t, "defaultSccOperatorImage", routes[0].Edge.Params["updateKey"])
}

func TestRoutesFromNameTheConsumerBySlugToo(t *testing.T) {
	cfg := fixtureConfig()
	cfg.Projects[0].Downstream[0].To = "Rancher/Rancher"
	e, _ := NewEngine(Builtin(), cfg)
	assert.Len(t, e.RoutesFrom(cfg.Projects[0]), 1)
}

func TestRoutesFromAProjectThatShipsNowhere(t *testing.T) {
	e, _ := NewEngine(Builtin(), fixtureConfig())
	assert.Empty(t, e.RoutesFrom(rancherProject()))
}

func TestBind(t *testing.T) {
	e, _ := NewEngine(Builtin(), fixtureConfig())
	m, _ := e.Method("rancher", "build-yaml-bump")

	in, err := m.Bind(config.Edge{Params: map[string]string{"updateKey": "k"}})
	require.NoError(t, err)
	assert.Equal(t, Inputs{"updateKey": "k", "value": "{tag}"}, in, "the default of a param the edge leaves out")

	in, err = m.Bind(config.Edge{Params: map[string]string{"updateKey": "k", "value": "{image}:{tag}"}})
	require.NoError(t, err)
	assert.Equal(t, "{image}:{tag}", in["value"], "what the edge passes beats the default")
}

func TestBindDoesNotLetTheEdgePassWhatTheMethodFixes(t *testing.T) {
	e, _ := NewEngine(Builtin(), fixtureConfig())
	m, _ := e.Method("rancher", "build-yaml-bump")
	_, err := m.Bind(config.Edge{Params: map[string]string{"updateKey": "k", "file": "other.yaml"}})
	require.Error(t, err, "file is under with, not allowedParams, so it is not the producer's to pass")
	assert.Contains(t, err.Error(), `"file"`)
}

func TestBindExplicitlyOptionalParam(t *testing.T) {
	cfg := fixtureConfig()
	cfg.Projects[1].UpdateMethods[0].AllowedParams = append(cfg.Projects[1].UpdateMethods[0].AllowedParams,
		config.AllowedParam{Key: "reviewers", Required: no()})
	cfg.Projects[0].Downstream[0].Params = map[string]string{"updateKey": "k"}
	_, err := NewEngine(Builtin(), cfg)
	assert.NoError(t, err, "required: false means the edge may leave it out")
}
