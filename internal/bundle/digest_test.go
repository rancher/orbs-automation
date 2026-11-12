package bundle

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// digestOfFixture is the digest of fixture(), worked out apart from this code: the
// SHA-256 of "a.yaml", a NUL, the SHA-256 of that file's bytes in hex and a newline,
// then the same for b.yaml.
const digestOfFixture = "0acbeadfc35fa371907c479d96a3c603a69f38cf4789b923a0ee2980287c8ed2"

func digestOf(t *testing.T, fsys fstest.MapFS) string {
	t.Helper()
	d, err := Digest(fsys)
	require.NoError(t, err)
	return d
}

func TestDigestIsTheHashOfTheNamesAndTheBytesOfTheYAMLFiles(t *testing.T) {
	assert.Equal(t, digestOfFixture, digestOf(t, fixture()))
}

func TestDigestDoesNotDependOnTheOrderFilesAreListed(t *testing.T) {
	forward := fstest.MapFS{}
	forward["a.yaml"] = &fstest.MapFile{Data: []byte("a: 1\n")}
	forward["b.yaml"] = &fstest.MapFile{Data: []byte("b: 2\n")}
	backward := fstest.MapFS{}
	backward["b.yaml"] = forward["b.yaml"]
	backward["a.yaml"] = forward["a.yaml"]

	assert.Equal(t, digestOf(t, forward), digestOf(t, backward))
}

func TestDigestChangesWithOneByteOrAName(t *testing.T) {
	base := digestOf(t, fixture())

	edited := fixture()
	edited["a.yaml"] = &fstest.MapFile{Data: append(append([]byte{}, edited["a.yaml"].Data...), '#')}
	assert.NotEqual(t, base, digestOf(t, edited), "one byte")

	renamed := fixture()
	renamed["c.yaml"] = renamed["b.yaml"]
	delete(renamed, "b.yaml")
	assert.NotEqual(t, base, digestOf(t, renamed), "a name")

	swapped := fixture()
	swapped["a.yaml"], swapped["b.yaml"] = swapped["b.yaml"], swapped["a.yaml"]
	assert.NotEqual(t, base, digestOf(t, swapped), "which file holds which bytes")
}

func TestDigestIgnoresWhatCompileDoesNotBundle(t *testing.T) {
	with := fixture()
	with["notes.txt"] = &fstest.MapFile{Data: []byte("changed\n")}
	with["sub/README.md"] = &fstest.MapFile{Data: []byte("# also not config\n")}

	assert.Equal(t, digestOfFixture, digestOf(t, with))
}

func TestTheIDIsTheHashOfTheRevisionAndTheDigestAlone(t *testing.T) {
	h := Header{GeroVersion: "v0.5.0", SourceCommit: "abc123", InputsDigest: "aa", CompilerRevision: 1}
	id := h.ID()

	assert.NotEmpty(t, id)
	assert.Equal(t, id, h.ID(), "the same every time")
	other := h
	other.GeroVersion, other.SourceCommit = "v9.9.9", "fedcba"
	assert.Equal(t, id, other.ID(), "the version and commit are provenance, not identity")
	other = h
	other.InputsDigest = "bb"
	assert.NotEqual(t, id, other.ID(), "other files")
	other = h
	other.CompilerRevision = 2
	assert.NotEqual(t, id, other.ID(), "another revision of Compile")
	assert.Empty(t, Header{GeroVersion: "v0.5.0", SourceCommit: "abc123"}.ID(), "no digest, no ID")
}
