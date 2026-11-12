package bundle

import (
	"io/fs"
	"os"
	"strconv"
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
  repo: {name: rancher/` + name + `, managedBy: orbs}
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

// headerFor is the header Compile writes for the files under fsys: the one the header
// values give, then the digest of the files and the revision of Compile.
func headerFor(t *testing.T, fsys fs.FS) string {
	t.Helper()
	d, err := Digest(fsys)
	require.NoError(t, err)
	return wantHeader + "inputsDigest: " + d + "\ncompilerRevision: " + strconv.Itoa(CompilerRevision) + "\n"
}

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
	want := headerFor(t, fixture()) +
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
	head := headerFor(t, fixture())
	require.True(t, strings.HasPrefix(string(got), head), "the header comes first")
	body := string(got)[len(head):]

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
	assert.True(t, strings.HasPrefix(out, headerFor(t, os.DirFS("config"))+"---\n# source: "))
	assert.NotContains(t, out, "yaml-language-server", "the schema comments of the files are dropped")
	assert.Contains(t, out, "# source: rancher.yaml\n")
	assert.NotContains(t, out, "# source: README.md")
}

func TestCompileDoesNotDependOnTheOrderFilesAreListed(t *testing.T) {
	forward := fstest.MapFS{}
	forward["a.yaml"] = &fstest.MapFile{Data: []byte(project("alpha"))}
	forward["b.yaml"] = &fstest.MapFile{Data: []byte(project("bravo"))}
	forward["c.yaml"] = &fstest.MapFile{Data: []byte(project("charlie"))}
	backward := fstest.MapFS{}
	backward["c.yaml"] = forward["c.yaml"]
	backward["b.yaml"] = forward["b.yaml"]
	backward["a.yaml"] = forward["a.yaml"]

	a, err := Compile(forward, header)
	require.NoError(t, err)
	b, err := Compile(backward, header)
	require.NoError(t, err)

	assert.Equal(t, string(a), string(b))
}

func TestTheBodyDoesNotDependOnTheHeader(t *testing.T) {
	other := Header{GeroVersion: "v9.9.9", SourceCommit: "fedcba", CompiledAt: time.Date(2030, 6, 7, 8, 9, 10, 0, time.UTC)}

	a, err := Compile(fixture(), header)
	require.NoError(t, err)
	b, err := Compile(fixture(), other)
	require.NoError(t, err)

	_, bodyA, _ := strings.Cut(string(a), "\n---\n")
	_, bodyB, _ := strings.Cut(string(b), "\n---\n")
	assert.NotEmpty(t, bodyA)
	assert.Equal(t, bodyA, bodyB, "the version, commit and date are in the header only")
}

// warningFixture is a config that is valid and has one warning: a line at general
// availability, at the date of the header, that has no branch written.
func warningFixture() fstest.MapFS {
	return fstest.MapFS{"w.yaml": {Data: []byte(`apiVersion: gero/v1alpha2
kind: RancherLines
name: rancher
spec:
  lines:
    - id: v2.16
    - id: v2.15
      dates: {ga: 2025-08-27, eom: 2026-08-27}
---
apiVersion: gero/v1alpha2
kind: Project
name: w
spec:
  repo: {name: rancher/w, managedBy: orbs}
  branches:
    layout: perLine
    lines: [{line: v2.16, branch: release/v2.16}, {line: v2.15}]
`)}}
}

func TestCompileReportGivesTheWarningsTheConfigHas(t *testing.T) {
	data, warnings, err := CompileReport(warningFixture(), header)

	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Error(), "v2.15")
	plain, err := Compile(warningFixture(), header)
	require.NoError(t, err)
	assert.Equal(t, string(plain), string(data), "Compile is the same bundle, and says nothing of the warnings")
}
