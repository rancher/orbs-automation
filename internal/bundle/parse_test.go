package bundle

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/chainload"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

func compiled(t *testing.T) []byte {
	t.Helper()
	data, err := Compile(fixture(), header)
	require.NoError(t, err)
	return data
}

func TestParseGivesBackTheHeaderThatWentIn(t *testing.T) {
	got, _, err := Parse(compiled(t))

	require.NoError(t, err)
	assert.Equal(t, Header{
		GeroVersion: header.GeroVersion, SourceCommit: header.SourceCommit, CompiledAt: header.CompiledAt, FormatVersion: 1,
	}, got)
}

func TestParseGivesTheConfigChunkAsOneFileTheLoaderReads(t *testing.T) {
	_, fsys, err := Parse(compiled(t))
	require.NoError(t, err)

	body, err := fs.ReadFile(fsys, Name)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(body), "---\n# source: a.yaml\n"), "the body starts where the header ends")
	assert.NotContains(t, string(body), "gero-bundle", "the header is not part of the config")
	g, rep := chainload.Load(fsys, config.Context{Now: header.CompiledAt})
	require.NoError(t, rep.Err())
	assert.Len(t, g.Projects(), 2)
}

func TestParseReadsWhatCompileWroteForTheRealConfig(t *testing.T) {
	t.Chdir("../..")
	shipped, err := Compile(os.DirFS("config"), header)
	require.NoError(t, err)

	_, fsys, err := Parse(shipped)

	require.NoError(t, err)
	g, rep := chainload.Load(fsys, config.Context{Now: header.CompiledAt})
	require.NoError(t, rep.Err())
	assert.NotEmpty(t, g.Projects())
}

func TestParseOfANewerAPIVersionIsUnsupportedAndNamesBothVersions(t *testing.T) {
	newer := strings.Replace(string(compiled(t)), "apiVersion: gero-bundle/v1", "apiVersion: gero-bundle/v2", 1)

	_, _, err := Parse([]byte(newer))

	require.ErrorIs(t, err, ErrUnsupported)
	assert.Contains(t, err.Error(), "gero-bundle/v2")
	assert.Contains(t, err.Error(), "gero-bundle/v1")
}

func TestParseOfANewerFormatVersionIsUnsupportedAndNamesBothVersions(t *testing.T) {
	newer := strings.Replace(string(compiled(t)), "formatVersion: 1", "formatVersion: 2", 1)

	_, _, err := Parse([]byte(newer))

	require.ErrorIs(t, err, ErrUnsupported)
	assert.Contains(t, err.Error(), "format version 2")
	assert.Contains(t, err.Error(), "reads 1")
}

func TestParseRefusesWhatIsNotABundle(t *testing.T) {
	good := string(compiled(t))
	for name, data := range map[string]string{
		"empty":                                 "",
		"a config file":                         project("alpha"),
		"no kind":                               strings.Replace(good, "kind: Bundle\n", "", 1),
		"another kind":                          strings.Replace(good, "kind: Bundle", "kind: Project", 1),
		"no format version":                     strings.Replace(good, "formatVersion: 1\n", "", 1),
		"a format version that is not a number": strings.Replace(good, "formatVersion: 1", "formatVersion: one", 1),
		"no version":                            strings.Replace(good, "geroVersion: v0.5.0\n", "", 1),
		"no commit":                             strings.Replace(good, "sourceCommit: abc123\n", "", 1),
		"a bad compile time":                    strings.Replace(good, "compiledAt: 2026-01-02T03:04:05Z", "compiledAt: yesterday", 1),
	} {
		_, _, err := Parse([]byte(data))

		require.ErrorIs(t, err, ErrMalformed, name)
		assert.NotErrorIs(t, err, ErrUnsupported, name)
	}
}

func TestParseIgnoresAHeaderFieldItDoesNotKnow(t *testing.T) {
	extra := strings.Replace(string(compiled(t)), "compiledAt:", "futureField: yes\ncompiledAt:", 1)

	got, _, err := Parse([]byte(extra))

	require.NoError(t, err)
	assert.Equal(t, "v0.5.0", got.GeroVersion)
}

func TestParseOfAHeaderAloneIsABundleWithNoConfig(t *testing.T) {
	_, fsys, err := Parse([]byte(wantHeader))

	require.NoError(t, err)
	body, err := fs.ReadFile(fsys, Name)
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(string(body)))
}
