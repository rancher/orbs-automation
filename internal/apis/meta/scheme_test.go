package meta

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type widgetV1 struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
	Spec       struct {
		Size int `yaml:"size"`
	} `yaml:"spec"`
}

type widgetV2 struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
	Spec       struct {
		Dimensions []int `yaml:"dimensions"`
	} `yaml:"spec"`
}

func twoVersions() *Scheme {
	s := NewScheme()
	s.AddKind("test/v1", "Widget", func() Object { return new(widgetV1) })
	s.AddKind("test/v1", "Gadget", func() Object { return new(widgetV1) })
	s.AddKind("test/v2", "Widget", func() Object { return new(widgetV2) })
	return s
}

func TestSchemeDecodesEachDocumentByItsVersionAndKind(t *testing.T) {
	objs, err := twoVersions().Decode(strings.NewReader(`
apiVersion: test/v1
kind: Widget
name: old
spec: {size: 3}
---
apiVersion: test/v2
kind: Widget
name: new
spec: {dimensions: [1, 2]}
`), "mixed.yaml")

	require.NoError(t, err)
	require.Len(t, objs, 2)
	require.IsType(t, &widgetV1{}, objs[0])
	require.IsType(t, &widgetV2{}, objs[1])
	assert.Equal(t, 3, objs[0].(*widgetV1).Spec.Size)
	assert.Equal(t, []int{1, 2}, objs[1].(*widgetV2).Spec.Dimensions)
	assert.Equal(t, "test/v2", objs[1].GetAPIVersion())
}

func TestSchemeNamesWhatIsKnownInTheError(t *testing.T) {
	_, err := twoVersions().Decode(strings.NewReader("apiVersion: test/v3\nkind: Widget\nname: a\n"), "a.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test/v1, test/v2")

	_, err = twoVersions().Decode(strings.NewReader("apiVersion: test/v1\nkind: Sprocket\nname: a\n"), "a.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Gadget, Widget")
	assert.Contains(t, err.Error(), "test/v1")
}

func TestSchemeKindOfOneVersionIsNotKnownInAnother(t *testing.T) {
	_, err := twoVersions().Decode(strings.NewReader("apiVersion: test/v2\nkind: Gadget\nname: a\n"), "a.yaml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `kind "Gadget"`)
}

func TestRefsAreANameOrAListOfNames(t *testing.T) {
	objs, err := twoVersions().Decode(strings.NewReader(`
apiVersion: test/v1
kind: Widget
name: a
refs: {one: x, many: [y, z]}
`), "a.yaml")

	require.NoError(t, err)
	require.Len(t, objs, 1)
	assert.Equal(t, Refs{"one": {"x"}, "many": {"y", "z"}}, objs[0].GetRefs())
}

func TestRefsRejectWhatIsNotANameOrAList(t *testing.T) {
	tests := []struct{ name, refs string }{
		{"a mapping as a value", "{one: {a: b}}"},
		{"a list holding a mapping", "{one: [{a: b}]}"},
		{"refs that is a list", "[a, b]"},
	}
	require.NotEmpty(t, tests)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := twoVersions().Decode(strings.NewReader("apiVersion: test/v1\nkind: Widget\nname: a\nrefs: "+tt.refs+"\n"), "a.yaml")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "refs")
			assert.Contains(t, err.Error(), "a.yaml: document 1")
		})
	}
}

func TestSchemeRejectsAFieldTheKindDoesNotHave(t *testing.T) {
	_, err := twoVersions().Decode(strings.NewReader("apiVersion: test/v1\nkind: Widget\nname: a\nspec: {size: 1, bogus: 2}\n"), "a.yaml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
}
