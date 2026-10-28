package checksemver

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/semver"
	"github.com/stretchr/testify/assert"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestResolveTag(t *testing.T) {
	ref := map[string]string{"GITHUB_REF_NAME": "v1.2.3-rc"}
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want string
		err  error
	}{
		{"argument", []string{"v1.0.0"}, nil, "v1.0.0", nil},
		{"env only", nil, ref, "v1.2.3-rc", nil},
		{"argument beats env", []string{"1.2.3"}, ref, "1.2.3", nil},
		{"empty argument falls to env", []string{""}, ref, "v1.2.3-rc", nil},
		{"extra arguments ignored", []string{"1.2.3", "other"}, nil, "1.2.3", nil},
		{"neither", nil, nil, "", ErrNoTag},
		{"empty argument, no env", []string{""}, nil, "", ErrNoTag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTag(tt.args, env(tt.env))

			assert.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClassify(t *testing.T) {
	got := Classify("v1.2.3-rc.1+b")

	assert.Equal(t, "1.2.3-rc.1+b", got.Version)
	assert.True(t, got.HasPrerelease)
	assert.True(t, got.HasBuildMeta)
}

func TestRender(t *testing.T) {
	tests := []struct {
		name string
		info semver.Info
		want string
	}{
		{"neither", semver.Info{}, "HAS_PRERELEASE=false\nHAS_BUILD_META=false\n"},
		{"prerelease", semver.Info{HasPrerelease: true}, "HAS_PRERELEASE=true\nHAS_BUILD_META=false\n"},
		{"build meta", semver.Info{HasBuildMeta: true}, "HAS_PRERELEASE=false\nHAS_BUILD_META=true\n"},
		{"both", semver.Info{HasPrerelease: true, HasBuildMeta: true}, "HAS_PRERELEASE=true\nHAS_BUILD_META=true\n"},
		{
			// The default format must not leak the extra fields.
			"extra fields ignored",
			semver.Info{Input: "v1.2.3", Version: "1.2.3", IsSemver: true, Major: 1, Minor: 2, Patch: 3},
			"HAS_PRERELEASE=false\nHAS_BUILD_META=false\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Render(tt.info))
		})
	}
}
