package cmd

import (
	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/spf13/cobra"
)

// addGlobalFlags declares the persistent flags that feed internal/clicfg on cmd.
func addGlobalFlags(cmd *cobra.Command) {}

// flagLookup adapts the parsed flags of cmd to the framework-neutral lookup that
// internal/clicfg reads. A flag counts as set only when it was given explicitly,
// so a flag's default never masks the environment or the user file.
func flagLookup(cmd *cobra.Command) clicfg.FlagLookup {
	return func(string) (string, bool) { return "", false }
}

// loadSettings returns the root PersistentPreRunE: it resolves clicfg.Settings
// from flags, getenv and the user file under home, and stores it on the command
// for settingsFrom.
func loadSettings(getenv func(string) string, home string) func(*cobra.Command, []string) error {
	return func(*cobra.Command, []string) error { return nil }
}

// settingsFrom returns the Settings stored by loadSettings, or the zero value
// when none was stored.
func settingsFrom(cmd *cobra.Command) clicfg.Settings {
	return clicfg.Settings{}
}
