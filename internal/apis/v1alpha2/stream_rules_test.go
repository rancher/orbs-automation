package v1alpha2

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func streamDoc(spec string) string {
	return "apiVersion: gero/v1alpha2\nkind: Stream\nname: s\nrefs: {project: p}\nspec:\n" + spec
}

func loadStream(t *testing.T, spec string) (*Stream, config.Report) {
	t.Helper()
	objs, rep := decodeReport(t, streamDoc(spec))
	if len(objs) == 0 {
		return nil, rep
	}
	require.Len(t, objs, 1)
	return objs[0].(*Stream), rep
}

func TestStreamDefaults(t *testing.T) {
	s, rep := loadStream(t, "  universal: true\n")

	require.NoError(t, rep.Err())
	require.NotNil(t, s)
	assert.Equal(t, "semver", s.Spec.Scheme)
	assert.Equal(t, "v*", s.Spec.TagPattern)
}

func TestStreamDefaultsDoNotOverwriteWhatWasWritten(t *testing.T) {
	s, rep := loadStream(t, "  scheme: calverLineStage\n  tagPattern: \"pkg-a/v*\"\n  lines: [{line: v2.16}]\n")

	require.NoError(t, rep.Err())
	require.NotNil(t, s)
	assert.Equal(t, "calverLineStage", s.Spec.Scheme)
	assert.Equal(t, "pkg-a/v*", s.Spec.TagPattern)
}

func TestAValidStreamHasNoFindings(t *testing.T) {
	for name, spec := range map[string]string{
		"a series for each line": "  scheme: semver\n  requiresPrerelease: true\n  lines: [{line: v2.16, series: \"0.9.x\"}, {line: v2.15, series: \"0.8.x\"}]\n",
		"a major series":         "  lines: [{line: v2.16, series: \"12.x\"}, {line: v2.15, series: \"11.x\"}]\n",
		"a chart series":         "  scheme: chartVersion\n  lines: [{line: v2.16, series: \"111.x\"}, {line: v2.15, series: \"110.x\"}]\n",
		"lines in the tags":      "  scheme: calverLineStage\n  lines: [{line: v2.16}]\n",
		"universal":              "  universal: true\n  tagExclude: [\"*-hotfix*\"]\n",
		"stages of its own":      "  universal: true\n  stages:\n    - {name: dev, prerelease: true}\n    - {name: stable}\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, rep := loadStream(t, spec)

			assert.Empty(t, rep.Findings)
		})
	}
}

func TestStreamRules(t *testing.T) {
	tests := []struct {
		name, spec, path, want string
	}{
		{"a scheme that is not one", "  scheme: weekly\n  universal: true\n", "spec.scheme", "semver, calverLineStage or chartVersion"},
		{"a tag pattern that is not a glob", "  universal: true\n  tagPattern: \"[\"\n", "spec.tagPattern", "pattern"},
		{"an exclude that is not a glob", "  universal: true\n  tagExclude: [\"[\"]\n", "spec.tagExclude[0]", "pattern"},
		{"a stage with no name", "  universal: true\n  stages: [{prerelease: true}]\n", "spec.stages[0].name", "required"},
		{"a stage listed twice", "  universal: true\n  stages: [{name: dev}, {name: dev}]\n", "spec.stages[1].name", "twice"},
		{"universal with lines", "  universal: true\n  lines: [{line: v2.16, series: \"1.x\"}]\n", "spec.lines", "universal"},
		{"a line with no id", "  lines: [{series: \"1.x\"}]\n", "spec.lines[0].line", "required"},
		{"a line that is not a line id", "  lines: [{line: 2.16, series: \"1.x\"}]\n", "spec.lines[0].line", "v2.16"},
		{"a line listed twice", "  lines: [{line: v2.16, series: \"1.x\"}, {line: v2.16, series: \"2.x\"}]\n", "spec.lines[1].line", "twice"},
		{"a semver line with no series", "  lines: [{line: v2.16}]\n", "spec.lines[0].series", "required"},
		{"a series that is not one", "  lines: [{line: v2.16, series: \"9\"}]\n", "spec.lines[0].series", "9.x"},
		{"a series with a letter", "  lines: [{line: v2.16, series: \"a.x\"}]\n", "spec.lines[0].series", "9.x"},
		{"a chart series with a minor", "  scheme: chartVersion\n  lines: [{line: v2.16, series: \"111.0.x\"}]\n", "spec.lines[0].series", "111.x"},
		{"a series in a stream whose lines are in the tags", "  scheme: calverLineStage\n  lines: [{line: v2.16, series: \"1.x\"}]\n", "spec.lines[0].series", "tag"},
		{"two lines with the same series", "  lines: [{line: v2.16, series: \"9.x\"}, {line: v2.15, series: \"9.x\"}]\n", "spec.lines[1].series", "overlaps"},
		{"a major series that covers a minor one", "  lines: [{line: v2.16, series: \"0.x\"}, {line: v2.15, series: \"0.8.x\"}]\n", "spec.lines[1].series", "overlaps"},
	}
	require.NotEmpty(t, tests)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, rep := loadStream(t, tt.spec)

			assert.Nil(t, s, "a stream that breaks a rule is dropped")
			assert.True(t, hasError(rep, tt.path, tt.want), "want an error at %s containing %q, got %v", tt.path, tt.want, rep.Findings)
		})
	}
}

func TestSeriesOfDifferentMinorsDoNotOverlap(t *testing.T) {
	_, rep := loadStream(t, "  lines: [{line: v2.16, series: \"0.9.x\"}, {line: v2.15, series: \"0.8.x\"}, {line: v2.14, series: \"1.x\"}]\n")

	assert.Empty(t, rep.Findings)
}
