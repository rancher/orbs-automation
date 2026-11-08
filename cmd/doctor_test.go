package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/cli/doctor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetDoctorFlags clears the local flags of gero doctor, which resetFlags
// (root's persistent flags only) does not know about.
func resetDoctorFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		cmd, _, err := rootCmd.Find([]string{"doctor"})
		require.NoError(t, err)
		for _, name := range []string{"online", "timeout", "strict", "list-checks"} {
			f := cmd.Flags().Lookup(name)
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
	})
}

// stubGh puts a fake gh first on PATH, so the gh check does not depend on the
// machine the tests run on. It reports a version and has no login. It installs
// stubGit too, because a full run checks both.
func stubGh(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	// "gh auth token" fails, as it does for a gh nobody has logged in with.
	script := "#!/bin/sh\n[ \"$1\" = auth ] && exit 1\necho 'gh version 9.9.9 (2030-01-01)'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755))
	t.Setenv("GH_PATH", "")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	stubGit(t)
}

// stubGit puts a fake git first on PATH, with a version and an identity, so the
// git check does not depend on the machine the tests run on.
func stubGit(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\ncase \"$1\" in\n--version) echo 'git version 9.9.9';;\n" +
		"config) case \"$3\" in user.name) echo 'Test User';; user.email) echo 'test@example.com';; esac;;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// withToken gives the auth-token check a token from the environment, so a run
// does not depend on the token state of the machine running the tests. go-gh
// reads gh's config once per process, so that points at an empty directory
// before the first read.
func withToken(t *testing.T) {
	t.Helper()
	for _, k := range []string{"GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GH_HOST"} {
		t.Setenv(k, "")
	}
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_TOKEN", "ghp_not_a_real_token")
}

// Wiring only: the logic is tested in internal/cli/doctor.
func TestDoctorWiring(t *testing.T) {
	withConfigDir(t, nil)
	t.Run("runs the checks offline and prints the report", func(t *testing.T) {
		resetDoctorFlags(t)
		stubGh(t)
		withToken(t)

		stdout, _, err := execRoot(t, "doctor")

		require.NoError(t, err)
		assert.Contains(t, stdout, "Environment\n  ✔ version: gero dev on ")
		assert.Contains(t, stdout, "\n  ✔ gh: gh version 9.9.9 (2030-01-01) at ")
		assert.Contains(t, stdout, "Authentication\n  ✔ auth-token: environment variable GH_TOKEN for github.com\n")
		assert.NotContains(t, stdout, "ghp_not_a_real_token")
		assert.Contains(t, stdout, "Summary: ")
		assert.Contains(t, stdout, "Your workstation is ready to use gero.")
	})

	t.Run("a named check runs alone", func(t *testing.T) {
		resetDoctorFlags(t)

		stdout, _, err := execRoot(t, "doctor", "version")

		require.NoError(t, err)
		assert.Contains(t, stdout, "✔ version")
		assert.NotContains(t, stdout, "gh:")
	})

	t.Run("an unknown check is an error that points at the list", func(t *testing.T) {
		resetDoctorFlags(t)

		stdout, _, err := execRoot(t, "doctor", "nope")

		require.Error(t, err)
		assert.ErrorContains(t, err, `unknown check "nope"`)
		assert.ErrorContains(t, err, "--list-checks")
		assert.Empty(t, stdout)
	})

	t.Run("--list-checks lists them without running", func(t *testing.T) {
		resetDoctorFlags(t)

		stdout, _, err := execRoot(t, "doctor", "--list-checks")

		require.NoError(t, err)
		assert.Contains(t, stdout, "NAME")
		assert.Contains(t, stdout, "version")
		assert.Contains(t, stdout, "gh")
		assert.NotContains(t, stdout, "Summary:")
	})

	t.Run("--json prints the report as json", func(t *testing.T) {
		resetDoctorFlags(t)
		stubGh(t)
		withToken(t)

		stdout, _, err := execRoot(t, "doctor", "--json")

		require.NoError(t, err)
		assert.Contains(t, stdout, `"checks": [`)
		assert.Contains(t, stdout, `"name": "version"`)
	})

	t.Run("a missing gh is a warning, not a failure", func(t *testing.T) {
		resetDoctorFlags(t)
		withToken(t)
		t.Setenv("GH_PATH", "")
		t.Setenv("PATH", t.TempDir())
		stubGit(t)

		stdout, _, err := execRoot(t, "doctor")

		require.NoError(t, err, "a warning does not fail the run")
		assert.Contains(t, stdout, "⚠ gh: gh not found on PATH\n")
		assert.Contains(t, stdout, "Ready, with warnings.")
	})

	t.Run("--strict fails the run on that warning", func(t *testing.T) {
		resetDoctorFlags(t)
		withToken(t)
		t.Setenv("GH_PATH", "")
		t.Setenv("PATH", t.TempDir())
		stubGit(t)

		stdout, _, err := execRoot(t, "doctor", "--strict")

		require.ErrorIs(t, err, doctor.ErrChecksFailed)
		assert.Contains(t, stdout, "⚠ gh")
	})

	t.Run("a missing token fails the run and says how to fix it", func(t *testing.T) {
		resetDoctorFlags(t)
		noGitHubAuth(t)
		stubGh(t)

		stdout, _, err := execRoot(t, "doctor")

		require.ErrorIs(t, err, doctor.ErrChecksFailed)
		assert.Contains(t, stdout, "✗ auth-token: no token found for github.com\n")
		assert.Contains(t, stdout, `→ set GH_TOKEN or GITHUB_TOKEN, or run "gh auth login"`)
		assert.Contains(t, stdout, "Some checks failed")
	})

	t.Run("offline, the online checks are skipped and counted", func(t *testing.T) {
		resetDoctorFlags(t)
		stubGh(t)
		withToken(t)

		stdout, _, err := execRoot(t, "doctor")

		require.NoError(t, err)
		assert.Contains(t, stdout, "○ auth-api: skipped, run with --online\n")
		assert.Contains(t, stdout, "○ rate-limit: skipped, run with --online\n")
		assert.Contains(t, stdout, "2 checks need --online. ")
	})

	t.Run("--online without a token skips auth-api instead of calling GitHub", func(t *testing.T) {
		resetDoctorFlags(t)
		noGitHubAuth(t)
		stubGh(t)

		stdout, _, err := execRoot(t, "doctor", "--online")

		require.ErrorIs(t, err, doctor.ErrChecksFailed)
		assert.Contains(t, stdout, "✗ auth-token: no token found for github.com\n")
		assert.Contains(t, stdout, "○ auth-api: skipped, needs auth-token\n")
		assert.Contains(t, stdout, "○ rate-limit: skipped, needs auth-api\n", "a skipped prerequisite skips what needs it")
		assert.NotContains(t, stdout, "--online. ", "nothing was left waiting on --online")
	})

	t.Run("naming rate-limit pulls in the whole chain it needs", func(t *testing.T) {
		resetDoctorFlags(t)
		noGitHubAuth(t)
		stubGh(t)

		stdout, _, err := execRoot(t, "doctor", "rate-limit")

		require.ErrorIs(t, err, doctor.ErrChecksFailed)
		assert.Contains(t, stdout, "✗ auth-token: no token found for github.com\n")
		assert.Contains(t, stdout, "○ auth-api: skipped, needs auth-token\n")
		assert.Contains(t, stdout, "○ rate-limit: skipped, needs auth-api\n")
		assert.NotContains(t, stdout, "gh:", "only the named check and what it needs")
	})

	t.Run("naming auth-api runs it online and pulls in auth-token", func(t *testing.T) {
		resetDoctorFlags(t)
		noGitHubAuth(t)
		stubGh(t)

		stdout, _, err := execRoot(t, "doctor", "auth-api")

		require.ErrorIs(t, err, doctor.ErrChecksFailed)
		assert.Contains(t, stdout, "✗ auth-token: no token found for github.com\n")
		assert.Contains(t, stdout, "○ auth-api: skipped, needs auth-token\n")
		assert.NotContains(t, stdout, "gh:", "only the named check and what it needs")
	})

	t.Run("a missing git fails the run and says why", func(t *testing.T) {
		resetDoctorFlags(t)
		noGitHubAuth(t) // no git on PATH, no gh, no token

		stdout, _, err := execRoot(t, "doctor", "git")

		require.ErrorIs(t, err, doctor.ErrChecksFailed)
		assert.Contains(t, stdout, "✗ git: git not found on PATH\n")
		assert.Contains(t, stdout, "→ gero runs git for a few operations")
	})

	t.Run("no config directory fails config-dir and skips what needs it", func(t *testing.T) {
		resetDoctorFlags(t)
		stubGh(t)
		withToken(t)
		t.Setenv("GERO_CONFIG_DIR", "") // the working directory, cmd/, holds no config either

		stdout, _, err := execRoot(t, "doctor")

		require.ErrorIs(t, err, doctor.ErrChecksFailed)
		assert.Contains(t, stdout, "✗ config-dir: no central config directory found\n")
		assert.Contains(t, stdout, "→ set one of --config-dir, GERO_CONFIG_DIR or configDir in the user file")
		assert.Contains(t, stdout, "○ config: skipped, needs config-dir\n")
		assert.Contains(t, stdout, "○ repos: skipped, needs config\n")
	})

	t.Run("an invalid config fails config and skips repos", func(t *testing.T) {
		resetDoctorFlags(t)
		stubGh(t)
		withToken(t)
		bad := "apiVersion: gero/v1alpha2\nkind: Project\nname: %s\nspec:\n  repo: {name: not-a-slug}\n"
		withConfigDir(t, map[string]string{
			"a.yaml": fmt.Sprintf(bad, "a"),
			"b.yaml": fmt.Sprintf(bad, "b"),
		})

		stdout, _, err := execRoot(t, "doctor")

		require.ErrorIs(t, err, doctor.ErrChecksFailed)
		assert.Regexp(t, `✗ config: \d+ problems in the central config\n`, stdout)
		assert.Contains(t, stdout, "→ run `gero config validate` for the details")
		assert.Contains(t, stdout, "○ repos: skipped, needs config\n")
	})

	t.Run("a project with no clone is a warning", func(t *testing.T) {
		resetDoctorFlags(t)
		stubGh(t)
		withToken(t)
		good := "apiVersion: gero/v1alpha2\nkind: Project\nname: a\nspec:\n  repo: {name: gero-test/zz-no-such-repo}\n"
		withConfigDir(t, map[string]string{"a.yaml": good})

		stdout, _, err := execRoot(t, "doctor")

		require.NoError(t, err, "a warning does not fail the run")
		assert.Contains(t, stdout, "✔ config: 1 project, 0 streams, 0 artifacts, 0 holdings, 0 lines\n")
		assert.Contains(t, stdout, "⚠ repos: 1 of 1 project has no unique local clone: gero-test/zz-no-such-repo (not found)\n")
		assert.Contains(t, stdout, "→ not found: clone it under a repos.search directory, or set repos.paths.<slug>; "+
			"run `gero config resolve repos` for the details")
	})

	t.Run("--output env is refused", func(t *testing.T) {
		resetDoctorFlags(t)

		stdout, _, err := execRoot(t, "doctor", "--output", "env")

		require.Error(t, err)
		assert.ErrorContains(t, err, "does not support output format")
		assert.Empty(t, stdout)
	})

	t.Run("flags are declared with their defaults", func(t *testing.T) {
		cmd, _, err := rootCmd.Find([]string{"doctor"})
		require.NoError(t, err)

		d, err := cmd.Flags().GetDuration("timeout")
		require.NoError(t, err)
		assert.Equal(t, 10*time.Second, d)
		for _, name := range []string{"online", "strict", "list-checks"} {
			b, err := cmd.Flags().GetBool(name)
			require.NoError(t, err)
			assert.False(t, b, name)
		}
	})
}

// withConfigDir points GERO_CONFIG_DIR at a new directory holding files (name to
// content), so the configuration checks do not depend on the working directory.
// No files is a config with one line in it and no projects: an empty directory is
// not a config, since it is almost always the wrong directory, and a project
// would add a repos warning to every report.
func withConfigDir(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "v1alpha2"), 0o750))
	if files == nil {
		files = map[string]string{"lines.yaml": "apiVersion: gero/v1alpha2\nkind: RancherLines\nname: rancher\nspec:\n  lines:\n    - id: v2.15\n"}
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "v1alpha2", name), []byte(content), 0o600))
	}
	t.Setenv("GERO_CONFIG_DIR", dir)
}
