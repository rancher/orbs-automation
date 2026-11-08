package appconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctx = config.Context{Now: time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)}

const (
	project = "apiVersion: gero/v1alpha2\nkind: Project\nname: p\nspec:\n  repo: {name: acme/a}\n"
	stream  = "apiVersion: gero/v1alpha2\nkind: Stream\nname: s\nrefs: {project: p}\nspec:\n  scheme: semver\n"
	lines   = "apiVersion: gero/v1alpha2\nkind: RancherLines\nname: rancher\nspec:\n  lines:\n    - id: v2.16\n"
)

func files(m map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, data := range m {
		fsys[name] = &fstest.MapFile{Data: []byte(data)}
	}
	return fsys
}

func errorText(rep config.Report) string {
	if err := rep.Err(); err != nil {
		return err.Error()
	}
	return ""
}

func TestLoadReadsEveryYAMLFileWhereverItIs(t *testing.T) {
	cfg, rep := Load(files(map[string]string{
		"anything.yaml":      project,
		"deep/er/place.yml":  stream,
		"rancher.lines.yaml": lines,
		"README.md":          "not config: apiVersion nonsense",
		"notes.txt":          "kind: Project",
	}), ctx)

	require.NoError(t, rep.Err())
	assert.Empty(t, rep.Findings)
	require.Len(t, cfg.Projects, 1)
	assert.Equal(t, "p", cfg.Projects[0].Name)
	require.Len(t, cfg.Streams, 1)
	assert.Equal(t, "s", cfg.Streams[0].Name)
	require.NotNil(t, cfg.Lines)
	assert.Equal(t, "v2.16", cfg.Lines.Spec.Lines[0].ID)
	assert.False(t, cfg.Empty())
}

func TestLoadReadsSeveralDocumentsOfAFile(t *testing.T) {
	cfg, rep := Load(files(map[string]string{"all.yaml": project + "---\n" + stream}), ctx)

	require.NoError(t, rep.Err())
	assert.Len(t, cfg.Projects, 1)
	assert.Len(t, cfg.Streams, 1)
}

func TestLoadOfNothingIsAnEmptyConfig(t *testing.T) {
	cfg, rep := Load(files(map[string]string{"README.md": "hello"}), ctx)

	require.NoError(t, rep.Err())
	assert.True(t, cfg.Empty())
}

func TestLoadReadsTheDesignFixture(t *testing.T) {
	dir := "../../docs/design/fixture"
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	m := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		m[e.Name()] = string(raw)
	}

	cfg, rep := Load(files(m), ctx)

	require.NoError(t, rep.Err())
	assert.Empty(t, rep.Warnings())
	assert.Len(t, cfg.Projects, 13)
	assert.Len(t, cfg.Streams, 10)
	assert.Len(t, cfg.Artifacts, 11)
	assert.Len(t, cfg.Holdings, 12)
	assert.Len(t, cfg.Edits, 2)
	assert.NotNil(t, cfg.Lines)
}

func TestLoadNamesTheFileAndTheDocumentOfAProblem(t *testing.T) {
	_, rep := Load(files(map[string]string{
		"a/good.yaml": project,
		"a/bad.yaml":  project + "---\napiVersion: gero/v1alpha2\nkind: Project\nname: q\nspec:\n  repo: {name: nope}\n",
	}), ctx)

	text := errorText(rep)
	assert.Contains(t, text, "a/bad.yaml: document 2: Project \"q\": spec.repo.name")
	assert.NotContains(t, text, "good.yaml")
}

func TestLoadReportsTheProblemsOfEveryFile(t *testing.T) {
	_, rep := Load(files(map[string]string{
		"a.yaml": "apiVersion: gero/v1alpha1\nkind: Project\nname: old\nspec: {}\n",
		"b.yaml": "kind: Project\nname: q\nspec:\n  repo: {name: nope}\n",
		"c.yaml": "apiVersion: gero/v1alpha2\nkind: Project\nname: [unclosed\n",
	}), ctx)

	text := errorText(rep)
	assert.Contains(t, text, "a.yaml")
	assert.Contains(t, text, "b.yaml")
	assert.Contains(t, text, "c.yaml")
}

func TestLoadReturnsNoConfigWhenThereIsAnError(t *testing.T) {
	cfg, rep := Load(files(map[string]string{"a.yaml": project, "b.yaml": "apiVersion: nope\nkind: Project\nname: q\n"}), ctx)

	require.Error(t, rep.Err())
	assert.True(t, cfg.Empty(), "a caller never sees a config that failed a rule")
}

func TestLoadChecksTheSetOnceEveryDocumentIsValid(t *testing.T) {
	_, rep := Load(files(map[string]string{"a.yaml": stream}), ctx)

	assert.Contains(t, errorText(rep), `a.yaml: Stream "s"`+": refs.project: no Project named \"p\"")
}

func TestLoadDoesNotCheckTheSetWhenADocumentIsInvalid(t *testing.T) {
	_, rep := Load(files(map[string]string{
		"a.yaml": "apiVersion: gero/v1alpha2\nkind: Project\nname: p\nspec:\n  repo: {name: nope}\n",
		"b.yaml": stream,
	}), ctx)

	text := errorText(rep)
	assert.Contains(t, text, "spec.repo.name")
	assert.False(t, strings.Contains(text, "no Project named"), "the stream's project is only missing because it was dropped: %s", text)
}

func TestLoadKeepsWarningsAndTheConfig(t *testing.T) {
	cfg, rep := Load(files(map[string]string{
		"lines.yaml": "apiVersion: gero/v1alpha2\nkind: RancherLines\nname: rancher\nspec:\n  lines:\n    - id: v2.15\n      dates: {ga: 2026-08-27}\n",
		"p.yaml":     "apiVersion: gero/v1alpha2\nkind: Project\nname: p\nspec:\n  repo: {name: acme/a}\n  branches:\n    layout: perLine\n    lines: [{line: v2.15}]\n",
	}), ctx)

	require.NoError(t, rep.Err())
	require.Len(t, rep.Warnings(), 1)
	assert.Contains(t, rep.Warnings()[0].Error(), "p.yaml")
	assert.Len(t, cfg.Projects, 1)
}
