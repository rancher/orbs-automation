package bump

import (
	"context"
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bg = context.Background()

// holdings is a consumer that holds the releases of a producer: an image in
// build.yaml, and the address of a file in a package list, on the stage dev.
func holdings(t *testing.T, specs ...string) []*chain.Holding {
	t.Helper()
	return holdingsOfStream(t, "  universal: true\n", specs...)
}

// holdingsOfStream is holdings for a producer whose stream has the given spec.
func holdingsOfStream(t *testing.T, streamSpec string, specs ...string) []*chain.Holding {
	t.Helper()
	docs := []string{
		chaintest.Lines("v2.16", "v2.15"),
		chaintest.Doc("Project", "prod", "", "  repo: {name: acme/prod, owned: true}\n"),
		chaintest.Doc("Stream", "prod", "{project: prod}", streamSpec),
		chaintest.Doc("Artifact", "img", "{stream: prod}", "  kind: image\n  name: acme/prod\n"),
		chaintest.Doc("Artifact", "chart", "{stream: prod}", "  kind: file\n  name: prod-{version}.tgz\n"),
		chaintest.Doc("Project", "cons", "", "  repo: {name: acme/cons}\n  branches:\n    layout: perLine\n    stages: [dev, stable]\n    lines: [{line: v2.16, branch: {dev: dev-v2.16, stable: rel-v2.16}}, {line: v2.15, branch: {dev: dev-v2.15, stable: rel-v2.15}}]\n"),
	}
	for i, spec := range specs {
		docs = append(docs, chaintest.Doc("Holding", "h"+string(rune('0'+i)), "{project: cons, artifact: "+refOf(spec)+"}", spec[len(refOf(spec))+1:]))
	}
	return chaintest.Load(t, docs...).Holdings()
}

// refOf is the artifact a spec names before its first newline: "img\n  at: ...".
func refOf(spec string) string {
	for i := range spec {
		if spec[i] == '\n' {
			return spec[:i]
		}
	}
	return spec
}

const (
	imageHolding = "img\n  at: {file: build.yaml, key: defaultImage}\n  holds: {kind: release, form: \"{image}:{tag}\"}\n"
	chartHolding = "chart\n  at: {file: 'pkg/{line}/package.yaml', key: '.items[0].\"{branch}\".url'}\n  holds: {kind: release, form: \"{url}\", stage: dev}\n"
)

func TestReleaseOfAnImage(t *testing.T) {
	h := holdings(t, imageHolding)[0]

	rel, err := ReleaseOf(h, "v0.5.3", "abc123")

	require.NoError(t, err)
	assert.Equal(t, Release{Tag: "v0.5.3", BareTag: "v0.5.3", Version: "0.5.3", Image: "acme/prod", SourceRepo: "acme/prod", Commit: "abc123"}, rel)
}

func TestReleaseOfAFile(t *testing.T) {
	h := holdings(t, chartHolding)[0]

	rel, err := ReleaseOf(h, "v1.2.0", "abc123")

	require.NoError(t, err)
	assert.Equal(t, Release{
		Tag: "v1.2.0", BareTag: "v1.2.0", Version: "1.2.0", File: "prod-1.2.0.tgz", SourceRepo: "acme/prod", Commit: "abc123",
		URL: "https://github.com/acme/prod/releases/download/v1.2.0/prod-1.2.0.tgz",
	}, rel)
}

func TestReleaseOfATagWithAPrefix(t *testing.T) {
	h := holdingsOfStream(t, "  tagPattern: \"pkg-a/v*\"\n  universal: true\n", imageHolding, chartHolding)

	img, err := ReleaseOf(h[0], "pkg-a/v1.2.3", "abc123")
	require.NoError(t, err)
	assert.Equal(t, Release{
		Tag: "pkg-a/v1.2.3", Prefix: "pkg-a/", BareTag: "v1.2.3", Version: "1.2.3", Image: "acme/prod", SourceRepo: "acme/prod", Commit: "abc123",
	}, img)

	file, err := ReleaseOf(h[1], "pkg-a/v1.2.3", "abc123")
	require.NoError(t, err)
	assert.Equal(t, "prod-1.2.3.tgz", file.File, "a file is named by the version, which has no prefix")
	assert.Equal(t, "https://github.com/acme/prod/releases/download/pkg-a/v1.2.3/prod-1.2.3.tgz", file.URL)
}

func TestNewReleaseSplitsTheTagByThePatternOfItsStream(t *testing.T) {
	tests := []struct {
		name, tag, pattern string
		prefix, bare, ver  string
	}{
		{"no prefix", "v0.9.1-rc.2", "v*", "", "v0.9.1-rc.2", "0.9.1-rc.2"},
		{"a prefix", "helmtools/v0.1.2", "helmtools/v*", "helmtools/", "v0.1.2", "0.1.2"},
		{"a prefix with a minor in the pattern", "v3.8.0", "v3.8.*", "", "v3.8.0", "3.8.0"},
		{"a pattern with a deeper path", "a/b/v1.0.0", "a/b/v*", "a/b/", "v1.0.0", "1.0.0"},
		{"a tag the pattern does not match", "other/v1.0.0", "pkg/v*", "", "other/v1.0.0", "other/v1.0.0"},
		{"no pattern", "v1.0.0", "", "", "v1.0.0", "1.0.0"},
		{"a tag with no v", "1.0.0", "*", "", "1.0.0", "1.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel := newRelease(tt.tag, tt.pattern, "acme/prod", "abc")

			assert.Equal(t, Release{Tag: tt.tag, Prefix: tt.prefix, BareTag: tt.bare, Version: tt.ver, SourceRepo: "acme/prod", Commit: "abc"}, rel)
		})
	}
}

func TestPlaceOfIsTheFileAndKeyRenderedForALineAndABranch(t *testing.T) {
	h := holdings(t, chartHolding, imageHolding)

	place, err := PlaceOf(h[0], "v2.16", "dev-v2.16")

	require.NoError(t, err)
	assert.Equal(t, Place{File: "pkg/v2.16/package.yaml", Key: `.items[0]."dev-v2.16".url`, Format: "yaml"}, place)

	plain, err := PlaceOf(h[1], "v2.16", "main")
	require.NoError(t, err)
	assert.Equal(t, Place{File: "build.yaml", Key: "defaultImage", Format: "yaml"}, plain)
}

func TestPlaceOfRendersTheStageOfTheHolding(t *testing.T) {
	h := holdings(t, "img\n  at: {file: lock.yaml, key: '.refs.{stage|replace:stable=prod}.image'}\n  holds: {kind: release, stage: stable}\n")[0]

	place, err := PlaceOf(h, "v2.16", "rel-v2.16")

	require.NoError(t, err)
	assert.Equal(t, ".refs.prod.image", place.Key)
}

func TestFormOfIsTheFormRenderedForTheRelease(t *testing.T) {
	h := holdings(t, imageHolding, chartHolding,
		"img\n  at: {file: a.yaml, key: .a}\n  holds: {kind: release}\n",
		"img\n  at: {file: a.yaml, key: .a}\n  holds: {kind: release, form: \"{tag|trimprefix:v}\"}\n",
	)
	img, _ := ReleaseOf(h[0], "v0.5.3", "c")
	file, _ := ReleaseOf(h[1], "v1.2.0", "c")

	for i, want := range map[int]string{0: "acme/prod:v0.5.3", 2: "v0.5.3", 3: "0.5.3"} {
		got, err := FormOf(h[i], img)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
	got, err := FormOf(h[1], file)
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/acme/prod/releases/download/v1.2.0/prod-1.2.0.tgz", got)
}

func TestFormOfRendersThePrefixAndTheBareTag(t *testing.T) {
	h := holdingsOfStream(t, "  tagPattern: \"helmtools/v*\"\n  universal: true\n",
		"img\n  at: {file: a.yaml, key: .a}\n  holds: {kind: release, form: \"{bareTag}\"}\n",
		"img\n  at: {file: a.yaml, key: .a}\n  holds: {kind: release, form: \"{prefix}{bareTag}\"}\n",
		"img\n  at: {file: a.yaml, key: .a}\n  holds: {kind: release, form: \"{version}\"}\n",
		"img\n  at: {file: a.yaml, key: .a}\n  holds: {kind: release, form: \"{tag}\"}\n",
	)
	rel, err := ReleaseOf(h[0], "helmtools/v0.1.2", "c")
	require.NoError(t, err)

	for i, want := range []string{"v0.1.2", "helmtools/v0.1.2", "0.1.2", "helmtools/v0.1.2"} {
		got, err := FormOf(h[i], rel)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

type files map[string]string

func (f files) ReadFile(_ context.Context, path string) ([]byte, error) {
	data, ok := f[path]
	if !ok {
		return nil, assert.AnError
	}
	return []byte(data), nil
}

func (f files) WriteFile(_ context.Context, path string, data []byte) error {
	f[path] = string(data)
	return nil
}

const buildYAML = "# the build\ndefaultImage: acme/prod:v0.5.2   # keep this comment\nother: 1\n"

func TestCheckHoldingSaysWhatIsHeldAndWhatIsWanted(t *testing.T) {
	h := holdings(t, imageHolding)[0]
	rel, _ := ReleaseOf(h, "v0.5.3", "c")

	got, err := CheckHolding(bg, files{"build.yaml": buildYAML}, h, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.Equal(t, Check{
		Current: "acme/prod:v0.5.2", Want: "acme/prod:v0.5.3", UpToDate: false, Relation: RelationNewer,
		Change: Change{Path: "build.yaml", From: "acme/prod:v0.5.2", To: "acme/prod:v0.5.3",
			Description: "Updated `defaultImage` in `build.yaml` from `acme/prod:v0.5.2` to `acme/prod:v0.5.3`"},
	}, got)
}

func TestCheckHoldingRelations(t *testing.T) {
	h := holdings(t, imageHolding)[0]
	for want, tag := range map[Relation]string{RelationNewer: "v0.5.3", RelationSame: "v0.5.2", RelationOlder: "v0.5.1"} {
		rel, _ := ReleaseOf(h, tag, "c")

		got, err := CheckHolding(bg, files{"build.yaml": buildYAML}, h, "v2.16", "main", rel)

		require.NoError(t, err)
		assert.Equal(t, want, got.Relation, tag)
		assert.Equal(t, want == RelationSame, got.UpToDate, tag)
	}
	rel, _ := ReleaseOf(h, "latest", "c")
	got, err := CheckHolding(bg, files{"build.yaml": "defaultImage: latest\n"}, h, "v2.16", "main", rel)
	require.NoError(t, err)
	assert.Equal(t, RelationUnknown, got.Relation, "a value with no version is not judged")
}

func TestCheckHoldingErrors(t *testing.T) {
	h := holdings(t, imageHolding)[0]
	rel, _ := ReleaseOf(h, "v0.5.3", "c")

	_, err := CheckHolding(bg, files{}, h, "v2.16", "main", rel)
	require.ErrorContains(t, err, "read build.yaml")

	_, err = CheckHolding(bg, files{"build.yaml": "other: 1\n"}, h, "v2.16", "main", rel)
	require.ErrorContains(t, err, "defaultImage")
}

func TestApplyHoldingChangesOnlyTheValue(t *testing.T) {
	h := holdings(t, imageHolding)[0]
	rel, _ := ReleaseOf(h, "v0.5.3", "c")
	tree := files{"build.yaml": buildYAML}

	got, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.True(t, got.Changed)
	assert.Equal(t, "# the build\ndefaultImage: acme/prod:v0.5.3   # keep this comment\nother: 1\n", tree["build.yaml"])
	require.Len(t, got.Changes, 1)
	assert.Equal(t, "Updated `defaultImage` in `build.yaml` from `acme/prod:v0.5.2` to `acme/prod:v0.5.3`", got.Changes[0].Description)

	again, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)
	require.NoError(t, err)
	assert.False(t, again.Changed, "already held")
	assert.Empty(t, again.Changes)
}

func TestApplyHoldingOfAFormatThatIsNotBuilt(t *testing.T) {
	h := holdings(t, "img\n  at: {file: Dockerfile, key: IMAGE, format: dockerfile}\n  holds: {kind: release}\n")[0]
	rel, _ := ReleaseOf(h, "v0.5.3", "c")
	tree := files{"Dockerfile": "ARG IMAGE=x\n"}

	_, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)

	require.ErrorIs(t, err, ErrFormatNotBuilt)
	assert.ErrorContains(t, err, "dockerfile")
	_, err = CheckHolding(bg, tree, h, "v2.16", "main", rel)
	require.ErrorIs(t, err, ErrFormatNotBuilt)
}
