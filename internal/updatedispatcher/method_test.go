package updatedispatcher

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
)

func no() *bool { b := false; return &b }

func TestTranslateConfig(t *testing.T) {
	raw := config.UpdateMethod{
		Name: "build-yaml-bump", Strategy: "yamlBump",
		With: map[string]string{"file": "build.yaml"},
		AllowedParams: []config.AllowedParam{
			{Key: "updateKey", Description: "the key"},
			{Key: "value", Default: "{tag}"},
			{Key: "reviewers", Required: no()},
		},
		Accept:   config.Accept{When: "all"},
		PreEdit:  []config.Step{{Run: config.Command{"pre"}}},
		PostEdit: []config.Step{{Run: config.Command{"post"}, Since: "v2.16"}},
	}

	got := TranslateConfig(raw)

	assert.Equal(t, "build-yaml-bump", got.Name)
	assert.Equal(t, "yamlBump", got.StrategyName)
	assert.Equal(t, map[string]string{"file": "build.yaml"}, got.With)
	assert.Equal(t, []ParamDef{
		{Key: "updateKey", Description: "the key", Required: true},
		{Key: "value", Default: "{tag}", Required: false},
		{Key: "reviewers", Required: false},
	}, got.AllowedParams, "required by the rule config owns: unless a default or required: false")
	assert.Equal(t, "all", got.Accept)
	assert.Equal(t, raw.PreEdit, got.PreEdit)
	assert.Equal(t, raw.PostEdit, got.PostEdit)
}

func TestActionMethodParam(t *testing.T) {
	m := ActionMethod{AllowedParams: []ParamDef{{Key: "a"}, {Key: "b", Default: "x"}}}
	p, ok := m.Param("b")
	assert.True(t, ok)
	assert.Equal(t, "x", p.Default)
	_, ok = m.Param("c")
	assert.False(t, ok)
}
