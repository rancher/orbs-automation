package graphresolve

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/gero/clicfg"
)

func erOf(t *testing.T) string {
	t.Helper()
	out, err := run(fakeWorkspace{err: errors.New("the schema reads no config")}, clicfg.OutputText, KindER, Options{})
	require.NoError(t, err)
	return out
}

func TestERStartsAsAMermaidERDiagram(t *testing.T) {
	assert.Contains(t, erOf(t), "erDiagram\n")
}

func TestERNamesEveryEntity(t *testing.T) {
	out := erOf(t)
	for _, e := range []string{"Project", "Stream", "Artifact", "Line", "LineBranch", "Holding", "Edit"} {
		assert.Regexp(t, `(?m)^\s+`+e+` \{$`, out, e)
	}
}

func TestERRelatesTheEntities(t *testing.T) {
	out := erOf(t)
	for _, rel := range []string{
		`Project ||--o{ Stream : releases`,
		`Stream ||--|{ Artifact : produces`,
		`Project ||--o{ LineBranch : lists`,
		`Line ||--o{ LineBranch : "is branched as"`,
		`Holding }o--o| Artifact : takes`,
		`Holding }o--o| Stream : "holds the series of"`,
		`Holding }o--o{ Line : "applies to"`,
		`Edit }o--|{ Holding : follows`,
		`Artifact }o--o{ Artifact : pins`,
	} {
		assert.Contains(t, out, rel)
	}
}

func TestERDrawsTheHolderAndTheProducerOfAHoldingAsTwoRelationshipsToProject(t *testing.T) {
	out := erOf(t)
	assert.Contains(t, out, `Holding }o--|| Project : holder`)
	assert.Contains(t, out, `Holding }o--|| Project : producer`)
}

func TestERSaysWhatTheBoxesCannot(t *testing.T) {
	out := erOf(t)
	assert.Contains(t, out, "%% Holding is the junction")
	assert.Contains(t, out, "%% what a pin makes is a virtual holding")
	assert.Contains(t, out, "%% a Line is a row of the RancherLines document")
}

func TestERIsTheSameEachTimeAndNeedsNoWorkspace(t *testing.T) {
	assert.Equal(t, erOf(t), erOf(t))
}

func TestERRefusesEachFilterByName(t *testing.T) {
	for flag, opts := range map[string]Options{
		"--external":   {External: true},
		"--project":    {Projects: []string{"x"}},
		"--managed-by": {ManagedBy: []string{"x"}},
		"--line":       {Line: "v2.16"},
	} {
		out, err := run(fakeWorkspace{}, clicfg.OutputText, KindER, opts)
		assert.ErrorContains(t, err, flag, flag)
		assert.Empty(t, out, flag)
	}
}

func TestERRefusesAnOutputFormatOtherThanText(t *testing.T) {
	out, err := run(fakeWorkspace{}, clicfg.OutputJSON, KindER, Options{})
	assert.Error(t, err)
	assert.Empty(t, out)
}
