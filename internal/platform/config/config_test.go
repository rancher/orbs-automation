package config

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// widget is a target with one defaulted field, one rule that reads the default,
// one rule that reads the clock and one that is always an error when it holds.
type widget struct {
	Layout  string    `yaml:"layout"`
	Size    int       `yaml:"size"`
	Expires time.Time `yaml:"expires"`

	calls     []string
	sawLayout string
}

func (w *widget) SetDefaults() {
	w.calls = append(w.calls, "defaults")
	if w.Layout == "" {
		w.Layout = "single"
	}
}

func (w *widget) Validate(ctx Context, r *Report) {
	w.calls = append(w.calls, "validate")
	w.sawLayout = w.Layout
	if w.Size < 0 {
		r.Errorf("spec.size", "must not be negative, got %d", w.Size)
	}
	if w.Layout != "single" && w.Layout != "perLine" {
		r.Errorf("spec.layout", "must be single or perLine, got %q", w.Layout)
	}
	if !w.Expires.IsZero() && ctx.Now.After(w.Expires) {
		r.Warnf("expired", "spec.expires", "passed on %s", w.Expires.Format("2006-01-02"))
	}
}

var ctx = Context{Now: time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)}

func node(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var n yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(src), &n))
	return &n
}

// joinedCount is the walk gero doctor uses to count the problems of an error.
func joinedCount(err error) int {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if joined, ok := e.(interface{ Unwrap() []error }); ok {
			return len(joined.Unwrap())
		}
	}
	return 1
}

func TestLoadDecodesThenDefaultsThenValidates(t *testing.T) {
	w := &widget{}

	r := Load("w.yaml", []byte("size: 3\n"), w, ctx)

	require.NoError(t, r.Err())
	assert.Equal(t, 3, w.Size)
	assert.Equal(t, "single", w.Layout)
	assert.Equal(t, []string{"defaults", "validate"}, w.calls)
	assert.Equal(t, "single", w.sawLayout, "validation reads the defaulted value")
}

func TestDefaultsDoNotOverwriteWhatWasWritten(t *testing.T) {
	w := &widget{}

	r := Load("w.yaml", []byte("layout: perLine\n"), w, ctx)

	require.NoError(t, r.Err())
	assert.Equal(t, "perLine", w.Layout)
}

func TestAFieldTheTargetDoesNotHaveIsAnErrorAndStopsTheRest(t *testing.T) {
	w := &widget{}

	r := Load("w.yaml", []byte("bogus: 1\n"), w, ctx)

	require.Error(t, r.Err())
	assert.Contains(t, r.Err().Error(), "bogus")
	assert.Contains(t, r.Err().Error(), "w.yaml")
	assert.Empty(t, w.calls, "a document that did not decode is neither defaulted nor validated")
}

func TestMalformedYAMLIsAnErrorThatNamesTheSource(t *testing.T) {
	r := Load("w.yaml", []byte("size: [\n"), &widget{}, ctx)

	require.Error(t, r.Err())
	assert.Contains(t, r.Err().Error(), "w.yaml")
}

func TestAnEmptyDocumentIsAnEmptyObject(t *testing.T) {
	w := &widget{}

	r := Load("w.yaml", nil, w, ctx)

	require.NoError(t, r.Err())
	assert.Equal(t, []string{"defaults", "validate"}, w.calls)
	assert.Equal(t, "single", w.Layout)
}

func TestFindingsNameTheSourceTheDocumentTheLabelAndThePath(t *testing.T) {
	w := &widget{}

	r := LoadNode("w.yaml", 2, `Widget "w"`, node(t, "size: -1\nlayout: x\n"), w, ctx)

	require.Len(t, r.Findings, 2)
	assert.Equal(t, `w.yaml: document 2: Widget "w": spec.size: must not be negative, got -1`, r.Findings[0].Error())
	assert.Equal(t, `w.yaml: document 2: Widget "w": spec.layout: must be single or perLine, got "x"`, r.Findings[1].Error())
	for _, f := range r.Findings {
		assert.Equal(t, Error, f.Severity)
	}
}

func TestAFindingLeavesOutWhatIsNotSet(t *testing.T) {
	assert.Equal(t, "a.yaml: bad", Finding{Source: "a.yaml", Msg: "bad"}.Error())
	assert.Equal(t, "a.yaml: spec.layout: bad", Finding{Source: "a.yaml", Path: "spec.layout", Msg: "bad"}.Error())
	assert.Equal(t, `a.yaml: document 3: Project "x": bad`, Finding{Source: "a.yaml", Doc: 3, Label: `Project "x"`, Msg: "bad"}.Error())
	assert.Equal(t, "bad", Finding{Msg: "bad"}.Error())
}

func TestAWarningIsNotAnError(t *testing.T) {
	w := &widget{}

	r := Load("w.yaml", []byte("expires: 2026-01-01\n"), w, ctx)

	assert.NoError(t, r.Err())
	require.Len(t, r.Warnings(), 1)
	assert.Equal(t, Warning, r.Warnings()[0].Severity)
	assert.Equal(t, "spec.expires", r.Warnings()[0].Path)
	assert.Contains(t, r.Warnings()[0].Msg, "2026-01-01")
}

func TestAWarningNamesItsRuleAndAnErrorDoesNot(t *testing.T) {
	r := Load("w.yaml", []byte("size: -1\nexpires: 2026-01-01\n"), &widget{}, ctx)

	require.Len(t, r.Findings, 2)
	assert.Empty(t, r.Findings[0].Rule, "an error has no rule")
	require.Len(t, r.Warnings(), 1)
	assert.Equal(t, "expired", r.Warnings()[0].Rule)
}

func TestAFindingShowsItsRuleAtTheEnd(t *testing.T) {
	assert.Equal(t, "bad [expired]", Finding{Msg: "bad", Rule: "expired"}.Error())
	assert.Equal(t, "a.yaml: document 2: Widget \"w\": spec.expires: passed [expired]",
		Finding{Source: "a.yaml", Doc: 2, Label: `Widget "w"`, Path: "spec.expires", Msg: "passed", Rule: "expired"}.Error())
}

func TestTheClockIsInjected(t *testing.T) {
	early := Context{Now: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)}
	w := &widget{}

	r := Load("w.yaml", []byte("size: 2\nexpires: 2026-01-01\n"), w, early)

	require.Equal(t, 2, w.Size, "the document was loaded")
	assert.NoError(t, r.Err())
	assert.Empty(t, r.Warnings(), "before the date there is nothing to warn about")
}

func TestErrIsAJoinEvenForOneFinding(t *testing.T) {
	r := Load("w.yaml", []byte("size: -1\n"), &widget{}, ctx)

	require.Error(t, r.Err())
	assert.Equal(t, 1, joinedCount(r.Err()))
	assert.Equal(t, 1, joinedCount(fmt.Errorf("config directory %s: %w", "config", r.Err())))
}

func TestErrJoinsEveryError(t *testing.T) {
	r := Load("w.yaml", []byte("size: -1\nlayout: x\n"), &widget{}, ctx)

	assert.Equal(t, 2, joinedCount(r.Err()))
	assert.Equal(t, 2, joinedCount(fmt.Errorf("config directory %s: %w", "config", r.Err())))
}

func TestWarningsAreNotInTheJoin(t *testing.T) {
	r := Load("w.yaml", []byte("size: -1\nexpires: 2026-01-01\n"), &widget{}, ctx)

	assert.Equal(t, 1, joinedCount(r.Err()))
	assert.Len(t, r.Warnings(), 1)
}

func TestFindingsAreErrorsFoundWithErrorsAs(t *testing.T) {
	r := Load("w.yaml", []byte("size: -1\n"), &widget{}, ctx)
	require.Error(t, r.Err())

	var f Finding
	require.True(t, errors.As(r.Err(), &f))

	assert.Equal(t, "spec.size", f.Path)
}

func TestNoErrorsIsANilErr(t *testing.T) {
	w := &widget{}

	r := Load("w.yaml", []byte("size: 1\n"), w, ctx)

	require.Equal(t, 1, w.Size, "the document was loaded")
	assert.NoError(t, r.Err())
	assert.Empty(t, r.Warnings())
}

func TestLoadTolerantTurnsUnknownKeysIntoWarningsAndKeepsTheRest(t *testing.T) {
	var w widget

	r := LoadTolerant("w.yaml", []byte("layout: perLine\nsize: 3\nbogus: 1\nextra: 2\n"), &w, Context{})

	require.NoError(t, r.Err())
	assert.Equal(t, "perLine", w.Layout)
	assert.Equal(t, 3, w.Size)
	warnings := r.Warnings()
	require.Len(t, warnings, 2)
	assert.Contains(t, warnings[0].Msg, "line 3")
	assert.Contains(t, warnings[0].Msg, "bogus")
	assert.Contains(t, warnings[1].Msg, "line 4")
	assert.Equal(t, "w.yaml", warnings[0].Source)
	assert.Equal(t, []string{"defaults", "validate"}, w.calls, "still defaulted and validated")
}

func TestLoadTolerantStillReportsWhatIsWrong(t *testing.T) {
	for name, data := range map[string]string{
		"a rule that fails": "layout: wrong\nbogus: 1\n",
		"a wrong type":      "size: many\nbogus: 1\n",
		"not valid YAML":    "layout: [\n",
	} {
		t.Run(name, func(t *testing.T) {
			var w widget

			r := LoadTolerant("w.yaml", []byte(data), &w, Context{})

			assert.Error(t, r.Err())
		})
	}
}

func TestLoadStaysStrict(t *testing.T) {
	var w widget

	r := Load("w.yaml", []byte("layout: perLine\nbogus: 1\n"), &w, Context{})

	assert.Error(t, r.Err())
}
