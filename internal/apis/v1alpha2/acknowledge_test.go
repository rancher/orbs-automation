package v1alpha2

import (
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// docAck is doc with an acknowledge list, written as the lines of a YAML list.
func docAck(kind, name, refs, ack, spec string) string {
	d := doc(kind, name, refs, spec)
	head := "name: " + name + "\n"
	i := strings.Index(d, head) + len(head)
	return d[:i] + "acknowledge:\n" + ack + d[i:]
}

// pastDevelopment is a set whose project p has a line past development and no
// branch for it, which is one warning: v2.15.
func pastDevelopment(ack string) []string {
	linesDoc := doc("RancherLines", "rancher", "", `  lines:
    - id: v2.16
    - id: v2.15
      dates: {ga: 2026-08-27, eom: 2027-02-27}
    - id: v2.14
      dates: {ga: 2026-04-30, eom: 2026-10-30}
`)
	spec := projectSpec + "  branches:\n    layout: perLine\n    lines: [{line: v2.15}]\n"
	if ack == "" {
		return []string{linesDoc, doc("Project", "p", "", spec)}
	}
	return []string{linesDoc, docAck("Project", "p", "", ack, spec)}
}

func warningsOf(rep config.Report, rule string) []config.Finding {
	var out []config.Finding
	for _, f := range rep.Warnings() {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func TestAWarningAboutALineNamesTheLine(t *testing.T) {
	rep := validateSet(t, pastDevelopment("")...)

	require.Len(t, rep.Warnings(), 1)
	assert.Equal(t, "v2.15", rep.Warnings()[0].Line)
	assert.Empty(t, rep.Acknowledged())
}

func TestAnAcknowledgementSilencesTheWarningsOfItsRule(t *testing.T) {
	rep := validateSet(t, pastDevelopment("  - {rule: default-branch-past-development, reason: read from main by design}\n")...)

	assert.NoError(t, rep.Err())
	assert.Empty(t, rep.Warnings(), "it is acknowledged")
	require.Len(t, rep.Acknowledged(), 1)
	f := rep.Acknowledged()[0]
	assert.Equal(t, config.Warning, f.Severity)
	assert.Equal(t, RuleDefaultBranchPastDevelopment, f.Rule)
	assert.Equal(t, "read from main by design", f.Reason)
	assert.Equal(t, `Project "p"`, f.Label)
	assert.Equal(t, "v2.15", f.Line)
}

func TestAnAcknowledgementLimitedToLinesSilencesTheWarningsOfThoseLines(t *testing.T) {
	for name, lines := range map[string]string{
		"the line itself":  "{since: v2.15, until: v2.15}",
		"a range with it":  "{since: v2.14, until: v2.16}",
		"open at the top":  "{since: v2.15}",
		"open at the foot": "{until: v2.15}",
	} {
		t.Run(name, func(t *testing.T) {
			ack := "  - {rule: default-branch-past-development, reason: expected, lines: " + lines + "}\n"

			rep := validateSet(t, pastDevelopment(ack)...)

			assert.Empty(t, rep.Warnings())
			assert.Len(t, rep.Acknowledged(), 1)
		})
	}
}

func TestAnAcknowledgementThatMatchesNothingIsAWarning(t *testing.T) {
	tests := map[string]string{
		"another rule":        "  - {rule: circular-holding, reason: expected}\n",
		"lines of no warning": "  - {rule: default-branch-past-development, reason: expected, lines: {until: v2.14}}\n",
	}
	for name, ack := range tests {
		t.Run(name, func(t *testing.T) {
			rep := validateSet(t, pastDevelopment(ack)...)

			assert.NoError(t, rep.Err())
			assert.Len(t, warningsOf(rep, RuleDefaultBranchPastDevelopment), 1, "nothing was silenced")
			unused := warningsOf(rep, RuleAcknowledgementUnused)
			require.Len(t, unused, 1)
			assert.Equal(t, `Project "p"`, unused[0].Label)
			assert.Equal(t, "acknowledge[0]", unused[0].Path)
		})
	}
}

func TestAnAcknowledgementOnAnotherDocumentSilencesNothing(t *testing.T) {
	set := append(pastDevelopment(""), docAck("Project", "q", "", "  - {rule: default-branch-past-development, reason: expected}\n", "  repo: {name: acme/b}\n"))

	rep := validateSet(t, set...)

	assert.Len(t, warningsOf(rep, RuleDefaultBranchPastDevelopment), 1)
	unused := warningsOf(rep, RuleAcknowledgementUnused)
	require.Len(t, unused, 1)
	assert.Equal(t, `Project "q"`, unused[0].Label)
}

func TestAnAcknowledgementSilencesACircularHolding(t *testing.T) {
	set := []string{
		doc("Project", "p", "", projectSpec),
		doc("Project", "q", "", "  repo: {name: acme/b}\n"),
		doc("Stream", "sp", "{project: p}", streamSpec),
		doc("Stream", "sq", "{project: q}", streamSpec),
		doc("Artifact", "ap", "{stream: sp}", artifactSpec),
		doc("Artifact", "aq", "{stream: sq}", "  kind: image\n  name: acme/b\n"),
		doc("Holding", "p-holds-q", "{project: p, artifact: aq}", holdingSpec),
		docAck("Holding", "q-holds-p", "{project: q, artifact: ap}", "  - {rule: circular-holding, reason: they build each other}\n", holdingSpec),
	}

	rep := validateSet(t, set...)

	assert.Empty(t, rep.Warnings())
	require.Len(t, rep.Acknowledged(), 1)
	assert.Equal(t, "they build each other", rep.Acknowledged()[0].Reason)
}

func TestAcknowledgementRules(t *testing.T) {
	tests := []struct {
		name, ack, path, want string
	}{
		{"no rule", "  - {reason: expected}\n", "acknowledge[0].rule", "required"},
		{"a rule that does not exist", "  - {rule: nope, reason: expected}\n", "acknowledge[0].rule", "default-branch-past-development"},
		{"a rule of the user file", "  - {rule: unknown-key, reason: expected}\n", "acknowledge[0].rule", "circular-holding"},
		{"the rule of an unused acknowledgement", "  - {rule: acknowledgement-unused, reason: expected}\n", "acknowledge[0].rule", "circular-holding"},
		{"no reason", "  - {rule: default-branch-past-development}\n", "acknowledge[0].reason", "required"},
		{"a reason of only spaces", "  - {rule: default-branch-past-development, reason: '  '}\n", "acknowledge[0].reason", "required"},
		{"a since that is not a line", "  - {rule: default-branch-past-development, reason: x, lines: {since: 2.16}}\n", "acknowledge[0].lines.since", "v2.16"},
		{"a since after the until", "  - {rule: default-branch-past-development, reason: x, lines: {since: v2.16, until: v2.14}}\n", "acknowledge[0].lines", "after"},
		{"the second one", "  - {rule: default-branch-past-development, reason: x}\n  - {rule: nope, reason: x}\n", "acknowledge[1].rule", "default-branch-past-development"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs, rep := decodeReport(t, strings.Join(pastDevelopment(tt.ack), "---\n"))
			require.NoError(t, rep.Err(), "an acknowledgement is valid as a document is")
			var set config.Report
			ValidateSet(objs, rulesCtx, &set)

			assert.True(t, hasSetError(set, `Project "p"`, tt.path, tt.want), "want an error at %s containing %q, got %v", tt.path, tt.want, set.Findings)
		})
	}
}

func TestEveryKindTakesAnAcknowledgeList(t *testing.T) {
	ack := "  - {rule: circular-holding, reason: expected, lines: {since: v2.15}}\n"
	for kind, d := range map[string]string{
		"Project":  docAck("Project", "p", "", ack, projectSpec),
		"Stream":   docAck("Stream", "s", "{project: p}", ack, streamSpec),
		"Artifact": docAck("Artifact", "a", "{stream: s}", ack, artifactSpec),
		"Holding":  docAck("Holding", "h", "{project: p, artifact: a}", ack, holdingSpec),
		"Edit":     docAck("Edit", "e", "{holding: h}", ack, "  strategy: chartsPackageBump\n"),
	} {
		t.Run(kind, func(t *testing.T) {
			objs, rep := decodeReport(t, d)

			require.NoError(t, rep.Err())
			require.Len(t, objs, 1)
			got := objs[0].GetAcknowledge()
			require.Len(t, got, 1)
			assert.Equal(t, "circular-holding", got[0].Rule)
			assert.Equal(t, "expected", got[0].Reason)
			assert.Equal(t, lines.Range{Since: "v2.15"}, got[0].Lines)
		})
	}
}

func TestAnAcknowledgementHasOnlyItsFields(t *testing.T) {
	d := docAck("Project", "p", "", "  - {rule: circular-holding, reason: x, until: 2027-01-01}\n", projectSpec)

	_, rep := decodeReport(t, d)

	require.Error(t, rep.Err())
	assert.Contains(t, rep.Err().Error(), "until")
}
