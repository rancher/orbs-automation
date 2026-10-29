package cmd

import (
	"context"
	"io"

	"github.com/rancher/orbs-automation/internal/cli/logging"
	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/spf13/cobra"
)

type settingsKey struct{}

// addGlobalFlags declares the persistent flags that feed internal/clicfg on cmd.
// Their defaults are empty on purpose: flagLookup reports a flag only when it was
// set, so the real defaults live in internal/clicfg.
func addGlobalFlags(cmd *cobra.Command) {
	f := cmd.PersistentFlags()
	f.String("config-dir", "", "directory of the central application config (env GERO_CONFIG_DIR)")
	f.String("upstream", "", "remote that points at the real source repository (env GERO_UPSTREAM)")
	f.String("output", "", "output format: text, json or env (env GERO_OUTPUT; default text)")
	f.Bool("json", false, "machine output, shorthand for --output json")
	f.CountP("verbose", "v", "log more to stderr: -v for info, -vv for debug (env GERO_LOG=debug|info|warn|error)")
}

// flagLookup adapts the parsed flags of cmd to the framework-neutral lookup that
// internal/clicfg reads. A flag counts as set only when it was given explicitly,
// so a flag's default never masks the environment or the user file. --json stands
// in for --output json unless --output was given, and -v, -vv stand in for the
// log level.
func flagLookup(cmd *cobra.Command) clicfg.FlagLookup {
	return func(name string) (string, bool) {
		flags := cmd.Flags()
		if name == "output" && !flags.Changed("output") {
			if json, err := flags.GetBool("json"); err == nil && json {
				return clicfg.OutputJSON, true
			}
		}
		if name == "log" {
			n, err := flags.GetCount("verbose")
			switch {
			case err != nil || n == 0:
				return "", false
			case n == 1:
				return clicfg.LogInfo, true
			default:
				return clicfg.LogDebug, true
			}
		}
		f := flags.Lookup(name)
		if f == nil || !flags.Changed(name) {
			return "", false
		}
		return f.Value.String(), true
	}
}

// loadSettings returns the root PersistentPreRunE: it resolves clicfg.Settings
// from flags, getenv and the user file under home, and stores it on the command
// for settingsFrom.
func loadSettings(getenv func(string) string, home string) func(*cobra.Command, []string) error {
	return settingsLoader(getenv, home, false)
}

// loadSettingsForDoctor is loadSettings for gero doctor, which has to run to
// report a user file that cannot be loaded.
func loadSettingsForDoctor(getenv func(string) string, home string) func(*cobra.Command, []string) error {
	return loadSettings(getenv, home)
}

// settingsLoader is loadSettings, optionally without reading the user file.
func settingsLoader(getenv func(string) string, home string, skipUserFile bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		var userFile io.Reader
		path := ""
		if !skipUserFile {
			file, p, err := clicfg.OpenUserFile(getenv, home)
			if err != nil {
				return err
			}
			path = p
			if file != nil {
				defer file.Close()
				userFile = file
			}
		}
		s, err := clicfg.Load(clicfg.Inputs{
			Flags:        flagLookup(cmd),
			Getenv:       getenv,
			UserFile:     userFile,
			UserFileName: path,
			Home:         home,
		})
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		cmd.SetContext(context.WithValue(ctx, settingsKey{}, s))
		logger := logging.New(cmd.ErrOrStderr(), s.LogLevel, clicfg.InCI(getenv))
		logger.Debug("settings loaded",
			"output", s.Output, "upstream", s.Upstream, "log", s.LogLevel)
		return nil
	}
}

// settingsFrom returns the Settings stored by loadSettings, or the zero value
// when none was stored.
func settingsFrom(cmd *cobra.Command) clicfg.Settings {
	ctx := cmd.Context()
	if ctx == nil {
		return clicfg.Settings{}
	}
	s, _ := ctx.Value(settingsKey{}).(clicfg.Settings)
	return s
}
