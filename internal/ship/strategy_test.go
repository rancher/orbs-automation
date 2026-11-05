package ship

import (
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/semver"
	"github.com/rancher/orbs-automation/internal/yamledit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var valueVars = map[string]string{
	"image": "rancher/scc-operator", "tag": "v0.5.3", "version": "0.5.3", "sourceRepo": "rancher/scc-operator",
}

const buildYAML = "# build.yaml\ndefaultSccOperatorImage: \"rancher/scc-operator:v0.5.2\"\nwebhookVersion: v0.9.0\nglobal:\n  kubectl:\n    tag: v1.2.3\n"

func TestLookupStrategy(t *testing.T) {
	for _, name := range []string{"buildYamlBump", "yamlKeyBump"} {
		s, err := LookupStrategy(name)
		require.NoError(t, err, name)
		assert.NotNil(t, s)
	}
	_, err := LookupStrategy("chartsPackageBump")
	assert.ErrorIs(t, err, ErrUnknownStrategy)
	assert.ErrorContains(t, err, "chartsPackageBump")
	assert.ErrorContains(t, err, "buildYamlBump", "the message lists what exists")
}

func TestStrategyNames(t *testing.T) {
	assert.Equal(t, []string{"buildYamlBump", "yamlKeyBump"}, StrategyNames())
}

func TestBuildYamlBump(t *testing.T) {
	s, err := LookupStrategy("buildYamlBump")
	require.NoError(t, err)
	p := Params{"file": "build.yaml", "key": "defaultSccOperatorImage"}

	file, err := s.File(p)
	require.NoError(t, err)
	assert.Equal(t, "build.yaml", file)

	want, err := s.Value(p, valueVars)
	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator:v0.5.3", want, "an image and its tag, as the script wrote it")

	current, err := s.Read([]byte(buildYAML), p)
	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator:v0.5.2", current)
}

func TestYamlKeyBumpDefaultsToTheTagAndTakesADottedKey(t *testing.T) {
	s, err := LookupStrategy("yamlKeyBump")
	require.NoError(t, err)
	p := Params{"file": "charts/values.yaml", "key": "global.kubectl.tag"}

	want, err := s.Value(p, valueVars)
	require.NoError(t, err)
	assert.Equal(t, "v0.5.3", want)

	current, err := s.Read([]byte(buildYAML), p)
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", current)
}

func TestValueTemplateParamOverridesTheDefault(t *testing.T) {
	s, _ := LookupStrategy("buildYamlBump")
	p := Params{"file": "build.yaml", "key": "webhookVersion", "value": "{version}"}
	want, err := s.Value(p, valueVars)
	require.NoError(t, err)
	assert.Equal(t, "0.5.3", want)
}

func TestValueRejectsAnUnknownPlaceholder(t *testing.T) {
	s, _ := LookupStrategy("buildYamlBump")
	_, err := s.Value(Params{"file": "f", "key": "k", "value": "{nope}"}, valueVars)
	assert.ErrorContains(t, err, "nope")
}

func TestStrategyParamsAreRequired(t *testing.T) {
	for _, name := range []string{"buildYamlBump", "yamlKeyBump"} {
		s, _ := LookupStrategy(name)
		_, err := s.File(Params{"key": "k"})
		assert.ErrorContains(t, err, "file", name)
		_, err = s.Read([]byte(buildYAML), Params{"file": "f"})
		assert.ErrorContains(t, err, "key", name)
	}
}

func TestReadErrors(t *testing.T) {
	s, _ := LookupStrategy("buildYamlBump")
	_, err := s.Read([]byte(buildYAML), Params{"file": "build.yaml", "key": "nope"})
	assert.True(t, errors.Is(err, yamledit.ErrKeyNotFound))
	_, err = s.Read([]byte(buildYAML), Params{"file": "build.yaml", "key": "global"})
	assert.True(t, errors.Is(err, yamledit.ErrNotScalar))
}

func TestStrategiesDescribeTheChange(t *testing.T) {
	p := Params{"file": "build.yaml", "key": "defaultSccOperatorImage"}
	for _, name := range []string{"buildYamlBump", "yamlKeyBump"} {
		s, err := LookupStrategy(name)
		require.NoError(t, err)
		assert.Equal(t,
			"Updated `defaultSccOperatorImage` in `build.yaml` from `rancher/scc-operator:v0.5.2` to `rancher/scc-operator:v0.5.3`",
			s.Describe(p, "rancher/scc-operator:v0.5.2", "rancher/scc-operator:v0.5.3"), name)
	}
}

func TestStrategiesWriteTheValueAndNothingElse(t *testing.T) {
	for _, name := range []string{"buildYamlBump", "yamlKeyBump"} {
		s, err := LookupStrategy(name)
		require.NoError(t, err)
		p := Params{"file": "build.yaml", "key": "defaultSccOperatorImage"}

		out, err := s.Write([]byte(buildYAML), p, "rancher/scc-operator:v0.5.3")
		require.NoError(t, err, name)

		want := "# build.yaml\ndefaultSccOperatorImage: \"rancher/scc-operator:v0.5.3\"\nwebhookVersion: v0.9.0\nglobal:\n  kubectl:\n    tag: v1.2.3\n"
		assert.Equal(t, want, string(out), name)
		back, err := s.Read(out, p)
		require.NoError(t, err)
		assert.Equal(t, "rancher/scc-operator:v0.5.3", back, "what was written is what is read")
	}
}

func TestWriteANestedKey(t *testing.T) {
	s, _ := LookupStrategy("yamlKeyBump")
	out, err := s.Write([]byte(buildYAML), Params{"file": "f", "key": "global.kubectl.tag"}, "v1.2.4")
	require.NoError(t, err)
	assert.Contains(t, string(out), "    tag: v1.2.4\n")
}

func TestWriteTheSameValueChangesNothing(t *testing.T) {
	s, _ := LookupStrategy("buildYamlBump")
	out, err := s.Write([]byte(buildYAML), Params{"file": "f", "key": "webhookVersion"}, "v0.9.0")
	require.NoError(t, err)
	assert.Equal(t, buildYAML, string(out))
}

func TestWriteErrors(t *testing.T) {
	s, _ := LookupStrategy("buildYamlBump")
	_, err := s.Write([]byte(buildYAML), Params{"file": "f"}, "x")
	assert.ErrorContains(t, err, "key")
	_, err = s.Write([]byte(buildYAML), Params{"file": "f", "key": "nope"}, "x")
	assert.ErrorIs(t, err, yamledit.ErrKeyNotFound)
	_, err = s.Write([]byte("a: |\n  v\n"), Params{"file": "f", "key": "a"}, "x")
	assert.ErrorIs(t, err, yamledit.ErrUnsupported, "a value it cannot replace in place is refused")
}

func TestStrategiesFindTheVersionInAValue(t *testing.T) {
	tests := []struct {
		value string
		want  semver.Version
		ok    bool
	}{
		{"v1.2.3", semver.Version{Major: 1, Minor: 2, Patch: 3}, true},
		{"1.2.3", semver.Version{Major: 1, Minor: 2, Patch: 3}, true},
		{"rancher/scc-operator:v0.5.3", semver.Version{Minor: 5, Patch: 3}, true},
		{"registry.example.com:5000/rancher/scc-operator:v0.5.3", semver.Version{Minor: 5, Patch: 3}, true},
		{"rancher/scc-operator:v0.6.0-rc.1", semver.Version{Minor: 6, Prerelease: "rc.1"}, true},
		{"v1.2.3+build.5", semver.Version{Major: 1, Minor: 2, Patch: 3}, true},
		{"rancher/shell:v0.6.2", semver.Version{Minor: 6, Patch: 2}, true},
		{"latest", semver.Version{}, false},
		{"", semver.Version{}, false},
		{"rancher/scc-operator", semver.Version{}, false},
		{"v1.2", semver.Version{}, false},
	}
	for _, name := range []string{"buildYamlBump", "yamlKeyBump"} {
		s, err := LookupStrategy(name)
		require.NoError(t, err)
		for _, tt := range tests {
			got, ok := s.Version(tt.value)
			assert.Equal(t, tt.ok, ok, "%s %q", name, tt.value)
			assert.Equal(t, tt.want, got, "%s %q", name, tt.value)
		}
	}
}
