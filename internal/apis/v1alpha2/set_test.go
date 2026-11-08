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
	streamSpec   = "  scheme: semver\n"
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
			sets[path] = nil // each example is a set of its own, with the lines
		} else {
			sets["fixture"] = append(sets["fixture"], path)
		}
		return nil
	})
	require.NoError(t, err)
	for name := range sets {
		if name != "fixture" {
			sets[name] = []string{name, filepath.Join(fixtureDir, "rancher.lines.yaml")}
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
