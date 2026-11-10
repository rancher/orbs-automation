package v1alpha2

import (
	"fmt"
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
		doc("Project", "q", "", "  repo: {name: acme/b}\n"),
		doc("Stream", "s", "{project: q}", streamSpec),
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

			// BRO holds wrangler 3.7 on v2.16, where Rancher holds 3.8: BRO is out of
			// step, which the fixture leaves visible. Nothing else is found.
			require.Len(t, rep.Findings, 1, "%v", rep.Findings)
			f := rep.Findings[0]
			assert.Equal(t, config.Warning, f.Severity)
			assert.Equal(t, RuleStreamDiffersFromDestination, f.Rule)
			assert.Equal(t, `Holding "bro-holds-wrangler-3-7"`, f.Label)
			assert.Equal(t, "v2.16", f.Line)
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

func TestAnAliasIsNotTheNameOrAnAliasOfAnotherProject(t *testing.T) {
	tests := []struct {
		name, want string
		other      string
	}{
		{"the name of another project", `already the name of Project "q"`, "  repo: {name: acme/b}\n"},
		{"the alias of another project", `already an alias of Project "q"`, "  repo: {name: acme/b}\n  aliases: [rk]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alias := "rk"
			if strings.Contains(tt.want, "name") {
				alias = "q"
			}
			rep := validateSet(t,
				doc("Project", "q", "", tt.other),
				doc("Project", "p", "", projectSpec+"  aliases: ["+alias+"]\n"),
			)

			assert.True(t, hasSetError(rep, `Project "p"`, "spec.aliases[0]", tt.want), "%v", rep.Findings)
		})
	}
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
		assert.Equal(t, RuleCircularHolding, rep.Warnings()[0].Rule)
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
			assert.Equal(t, RuleDefaultBranchPastDevelopment, rep.Warnings()[0].Rule)
		})
	}
}

func TestOnlyOneRancherLinesDocument(t *testing.T) {
	one := doc("RancherLines", "rancher", "", "  lines:\n    - id: v2.16\n")
	two := doc("RancherLines", "other", "", "  lines:\n    - id: v2.15\n")

	rep := validateSet(t, one, two)

	assert.True(t, hasSetError(rep, `RancherLines "other"`, "spec", "only one RancherLines"), "%v", rep.Findings)
}

func TestRepoSlugsAreUniqueWhateverTheirCase(t *testing.T) {
	rep := validateSet(t,
		doc("Project", "p", "", projectSpec),
		doc("Project", "q", "", "  repo: {name: ACME/A}\n"),
	)

	assert.True(t, hasSetError(rep, `Project "q"`, "spec.repo.name", `already the repo of Project "p"`), "%v", rep.Findings)
}

func TestAnImageNameIsUniqueWithinAProject(t *testing.T) {
	base := []string{
		doc("Project", "p", "", projectSpec),
		doc("Project", "q", "", "  repo: {name: acme/b}\n"),
		doc("Stream", "s1", "{project: p}", streamSpec),
		doc("Stream", "s2", "{project: p}", streamSpec),
		doc("Stream", "t", "{project: q}", streamSpec),
		doc("Artifact", "a1", "{stream: s1}", artifactSpec),
	}

	t.Run("in another stream of the project", func(t *testing.T) {
		rep := validateSet(t, append(base, doc("Artifact", "a2", "{stream: s2}", artifactSpec))...)

		assert.True(t, hasSetError(rep, `Artifact "a2"`, "spec.name", `already an artifact of Project "p"`), "%v", rep.Findings)
	})
	t.Run("in another project is fine", func(t *testing.T) {
		rep := validateSet(t, append(base, doc("Artifact", "a2", "{stream: t}", artifactSpec))...)

		assert.Empty(t, rep.Findings)
	})
	t.Run("a file and an image of one name are not the same", func(t *testing.T) {
		rep := validateSet(t, append(base, doc("Artifact", "a2", "{stream: s2}", "  kind: file\n  name: acme/a\n"))...)

		assert.Empty(t, rep.Findings)
	})
}

func TestAModuleNameIsUniqueWithinAStreamNotAProject(t *testing.T) {
	const module = "  kind: gomod\n  name: github.com/rancher/wrangler/v3\n"
	base := []string{
		doc("Project", "p", "", projectSpec),
		doc("Stream", "s1", "{project: p}", streamSpec),
		doc("Stream", "s2", "{project: p}", streamSpec),
		doc("Artifact", "m1", "{stream: s1}", module),
	}

	t.Run("one module in the streams of two minors is fine", func(t *testing.T) {
		rep := validateSet(t, append(base, doc("Artifact", "m2", "{stream: s2}", module))...)

		assert.Empty(t, rep.Findings)
	})
	t.Run("twice in one stream is not", func(t *testing.T) {
		rep := validateSet(t, append(base, doc("Artifact", "m2", "{stream: s1}", module))...)

		assert.True(t, hasSetError(rep, `Artifact "m2"`, "spec.name", `already an artifact of Stream "s1"`), "%v", rep.Findings)
	})
}

func TestAProjectDoesNotHoldSomethingOfItself(t *testing.T) {
	p := doc("Project", "p", "", projectSpec)
	s := doc("Stream", "s", "{project: p}", streamSpec)
	a := doc("Artifact", "a", "{stream: s}", artifactSpec)

	t.Run("a release", func(t *testing.T) {
		rep := validateSet(t, p, s, a, doc("Holding", "h", "{project: p, artifact: a}", holdingSpec))

		assert.True(t, hasSetError(rep, `Holding "h"`, "refs.artifact", "its own project"), "%v", rep.Findings)
	})
	t.Run("a commit", func(t *testing.T) {
		rep := validateSet(t, p, doc("Holding", "h", "{project: p, from: p}", "  at: {file: f.yaml, key: .a}\n  holds: {kind: commit}\n"))

		assert.True(t, hasSetError(rep, `Holding "h"`, "refs.from", "its own project"), "%v", rep.Findings)
	})
}

func TestACalverStreamNeedsASingleLayoutProject(t *testing.T) {
	lines := doc("RancherLines", "rancher", "", "  lines:\n    - id: v2.16\n")
	calver := doc("Stream", "s", "{project: p}", "  scheme: calverLineStage\n  lines: [{line: v2.16}]\n")

	t.Run("per line", func(t *testing.T) {
		rep := validateSet(t, lines, doc("Project", "p", "", projectSpec+"  branches:\n    layout: perLine\n    lines: [{line: v2.16, branch: x}]\n"), calver)

		assert.True(t, hasSetError(rep, `Stream "s"`, "spec.scheme", "layout single"), "%v", rep.Findings)
	})
	t.Run("single", func(t *testing.T) {
		rep := validateSet(t, lines, doc("Project", "p", "", projectSpec), calver)

		assert.Empty(t, rep.Findings)
	})
}

func TestTheStagesAHoldingNamesAreStagesOfTheProjectsInvolved(t *testing.T) {
	charts := doc("Project", "charts", "", "  repo: {name: acme/charts}\n  branches:\n    layout: perLine\n    stages: [dev, stable]\n    lines: [{line: v2.16, branch: {dev: d, stable: s}}]\n")
	assets := doc("Project", "assets", "", "  repo: {name: acme/assets}\n")
	assetsStream := doc("Stream", "assets", "{project: assets}", "  scheme: calverLineStage\n  lines: [{line: v2.16}]\n")
	chartStream := doc("Stream", "chart", "{project: charts}", "  scheme: chartVersion\n  lines: [{line: v2.16, series: \"111.x\"}]\n")
	chartArtifact := doc("Artifact", "chart-file", "{stream: chart}", "  kind: file\n  name: chart-{version}.tgz\n")
	holder := doc("Project", "holder", "", "  repo: {name: acme/holder}\n  branches:\n    layout: perLine\n    stages: [dev, stable]\n    lines: [{line: v2.16, branch: {dev: d, stable: s}}]\n")
	plain := doc("Project", "plain", "", "  repo: {name: acme/plain}\n")
	lines := doc("RancherLines", "rancher", "", "  lines:\n    - id: v2.16\n")
	at := "  at: {file: f.yaml, key: '.a.{stage}'}\n"

	t.Run("a stage of the holder", func(t *testing.T) {
		h := doc("Holding", "h", "{project: holder, artifact: chart-file}", at+"  holds: {kind: release, stage: dev}\n")

		assert.Empty(t, validateSet(t, lines, charts, holder, chartStream, chartArtifact, h).Findings)
	})
	t.Run("a stage the holder does not have", func(t *testing.T) {
		h := doc("Holding", "h", "{project: holder, artifact: chart-file}", at+"  holds: {kind: release, stage: nightly}\n")

		rep := validateSet(t, lines, charts, holder, chartStream, chartArtifact, h)

		assert.True(t, hasSetError(rep, `Holding "h"`, "spec.holds.stage", `"nightly" is not a stage of branches.stages`), "%v", rep.Findings)
	})
	t.Run("a stage of a holder with none", func(t *testing.T) {
		h := doc("Holding", "h", "{project: plain, artifact: chart-file}", at+"  holds: {kind: release, stage: dev}\n")

		rep := validateSet(t, lines, charts, chartStream, chartArtifact, plain, h)

		assert.True(t, hasSetError(rep, `Holding "h"`, "spec.holds.stage", "branches.stages"), "%v", rep.Findings)
	})
	t.Run("stages of the producer", func(t *testing.T) {
		h := doc("Holding", "h", "{project: assets, from: charts}", at+"  holds: {kind: commit, stages: [dev, stable]}\n")

		assert.Empty(t, validateSet(t, lines, charts, chartStream, assets, assetsStream, h).Findings)
	})
	t.Run("a stage the producer does not have", func(t *testing.T) {
		h := doc("Holding", "h", "{project: assets, from: charts}", at+"  holds: {kind: commit, stages: [dev, beta]}\n")

		rep := validateSet(t, lines, charts, chartStream, assets, assetsStream, h)

		assert.True(t, hasSetError(rep, `Holding "h"`, "spec.holds.stages[1]", `"beta" is not a stage of Project "charts"`), "%v", rep.Findings)
	})
}

func TestAStepFromTheProducerNeedsAProducerATeamManages(t *testing.T) {
	holder := doc("Project", "p", "", projectSpec)
	step := "  postEdit:\n    - {run: x, from: producer}\n"
	build := func(producer string) []string {
		return []string{
			holder,
			doc("Project", "q", "", producer),
			doc("Stream", "s", "{project: q}", streamSpec),
			doc("Artifact", "a", "{stream: s}", artifactSpec),
			doc("Holding", "h", "{project: p, artifact: a}", holdingSpec),
			doc("Edit", "e", "{holding: h}", step),
		}
	}

	t.Run("a team manages it", func(t *testing.T) {
		assert.Empty(t, validateSet(t, build("  repo: {name: acme/b, managedBy: orbs}\n")...).Findings)
	})
	for name, repo := range map[string]string{
		"no team":         "  repo: {name: acme/b}\n",
		"shared":          "  repo: {name: acme/b, managedBy: shared}\n",
		"outside the org": "  repo: {name: acme/b, external: true}\n",
	} {
		t.Run(name, func(t *testing.T) {
			rep := validateSet(t, build(repo)...)

			assert.True(t, hasSetError(rep, `Edit "e"`, "spec.postEdit[0].from", `no team manages Project "q"`), "%v", rep.Findings)
		})
	}
}

func TestAGoBumpEditNeedsAHoldingInTheGoModFormat(t *testing.T) {
	base := []string{
		doc("Project", "p", "", projectSpec),
		doc("Project", "q", "", "  repo: {name: acme/b}\n"),
		doc("Stream", "s", "{project: p}", streamSpec),
		doc("Artifact", "a", "{stream: s}", "  kind: gomod\n  name: github.com/acme/a\n"),
	}
	yamlHolding := doc("Holding", "yaml-holds", "{project: q, artifact: a}", holdingSpec)
	goHolding := doc("Holding", "go-holds", "{project: q, artifact: a}", "  at: {file: go.mod, key: github.com/acme/a, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\"}\n")
	mixedHolding := doc("Holding", "mixed-holds", "{project: q, artifact: a}", "  at:\n    - {file: other.yaml, key: .a}\n    - {file: go.mod, key: github.com/acme/a, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\"}\n")
	edit := func(holdings, strategy string) string {
		return doc("Edit", "e", "{holding: "+holdings+"}", "  strategy: "+strategy+"\n")
	}

	t.Run("over a holding in the yaml format", func(t *testing.T) {
		rep := validateSet(t, append(base, yamlHolding, edit("yaml-holds", "goBump"))...)

		assert.True(t, hasSetError(rep, `Edit "e"`, "refs.holding", "gomod"), "%v", rep.Findings)
	})
	t.Run("over a holding in the gomod format", func(t *testing.T) {
		rep := validateSet(t, append(base, goHolding, edit("go-holds", "goBump"))...)

		assert.Empty(t, rep.Findings)
	})
	t.Run("over a holding with one location of each", func(t *testing.T) {
		rep := validateSet(t, append(base, mixedHolding, edit("mixed-holds", "goBump"))...)

		assert.Empty(t, rep.Findings)
	})
	t.Run("over several, one of them in the gomod format", func(t *testing.T) {
		rep := validateSet(t, append(base, yamlHolding, goHolding, edit("[yaml-holds, go-holds]", "goBump"))...)

		assert.Empty(t, rep.Findings)
	})
	t.Run("another strategy has no such need", func(t *testing.T) {
		rep := validateSet(t, append(base, yamlHolding, edit("yaml-holds", "chartsPackageBump"))...)

		assert.Empty(t, rep.Findings)
	})
}

// pinWorld is a producer with a stream and two Go modules, the first of which pins
// the second, and one more producer a Go module of its own.
func pinWorld(extra ...string) []string {
	const gomod = "  kind: gomod\n  name: github.com/acme/"
	docs := []string{
		doc("Project", "wr", "", "  repo: {name: acme/wr}\n"),
		doc("Project", "la", "", "  repo: {name: acme/la}\n"),
		doc("Stream", "wr", "{project: wr}", streamSpec),
		doc("Stream", "la", "{project: la}", streamSpec),
		doc("Artifact", "la-mod", "{stream: la}", gomod+"la\n"),
		doc("Artifact", "la-img", "{stream: la}", artifactSpec),
	}
	return append(docs, extra...)
}

func TestAPinOfAGoModuleOnAGoModuleIsValid(t *testing.T) {
	rep := validateSet(t, pinWorld(doc("Artifact", "wr-mod", "{stream: wr, pins: la-mod}", "  kind: gomod\n  name: github.com/acme/wr\n"))...)

	assert.Empty(t, rep.Findings)
}

func TestAPinNeedsAnArtifactThatExistsAndIsAGoModule(t *testing.T) {
	const wr = "  kind: gomod\n  name: github.com/acme/wr\n"

	rep := validateSet(t, pinWorld(doc("Artifact", "wr-mod", "{stream: wr, pins: la-img}", wr))...)
	assert.True(t, hasSetError(rep, `Artifact "wr-mod"`, "refs.pins[0]", "gomod"), "%v", rep.Findings)

	rep = validateSet(t, pinWorld(doc("Artifact", "wr-mod", "{stream: wr, pins: nope}", wr))...)
	assert.True(t, hasSetError(rep, `Artifact "wr-mod"`, "refs.pins[0]", `no Artifact named "nope"`), "%v", rep.Findings)
}

func TestAnArtifactCannotPinItselfOrCircle(t *testing.T) {
	const wr = "  kind: gomod\n  name: github.com/acme/wr\n"

	rep := validateSet(t, pinWorld(doc("Artifact", "wr-mod", "{stream: wr, pins: wr-mod}", wr))...)
	assert.True(t, hasSetError(rep, `Artifact "wr-mod"`, "refs.pins[0]", "itself"), "%v", rep.Findings)

	world := pinWorld(
		doc("Artifact", "wr-mod", "{stream: wr, pins: la-pin}", wr),
		doc("Artifact", "la-pin", "{stream: la, pins: wr-mod}", "  kind: gomod\n  name: github.com/acme/lb\n"),
	)
	rep = validateSet(t, world...)
	assert.True(t, hasSetError(rep, `Artifact "wr-mod"`, "refs.pins", "cycle"), "%v", rep.Findings)
	assert.True(t, hasSetError(rep, `Artifact "la-pin"`, "refs.pins", "cycle"), "%v", rep.Findings)
}

// pinned is a consumer that holds a Go module that pins another: bro holds
// wr-mod for v2.15 and newer, and the pin makes a virtual Holding of la-mod there.
func pinned(extra ...string) []string {
	const gomodAt = "  at: {file: go.mod, key: github.com/acme/%s, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all, lines: %s}\n"
	docs := []string{
		doc("RancherLines", "rancher", "", "  lines:\n    - id: v2.16\n    - id: v2.15\n    - id: v2.14\n"),
		doc("Project", "wr", "", "  repo: {name: acme/wr}\n"),
		doc("Project", "la", "", "  repo: {name: acme/la}\n"),
		doc("Project", "bro", "", "  repo: {name: acme/bro}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: r15}, {line: v2.14, branch: r14}]\n"),
		doc("Stream", "wr", "{project: wr}", streamSpec),
		doc("Stream", "la", "{project: la}", streamSpec),
		doc("Artifact", "la-mod", "{stream: la}", "  kind: gomod\n  name: github.com/acme/la\n"),
		doc("Artifact", "wr-mod", "{stream: wr, pins: la-mod}", "  kind: gomod\n  name: github.com/acme/wr\n"),
		doc("Holding", "bro-holds-wr", "{project: bro, artifact: wr-mod}", fmt.Sprintf(gomodAt, "wr", "{since: v2.15}")),
	}
	return append(docs, extra...)
}

func explicitLasso(name, file, lines string) string {
	return doc("Holding", name, "{project: bro, artifact: la-mod}",
		"  at: {file: "+file+", key: github.com/acme/la, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all, lines: "+lines+"}\n")
}

func TestAPinWithNoExplicitHoldingOfThePinnedModuleIsValid(t *testing.T) {
	rep := validateSet(t, pinned()...)

	assert.Empty(t, rep.Findings)
}

func TestAnExplicitHoldingThatOverlapsAVirtualOneIsAnError(t *testing.T) {
	rep := validateSet(t, pinned(explicitLasso("bro-holds-la", "go.mod", "{}"))...)

	require.Error(t, rep.Err())
	assert.True(t, hasSetError(rep, `Holding "bro-holds-la"`, "spec.at", "virtual"), "%v", rep.Findings)
	assert.True(t, hasSetError(rep, `Holding "bro-holds-la"`, "spec.at", `"bro-holds-wr"`), "it names the Holding the pin comes from: %v", rep.Findings)
}

func TestAnExplicitHoldingForTheLinesNoPinCoversIsValid(t *testing.T) {
	rep := validateSet(t, pinned(explicitLasso("bro-holds-la", "go.mod", "{until: v2.14}"))...)

	assert.Empty(t, rep.Findings, "the wrangler Holding is for v2.15 and newer")
}

func TestAnExplicitHoldingOfAnotherFileIsValid(t *testing.T) {
	rep := validateSet(t, pinned(explicitLasso("bro-holds-la", "tool/go.mod", "{}"))...)

	assert.Empty(t, rep.Findings)
}

func TestTwoPinsOfOneModuleInOnePlaceOverlap(t *testing.T) {
	rep := validateSet(t, pinned(
		doc("Stream", "wr2", "{project: wr}", streamSpec),
		doc("Artifact", "wr2-mod", "{stream: wr2, pins: la-mod}", "  kind: gomod\n  name: github.com/acme/wr2\n"),
		doc("Holding", "bro-holds-wr2", "{project: bro, artifact: wr2-mod}", "  at: {file: go.mod, key: github.com/acme/wr2, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all, lines: {until: v2.15}}\n"),
	)...)

	require.Error(t, rep.Err())
	assert.True(t, hasSetError(rep, `Holding "bro-holds-wr2"`, "refs.artifact", "virtual"), "%v", rep.Findings)
}

func TestTwoPinsOfOneModuleForDifferentLinesDoNotOverlap(t *testing.T) {
	rep := validateSet(t, pinned(
		doc("Stream", "wr2", "{project: wr}", streamSpec),
		doc("Artifact", "wr2-mod", "{stream: wr2, pins: la-mod}", "  kind: gomod\n  name: github.com/acme/wr2\n"),
		doc("Holding", "bro-holds-wr2", "{project: bro, artifact: wr2-mod}", "  at: {file: go.mod, key: github.com/acme/wr2, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all, lines: {until: v2.14}}\n"),
	)...)

	assert.Empty(t, rep.Findings)
}
