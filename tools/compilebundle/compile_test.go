package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rancher/orbs-automation/internal/chainload"
	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const schemaComment = "# yaml-language-server: $schema=https://raw.githubusercontent.com/rancher/orbs-automation/main/generated/jsonschema/gero.v1alpha2.schema.json\n"

// project is the config of one project with a stream, the least that is valid.
func project(name string) string {
	return `apiVersion: gero/v1alpha2
kind: Project
name: ` + name + `
spec:
  repo: {name: rancher/` + name + `, owned: true}
  branches: {layout: single}
---
apiVersion: gero/v1alpha2
kind: Stream
name: ` + name + `
refs: {project: ` + name + `}
spec:
  scheme: semver
  requiresPrerelease: true
  universal: true
`
}

var header = Header{
	GeroVersion:  "v0.5.0",
	SourceCommit: "abc123",
	CompiledAt:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
}

const wantHeader = `apiVersion: gero-bundle/v1
kind: Bundle
formatVersion: 1
geroVersion: v0.5.0
sourceCommit: abc123
compiledAt: 2026-01-02T03:04:05Z
`

// fixture is two files: a.yaml with the schema comment of the config files, b.yaml
// that starts with a document marker and has a schema comment further down.
func fixture() fstest.MapFS {
	return fstest.MapFS{
		"b.yaml":    {Data: []byte("---\n# b: second\n" + project("bravo") + "# yaml-language-server: $schema=elsewhere\n")},
		"a.yaml":    {Data: []byte(schemaComment + "# a: first\n" + project("alpha"))},
		"README.md": {Data: []byte("# not config\n")},
		"notes.txt": {Data: []byte("not config\n")},
	}
}

func TestCompileMakesTheHeaderThenEachFileInOrder(t *testing.T) {
	got, err := Compile(fixture(), header)

	require.NoError(t, err)
	want := wantHeader +
		"---\n# source: a.yaml\n# a: first\n" + project("alpha") +
		"---\n# source: b.yaml\n# b: second\n" + project("bravo") + "# yaml-language-server: $schema=elsewhere\n"
	assert.Equal(t, want, string(got))
}

func TestCompileRepeatsByteForByte(t *testing.T) {
	first, err := Compile(fixture(), header)
	require.NoError(t, err)

	for range 5 {
		again, err := Compile(fixture(), header)

		require.NoError(t, err)
		assert.Equal(t, string(first), string(again))
	}
}

func TestCompileTakesYAMLFilesOnly(t *testing.T) {
	fsys := fixture()
	fsys["sub/c.yml"] = &fstest.MapFile{Data: []byte(project("charlie"))}

	got, err := Compile(fsys, header)

	require.NoError(t, err)
	out := string(got)
	assert.Contains(t, out, "# source: sub/c.yml\n")
	assert.NotContains(t, out, "README")
	assert.NotContains(t, out, "notes.txt")
	assert.NotContains(t, out, "not config")
}

func TestCompileDropsOnlyTheSchemaCommentThatStartsAFile(t *testing.T) {
	got, err := Compile(fixture(), header)

	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(got), "yaml-language-server"), "the one that is not the first line stays")
	assert.Contains(t, string(got), "$schema=elsewhere")
}

func TestCompileDoesNotDoubleADocumentMarker(t *testing.T) {
	got, err := Compile(fixture(), header)

	require.NoError(t, err)
	assert.NotContains(t, string(got), "---\n---\n")
	assert.NotContains(t, string(got), "# source: b.yaml\n---\n")
}

func TestCompileAddsAFinalNewline(t *testing.T) {
	fsys := fstest.MapFS{"a.yaml": {Data: []byte(strings.TrimSuffix(project("alpha"), "\n"))}}

	got, err := Compile(fsys, header)

	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(string(got), "\n"))
	assert.False(t, strings.HasSuffix(string(got), "\n\n"))
}

func TestCompileRefusesConfigThatDoesNotValidate(t *testing.T) {
	for name, fsys := range map[string]fstest.MapFS{
		"a version that is not known": {"a.yaml": {Data: []byte(strings.Replace(project("alpha"), "gero/v1alpha2", "gero/v9", 1))}},
		"a stream of no project":      {"a.yaml": {Data: []byte(strings.Replace(project("alpha"), "refs: {project: alpha}", "refs: {project: nope}", 1))}},
		"not YAML":                    {"a.yaml": {Data: []byte("a: [\n")}},
		"no config at all":            {"README.md": {Data: []byte("# nothing\n")}},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := Compile(fsys, header)

			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

func TestCompileRefusesAHeaderValueThatBreaksTheHeader(t *testing.T) {
	for name, h := range map[string]Header{
		"a newline in the version": {GeroVersion: "v1\nx: y", SourceCommit: "c", CompiledAt: header.CompiledAt},
		"a space in the commit":    {GeroVersion: "v1", SourceCommit: "a b", CompiledAt: header.CompiledAt},
		"an empty version":         {GeroVersion: "", SourceCommit: "c", CompiledAt: header.CompiledAt},
		"an empty commit":          {GeroVersion: "v1", SourceCommit: "", CompiledAt: header.CompiledAt},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := Compile(fixture(), h)

			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

// What a consumer will do: read the body and get the config that went in.
func TestTheBodyLoadsToTheConfigThatWentIn(t *testing.T) {
	got, err := Compile(fixture(), header)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(got), wantHeader), "the header comes first")
	body := string(got)[len(wantHeader):]

	g, rep := chainload.Load(fstest.MapFS{"bundle.yaml": {Data: []byte(body)}}, config.Context{Now: header.CompiledAt})

	require.NoError(t, rep.Err())
	want, rep := chainload.Load(fixture(), config.Context{Now: header.CompiledAt})
	require.NoError(t, rep.Err())
	assert.Len(t, g.Projects(), 2)
	assert.Len(t, g.Projects(), len(want.Projects()))
	assert.Len(t, g.Holdings(), len(want.Holdings()))
}

// A bad edit to config/ fails here, before a release does.
func TestTheRealConfigCompiles(t *testing.T) {
	t.Chdir("../..")

	got, err := Compile(os.DirFS("config"), header)

	require.NoError(t, err)
	out := string(got)
	assert.True(t, strings.HasPrefix(out, wantHeader+"---\n# source: "))
	assert.NotContains(t, out, "yaml-language-server", "the schema comments of the files are dropped")
	assert.Contains(t, out, "# source: rancher.yaml\n")
	assert.NotContains(t, out, "# source: README.md")
}

func TestRunWritesTheBundleToTheOutputDirectory(t *testing.T) {
	t.Chdir("../..")
	out := filepath.Join(t.TempDir(), "dist", "nested")

	require.NoError(t, run("config", out, header))

	got, err := os.ReadFile(filepath.Join(out, "gero-config-bundle.yaml"))
	require.NoError(t, err)
	want, err := Compile(os.DirFS("config"), header)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

func TestRunWritesNothingForConfigThatDoesNotValidate(t *testing.T) {
	t.Chdir("../..")
	bad := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bad, "a.yaml"), []byte("a: [\n"), 0o644))
	out := filepath.Join(t.TempDir(), "dist")

	require.Error(t, run(bad, out, header))

	assert.NoDirExists(t, out)
}

func TestRunMustStartAtTheRepositoryRoot(t *testing.T) {
	t.Chdir(t.TempDir())

	err := run("config", t.TempDir(), header)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository root")
}
