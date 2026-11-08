package v1alpha2

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func editDoc(spec string) string {
	return "apiVersion: gero/v1alpha2\nkind: Edit\nname: e\nrefs: {holding: h}\nspec:\n" + spec
}

func loadEdit(t *testing.T, spec string) (*Edit, config.Report) {
	t.Helper()
	objs, rep := decodeReport(t, editDoc(spec))
	if len(objs) == 0 {
		return nil, rep
	}
	require.Len(t, objs, 1)
	return objs[0].(*Edit), rep
}

func TestAValidEditHasNoFindings(t *testing.T) {
	for name, spec := range map[string]string{
		"a step after":           "  postEdit:\n    - {run: scripts/generate-buildconfig}\n",
		"a step before":          "  preEdit:\n    - {run: [make, prepare]}\n",
		"a step for some lines":  "  postEdit:\n    - {run: scripts/regen, since: v2.14, until: v2.16}\n",
		"a strategy with values": "  strategy: chartsPackageBump\n  with: {package: rancher-backup}\n",
		"a strategy alone":       "  strategy: chartsPackageBump\n",
	} {
		t.Run(name, func(t *testing.T) {
			e, rep := loadEdit(t, spec)

			assert.Empty(t, rep.Findings)
			assert.NotNil(t, e)
		})
	}
}

func TestEditRules(t *testing.T) {
	tests := []struct {
		name, spec, path, want string
	}{
		{"nothing to do", "  with: {}\n", "spec", "preEdit, postEdit or strategy"},
		{"values with no strategy", "  postEdit:\n    - {run: x}\n  with: {package: a}\n", "spec.with", "strategy"},
		{"a strategy that is not a name", "  strategy: \"bump it\"\n", "spec.strategy", "name"},
		{"a step that runs nothing", "  postEdit:\n    - {since: v2.14}\n", "spec.postEdit[0].run", "required"},
		{"a step that runs an empty program", "  preEdit:\n    - {run: [\"\"]}\n", "spec.preEdit[0].run", "empty"},
		{"a since that is not a line", "  postEdit:\n    - {run: x, since: 2.14}\n", "spec.postEdit[0].since", "v2.16"},
		{"an until that is not a line", "  preEdit:\n    - {run: x, until: latest}\n", "spec.preEdit[0].until", "v2.16"},
		{"a since after the until", "  postEdit:\n    - {run: x, since: v2.16, until: v2.14}\n", "spec.postEdit[0]", "after"},
	}
	require.NotEmpty(t, tests)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, rep := loadEdit(t, tt.spec)

			assert.Nil(t, e, "an edit that breaks a rule is dropped")
			assert.True(t, hasError(rep, tt.path, tt.want), "want an error at %s containing %q, got %v", tt.path, tt.want, rep.Findings)
		})
	}
}

func TestAStepSaysWhereItsProgramComesFrom(t *testing.T) {
	for _, from := range []string{"consumer", "producer", "central"} {
		t.Run(from, func(t *testing.T) {
			e, rep := loadEdit(t, "  postEdit:\n    - {run: x, from: "+from+"}\n")

			assert.Empty(t, rep.Findings)
			assert.NotNil(t, e)
		})
	}

	e, rep := loadEdit(t, "  postEdit:\n    - {run: x, from: elsewhere}\n")

	assert.Nil(t, e)
	assert.True(t, hasError(rep, "spec.postEdit[0].from", "consumer, producer or central"), "%v", rep.Findings)
}
