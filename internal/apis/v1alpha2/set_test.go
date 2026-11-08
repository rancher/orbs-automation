package v1alpha2

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// doc writes a document of a kind, with its refs and spec as flow or block YAML.
func doc(kind, name, refs, spec string) string {
	d := "apiVersion: gero/v1alpha2\nkind: " + kind + "\nname: " + name + "\n"
	if refs != "" {
		d += "refs: " + refs + "\n"
	}
	if spec != "" {
		d += "spec:\n" + spec
	}
	return d
}

const (
	projectSpec  = "  repo: {name: acme/a}\n"
	streamSpec   = "  scheme: semver\n  universal: true\n"
	artifactSpec = "  kind: image\n  name: acme/a\n"
	holdingSpec  = "  at: {file: f.yaml, key: .a}\n  holds: {kind: release}\n"
)

// validateSet decodes the documents, which must all be valid alone, and checks
// them as a set.
func validateSet(t *testing.T, docs ...string) config.Report {
	t.Helper()
	objs, rep := decodeReport(t, strings.Join(docs, "---\n"))
	require.NoError(t, rep.Err(), "each document is valid alone")
	require.NotEmpty(t, objs)
	var set config.Report
	ValidateSet(objs, rulesCtx, &set)
	return set
}

// hasSetError reports whether an error finding on the labelled document has the
// path and a message that contains want.
func hasSetError(rep config.Report, label, path, want string) bool {
	for _, f := range rep.Findings {
		if f.Severity == config.Error && f.Label == label && f.Path == path && strings.Contains(f.Msg, want) {
			return true
		}
	}
	return false
}

func TestAValidSetHasNoFindings(t *testing.T) {
	rep := validateSet(t,
		doc("Project", "p", "", projectSpec),
		doc("Stream", "s", "{project: p}", streamSpec),
		doc("Artifact", "a", "{stream: s}", artifactSpec),
		doc("Holding", "h", "{project: p, artifact: a}", holdingSpec),
		doc("Edit", "e", "{holding: [h]}", "  postEdit:\n    - {run: x}\n"),
	)

	assert.Empty(t, rep.Findings)
}

func TestTheFixtureIsAValidSet(t *testing.T) {
	sets := map[string][]string{"fixture": nil}
	err := filepath.WalkDir(fixtureDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".yaml" {
			return err
		}
		if filepath.Base(filepath.Dir(path)) == "examples" {
			sets[path] = nil // each example is a set with the fixture it builds on
		} else {
			sets["fixture"] = append(sets["fixture"], path)
		}
		return nil
	})
	require.NoError(t, err)
	for name := range sets {
		if name != "fixture" {
			sets[name] = append([]string{name}, sets["fixture"]...)
		}
	}

	for name, files := range sets {
		t.Run(filepath.Base(name), func(t *testing.T) {
			require.NotEmpty(t, files)
			var docs []string
			for _, f := range files {
				raw, err := os.ReadFile(f)
				require.NoError(t, err)
				docs = append(docs, string(raw))
			}

			rep := validateSet(t, docs...)

			assert.Empty(t, rep.Findings)
		})
	}
}

func TestNamesAreUniqueWithinAKind(t *testing.T) {
	rep := validateSet(t,
		doc("Project", "p", "", projectSpec),
		doc("Project", "p", "", "  repo: {name: acme/b}\n"),
	)

	assert.True(t, hasSetError(rep, `Project "p"`, "name", "twice"), "%v", rep.Findings)
}

func TestTheSameNameInTwoKindsIsFine(t *testing.T) {
	rep := validateSet(t,
		doc("Project", "x", "", projectSpec),
		doc("Stream", "x", "{project: x}", streamSpec),
	)

	assert.Empty(t, rep.Findings)
}

func TestRefsRules(t *testing.T) {
	p := doc("Project", "p", "", projectSpec)
	s := doc("Stream", "s", "{project: p}", streamSpec)
	a := doc("Artifact", "a", "{stream: s}", artifactSpec)
	h := doc("Holding", "h", "{project: p, artifact: a}", holdingSpec)
	tests := []struct {
		name        string
		docs        []string
		label, path string
		want        string
	}{
		{"a stream with no project", []string{p, doc("Stream", "s", "", streamSpec)}, `Stream "s"`, "refs.project", "required"},
		{"an artifact with no stream", []string{p, s, doc("Artifact", "a", "", artifactSpec)}, `Artifact "a"`, "refs.stream", "required"},
		{"a holding with no project", []string{p, s, a, doc("Holding", "h", "{artifact: a}", holdingSpec)}, `Holding "h"`, "refs.project", "required"},
		{"an edit with no holding", []string{p, s, a, h, doc("Edit", "e", "", "  postEdit:\n    - {run: x}\n")}, `Edit "e"`, "refs.holding", "required"},
		{"a stream whose project is missing", []string{doc("Stream", "s", "{project: nope}", streamSpec)}, `Stream "s"`, "refs.project", `no Project named "nope"`},
		{"a stream whose project is of another kind", []string{doc("Project", "q", "", projectSpec), doc("Stream", "s", "{project: p}", streamSpec), doc("Artifact", "p", "{stream: s}", artifactSpec)}, `Stream "s"`, "refs.project", `no Project named "p"`},
		{"an artifact whose stream is missing", []string{p, doc("Artifact", "a", "{stream: nope}", artifactSpec)}, `Artifact "a"`, "refs.stream", `no Stream named "nope"`},
		{"a holding whose artifact is missing", []string{p, s, doc("Holding", "h", "{project: p, artifact: nope}", holdingSpec)}, `Holding "h"`, "refs.artifact", `no Artifact named "nope"`},
		{"a holding whose from is missing", []string{p, doc("Holding", "h", "{project: p, from: nope}", "  at: {file: f.yaml, key: .a}\n  holds: {kind: commit}\n")}, `Holding "h"`, "refs.from", `no Project named "nope"`},
		{"an edit whose second holding is missing", []string{p, s, a, h, doc("Edit", "e", "{holding: [h, nope]}", "  postEdit:\n    - {run: x}\n")}, `Edit "e"`, "refs.holding[1]", `no Holding named "nope"`},
		{"a key that is not a ref of the kind", []string{p, doc("Stream", "s", "{project: p, colour: red}", streamSpec)}, `Stream "s"`, "refs.colour", "not a ref of a Stream"},
		{"a ref on a project", []string{doc("Project", "p", "{stream: s}", projectSpec)}, `Project "p"`, "refs.stream", "not a ref of a Project"},
		{"two projects for a stream", []string{p, doc("Project", "q", "", "  repo: {name: acme/b}\n"), doc("Stream", "s", "{project: [p, q]}", streamSpec)}, `Stream "s"`, "refs.project", "one name"},
		{"an empty name", []string{p, doc("Stream", "s", "{project: ''}", streamSpec)}, `Stream "s"`, "refs.project", "required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := validateSet(t, tt.docs...)

			assert.True(t, hasSetError(rep, tt.label, tt.path, tt.want), "want an error on %s at %s containing %q, got %v", tt.label, tt.path, tt.want, rep.Findings)
		})
	}
}

func TestRepoSlugsAreUnique(t *testing.T) {
	rep := validateSet(t,
		doc("Project", "p", "", projectSpec),
		doc("Project", "q", "", projectSpec),
	)

	assert.True(t, hasSetError(rep, `Project "q"`, "spec.repo.name", `already the repo of Project "p"`), "%v", rep.Findings)
}

func TestAtMostOneProjectIsTheFinalDestination(t *testing.T) {
	rep := validateSet(t,
		doc("Project", "p", "", projectSpec+"  finalDestination: true\n"),
		doc("Project", "q", "", "  repo: {name: acme/b}\n  finalDestination: true\n"),
	)

	assert.True(t, hasSetError(rep, `Project "q"`, "spec.finalDestination", `already Project "p"`), "%v", rep.Findings)
}

func TestOneFinalDestinationIsFine(t *testing.T) {
	rep := validateSet(t,
		doc("Project", "p", "", projectSpec+"  finalDestination: true\n"),
		doc("Project", "q", "", "  repo: {name: acme/b}\n"),
	)

	assert.Empty(t, rep.Findings)
}

func TestLinesNamedAreInRancherLines(t *testing.T) {
	lines := doc("RancherLines", "rancher", "", "  lines:\n    - id: v2.16\n    - id: v2.15\n")
	p := doc("Project", "p", "", projectSpec)
	s := doc("Stream", "s", "{project: p}", streamSpec)
	a := doc("Artifact", "a", "{stream: s}", artifactSpec)
	h := doc("Holding", "h", "{project: p, artifact: a}", holdingSpec)
	tests := []struct {
		name        string
		docs        []string
		label, path string
	}{
		{"a branch line", []string{lines, doc("Project", "p", "", projectSpec+"  branches:\n    layout: perLine\n    lines: [{line: v2.99, branch: x}]\n")}, `Project "p"`, "spec.branches.lines[0].line"},
		{"a stream line", []string{lines, p, doc("Stream", "s", "{project: p}", "  lines: [{line: v2.99, series: 9.x}]\n")}, `Stream "s"`, "spec.lines[0].line"},
		{"an artifact since", []string{lines, p, s, doc("Artifact", "a", "{stream: s}", artifactSpec+"  lines: {since: v2.99}\n")}, `Artifact "a"`, "spec.lines.since"},
		{"a holding until", []string{lines, p, s, a, doc("Holding", "h", "{project: p, artifact: a}", "  at: {file: f.yaml, key: .a}\n  holds: {kind: release, lines: {until: v2.99}}\n")}, `Holding "h"`, "spec.holds.lines.until"},
		{"an edit since", []string{lines, p, s, a, h, doc("Edit", "e", "{holding: h}", "  postEdit:\n    - {run: x, since: v2.99}\n")}, `Edit "e"`, "spec.postEdit[0].since"},
		{"a line with no lines document", []string{p, doc("Stream", "s", "{project: p}", "  lines: [{line: v2.16, series: 9.x}]\n")}, `Stream "s"`, "spec.lines[0].line"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := validateSet(t, tt.docs...)

			assert.True(t, hasSetError(rep, tt.label, tt.path, "is not in RancherLines"), "want an error on %s at %s, got %v", tt.label, tt.path, rep.Findings)
		})
	}
}

func TestLinesInRancherLinesAreFine(t *testing.T) {
	rep := validateSet(t,
		doc("RancherLines", "rancher", "", "  lines:\n    - id: v2.16\n"),
		doc("Project", "p", "", projectSpec+"  branches:\n    layout: perLine\n    lines: [{line: v2.16, branch: x}]\n"),
		doc("Stream", "s", "{project: p}", "  lines: [{line: v2.16, series: 9.x}]\n"),
		doc("Artifact", "a", "{stream: s}", artifactSpec+"  lines: {since: v2.16, until: v2.16}\n"),
	)

	assert.Empty(t, rep.Findings)
}

// hasSetWarning reports whether a warning on the labelled document has the path
// and a message that contains want.
func hasSetWarning(rep config.Report, label, path, want string) bool {
	for _, f := range rep.Warnings() {
		if f.Label == label && f.Path == path && strings.Contains(f.Msg, want) {
			return true
		}
	}
	return false
}

func TestAHoldingRefsWhatItsKindHolds(t *testing.T) {
	p := doc("Project", "p", "", projectSpec)
	q := doc("Project", "q", "", "  repo: {name: acme/b}\n")
	s := doc("Stream", "s", "{project: q}", streamSpec)
	a := doc("Artifact", "a", "{stream: s}", artifactSpec)
	commit := "  at: {file: f.yaml, key: .a}\n  holds: {kind: commit}\n"
	branch := "  at: {file: f.yaml, key: .a}\n  holds: {kind: branch}\n"
	tests := []struct {
		name, holding, path, want string
	}{
		{"a release with no artifact", doc("Holding", "h", "{project: p}", holdingSpec), "refs.artifact", "required"},
		{"a release with a from", doc("Holding", "h", "{project: p, artifact: a, from: q}", holdingSpec), "refs.from", "release"},
		{"a commit with no from", doc("Holding", "h", "{project: p}", commit), "refs.from", "required"},
		{"a commit with an artifact", doc("Holding", "h", "{project: p, from: q, artifact: a}", commit), "refs.artifact", "commit"},
		{"a branch with no from", doc("Holding", "h", "{project: p}", branch), "refs.from", "required"},
		{"a branch with an artifact", doc("Holding", "h", "{project: p, from: q, artifact: a}", branch), "refs.artifact", "branch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := validateSet(t, p, q, s, a, tt.holding)

			assert.True(t, hasSetError(rep, `Holding "h"`, tt.path, tt.want), "want an error at %s containing %q, got %v", tt.path, tt.want, rep.Findings)
		})
	}
}

func TestAHoldingOfTheRightRefsIsFine(t *testing.T) {
	rep := validateSet(t,
		doc("Project", "p", "", projectSpec),
		doc("Project", "q", "", "  repo: {name: acme/b}\n"),
		doc("Stream", "s", "{project: q}", streamSpec),
		doc("Artifact", "a", "{stream: s}", artifactSpec),
		doc("Holding", "hr", "{project: p, artifact: a}", holdingSpec),
		doc("Holding", "hc", "{project: p, from: q}", "  at: {file: f.yaml, key: .a}\n  holds: {kind: commit}\n"),
	)

	assert.Empty(t, rep.Findings)
}

func TestProjectsThatHoldSomethingOfEachOtherAreAWarning(t *testing.T) {
	set := []string{
		doc("Project", "p", "", projectSpec),
		doc("Project", "q", "", "  repo: {name: acme/b}\n"),
		doc("Stream", "sp", "{project: p}", streamSpec),
		doc("Stream", "sq", "{project: q}", streamSpec),
		doc("Artifact", "ap", "{stream: sp}", artifactSpec),
		doc("Artifact", "aq", "{stream: sq}", "  kind: image\n  name: acme/b\n"),
		doc("Holding", "p-holds-q", "{project: p, artifact: aq}", holdingSpec),
	}

	t.Run("one way is fine", func(t *testing.T) {
		assert.Empty(t, validateSet(t, set...).Findings)
	})
	t.Run("both ways is a warning on the later holding", func(t *testing.T) {
		rep := validateSet(t, append(set, doc("Holding", "q-holds-p", "{project: q, artifact: ap}", holdingSpec))...)

		assert.NoError(t, rep.Err(), "a circular dependency is not an error")
		assert.True(t, hasSetWarning(rep, `Holding "q-holds-p"`, "refs", "circular"), "%v", rep.Findings)
		assert.Len(t, rep.Warnings(), 1)
	})
	t.Run("through a commit it is too", func(t *testing.T) {
		rep := validateSet(t, append(set, doc("Holding", "q-tracks-p", "{project: q, from: p}", "  at: {file: f.yaml, key: .a}\n  holds: {kind: commit}\n"))...)

		assert.True(t, hasSetWarning(rep, `Holding "q-tracks-p"`, "refs", "circular"), "%v", rep.Findings)
	})
	t.Run("a project that holds its own is not", func(t *testing.T) {
		rep := validateSet(t, append(set, doc("Holding", "p-holds-p", "{project: p, artifact: ap}", holdingSpec))...)

		assert.Empty(t, rep.Warnings())
	})
}

func TestALineWithNoBranchAfterDevelopmentIsAWarning(t *testing.T) {
	lines := doc("RancherLines", "rancher", "", `  lines:
    - id: v2.16
    - id: v2.15
      dates: {ga: 2026-08-27, eom: 2027-02-27}
    - id: v2.14
      dates: {ga: 2026-04-30, eom: 2026-10-30}
`)
	project := func(branchLines string) string {
		return doc("Project", "p", "", projectSpec+"  branches:\n    layout: perLine\n    lines: ["+branchLines+"]\n")
	}

	tests := []struct {
		name, lines, want string
	}{
		{"a line in development", "{line: v2.16}", ""},
		{"a line at ga", "{line: v2.15}", "ga"},
		{"a line at eom", "{line: v2.14}", "eom"},
		{"a line at ga with a branch", "{line: v2.15, branch: release/v2.15}", ""},
		{"a line at ga with no branch yet", "{line: v2.15, branch: \"\"}", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := validateSet(t, lines, project(tt.lines))

			assert.NoError(t, rep.Err())
			if tt.want == "" {
				assert.Empty(t, rep.Findings)
				return
			}
			assert.True(t, hasSetWarning(rep, `Project "p"`, "spec.branches.lines[0].branch", tt.want), "%v", rep.Findings)
		})
	}
}
