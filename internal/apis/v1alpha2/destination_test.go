package v1alpha2

import (
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minors is a library with a Stream and an Artifact for each of two minors, a
// destination that holds one minor for each of two lines, and the rancher lines.
func minors() []string {
	return []string{
		doc("RancherLines", "rancher", "", "  lines:\n    - id: v2.16\n    - id: v2.15\n"),
		doc("Project", "lib", "", "  repo: {name: acme/lib, owned: true}\n"),
		doc("Stream", "lib-1-2", "{project: lib}", "  scheme: semver\n  tagPattern: \"v1.2.*\"\n  universal: true\n"),
		doc("Stream", "lib-1-1", "{project: lib}", "  scheme: semver\n  tagPattern: \"v1.1.*\"\n  universal: true\n"),
		doc("Artifact", "lib-1-2-module", "{stream: lib-1-2}", "  kind: gomod\n  name: github.com/acme/lib\n"),
		doc("Artifact", "lib-1-1-module", "{stream: lib-1-1}", "  kind: gomod\n  name: github.com/acme/lib\n"),
		doc("Project", "dest", "", "  repo: {name: acme/dest}\n  finalDestination: true\n"+perLine),
		docHolds("dest-holds-1-2", "dest", "lib-1-2-module", "{since: v2.16}"),
		docHolds("dest-holds-1-1", "dest", "lib-1-1-module", "{until: v2.15}"),
	}
}

const perLine = "  branches:\n    layout: perLine\n    lines: [{line: v2.16, branch: main}, {line: v2.15, branch: release/v2.15}]\n"

// docHolds is a holding of the library's go.mod by a project, for some lines.
func docHolds(name, project, artifact, lines string) string {
	return docHoldsAck(name, project, artifact, lines, "")
}

func docHoldsAck(name, project, artifact, lines, ack string) string {
	spec := "  at: {file: go.mod, key: github.com/acme/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all, lines: " + lines + "}\n"
	refs := "{project: " + project + ", artifact: " + artifact + "}"
	if ack == "" {
		return doc("Holding", name, refs, spec)
	}
	return docAck("Holding", name, refs, ack, spec)
}

// holder is a project with a branch for each line, or one with a single layout.
func holder(layout string) string {
	if layout == "single" {
		return doc("Project", "svc", "", "  repo: {name: acme/svc}\n")
	}
	return doc("Project", "svc", "", "  repo: {name: acme/svc}\n"+perLine)
}

func differs(rep config.Report) []config.Finding {
	var out []config.Finding
	for _, f := range rep.Warnings() {
		if f.Rule == RuleStreamDiffersFromDestination {
			out = append(out, f)
		}
	}
	return out
}

func TestAHolderOnTheSameStreamsAsTheDestinationHasNoWarning(t *testing.T) {
	set := append(minors(),
		holder("perLine"),
		docHolds("svc-holds-1-2", "svc", "lib-1-2-module", "{since: v2.16}"),
		docHolds("svc-holds-1-1", "svc", "lib-1-1-module", "{until: v2.15}"),
	)

	rep := validateSet(t, set...)

	assert.Empty(t, rep.Findings)
}

func TestAHolderOnAnotherStreamForALineIsAWarningNamingTheLine(t *testing.T) {
	set := append(minors(),
		holder("perLine"),
		docHolds("svc-holds-1-1", "svc", "lib-1-1-module", "{since: v2.15}"), // the destination has 1.2 on v2.16
	)

	rep := validateSet(t, set...)

	assert.NoError(t, rep.Err())
	got := differs(rep)
	require.Len(t, got, 1)
	assert.Equal(t, `Holding "svc-holds-1-1"`, got[0].Label)
	assert.Equal(t, "v2.16", got[0].Line)
	assert.Contains(t, got[0].Msg, "lib-1-1")
	assert.Contains(t, got[0].Msg, "lib-1-2")
	assert.Contains(t, got[0].Msg, "dest")
}

func TestOnlyTheLinesThatDifferAreWarnings(t *testing.T) {
	set := append(minors(),
		holder("perLine"),
		docHolds("svc-holds-1-1", "svc", "lib-1-1-module", "{since: v2.15, until: v2.16}"),
		docHolds("svc-holds-1-2", "svc", "lib-1-2-module", "{until: v2.14}"), // no line of the holder is this
	)

	rep := validateSet(t, set...)

	got := differs(rep)
	require.Len(t, got, 1, "v2.15 is on 1.1 in both")
	assert.Equal(t, "v2.16", got[0].Line)
}

func TestWhatIsNotComparedHasNoWarning(t *testing.T) {
	tests := map[string][]string{
		"a holder with a single layout": {
			holder("single"),
			docHolds("svc-holds-1-1", "svc", "lib-1-1-module", "{since: v2.15}"),
		},
		"a module the destination does not hold": {
			holder("perLine"),
			doc("Stream", "other", "{project: lib}", "  scheme: semver\n  tagPattern: \"v9.*\"\n  universal: true\n"),
			doc("Artifact", "other-module", "{stream: other}", "  kind: gomod\n  name: github.com/acme/other\n"),
			"apiVersion: gero/v1alpha2\nkind: Holding\nname: svc-holds-other\nrefs: {project: svc, artifact: other-module}\nspec:\n  at: {file: go.mod, key: github.com/acme/other, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\"}\n",
		},
	}
	for name, extra := range tests {
		t.Run(name, func(t *testing.T) {
			rep := validateSet(t, append(minors(), extra...)...)

			assert.Empty(t, differs(rep))
		})
	}
}

func TestNothingIsComparedWithoutAFinalDestination(t *testing.T) {
	set := minors()
	for i, d := range set {
		set[i] = strings.Replace(d, "  finalDestination: true\n", "", 1)
	}
	set = append(set, holder("perLine"), docHolds("svc-holds-1-1", "svc", "lib-1-1-module", "{since: v2.15}"))

	rep := validateSet(t, set...)

	assert.Empty(t, differs(rep))
}

func TestTheDestinationIsNotComparedWithItself(t *testing.T) {
	rep := validateSet(t, minors()...)

	assert.Empty(t, differs(rep))
}

func TestADifferenceCanBeAcknowledgedOnTheHolderAndNotOnTheDestination(t *testing.T) {
	ack := "  - {rule: stream-differs-from-destination, reason: svc stays on 1.1 until its next release, lines: {since: v2.16, until: v2.16}}\n"

	t.Run("on the holder", func(t *testing.T) {
		set := append(minors(), holder("perLine"), docHoldsAck("svc-holds-1-1", "svc", "lib-1-1-module", "{since: v2.15}", ack))

		rep := validateSet(t, set...)

		assert.Empty(t, rep.Warnings())
		require.Len(t, rep.Acknowledged(), 1)
		assert.Equal(t, "svc stays on 1.1 until its next release", rep.Acknowledged()[0].Reason)
	})
	t.Run("on the destination it does not apply", func(t *testing.T) {
		set := minors()
		set[7] = docHoldsAck("dest-holds-1-2", "dest", "lib-1-2-module", "{since: v2.16}", ack)
		set = append(set, holder("perLine"), docHolds("svc-holds-1-1", "svc", "lib-1-1-module", "{since: v2.15}"))

		rep := validateSet(t, set...)

		assert.Len(t, differs(rep), 1, "the holder still warns")
		assert.Len(t, warningsOf(rep, RuleAcknowledgementUnused), 1, "the destination's acknowledgement matches nothing")
	})
}

func TestTheRuleIsOneADocumentCanAcknowledge(t *testing.T) {
	ack := "  - {rule: stream-differs-from-destination, reason: x}\n"
	set := append(minors(), holder("perLine"), docHoldsAck("svc-holds-1-2", "svc", "lib-1-2-module", "{since: v2.16}", ack))
	objs, rep := decodeReport(t, strings.Join(set, "---\n"))
	require.NoError(t, rep.Err())

	var out config.Report
	ValidateSet(objs, rulesCtx, &out)

	assert.NoError(t, out.Err(), "it is a rule a document may name, though this one has nothing to acknowledge")
}
