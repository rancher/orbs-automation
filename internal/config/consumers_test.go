package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConsumers(t *testing.T) {
	cfg := Config{Projects: []Project{
		{Name: "scc-operator"},
		{Name: "rancher", Sources: []Source{
			{From: "scc-operator", Update: Update{Strategy: "buildYamlBump"}},
			{From: "other"},
		}},
		{Name: "charts", Sources: []Source{{From: "other"}}},
		{Name: "bro", Sources: []Source{{From: "scc-operator", Update: Update{Strategy: "yamlKeyBump"}}}},
	}}

	got := cfg.Consumers("scc-operator")

	assert.Equal(t, []Consumer{
		{Project: cfg.Projects[1], Source: cfg.Projects[1].Sources[0]},
		{Project: cfg.Projects[3], Source: cfg.Projects[3].Sources[0]},
	}, got, "each project that names it as a source, in config order")
}

func TestConsumersOfAProjectNobodyTakesFrom(t *testing.T) {
	cfg := Config{Projects: []Project{{Name: "a"}, {Name: "b", Sources: []Source{{From: "a"}}}}}
	assert.Empty(t, cfg.Consumers("b"))
	assert.Empty(t, cfg.Consumers("nope"))
	assert.Empty(t, Config{}.Consumers("a"))
}
