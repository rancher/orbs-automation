package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// flagged returns a command with the global flags declared and args parsed.
func flagged(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "x"}
	cmd.SetContext(context.Background())
	addGlobalFlags(cmd)
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}

func TestGlobalFlagsDeclared(t *testing.T) {
	for _, name := range []string{"config-dir", "upstream", "output", "json", "dry-run"} {
		t.Run(name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "x"}
			addGlobalFlags(cmd)

			f := cmd.PersistentFlags().Lookup(name)
			require.NotNil(t, f, "flag --%s", name)
			assert.Empty(t, f.Shorthand)
		})
	}
}

func TestRootDeclaresGlobalFlagsAndLoadsSettings(t *testing.T) {
	for _, name := range []string{"config-dir", "upstream", "output", "json", "verbose", "dry-run"} {
		assert.NotNil(t, rootCmd.PersistentFlags().Lookup(name), "flag --%s", name)
	}
	assert.NotNil(t, rootCmd.PersistentPreRunE)
}

func TestFlagLookup(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		key       string
		wantValue string
		wantSet   bool
	}{
		{"unset flag is not set", nil, "upstream", "", false},
		{"set flag", []string{"--upstream", "rancher"}, "upstream", "rancher", true},
		{"kebab name", []string{"--config-dir", "/cfg"}, "config-dir", "/cfg", true},
		{"explicitly empty counts as set", []string{"--upstream="}, "upstream", "", true},
		{"json is output json", []string{"--json"}, "output", "json", true},
		{"explicit output beats json", []string{"--json", "--output", "env"}, "output", "env", true},
		{"dry-run is not set unless given", nil, "dry-run", "", false},
		{"dry-run", []string{"--dry-run"}, "dry-run", "true", true},
		{"dry-run can be turned off explicitly", []string{"--dry-run=false"}, "dry-run", "false", true},
		{"unknown name", []string{"--upstream", "x"}, "nope", "", false},
		{"no -v leaves log unset", nil, "log", "", false},
		{"-v is info", []string{"-v"}, "log", "info", true},
		{"--verbose is info", []string{"--verbose"}, "log", "info", true},
		{"-vv is debug", []string{"-vv"}, "log", "debug", true},
		{"-v -v is debug", []string{"-v", "-v"}, "log", "debug", true},
		{"-vvv stays debug", []string{"-vvv"}, "log", "debug", true},
		{"no color flags leaves color unset", nil, "color", "", false},
		{"color", []string{"--color", "always"}, "color", "always", true},
		{"no-color is color never", []string{"--no-color"}, "color", "never", true},
		{"explicit color beats no-color", []string{"--no-color", "--color", "always"}, "color", "always", true},
		{"explicit auto beats no-color", []string{"--color=auto", "--no-color"}, "color", "auto", true},
		{"no-color=false is not never", []string{"--no-color=false"}, "color", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, set := flagLookup(flagged(t, tt.args...))(tt.key)

			assert.Equal(t, tt.wantValue, v)
			assert.Equal(t, tt.wantSet, set)
		})
	}
}

func userFile(t *testing.T, content string) map[string]string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return map[string]string{"GERO_CONFIG": p}
}

func TestLoadSettingsResolvesAllLayers(t *testing.T) {
	e := userFile(t, "fork: origin\noutput: text\nupstream: fromfile\n")
	e["GERO_OUTPUT"] = "json"
	cmd := flagged(t, "--upstream", "fromflag")

	require.NoError(t, loadSettings(env(e), t.TempDir())(cmd, nil))

	s := settingsFrom(cmd)
	assert.Equal(t, "fromflag", s.Upstream)
	assert.Equal(t, clicfg.SourceFlag, s.Source("upstream"))
	assert.Equal(t, clicfg.OutputJSON, s.Output)
	assert.Equal(t, clicfg.SourceEnv, s.Source("output"))
	assert.Equal(t, "origin", s.Fork)
}

func TestLoadSettingsWithoutUserFile(t *testing.T) {
	cmd := flagged(t)

	require.NoError(t, loadSettings(env(nil), t.TempDir())(cmd, nil))

	assert.Equal(t, clicfg.OutputText, settingsFrom(cmd).Output)
}

func TestLoadSettingsReportsFileErrors(t *testing.T) {
	e := userFile(t, "upstrem: x\n")

	err := loadSettings(env(e), t.TempDir())(flagged(t), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), e["GERO_CONFIG"])
}

func TestLoadSettingsReportsInvalidFlag(t *testing.T) {
	err := loadSettings(env(nil), t.TempDir())(flagged(t, "--output", "xml"), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "output")
}

func TestSettingsFromWithoutLoad(t *testing.T) {
	assert.Equal(t, clicfg.Settings{}, settingsFrom(flagged(t)))
}

func TestLoadSettingsDryRun(t *testing.T) {
	cmd := flagged(t, "--dry-run")
	require.NoError(t, loadSettings(env(nil), t.TempDir())(cmd, nil))
	assert.True(t, settingsFrom(cmd).DryRun)

	cmd = flagged(t)
	require.NoError(t, loadSettings(env(map[string]string{"GERO_DRY_RUN": "true"}), t.TempDir())(cmd, nil))
	assert.True(t, settingsFrom(cmd).DryRun, "from the environment")

	cmd = flagged(t)
	require.NoError(t, loadSettings(env(nil), t.TempDir())(cmd, nil))
	assert.False(t, settingsFrom(cmd).DryRun, "off unless asked for")
}

func TestShipsYesFlagsAndTheEnvAreOneSettingThatSkipsTheQuestion(t *testing.T) {
	for name, tt := range map[string]struct {
		args []string
		env  map[string]string
	}{
		"--yes":             {args: []string{"--yes"}},
		"-y":                {args: []string{"-y"}},
		"--non-interactive": {args: []string{"--non-interactive"}},
		"the env":           {env: map[string]string{"GERO_NON_INTERACTIVE": "true"}},
	} {
		t.Run(name, func(t *testing.T) {
			ship, _, err := rootCmd.Find([]string{"ship"})
			require.NoError(t, err)
			require.NoError(t, ship.Flags().Parse(tt.args))
			t.Cleanup(func() {
				for _, f := range []string{"yes", "non-interactive"} {
					_ = ship.Flags().Set(f, "false")
					ship.Flags().Lookup(f).Changed = false
				}
			})

			s, err := clicfg.Load(clicfg.Inputs{Flags: flagLookup(ship), Getenv: env(tt.env)})

			require.NoError(t, err)
			assert.True(t, s.NonInteractive)
		})
	}
}

func TestRepoTagsYesFlagsAreTheSameSettingAsShips(t *testing.T) {
	tag, _, err := rootCmd.Find([]string{"repo", "tag"})
	require.NoError(t, err)
	require.Equal(t, "tag", tag.Name())
	for _, args := range [][]string{{"-y"}, {"--yes"}, {"--non-interactive"}} {
		require.NoError(t, tag.Flags().Parse(args))

		s, err := clicfg.Load(clicfg.Inputs{Flags: flagLookup(tag), Getenv: env(nil)})

		require.NoError(t, err)
		assert.True(t, s.NonInteractive, "%v", args)
		for _, f := range []string{"yes", "non-interactive"} {
			_ = tag.Flags().Set(f, "false")
			tag.Flags().Lookup(f).Changed = false
		}
	}
}

func TestRepoTagMinorAndMajorAreExclusive(t *testing.T) {
	tag, _, err := rootCmd.Find([]string{"repo", "tag"})
	require.NoError(t, err)
	for _, name := range []string{"stable", "minor", "major"} {
		require.NotNil(t, tag.Flags().Lookup(name), name)
	}
	t.Cleanup(func() {
		for _, f := range []string{"minor", "major"} {
			_ = tag.Flags().Set(f, "false")
			tag.Flags().Lookup(f).Changed = false
		}
	})
	require.NoError(t, tag.ParseFlags([]string{"--minor", "--major"}))

	assert.Error(t, tag.ValidateFlagGroups())
}

func TestRepoTagHasAPushFlag(t *testing.T) {
	tag, _, err := rootCmd.Find([]string{"repo", "tag"})
	require.NoError(t, err)

	assert.NotNil(t, tag.Flags().Lookup("push"))
}

func TestRepoStatusCanLeaveOutTheConsumersAndTheSources(t *testing.T) {
	status, _, err := rootCmd.Find([]string{"repo", "status"})
	require.NoError(t, err)

	assert.NotNil(t, status.Flags().Lookup("no-consumers"))
	assert.NotNil(t, status.Flags().Lookup("no-sources"))
}

func TestLinesResolveTakesAHubAndPhases(t *testing.T) {
	resolve, _, err := rootCmd.Find([]string{"lines", "resolve"})
	require.NoError(t, err)

	assert.NotNil(t, resolve.Flags().Lookup("hub"))
	assert.NotNil(t, resolve.Flags().Lookup("phase"))
}

func TestLoadSettingsNoColorTurnsTheTerminalOff(t *testing.T) {
	old := terminalFacts
	t.Cleanup(func() { terminalFacts = old })
	terminalFacts = func() clicfg.Facts { return clicfg.Facts{StdinTTY: true, StdoutTTY: true} }
	e := map[string]string{"TERM": "xterm-256color"}

	on := flagged(t)
	require.NoError(t, loadSettings(env(e), t.TempDir())(on, nil))
	off := flagged(t, "--no-color")
	require.NoError(t, loadSettings(env(e), t.TempDir())(off, nil))

	assert.Equal(t, clicfg.Profile256, settingsFrom(on).Terminal.Profile, "on by default at a capable terminal")
	assert.Equal(t, clicfg.ProfileNone, settingsFrom(off).Terminal.Profile)
	assert.Equal(t, clicfg.ColorNever, settingsFrom(off).Color)
}
