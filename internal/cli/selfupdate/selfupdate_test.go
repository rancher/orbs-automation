package selfupdate

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	platform "github.com/rancher/orbs-automation/internal/gero/selfupdate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUpdater struct {
	status   platform.Status
	checkErr error
	blocked  string
	applyErr error
	applied  []platform.Status
}

func (f *fakeUpdater) Check(context.Context) (platform.Status, error) { return f.status, f.checkErr }
func (f *fakeUpdater) Blocked() string                                { return f.blocked }
func (f *fakeUpdater) Apply(_ context.Context, s platform.Status) (string, error) {
	f.applied = append(f.applied, s)
	return s.Latest.Version, f.applyErr
}

func newer() *fakeUpdater {
	return &fakeUpdater{status: platform.Status{
		Current: "1.0.0", Latest: platform.Release{Version: "v1.2.3"}, Available: true,
	}}
}

// run runs with the updater, tweaks the options and answers what is asked with
// answer, and returns what was printed and the error.
func run(t *testing.T, u *fakeUpdater, answer string, tweak func(*Options)) (out string, asked string, err error) {
	t.Helper()
	var prompt bytes.Buffer
	o := Options{Updater: u, In: strings.NewReader(answer), Out: &prompt, Interactive: true}
	if tweak != nil {
		tweak(&o)
	}
	out, err = Run(context.Background(), o)
	return out, prompt.String(), err
}

func TestUpToDate(t *testing.T) {
	u := newer()
	u.status = platform.Status{Current: "1.2.3", Latest: platform.Release{Version: "v1.2.3"}}

	out, asked, err := run(t, u, "", nil)

	require.NoError(t, err)
	assert.Contains(t, out, "gero 1.2.3 is up to date")
	assert.Contains(t, out, "v1.2.3")
	assert.Empty(t, asked)
	assert.Empty(t, u.applied)
}

func TestADevelopmentBuildIsLeftAlone(t *testing.T) {
	u := newer()
	u.status = platform.Status{Current: "dev", Latest: platform.Release{Version: "v1.2.3"}}

	out, _, err := run(t, u, "", nil)

	require.NoError(t, err)
	assert.Contains(t, out, "development build")
	assert.Contains(t, out, "v1.2.3")
	assert.Empty(t, u.applied)
}

func TestCheckOnlyAndDryRunSayWhatIsAvailableAndChangeNothing(t *testing.T) {
	for name, tweak := range map[string]func(*Options){
		"check": func(o *Options) { o.CheckOnly = true },
		"dry":   func(o *Options) { o.DryRun = true },
	} {
		t.Run(name, func(t *testing.T) {
			u := newer()

			out, asked, err := run(t, u, "y\n", tweak)

			require.NoError(t, err)
			assert.Contains(t, out, "v1.2.3 is available")
			assert.Contains(t, out, "1.0.0")
			assert.Empty(t, asked)
			assert.Empty(t, u.applied)
		})
	}
	t.Run("dry run names what it would do", func(t *testing.T) {
		out, _, err := run(t, newer(), "", func(o *Options) { o.DryRun = true })

		require.NoError(t, err)
		assert.Contains(t, out, "Would update gero 1.0.0 to v1.2.3")
	})
}

func TestABinaryThatCannotBeReplacedIsNotOfferedAnUpdate(t *testing.T) {
	u := newer()
	u.blocked = "installed by Homebrew: run brew upgrade gero"

	out, asked, err := run(t, u, "y\n", nil)

	require.NoError(t, err)
	assert.Contains(t, out, "v1.2.3 is available")
	assert.Contains(t, out, "installed by Homebrew: run brew upgrade gero")
	assert.Empty(t, asked, "no question that cannot be acted on")
	assert.Empty(t, u.applied)
}

func TestApproval(t *testing.T) {
	t.Run("--yes installs without asking", func(t *testing.T) {
		u := newer()

		out, asked, err := run(t, u, "", func(o *Options) { o.Yes = true; o.Interactive = false })

		require.NoError(t, err)
		assert.Empty(t, asked)
		assert.Len(t, u.applied, 1)
		assert.Contains(t, out, "Updated gero to v1.2.3")
	})
	for _, answer := range []string{"y\n", "Y\n", "yes\n"} {
		t.Run("answer "+strings.TrimSpace(answer), func(t *testing.T) {
			u := newer()

			out, asked, err := run(t, u, answer, nil)

			require.NoError(t, err)
			assert.Contains(t, asked, "Update gero 1.0.0 to v1.2.3? [y/N]")
			assert.Len(t, u.applied, 1)
			assert.Contains(t, out, "Updated gero to v1.2.3")
		})
	}
	for _, answer := range []string{"n\n", "\n", "", "later\n"} {
		t.Run("declined by "+strings.TrimSpace(answer), func(t *testing.T) {
			u := newer()

			_, _, err := run(t, u, answer, nil)

			assert.ErrorIs(t, err, ErrDeclined)
			assert.Empty(t, u.applied)
		})
	}
	t.Run("without a terminal or --yes it refuses before asking", func(t *testing.T) {
		u := newer()

		_, asked, err := run(t, u, "y\n", func(o *Options) { o.Interactive = false })

		assert.ErrorIs(t, err, ErrNeedsApproval)
		assert.Empty(t, asked)
		assert.Empty(t, u.applied)
	})
}

func TestAPrereleaseIsSaidToBeOne(t *testing.T) {
	u := newer()
	u.status.Latest.Prerelease = true

	out, asked, err := run(t, u, "n\n", nil)

	assert.ErrorIs(t, err, ErrDeclined)
	assert.Contains(t, out+asked, "pre-release")
}

func TestErrorsAreReturned(t *testing.T) {
	boom := errors.New("boom")
	t.Run("check", func(t *testing.T) {
		u := newer()
		u.checkErr = boom

		_, _, err := run(t, u, "", nil)

		assert.ErrorIs(t, err, boom)
		assert.Empty(t, u.applied)
	})
	t.Run("apply, with nothing said to have been installed", func(t *testing.T) {
		u := newer()
		u.applyErr = boom

		out, _, err := run(t, u, "y\n", nil)

		assert.ErrorIs(t, err, boom)
		assert.NotContains(t, out, "Updated")
	})
}
