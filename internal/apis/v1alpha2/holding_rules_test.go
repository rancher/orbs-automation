package v1alpha2

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func holdingDoc(spec string) string {
	return "apiVersion: gero/v1alpha2\nkind: Holding\nname: h\nrefs: {project: p, artifact: a}\nspec:\n" + spec
}

func loadHolding(t *testing.T, spec string) (*Holding, config.Report) {
	t.Helper()
	objs, rep := decodeReport(t, holdingDoc(spec))
	if len(objs) == 0 {
		return nil, rep
	}
	require.Len(t, objs, 1)
	return objs[0].(*Holding), rep
}

func TestHoldingDefaultsForARelease(t *testing.T) {
	h, rep := loadHolding(t, "  at: {file: build.yaml, key: defaultShellVersion}\n  holds: {kind: release}\n")

	require.NoError(t, rep.Err())
	require.NotNil(t, h)
	assert.Equal(t, "yaml", h.Spec.At.Format)
	assert.Equal(t, "{tag}", h.Spec.Holds.Form)
	assert.Equal(t, "stable", h.Spec.Holds.Accept)
}

func TestHoldingDefaultsForACommitHaveNoForm(t *testing.T) {
	h, rep := loadHolding(t, "  at: {file: lock.yaml, key: .a}\n  holds: {kind: commit}\n")

	require.NoError(t, rep.Err())
	require.NotNil(t, h)
	assert.Empty(t, h.Spec.Holds.Form)
	assert.Equal(t, "stable", h.Spec.Holds.Accept)
}

func TestHoldingDefaultsDoNotOverwriteWhatWasWritten(t *testing.T) {
	h, rep := loadHolding(t, "  at: {file: package/Dockerfile, key: CHART_DEFAULT_BRANCH, format: dockerfile}\n  holds: {kind: release, form: \"{image}:{tag}\", accept: all}\n")

	require.NoError(t, rep.Err())
	require.NotNil(t, h)
	assert.Equal(t, "dockerfile", h.Spec.At.Format)
	assert.Equal(t, "{image}:{tag}", h.Spec.Holds.Form)
	assert.Equal(t, "all", h.Spec.Holds.Accept)
}

func TestAValidHoldingHasNoFindings(t *testing.T) {
	for name, spec := range map[string]string{
		"an image tag":                         "  at: {file: build.yaml, key: defaultAssetsImage}\n  holds: {kind: release, form: \"{image}:{tag}\", accept: all, lines: {since: v2.16}}\n",
		"a chart URL":                          "  at: {file: packages/rancher-backup/package.yaml, key: '.additionalCharts[0].upstreamOptions.url'}\n  holds: {kind: release, form: \"{url}\", stage: dev, accept: all}\n",
		"a respelled stage":                    "  at: {file: lock.yaml, key: '.chart-versions.\"{line|trimprefix:v}\".upstream-refs.{stage|replace:stable=prod}.charts.commit'}\n  holds: {kind: commit, stages: [dev, stable]}\n",
		"a branch":                             "  at: {file: package/Dockerfile, key: CHART_DEFAULT_BRANCH, format: dockerfile}\n  holds: {kind: branch, lines: {until: v2.15}}\n",
		"a version with a reversible modifier": "  at: {file: Dockerfile, key: PKG_A_VERSION, format: dockerfile}\n  holds: {kind: release, form: \"{tag|trimprefix:v}\"}\n",
		"a file named by the line":             "  at: {file: 'docs/{line}.yaml', key: .a}\n  holds: {kind: release}\n",
	} {
		t.Run(name, func(t *testing.T) {
			h, rep := loadHolding(t, spec)

			assert.Empty(t, rep.Findings)
			assert.NotNil(t, h)
		})
	}
}

func TestHoldingRules(t *testing.T) {
	tests := []struct {
		name, spec, path, want string
	}{
		{"no file", "  at: {key: .a}\n  holds: {kind: release}\n", "spec.at.file", "required"},
		{"no key", "  at: {file: f.yaml}\n  holds: {kind: release}\n", "spec.at.key", "required"},
		{"a file with an unknown variable", "  at: {file: 'x/{nope}.yaml', key: .a}\n  holds: {kind: release}\n", "spec.at.file", "unknown placeholder {nope}"},
		{"a key with an unknown variable", "  at: {file: f.yaml, key: '.{tag}'}\n  holds: {kind: release}\n", "spec.at.key", "unknown placeholder {tag}"},
		{"a key with an unknown modifier", "  at: {file: f.yaml, key: '.{line|reverse}'}\n  holds: {kind: release}\n", "spec.at.key", "unknown modifier"},
		{"a format that is documented and not listed", "  at: {file: f.yaml, key: .a, format: toml}\n  holds: {kind: release}\n", "spec.at.format", "yaml or dockerfile"},
		{"a format that does not exist", "  at: {file: f.yaml, key: .a, format: ini}\n  holds: {kind: release}\n", "spec.at.format", "yaml or dockerfile"},
		{"no kind", "  at: {file: f.yaml, key: .a}\n  holds: {}\n", "spec.holds.kind", "release, commit or branch"},
		{"a kind that is none of them", "  at: {file: f.yaml, key: .a}\n  holds: {kind: tag}\n", "spec.holds.kind", "release, commit or branch"},
		{"a form with an unknown variable", "  at: {file: f.yaml, key: .a}\n  holds: {kind: release, form: \"{nope}\"}\n", "spec.holds.form", "unknown placeholder {nope}"},
		{"a form with a modifier that loses information", "  at: {file: f.yaml, key: .a}\n  holds: {kind: release, form: \"{tag|upper}\"}\n", "spec.holds.form", "upper"},
		{"a form on a commit", "  at: {file: f.yaml, key: .a}\n  holds: {kind: commit, form: \"{tag}\"}\n", "spec.holds.form", "release"},
		{"stages on a release", "  at: {file: f.yaml, key: .a}\n  holds: {kind: release, stages: [dev]}\n", "spec.holds.stages", "commit or a branch"},
		{"a stage with no name", "  at: {file: f.yaml, key: .a}\n  holds: {kind: commit, stages: [dev, \"\"]}\n", "spec.holds.stages[1]", "empty"},
		{"an accept that is neither", "  at: {file: f.yaml, key: .a}\n  holds: {kind: release, accept: some}\n", "spec.holds.accept", "stable or all"},
		{"a since that is not a line", "  at: {file: f.yaml, key: .a}\n  holds: {kind: release, lines: {since: 2.16}}\n", "spec.holds.lines.since", "v2.16"},
		{"a since after the until", "  at: {file: f.yaml, key: .a}\n  holds: {kind: release, lines: {since: v2.16, until: v2.15}}\n", "spec.holds.lines", "after"},
		{"a key that reads the stage with none given", "  at: {file: f.yaml, key: '.a.{stage}'}\n  holds: {kind: release}\n", "spec.at.key", "stage"},
	}
	require.NotEmpty(t, tests)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, rep := loadHolding(t, tt.spec)

			assert.Nil(t, h, "a holding that breaks a rule is dropped")
			assert.True(t, hasFinding(rep, config.Error, tt.path, tt.want), "want an error at %s containing %q, got %v", tt.path, tt.want, rep.Findings)
		})
	}
}
