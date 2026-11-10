package gomodedit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rancher is the shape of rancher/rancher's go.mod: a block, an indirect module
// of the same name with no major suffix, and a version with a prerelease.
const rancher = `module github.com/rancher/rancher

go 1.26.0

// the rancher family
require (
	github.com/rancher/lasso v0.2.10
	github.com/rancher/wrangler v1.1.2 // indirect
	github.com/rancher/wrangler/v3 v3.8.0-rc.3 // the one we bump
)

require github.com/stretchr/testify v1.11.1
`

func TestGetReadsTheVersionOfAModule(t *testing.T) {
	tests := []struct {
		name, src, module, want string
	}{
		{"a module in a block", rancher, "github.com/rancher/lasso", "v0.2.10"},
		{"a prerelease with a comment after it", rancher, "github.com/rancher/wrangler/v3", "v3.8.0-rc.3"},
		{"an indirect module", rancher, "github.com/rancher/wrangler", "v1.1.2"},
		{"a module on a require line of its own", rancher, "github.com/stretchr/testify", "v1.11.1"},
		{"a path that is a prefix of another is not it", rancher, "github.com/rancher/wrangler/v3", "v3.8.0-rc.3"},
		{"a quoted path", "module m\n\nrequire \"github.com/a/b\" v1.2.3\n", "github.com/a/b", "v1.2.3"},
		{"a pseudo-version", "module m\n\nrequire github.com/a/b v0.0.0-20260102030405-abcdef123456\n", "github.com/a/b", "v0.0.0-20260102030405-abcdef123456"},
		{"a file with windows line endings", "module m\r\n\r\nrequire (\r\n\tgithub.com/a/b v1.2.3\r\n)\r\n", "github.com/a/b", "v1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Get([]byte(tt.src), tt.module)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetOfAModuleThatIsNotRequired(t *testing.T) {
	tests := map[string]struct{ src, module string }{
		"no such module":              {rancher, "github.com/rancher/steve"},
		"a module with a longer path": {rancher, "github.com/rancher/wrangler/v4"},
		"a module only replaced":      {"module m\n\nreplace github.com/a/b => github.com/a/c v1.0.0\n", "github.com/a/b"},
		"a module only excluded":      {"module m\n\nexclude github.com/a/b v1.0.0\n", "github.com/a/b"},
		"no requires at all":          {"module m\n", "github.com/a/b"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Get([]byte(tt.src), tt.module)

			require.ErrorIs(t, err, ErrNotRequired)
			assert.ErrorContains(t, err, tt.module)
		})
	}
}

func TestGetOfAFileThatIsNotAGoMod(t *testing.T) {
	_, err := Get([]byte("module m\n\nrequire (\n\tgithub.com/a/b v1.2.3\n"), "github.com/a/b")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotRequired)
}

func TestSetReplacesOnlyTheVersion(t *testing.T) {
	out, old, err := Set([]byte(rancher), "github.com/rancher/wrangler/v3", "v3.8.0")

	require.NoError(t, err)
	assert.Equal(t, "v3.8.0-rc.3", old)
	want := strings.Replace(rancher, "v3.8.0-rc.3", "v3.8.0", 1)
	assert.Equal(t, want, string(out), "every other byte is as it was, comments included")
}

func TestSetKeepsWhatFollowsTheVersion(t *testing.T) {
	tests := []struct {
		name, src, module, version, want string
	}{
		{
			"indirect", "module m\n\nrequire (\n\tgithub.com/a/b v1.0.0 // indirect\n)\n", "github.com/a/b", "v1.1.0",
			"module m\n\nrequire (\n\tgithub.com/a/b v1.1.0 // indirect\n)\n",
		},
		{
			"a require line with a comment", "module m\n\nrequire github.com/a/b v1.0.0 // pinned for now\n", "github.com/a/b", "v1.1.0",
			"module m\n\nrequire github.com/a/b v1.1.0 // pinned for now\n",
		},
		{
			"a quoted path", "module m\n\nrequire \"github.com/a/b\" v1.0.0\n", "github.com/a/b", "v1.1.0",
			"module m\n\nrequire \"github.com/a/b\" v1.1.0\n",
		},
		{
			"windows line endings", "module m\r\n\r\nrequire (\r\n\tgithub.com/a/b v1.0.0\r\n)\r\n", "github.com/a/b", "v1.1.0",
			"module m\r\n\r\nrequire (\r\n\tgithub.com/a/b v1.1.0\r\n)\r\n",
		},
		{
			"a version that appears elsewhere on the line", "module m\n\nrequire github.com/a/b v1.0.0 // was v1.0.0 before\n", "github.com/a/b", "v1.1.0",
			"module m\n\nrequire github.com/a/b v1.1.0 // was v1.0.0 before\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _, err := Set([]byte(tt.src), tt.module, tt.version)

			require.NoError(t, err)
			assert.Equal(t, tt.want, string(out))
		})
	}
}

func TestSetLeavesAReplaceDirectiveAlone(t *testing.T) {
	src := "module m\n\nrequire github.com/rancher/wrangler/v3 v3.3.5 // indirect\n\nreplace github.com/rancher/wrangler/v3 => github.com/rancher/wrangler/v3 v3.2.2\n"

	out, old, err := Set([]byte(src), "github.com/rancher/wrangler/v3", "v3.3.6")

	require.NoError(t, err)
	assert.Equal(t, "v3.3.5", old)
	assert.Equal(t, "module m\n\nrequire github.com/rancher/wrangler/v3 v3.3.6 // indirect\n\nreplace github.com/rancher/wrangler/v3 => github.com/rancher/wrangler/v3 v3.2.2\n", string(out))
}

func TestSetOfTheVersionItAlreadyHasChangesNothing(t *testing.T) {
	out, old, err := Set([]byte(rancher), "github.com/rancher/lasso", "v0.2.10")

	require.NoError(t, err)
	assert.Equal(t, "v0.2.10", old)
	assert.Equal(t, rancher, string(out))
}

func TestSetChangesEveryRequireLineOfTheModule(t *testing.T) {
	src := "module m\n\nrequire github.com/a/b v1.0.0\n\nrequire (\n\tgithub.com/a/b v1.0.0 // indirect\n)\n"

	out, old, err := Set([]byte(src), "github.com/a/b", "v1.2.0")

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", old)
	assert.Equal(t, "module m\n\nrequire github.com/a/b v1.2.0\n\nrequire (\n\tgithub.com/a/b v1.2.0 // indirect\n)\n", string(out))
}

func TestSetOfAModuleThatIsNotRequired(t *testing.T) {
	_, _, err := Set([]byte(rancher), "github.com/rancher/steve", "v1.0.0")

	require.ErrorIs(t, err, ErrNotRequired)
	assert.ErrorContains(t, err, "github.com/rancher/steve")
}

func TestSetRefusesAVersionThatIsNotOne(t *testing.T) {
	for _, version := range []string{"", " ", "v1.0.0 // indirect", "v1.0.0\nrequire x v1"} {
		_, _, err := Set([]byte(rancher), "github.com/rancher/lasso", version)

		require.ErrorIs(t, err, ErrInvalidVersion, "%q", version)
	}
}

func TestSetThenGetAgree(t *testing.T) {
	out, _, err := Set([]byte(rancher), "github.com/rancher/wrangler/v3", "v3.9.1")
	require.NoError(t, err)

	got, err := Get(out, "github.com/rancher/wrangler/v3")

	require.NoError(t, err)
	assert.Equal(t, "v3.9.1", got)
	other, err := Get(out, "github.com/rancher/wrangler")
	require.NoError(t, err)
	assert.Equal(t, "v1.1.2", other, "the module with no suffix is another module")
}

func TestDirectivesReadTheGoAndToolchainLines(t *testing.T) {
	tests := []struct {
		name, src, goVersion, toolchain string
	}{
		{"both", "module m\n\ngo 1.26\n\ntoolchain go1.26.10\n", "1.26", "go1.26.10"},
		{"a go line with a patch", "module m\n\ngo 1.26.3\n", "1.26.3", ""},
		{"a go line with no patch", "module m\n\ngo 1.26\n", "1.26", ""},
		{"a prerelease", "module m\n\ngo 1.27rc1\n", "1.27rc1", ""},
		{"neither", "module m\n", "", ""},
		{"the lines among others", "module m\n\nrequire github.com/a/b v1.2.3\n\ngo 1.25.0 // the language\n\ntoolchain go1.26.2\n", "1.25.0", "go1.26.2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goVersion, toolchain, err := Directives([]byte(tt.src))

			require.NoError(t, err)
			assert.Equal(t, tt.goVersion, goVersion)
			assert.Equal(t, tt.toolchain, toolchain)
		})
	}
}

func TestDirectivesOfAFileThatDoesNotParse(t *testing.T) {
	_, _, err := Directives([]byte("module m\n\ngo\n"))

	require.Error(t, err)
}
