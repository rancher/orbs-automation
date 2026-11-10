package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rancher/orbs-automation/cmd/ci"
	"github.com/rancher/orbs-automation/cmd/version"
	"github.com/rancher/orbs-automation/internal/cli/cigate"
	"github.com/rancher/orbs-automation/internal/cli/logging"
	"github.com/rancher/orbs-automation/internal/cli/repl"
	"github.com/rancher/orbs-automation/internal/cli/termfacts"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/spf13/cobra"
)

type settingsKey struct{}

// addGlobalFlags declares the persistent flags that feed internal/platform/clicfg on cmd.
// Their defaults are empty on purpose: flagLookup reports a flag only when it was
// set, so the real defaults live in internal/platform/clicfg.
func addGlobalFlags(cmd *cobra.Command) {
	f := cmd.PersistentFlags()
	f.String("central-config-dir", "", "directory of the central application config (env GERO_CENTRAL_CONFIG_DIR)")
	f.String("config-source", "", "where the central config comes from: auto, bundle or dir (env GERO_CONFIG_SOURCE; default auto)")
	f.String("bundle-channel", "", "which releases the central config bundle may come from: auto, stable or prerelease (env GERO_BUNDLE_CHANNEL; default auto)")
	f.String("bundle-file", "", "use this bundle file, downloaded by hand, as the central config (env GERO_BUNDLE_FILE)")
	f.String("upstream", "", "remote that points at the real source repository (env GERO_UPSTREAM)")
	f.String("team", "", "the active team context: projects whose managedBy is this team are the ones gero ships from (env GERO_TEAM; default orbs)")
	f.String("output", "", "output format: text, json or env (env GERO_OUTPUT; default text)")
	f.String("color", "", "color: auto, always or never (env GERO_COLOR; default auto: as much as the terminal supports)")
	f.Bool("no-color", false, "shorthand for --color never")
	f.Bool("json", false, "machine output, shorthand for --output json")
	f.Bool("dry-run", false, "for a command that writes: compute and print what it would do, and change nothing (env GERO_DRY_RUN)")
	f.CountP("verbose", "v", "log more to stderr: -v for info, -vv for debug (env GERO_LOG=debug|info|warn|error)")
}

// flagLookup adapts the parsed flags of cmd to the framework-neutral lookup that
// internal/platform/clicfg reads. A flag counts as set only when it was given explicitly,
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
		if name == "color" && !flags.Changed("color") {
			if no, err := flags.GetBool("no-color"); err == nil && no {
				return clicfg.ColorNever, true
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
// report a user file that cannot be loaded. When loading fails it loads again
// without the user file, so doctor still sees the flags, the environment and the
// defaults, and its user-file check reports what was wrong with the file. If the
// second load fails too, the problem is not the file, and that error is returned.
func loadSettingsForDoctor(getenv func(string) string, home string) func(*cobra.Command, []string) error {
	strict := loadSettings(getenv, home)
	withoutFile := settingsLoader(getenv, home, true)
	return func(cmd *cobra.Command, args []string) error {
		if err := strict(cmd, args); err == nil {
			return nil
		}
		return withoutFile(cmd, args)
	}
}

// terminalFacts is what the process knows about its terminal, which Load cannot
// read itself. It is a variable so a test can give another.
var terminalFacts = func() clicfg.Facts {
	return termfacts.Detect(os.Stdin, os.Stdout)
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
			Facts:        terminalFacts(),
		})
		if err != nil {
			return err
		}
		// The release is what picks the central config bundle when no local config
		// is named (docs/specs/CENTRAL_BUNDLE.md); only here is it known.
		s.GeroVersion = version.Version
		if err := requireContext(cmd, s, getenv); err != nil {
			return err
		}
		// A key the file has that gero does not know is ignored, not fatal, so
		// say so: it is likely a typo, and the command runs without it.
		for _, w := range s.Warnings {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", w)
		}
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		cmd.SetContext(context.WithValue(ctx, settingsKey{}, s))
		logger := logging.New(cmd.ErrOrStderr(), s.LogLevel, s.CI)
		// The scan skips a search directory that is not there, so a typo hides the
		// clones under it. Only gero doctor fails on it; the rest say so with -v.
		for _, dir := range s.Repos.MissingSearch() {
			logger.Info("repos.search is not a directory, so the scan skips it", "dir", dir)
		}
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

// requireContext enforces the gero/context annotation of cmd against s: a
// CI-only command needs the CI context and a token, and a workstation-only one
// needs neither CI nor the REPL, which getenv tells (internal/cli/cigate).
func requireContext(cmd *cobra.Command, s clicfg.Settings, getenv func(string) string) error {
	switch cmd.Annotations[ci.ContextAnnotation] {
	case ci.ContextCIOnly:
		return cigate.Require(cmd.CommandPath(), s)
	case ci.ContextWorkstationOnly:
		return cigate.RequireWorkstation(cmd.CommandPath(), s, getenv(repl.EnvRepl) != "")
	}
	return nil
}
