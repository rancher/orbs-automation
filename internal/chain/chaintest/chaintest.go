// Package chaintest builds a chain.Graph from small YAML documents, for the tests
// of the packages that read one.
package chaintest

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rancher/orbs-automation/internal/appconfig"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chainload"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

// Now is the time the documents are validated at.
var Now = time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)

// Load decodes, validates and joins the documents, which must be a valid config:
// any error fails the test. Warnings are allowed.
func Load(t testing.TB, docs ...string) *chain.Graph {
	t.Helper()
	fsys := fstest.MapFS{}
	for i, d := range docs {
		fsys[fmt.Sprintf("doc%02d.yaml", i)] = &fstest.MapFile{Data: []byte(d)}
	}
	cfg, rep := appconfig.Load(fsys, config.Context{Now: Now})
	if err := rep.Err(); err != nil {
		t.Fatalf("the documents are not a valid config: %v", err)
	}
	return chain.Build(cfg)
}

// Doc writes a document of a kind. refs is a flow mapping such as {project: p},
// or empty; spec is the block of the spec's fields, indented two spaces.
func Doc(kind, name, refs, spec string) string {
	d := "apiVersion: gero/v1alpha2\nkind: " + kind + "\nname: " + name + "\n"
	if refs != "" {
		d += "refs: " + refs + "\n"
	}
	if spec != "" {
		d += "spec:\n" + spec
	}
	return d
}

// Lines writes the RancherLines document with the lines, newest first.
func Lines(ids ...string) string {
	var b strings.Builder
	b.WriteString("  lines:\n")
	for _, id := range ids {
		b.WriteString("    - id: " + id + "\n")
	}
	return Doc("RancherLines", "rancher", "", b.String())
}

// Projects builds a graph of plain projects, given as pairs of a document name and
// a repo slug: Projects(t, "shell", "rancher/shell", "rancher", "rancher/rancher").
func Projects(t testing.TB, namesAndSlugs ...string) *chain.Graph {
	t.Helper()
	if len(namesAndSlugs)%2 != 0 {
		t.Fatalf("chaintest.Projects wants pairs of a name and a slug, got %d values", len(namesAndSlugs))
	}
	var docs []string
	for i := 0; i < len(namesAndSlugs); i += 2 {
		docs = append(docs, Doc("Project", namesAndSlugs[i], "", "  repo: {name: "+namesAndSlugs[i+1]+"}\n"))
	}
	return Load(t, docs...)
}

// LoadFS reads the config under a file system as the workspace does, at Now, and
// returns the graph, nil when the config has an error, with every finding.
func LoadFS(fsys fs.FS) (*chain.Graph, config.Report) {
	return chainload.Load(fsys, config.Context{Now: Now})
}
