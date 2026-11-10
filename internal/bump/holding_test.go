package bump

import (
	"context"
	"strings"
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
		chaintest.Doc("Project", "prod", "", "  repo: {name: acme/prod, managedBy: orbs}\n"),
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

	places, err := PlacesOf(h[0], "v2.16", "dev-v2.16")

	require.NoError(t, err)
	assert.Equal(t, []Place{{File: "pkg/v2.16/package.yaml", Key: `.items[0]."dev-v2.16".url`, Format: "yaml"}}, places)

	plain, err := PlacesOf(h[1], "v2.16", "main")
	require.NoError(t, err)
	assert.Equal(t, []Place{{File: "build.yaml", Key: "defaultImage", Format: "yaml"}}, plain)
}

func TestPlaceOfRendersTheStageOfTheHolding(t *testing.T) {
	h := holdings(t, "img\n  at: {file: lock.yaml, key: '.refs.{stage|replace:stable=prod}.image'}\n  holds: {kind: release, stage: stable}\n")[0]

	places, err := PlacesOf(h, "v2.16", "rel-v2.16")

	require.NoError(t, err)
	require.Len(t, places, 1)
	assert.Equal(t, ".refs.prod.image", places[0].Key)
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
	change := Change{Path: "build.yaml", From: "acme/prod:v0.5.2", To: "acme/prod:v0.5.3",
		Description: "Updated `defaultImage` in `build.yaml` from `acme/prod:v0.5.2` to `acme/prod:v0.5.3`"}
	assert.Equal(t, Check{
		Current: "acme/prod:v0.5.2", Want: "acme/prod:v0.5.3", UpToDate: false, Relation: RelationNewer, Change: change,
		Locations: []LocationCheck{{
			Place: Place{File: "build.yaml", Key: "defaultImage", Format: "yaml"}, Current: "acme/prod:v0.5.2",
			UpToDate: false, Relation: RelationNewer, Change: change,
		}},
	}, got)
	assert.Equal(t, []Change{change}, got.Changes())
	assert.False(t, got.Differ())
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

// twoLocations is a holding of the image at two places, in two files.
const twoLocations = "img\n  at:\n    - {file: build.yaml, key: defaultImage}\n    - {file: pkg/build.yaml, key: .nested.image}\n  holds: {kind: release, form: \"{image}:{tag}\"}\n"

const nestedYAML = "nested:\n  image: acme/prod:v0.5.2  # keep this too\n"

func TestPlacesOfEveryLocationInTheOrderTheyAreListed(t *testing.T) {
	h := holdings(t, twoLocations)[0]

	places, err := PlacesOf(h, "v2.16", "main")

	require.NoError(t, err)
	assert.Equal(t, []Place{
		{File: "build.yaml", Key: "defaultImage", Format: "yaml"},
		{File: "pkg/build.yaml", Key: ".nested.image", Format: "yaml"},
	}, places)
}

func TestPlacesOfLeavesOutALocationThatDoesNotExistForTheLine(t *testing.T) {
	h := holdings(t, "img\n  at:\n    - {file: 'a/{line}.yaml', key: .a}\n    - {file: 'b/{branch}.yaml', key: .b, lines: {since: v2.16}}\n  holds: {kind: release}\n")[0]

	newer, err := PlacesOf(h, "v2.16", "main")
	require.NoError(t, err)
	assert.Equal(t, []Place{{File: "a/v2.16.yaml", Key: ".a", Format: "yaml"}, {File: "b/main.yaml", Key: ".b", Format: "yaml"}}, newer, "each is rendered with the line and the branch")

	older, err := PlacesOf(h, "v2.15", "rel-v2.15")
	require.NoError(t, err)
	assert.Equal(t, []Place{{File: "a/v2.15.yaml", Key: ".a", Format: "yaml"}}, older)
}

func TestCheckHoldingOfSeveralLocationsAgreeing(t *testing.T) {
	h := holdings(t, twoLocations)[0]
	rel, _ := ReleaseOf(h, "v0.5.2", "c")

	got, err := CheckHolding(bg, files{"build.yaml": buildYAML, "pkg/build.yaml": nestedYAML}, h, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.True(t, got.UpToDate)
	assert.False(t, got.Differ())
	assert.Len(t, got.Locations, 2)
	assert.Empty(t, got.Changes())
	assert.Equal(t, RelationSame, got.Relation)
}

func TestCheckHoldingOfSeveralLocationsThatDiffer(t *testing.T) {
	h := holdings(t, twoLocations)[0]
	tree := files{"build.yaml": buildYAML, "pkg/build.yaml": "nested:\n  image: acme/prod:v0.5.1\n"}

	t.Run("one is up to date", func(t *testing.T) {
		rel, _ := ReleaseOf(h, "v0.5.2", "c")

		got, err := CheckHolding(bg, tree, h, "v2.16", "main", rel)

		require.NoError(t, err)
		assert.True(t, got.Differ())
		assert.False(t, got.UpToDate, "it is not while one location is behind")
		assert.Equal(t, RelationNewer, got.Relation, "the relation of the location that is behind")
		require.Len(t, got.Locations, 2)
		assert.True(t, got.Locations[0].UpToDate)
		assert.False(t, got.Locations[1].UpToDate)
		assert.Equal(t, "acme/prod:v0.5.1", got.Locations[1].Current)
		require.Len(t, got.Changes(), 1)
		assert.Equal(t, "pkg/build.yaml", got.Changes()[0].Path)
		assert.Equal(t, got.Changes()[0], got.Change, "the change of a check is the first that a bump would make")
	})
	t.Run("both are behind", func(t *testing.T) {
		rel, _ := ReleaseOf(h, "v0.5.3", "c")

		got, err := CheckHolding(bg, tree, h, "v2.16", "main", rel)

		require.NoError(t, err)
		assert.True(t, got.Differ())
		assert.Len(t, got.Changes(), 2)
		assert.Equal(t, "acme/prod:v0.5.2", got.Current, "the value of the first location")
	})
}

func TestCheckHoldingOfSeveralLocationsHoldingTheSameValueThatIsNotWanted(t *testing.T) {
	h := holdings(t, twoLocations)[0]
	rel, _ := ReleaseOf(h, "v0.5.3", "c")

	got, err := CheckHolding(bg, files{"build.yaml": buildYAML, "pkg/build.yaml": nestedYAML}, h, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.False(t, got.Differ(), "they agree with each other, though not with the release")
	assert.False(t, got.UpToDate)
	assert.Len(t, got.Changes(), 2)
}

func TestApplyHoldingWritesEveryLocation(t *testing.T) {
	h := holdings(t, twoLocations)[0]
	rel, _ := ReleaseOf(h, "v0.5.3", "c")
	tree := files{"build.yaml": buildYAML, "pkg/build.yaml": nestedYAML}

	got, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.True(t, got.Changed)
	assert.Equal(t, "# the build\ndefaultImage: acme/prod:v0.5.3   # keep this comment\nother: 1\n", tree["build.yaml"])
	assert.Equal(t, "nested:\n  image: acme/prod:v0.5.3  # keep this too\n", tree["pkg/build.yaml"])
	require.Len(t, got.Changes, 2)
	assert.Equal(t, "build.yaml", got.Changes[0].Path)
	assert.Equal(t, "pkg/build.yaml", got.Changes[1].Path)

	again, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)
	require.NoError(t, err)
	assert.False(t, again.Changed)
	assert.Empty(t, again.Changes)
}

func TestApplyHoldingChangesOnlyTheLocationsThatAreBehind(t *testing.T) {
	h := holdings(t, twoLocations)[0]
	rel, _ := ReleaseOf(h, "v0.5.3", "c")
	tree := files{"build.yaml": "defaultImage: acme/prod:v0.5.3\n", "pkg/build.yaml": nestedYAML}

	got, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)

	require.NoError(t, err)
	require.Len(t, got.Changes, 1)
	assert.Equal(t, "pkg/build.yaml", got.Changes[0].Path)
}

func TestApplyHoldingTwoLocationsInOneFile(t *testing.T) {
	h := holdings(t, "img\n  at:\n    - {file: build.yaml, key: defaultImage}\n    - {file: build.yaml, key: other.image}\n  holds: {kind: release, form: \"{image}:{tag}\"}\n")[0]
	rel, _ := ReleaseOf(h, "v0.5.3", "c")
	tree := files{"build.yaml": "defaultImage: acme/prod:v0.5.2\nother:\n  image: acme/prod:v0.5.2\n"}

	got, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.Equal(t, "defaultImage: acme/prod:v0.5.3\nother:\n  image: acme/prod:v0.5.3\n", tree["build.yaml"])
	assert.Len(t, got.Changes, 2)
}

func TestApplyHoldingOnlyWritesTheLocationsThatExistForTheLine(t *testing.T) {
	h := holdings(t, "img\n  at:\n    - {file: a.yaml, key: .v}\n    - {file: b.yaml, key: .v, lines: {since: v2.16}}\n  holds: {kind: release, form: \"{tag}\"}\n")[0]
	rel, _ := ReleaseOf(h, "v0.5.3", "c")
	tree := files{"a.yaml": "v: v0.5.2\n", "b.yaml": "v: v0.5.2\n"}

	_, err := ApplyHolding(bg, tree, h, "v2.15", "rel-v2.15", rel)

	require.NoError(t, err)
	assert.Equal(t, "v: v0.5.3\n", tree["a.yaml"])
	assert.Equal(t, "v: v0.5.2\n", tree["b.yaml"], "b.yaml does not exist for v2.15")
}

func TestAHoldingWithNoLocationForTheLineIsAnErrorNotANoOp(t *testing.T) {
	h := holdings(t, "img\n  at: {file: a.yaml, key: .v, lines: {since: v2.16}}\n  holds: {kind: release, form: \"{tag}\"}\n")[0]
	rel, _ := ReleaseOf(h, "v0.5.3", "c")
	tree := files{"a.yaml": "v: v0.5.2\n"}

	_, err := ApplyHolding(bg, tree, h, "v2.15", "rel-v2.15", rel)
	require.ErrorContains(t, err, "no location")
	assert.ErrorContains(t, err, "v2.15")

	_, err = CheckHolding(bg, tree, h, "v2.15", "rel-v2.15", rel)
	assert.ErrorContains(t, err, "no location")
}

func TestAHoldingWithALocationInAFormatNotBuiltWritesNothing(t *testing.T) {
	h := holdings(t, "img\n  at:\n    - {file: build.yaml, key: defaultImage}\n    - {file: Dockerfile, key: IMAGE, format: dockerfile}\n  holds: {kind: release, form: \"{image}:{tag}\"}\n")[0]
	rel, _ := ReleaseOf(h, "v0.5.3", "c")
	tree := files{"build.yaml": buildYAML, "Dockerfile": "ARG IMAGE=x\n"}

	_, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)

	require.ErrorIs(t, err, ErrFormatNotBuilt)
	assert.Equal(t, buildYAML, tree["build.yaml"], "the first location is not written when the second cannot be")
}

// libMod is a go.mod that requires the library at two versions of a family.
const libMod = "module acme.test/svc\n\ngo 1.26\n\nrequire (\n\tgithub.com/acme/lib v1.0.0 // indirect\n\tgithub.com/acme/other v0.3.0\n)\n"

const goModHolding = "img\n  at: {file: go.mod, key: github.com/acme/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\"}\n"

func TestCheckHoldingOfAGoModule(t *testing.T) {
	h := holdings(t, goModHolding)[0]
	rel, _ := ReleaseOf(h, "v1.2.3", "c")

	got, err := CheckHolding(bg, files{"go.mod": libMod}, h, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", got.Current)
	assert.Equal(t, "v1.2.3", got.Want)
	assert.False(t, got.UpToDate)
	assert.Equal(t, RelationNewer, got.Relation)
	assert.Equal(t, "Updated `github.com/acme/lib` in `go.mod` from `v1.0.0` to `v1.2.3`", got.Change.Description)
}

func TestApplyHoldingChangesOnlyTheVersionOfAGoModule(t *testing.T) {
	h := holdings(t, goModHolding)[0]
	rel, _ := ReleaseOf(h, "v1.2.3", "c")
	tree := files{"go.mod": libMod}

	got, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.True(t, got.Changed)
	assert.Equal(t, strings.Replace(libMod, "v1.0.0", "v1.2.3", 1), tree["go.mod"], "the indirect comment and the other module stay")
	require.Len(t, got.Changes, 1)

	again, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)
	require.NoError(t, err)
	assert.False(t, again.Changed)
}

func TestApplyHoldingOfAGoModuleInEachOfSeveralGoMods(t *testing.T) {
	h := holdings(t, "img\n  at:\n    - {file: go.mod, key: github.com/acme/lib, format: gomod}\n    - {file: pkg/apis/go.mod, key: github.com/acme/lib, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\"}\n")[0]
	rel, _ := ReleaseOf(h, "v1.2.3", "c")
	tree := files{"go.mod": libMod, "pkg/apis/go.mod": "module acme.test/svc/pkg/apis\n\nrequire github.com/acme/lib v1.1.0\n"}

	got, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)

	require.NoError(t, err)
	require.Len(t, got.Changes, 2)
	assert.Contains(t, tree["go.mod"], "github.com/acme/lib v1.2.3 // indirect")
	assert.Contains(t, tree["pkg/apis/go.mod"], "github.com/acme/lib v1.2.3")
}

func TestAGoModuleThatIsNotRequiredIsAnErrorNamingIt(t *testing.T) {
	h := holdings(t, "img\n  at: {file: go.mod, key: github.com/acme/missing, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\"}\n")[0]
	rel, _ := ReleaseOf(h, "v1.2.3", "c")

	_, err := CheckHolding(bg, files{"go.mod": libMod}, h, "v2.16", "main", rel)
	require.ErrorContains(t, err, "github.com/acme/missing")
	assert.ErrorContains(t, err, "go.mod")

	tree := files{"go.mod": libMod}
	_, err = ApplyHolding(bg, tree, h, "v2.16", "main", rel)
	require.ErrorContains(t, err, "github.com/acme/missing")
	assert.Equal(t, libMod, tree["go.mod"])
}

// braValues is BRO's chart values in shape: a placeholder that is not YAML, and the
// kuberlr tag under global.kubectl.
const braValues = "image:\n  tag: %TAG%\n\nglobal:\n  kubectl:\n    repository: rancher/kuberlr-kubectl\n    tag: v8.2.0  # read by gero\n"

const regexHolding = "img\n  at: {file: charts/values.yaml, key: 'kubectl:\\n(?:[ \\t]+.*\\n)*?[ \\t]+tag: (\\S+)', format: regex}\n  holds: {kind: release, form: \"{tag}\"}\n"

func TestAHoldingInTheRegexFormatIsCheckedAndWritten(t *testing.T) {
	h := holdings(t, regexHolding)[0]
	rel, _ := ReleaseOf(h, "v8.3.0", "c")
	tree := files{"charts/values.yaml": braValues}

	check, err := CheckHolding(bg, tree, h, "v2.16", "main", rel)
	require.NoError(t, err)
	assert.Equal(t, "v8.2.0", check.Current)
	assert.Equal(t, "v8.3.0", check.Want)
	assert.Equal(t, RelationNewer, check.Relation)

	got, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)
	require.NoError(t, err)
	assert.True(t, got.Changed)
	assert.Equal(t, strings.Replace(braValues, "v8.2.0", "v8.3.0", 1), tree["charts/values.yaml"], "the placeholder and the comment are as they were")

	again, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)
	require.NoError(t, err)
	assert.False(t, again.Changed)
}

func TestAHoldingInTheRegexFormatRendersItsKeyForTheLine(t *testing.T) {
	h := holdings(t, "img\n  at: {file: Makefile, key: '^TOOL_{line|trimprefix:v}_VERSION = (.*)$', format: regex}\n  holds: {kind: release, form: \"{tag}\"}\n")[0]
	rel, _ := ReleaseOf(h, "v0.5.3", "c")
	tree := files{"Makefile": "TOOL_2.15_VERSION = v0.5.2\nTOOL_2.16_VERSION = v0.5.2\n"}

	_, err := ApplyHolding(bg, tree, h, "v2.16", "main", rel)

	require.NoError(t, err)
	assert.Equal(t, "TOOL_2.15_VERSION = v0.5.2\nTOOL_2.16_VERSION = v0.5.3\n", tree["Makefile"])
}

func TestAHoldingInTheRegexFormatThatMatchesNothingIsAnErrorNamingThePattern(t *testing.T) {
	h := holdings(t, regexHolding)[0]
	rel, _ := ReleaseOf(h, "v8.3.0", "c")

	_, err := CheckHolding(bg, files{"charts/values.yaml": "other: 1\n"}, h, "v2.16", "main", rel)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "charts/values.yaml")
	assert.Contains(t, err.Error(), "matches nowhere")
}
