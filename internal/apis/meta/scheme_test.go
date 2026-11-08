package meta

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/platform/config"
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

// rulesWidget is a kind with a default and rules of its own.
type rulesWidget struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
	Spec       struct {
		Layout string `yaml:"layout"`
		Size   int    `yaml:"size"`
	} `yaml:"spec"`
}

func (w *rulesWidget) SetDefaults() {
	if w.Spec.Layout == "" {
		w.Spec.Layout = "single"
	}
}

func (w *rulesWidget) Validate(_ config.Context, r *config.Report) {
	if w.Spec.Size < 0 {
		r.Errorf("spec.size", "must not be negative, got %d", w.Spec.Size)
	}
	if w.Spec.Size > 100 {
		r.Warnf("spec.size", "is large, %d", w.Spec.Size)
	}
}

func rulesScheme() *Scheme {
	s := NewScheme()
	s.AddKind("test/v1", "Widget", func() Object { return new(rulesWidget) })
	return s
}

var testCtx = config.Context{Now: time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)}

func TestDecodeReportAppliesTheDefaultsOfAKind(t *testing.T) {
	objs, rep := rulesScheme().DecodeReport(strings.NewReader("apiVersion: test/v1\nkind: Widget\nname: w\n"), "w.yaml", testCtx)

	require.NoError(t, rep.Err())
	require.Len(t, objs, 1)
	assert.Equal(t, "single", objs[0].(*rulesWidget).Spec.Layout)
}

func TestDecodeReportCarriesWarningsAndKeepsTheObject(t *testing.T) {
	objs, rep := rulesScheme().DecodeReport(strings.NewReader("apiVersion: test/v1\nkind: Widget\nname: w\nspec: {size: 500}\n"), "w.yaml", testCtx)

	require.NoError(t, rep.Err())
	require.Len(t, objs, 1, "a warning does not drop the object")
	require.Len(t, rep.Warnings(), 1)
	assert.Equal(t, `w.yaml: document 1: Widget "w": spec.size: is large, 500`, rep.Warnings()[0].Error())
}

func TestDecodeReportAnInvalidObjectIsDroppedAndTheRestKept(t *testing.T) {
	objs, rep := rulesScheme().DecodeReport(strings.NewReader(`
apiVersion: test/v1
kind: Widget
name: good
---
apiVersion: test/v1
kind: Widget
name: bad
spec: {size: -1}
`), "a.yaml", testCtx)

	require.Error(t, rep.Err())
	assert.Equal(t, `a.yaml: document 2: Widget "bad": spec.size: must not be negative, got -1`, rep.Err().Error())
	require.Len(t, objs, 1)
	assert.Equal(t, "good", objs[0].GetName())
}

func TestDecodeReportFindingsAreInTheOrderOfTheDocuments(t *testing.T) {
	_, rep := rulesScheme().DecodeReport(strings.NewReader(`
apiVersion: test/v1
kind: Widget
name: one
spec: {size: 500}
---
apiVersion: test/v1
kind: Sprocket
name: two
---
apiVersion: test/v1
kind: Widget
name: three
spec: {size: -1}
`), "a.yaml", testCtx)

	require.Len(t, rep.Findings, 3)
	assert.Equal(t, []int{1, 2, 3}, []int{rep.Findings[0].Doc, rep.Findings[1].Doc, rep.Findings[2].Doc})
	assert.Equal(t, []config.Severity{config.Warning, config.Error, config.Error},
		[]config.Severity{rep.Findings[0].Severity, rep.Findings[1].Severity, rep.Findings[2].Severity})
}

func TestDecodeReturnsTheErrorsAndNotTheWarnings(t *testing.T) {
	objs, err := rulesScheme().Decode(strings.NewReader("apiVersion: test/v1\nkind: Widget\nname: w\nspec: {size: 500}\n"), "w.yaml")

	require.NoError(t, err)
	assert.Len(t, objs, 1)
}

func TestANameIsALowercaseSlug(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"w", true},
		{"rancher-charts", true},
		{"v2", true},
		{"a1-b2", true},
		{"Rancher", false},
		{"rancher_charts", false},
		{"-rancher", false},
		{"rancher-", false},
		{"rancher charts", false},
		{"rancher.charts", false},
		{"rancher/charts", false},
	}
	require.NotEmpty(t, tests)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs, err := rulesScheme().Decode(strings.NewReader(fmt.Sprintf("apiVersion: test/v1\nkind: Widget\nname: %q\n", tt.name)), "w.yaml")

			if tt.ok {
				require.NoError(t, err)
				assert.Len(t, objs, 1)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "name")
			assert.Contains(t, err.Error(), "lowercase slug")
			assert.Empty(t, objs)
		})
	}
}

func TestSchemeListsTheKindsOfAVersionSorted(t *testing.T) {
	s := twoVersions()
	assert.Equal(t, []string{"Gadget", "Widget"}, s.Kinds("test/v1"))
	assert.Equal(t, []string{"Widget"}, s.Kinds("test/v2"))
	assert.Empty(t, s.Kinds("test/v3"))
}

func TestSchemeMakesANewObjectOfAKind(t *testing.T) {
	s := twoVersions()
	obj, ok := s.New("test/v2", "Widget")
	require.True(t, ok)
	assert.IsType(t, new(widgetV2), obj)
	other, _ := s.New("test/v2", "Widget")
	assert.NotSame(t, obj, other, "each call is a new object")

	_, ok = s.New("test/v2", "Gadget")
	assert.False(t, ok, "a kind of another version")
	_, ok = s.New("test/v3", "Widget")
	assert.False(t, ok, "an unknown version")
}
