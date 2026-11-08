package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, tag string) Version {
	t.Helper()
	v, ok := Parse(tag)
	require.True(t, ok, tag)
	return v
}

func TestVersionString(t *testing.T) {
	for _, tag := range []string{"1.2.3", "v1.2.3", "1.2.4-rc.1", "0.9.0-alpha.beta.2"} {
		want := tag
		if want[0] == 'v' {
			want = want[1:]
		}
		assert.Equal(t, want, mustParse(t, tag).String(), tag)
	}
}

func TestNextStable(t *testing.T) {
	tests := []struct {
		from string
		bump Bump
		want string
	}{
		{"1.2.3", Patch, "1.2.4"},
		{"1.2.3", Minor, "1.3.0"},
		{"1.2.3", Major, "2.0.0"},
		{"1.9.9", Patch, "1.9.10"},
		{"0.0.0", Major, "1.0.0"},
		// After a prerelease it is the version being previewed, whatever the bump.
		{"1.2.4-rc.2", Patch, "1.2.4"},
		{"1.2.4-rc.2", Minor, "1.2.4"},
		{"1.2.4-rc.2", Major, "1.2.4"},
		{"2.0.0-alpha.1", Patch, "2.0.0"},
		{"1.2.4-rc.2+build.7", Patch, "1.2.4"},
	}
	for _, tt := range tests {
		t.Run(tt.from+" "+tt.want, func(t *testing.T) {
			got := mustParse(t, tt.from).NextStable(tt.bump)

			assert.Equal(t, tt.want, got.String())
			assert.False(t, got.IsPrerelease())
		})
	}
}

func TestNextPrerelease(t *testing.T) {
	tests := []struct {
		from  string
		bump  Bump
		label string
		want  string
	}{
		// After a stable version, the first prerelease of the next one.
		{"1.2.3", Patch, "rc", "1.2.4-rc.1"},
		{"1.2.3", Minor, "rc", "1.3.0-rc.1"},
		{"1.2.3", Major, "rc", "2.0.0-rc.1"},
		{"1.2.3", Patch, "beta", "1.2.4-beta.1"},
		// After a prerelease, its next iteration, whatever the bump.
		{"1.2.4-rc.1", Patch, "rc", "1.2.4-rc.2"},
		{"1.2.4-rc.1", Major, "rc", "1.2.4-rc.2"},
		{"1.2.4-rc.1", Minor, "", "1.2.4-rc.2"},
		{"1.2.4-rc.9", Patch, "rc", "1.2.4-rc.10"},
		{"1.2.4-rc.99", Patch, "", "1.2.4-rc.100"},
		// The style of the last identifier is kept.
		{"1.2.4-rc2", Patch, "rc", "1.2.4-rc3"},
		{"1.2.4-rc9", Patch, "", "1.2.4-rc10"},
		{"1.2.4-rc", Patch, "rc", "1.2.4-rc.1"},
		{"1.2.4-alpha.beta", Patch, "", "1.2.4-alpha.beta.1"},
		{"1.2.4-3", Patch, "", "1.2.4-4"},
		// A label that is not the current one starts over on the same version.
		{"1.2.4-beta.2", Patch, "rc", "1.2.4-rc.1"},
		{"1.2.4-rc2", Patch, "beta", "1.2.4-beta.1"},
		{"1.2.4-rc.1+build.7", Patch, "rc", "1.2.4-rc.2"},
	}
	for _, tt := range tests {
		t.Run(tt.from+" "+tt.label+" "+tt.want, func(t *testing.T) {
			got, err := mustParse(t, tt.from).NextPrerelease(tt.bump, tt.label)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.String())
			assert.True(t, got.IsPrerelease())
		})
	}
}

func TestNextPrereleaseRejectsALabelThatIsNotOneIdentifier(t *testing.T) {
	stable, pre := mustParse(t, "1.2.3"), mustParse(t, "1.2.4-rc.1")
	for _, label := range []string{"1", "rc.1", "r c", "rc!", "a.b"} {
		_, err := stable.NextPrerelease(Patch, label)
		require.ErrorIs(t, err, ErrInvalidLabel, "stable, %q", label)
		_, err = pre.NextPrerelease(Patch, label)
		require.ErrorIs(t, err, ErrInvalidLabel, "prerelease, %q", label)
	}
}

func TestNextPrereleaseAfterAStableVersionNeedsALabel(t *testing.T) {
	_, err := mustParse(t, "1.2.3").NextPrerelease(Patch, "")

	require.ErrorIs(t, err, ErrInvalidLabel, "there is no current label to keep")
}

// Whatever the next version is, it sorts after the one it follows, by the same
// ordering the tags are listed in, including the digits-in-the-identifier style.
func TestNextVersionsSortAfterWhatTheyFollow(t *testing.T) {
	for _, from := range []string{"1.2.3", "1.2.4-rc.1", "1.2.4-rc.9", "1.2.4-rc2", "1.2.4-rc9", "1.2.4-rc", "1.2.4-3", "0.0.1-alpha.beta"} {
		v := mustParse(t, from)
		stable := v.NextStable(Patch)
		assert.Equal(t, 1, Compare(stable, v), "stable after %s", from)
		pre, err := v.NextPrerelease(Patch, "rc")
		require.NoError(t, err, from)
		assert.Equal(t, 1, Compare(pre, v), "prerelease after %s: %s", from, pre)
		if v.IsPrerelease() {
			assert.Equal(t, 1, Compare(stable, pre), "the stable of a version follows its prerelease: %s", from)
		}
	}
}
