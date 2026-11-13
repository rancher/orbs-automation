package v1alpha2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overlapSet is a Project with the streams of the patterns, named s0, s1 and so on.
func overlapSet(patterns ...string) []string {
	docs := []string{doc("Project", "p", "", projectSpec)}
	for i, pattern := range patterns {
		docs = append(docs, doc("Stream", "s"+string(rune('0'+i)), "{project: p}", streamSpec+"  tagPattern: \""+pattern+"\"\n"))
	}
	return docs
}

func TestTwoStreamsOfOneProjectWhosePatternsCanBothMatchATagAreAWarning(t *testing.T) {
	tests := map[string][]string{
		"a pattern that holds the other":          {"v*", "v3.8.*"},
		"the same pattern twice":                  {"v3.8.*", "v3.8.*"},
		"a prefix that is the start of the other": {"v3.*", "v3.8.*"},
		"a pattern with no literal start":         {"*-lts", "v3.8.*"},
	}
	for name, patterns := range tests {
		t.Run(name, func(t *testing.T) {
			rep := validateSet(t, overlapSet(patterns...)...)

			assert.NoError(t, rep.Err(), "it is a warning, never an error")
			found := warningsOf(rep, RuleStreamsOverlap)
			require.Len(t, found, 1)
			assert.Equal(t, `Stream "s1"`, found[0].Label, "on the later stream")
			assert.Equal(t, "spec.tagPattern", found[0].Path)
			assert.Contains(t, found[0].Msg, `"s0"`)
			assert.Contains(t, found[0].Msg, patterns[0])
			assert.Contains(t, found[0].Msg, patterns[1])
		})
	}
}

func TestStreamsWhosePatternsCannotBothMatchATagAreNoWarning(t *testing.T) {
	tests := map[string][]string{
		"the series of a minor":   {"v3.8.*", "v3.7.*", "v3.6.*"},
		"a prefix and none":       {"v*", "pkg-a/v*"},
		"two prefixes of modules": {"pkg-a/v*", "pkg-b/v*"},
		"one stream":              {"v*"},
	}
	for name, patterns := range tests {
		t.Run(name, func(t *testing.T) {
			rep := validateSet(t, overlapSet(patterns...)...)

			assert.Empty(t, warningsOf(rep, RuleStreamsOverlap))
		})
	}
}

func TestStreamsOfDifferentProjectsNeverOverlap(t *testing.T) {
	rep := validateSet(t,
		doc("Project", "p", "", projectSpec),
		doc("Project", "q", "", "  repo: {name: acme/b}\n"),
		doc("Stream", "sp", "{project: p}", streamSpec+"  tagPattern: \"v*\"\n"),
		doc("Stream", "sq", "{project: q}", streamSpec+"  tagPattern: \"v*\"\n"),
	)

	assert.Empty(t, warningsOf(rep, RuleStreamsOverlap))
}

func TestAStreamsOverlapCanBeAcknowledged(t *testing.T) {
	docs := overlapSet("v*", "v3.8.*")
	docs[2] = docAck("Stream", "s1", "{project: p}", "  - {rule: streams-overlap, reason: the narrow one is listed first by design}\n", streamSpec+"  tagPattern: \"v3.8.*\"\n")

	rep := validateSet(t, docs...)

	assert.Empty(t, rep.Warnings())
	require.Len(t, rep.Acknowledged(), 1)
	assert.Equal(t, RuleStreamsOverlap, rep.Acknowledged()[0].Rule)
}
