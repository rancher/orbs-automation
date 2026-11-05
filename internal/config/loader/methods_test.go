package loader

import (
	"testing"
	"testing/fstest"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDecodesMethodsAndEdges(t *testing.T) {
	fsys := fstest.MapFS{
		"rancher.project.yaml": project("rancher", `  repo:
    name: rancher/rancher
  updateMethods:
    - name: build-yaml-bump
      strategy: yamlBump
      with:
        file: build.yaml
      allowedParams:
        - key: updateKey
          description: "The key to set"
        - key: value
          default: "{tag}"
        - key: reviewers
          required: false
      accept:
        when: all
      preEdit:
        - run: scripts/prepare
      postEdit:
        - run: scripts/generate-buildconfig
          since: v2.16
        - run: [go, generate, ./pkg/...]
          until: v2.15
`),
		"scc.project.yaml": project("scc-operator", `  repo:
    name: rancher/scc-operator
  downstream:
    - to: rancher
      method: build-yaml-bump
      lines: universal
      params:
        updateKey: defaultSccOperatorImage
      postEdit:
        - run: [hack/bump.sh, --verify]
          from: producer
`),
	}

	cfg, err := Load(fsys)

	require.NoError(t, err)
	require.Len(t, cfg.Projects, 2)
	var rancher, scc config.Project
	for _, p := range cfg.Projects {
		if p.Name == "rancher" {
			rancher = p
		} else {
			scc = p
		}
	}
	require.Len(t, rancher.UpdateMethods, 1)
	m := rancher.UpdateMethods[0]
	assert.Equal(t, "build-yaml-bump", m.Name)
	assert.Equal(t, "yamlBump", m.Strategy)
	assert.Equal(t, map[string]string{"file": "build.yaml"}, m.With)
	require.Len(t, m.AllowedParams, 3)
	assert.Equal(t, "updateKey", m.AllowedParams[0].Key)
	assert.Equal(t, "The key to set", m.AllowedParams[0].Description)
	assert.Equal(t, "{tag}", m.AllowedParams[1].Default)
	require.NotNil(t, m.AllowedParams[2].Required)
	assert.False(t, *m.AllowedParams[2].Required)
	assert.Equal(t, "all", m.Accept.When)
	assert.Equal(t, []config.Step{{Run: config.Command{"scripts/prepare"}}}, m.PreEdit)
	assert.Equal(t, []config.Step{
		{Run: config.Command{"scripts/generate-buildconfig"}, Since: "v2.16"},
		{Run: config.Command{"go", "generate", "./pkg/..."}, Until: "v2.15"},
	}, m.PostEdit, "run is a string or a list")

	require.Len(t, scc.Downstream, 1)
	e := scc.Downstream[0]
	assert.Equal(t, "rancher", e.To)
	assert.Equal(t, "build-yaml-bump", e.Method)
	assert.Equal(t, "universal", e.Lines)
	assert.Equal(t, map[string]string{"updateKey": "defaultSccOperatorImage"}, e.Params)
	assert.Equal(t, []config.Step{{Run: config.Command{"hack/bump.sh", "--verify"}, From: "producer"}}, e.PostEdit)
}

func TestLoadRejectsAnUnknownMethodKey(t *testing.T) {
	for name, body := range map[string]string{
		"a typo in a method": "  updateMethods:\n    - name: m\n      strategy: yamlBump\n      acept:\n        when: all\n",
		"a typo in an edge":  "  downstream:\n    - to: x\n      method: m\n      param:\n        a: b\n",
		"a typo in a step":   "  updateMethods:\n    - name: m\n      postEdit:\n        - run: x\n          sinse: v2.16\n",
		"a bad run":          "  updateMethods:\n    - name: m\n      postEdit:\n        - run: {a: b}\n",
	} {
		_, err := Load(fstest.MapFS{"p.project.yaml": project("p", "  repo:\n    name: o/p\n"+body)})
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "p.project.yaml", name)
	}
}
