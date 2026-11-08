package updatedispatcher

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/yamledit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memTree is a tree in memory.
type memTree struct {
	files  map[string]string
	writes int
}

func (t *memTree) ReadFile(_ context.Context, path string) ([]byte, error) {
	c, ok := t.files[path]
	if !ok {
		return nil, errors.New("file not found")
	}
	return []byte(c), nil
}

func (t *memTree) WriteFile(_ context.Context, path string, data []byte) error {
	t.writes++
	t.files[path] = string(data)
	return nil
}

const buildYAML = "# build.yaml\ndefaultSccOperatorImage: \"rancher/scc-operator:v0.5.2\"\nwebhookVersion: v0.9.0\nglobal:\n  kubectl:\n    tag: v1.2.3\n"

var rel = Release{Tag: "v0.5.3", Version: "0.5.3", Image: "rancher/scc-operator", SourceRepo: "rancher/scc-operator"}

func yamlUpdaterFor(t *testing.T) Updater {
	t.Helper()
	u, err := YAMLBump{}.Configure(map[string]string{"file": "build.yaml"})
	require.NoError(t, err)
	return u
}

func TestYAMLBumpRequiresUpdateKey(t *testing.T) {
	assert.Equal(t, []string{"updateKey"}, YAMLBump{}.Requires())
}

func TestYAMLBumpConfigure(t *testing.T) {
	_, err := YAMLBump{}.Configure(map[string]string{"file": "build.yaml"})
	assert.NoError(t, err)

	_, err = YAMLBump{}.Configure(nil)
	assert.ErrorContains(t, err, "with.file", "the file is needed")

	_, err = YAMLBump{}.Configure(map[string]string{"file": "build.yaml", "fiel": "x"})
	assert.ErrorContains(t, err, `"fiel"`, "a typo in with is found when the config loads")
}

func TestYAMLBumpCheckAnImageValue(t *testing.T) {
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	in := Inputs{"updateKey": "defaultSccOperatorImage", "value": "{image}:{tag}"}

	got, err := yamlUpdaterFor(t).Check(context.Background(), tree, in, rel)

	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator:v0.5.2", got.Current)
	assert.Equal(t, "rancher/scc-operator:v0.5.3", got.Want)
	assert.False(t, got.UpToDate)
	assert.Equal(t, RelationNewer, got.Relation)
	assert.Equal(t, "Updated `defaultSccOperatorImage` in `build.yaml` from `rancher/scc-operator:v0.5.2` to `rancher/scc-operator:v0.5.3`",
		got.Change.Description)
}

func TestYAMLBumpValueDefaultsToTheTag(t *testing.T) {
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	got, err := yamlUpdaterFor(t).Check(context.Background(), tree, Inputs{"updateKey": "webhookVersion"}, rel)
	require.NoError(t, err)
	assert.Equal(t, "v0.5.3", got.Want, "a bare tag when the call passes no value")
}

func TestYAMLBumpCheckRelations(t *testing.T) {
	tests := []struct {
		name, current string
		want          Relation
		upToDate      bool
	}{
		{"older value", "rancher/scc-operator:v0.5.2", RelationNewer, false},
		{"newer value is a downgrade", "rancher/scc-operator:v0.9.0", RelationOlder, false},
		{"the same", "rancher/scc-operator:v0.5.3", RelationSame, true},
		{"a prerelease of the release", "rancher/scc-operator:v0.5.3-rc.1", RelationNewer, false},
		{"no version in the value", "rancher/scc-operator:latest", RelationUnknown, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := &memTree{files: map[string]string{"build.yaml": "image: " + tt.current + "\n"}}
			got, err := yamlUpdaterFor(t).Check(context.Background(), tree, Inputs{"updateKey": "image", "value": "{image}:{tag}"}, rel)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Relation)
			assert.Equal(t, tt.upToDate, got.UpToDate)
		})
	}
}

func TestYAMLBumpCheckNeverWrites(t *testing.T) {
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	_, err := yamlUpdaterFor(t).Check(context.Background(), tree, Inputs{"updateKey": "webhookVersion"}, rel)
	require.NoError(t, err)
	assert.Zero(t, tree.writes)
	assert.Equal(t, buildYAML, tree.files["build.yaml"])
}

func TestYAMLBumpCheckErrors(t *testing.T) {
	u := yamlUpdaterFor(t)
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	_, err := u.Check(context.Background(), tree, Inputs{}, rel)
	assert.ErrorContains(t, err, "updateKey")

	_, err = u.Check(context.Background(), tree, Inputs{"updateKey": "nope"}, rel)
	assert.ErrorIs(t, err, yamledit.ErrKeyNotFound)

	_, err = u.Check(context.Background(), tree, Inputs{"updateKey": "global"}, rel)
	assert.ErrorIs(t, err, yamledit.ErrNotScalar)

	_, err = u.Check(context.Background(), &memTree{files: map[string]string{}}, Inputs{"updateKey": "a"}, rel)
	assert.ErrorContains(t, err, "build.yaml")

	_, err = u.Check(context.Background(), tree, Inputs{"updateKey": "webhookVersion", "value": "{nope}"}, rel)
	assert.ErrorContains(t, err, "nope")
}

func TestYAMLBumpApplyChangesOnlyTheValue(t *testing.T) {
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}

	res, err := yamlUpdaterFor(t).Apply(context.Background(), tree, Inputs{"updateKey": "defaultSccOperatorImage", "value": "{image}:{tag}"}, rel)

	require.NoError(t, err)
	assert.True(t, res.Changed)
	require.Len(t, res.Changes, 1)
	assert.Equal(t, "rancher/scc-operator:v0.5.2", res.Changes[0].From)
	assert.Equal(t, "rancher/scc-operator:v0.5.3", res.Changes[0].To)
	want := "# build.yaml\ndefaultSccOperatorImage: \"rancher/scc-operator:v0.5.3\"\nwebhookVersion: v0.9.0\nglobal:\n  kubectl:\n    tag: v1.2.3\n"
	assert.Equal(t, want, tree.files["build.yaml"], "a quoted value stays quoted and the rest is untouched")
}

func TestYAMLBumpApplyANestedKey(t *testing.T) {
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	_, err := yamlUpdaterFor(t).Apply(context.Background(), tree, Inputs{"updateKey": "global.kubectl.tag"}, rel)
	require.NoError(t, err)
	assert.Contains(t, tree.files["build.yaml"], "    tag: v0.5.3\n")
}

func TestYAMLBumpApplyWhenAlreadyUpToDateWritesNothing(t *testing.T) {
	tree := &memTree{files: map[string]string{"build.yaml": buildYAML}}
	res, err := yamlUpdaterFor(t).Apply(context.Background(), tree, Inputs{"updateKey": "webhookVersion", "value": "v0.9.0"}, rel)
	require.NoError(t, err)
	assert.False(t, res.Changed)
	assert.Empty(t, res.Changes)
	assert.Zero(t, tree.writes)
}

func TestYAMLBumpApplyErrors(t *testing.T) {
	u := yamlUpdaterFor(t)
	tree := &memTree{files: map[string]string{"build.yaml": "a: |\n  v\n"}}
	_, err := u.Apply(context.Background(), tree, Inputs{"updateKey": "a"}, rel)
	assert.ErrorIs(t, err, yamledit.ErrUnsupported, "a value it cannot replace in place is refused")
	assert.Zero(t, tree.writes)
	_, err = u.Apply(context.Background(), tree, Inputs{}, rel)
	assert.ErrorContains(t, err, "updateKey")
}
