// Package cmd is the cobra layer: the root command, the command tree, flags,
// help text, groups and gating. Command logic lives in internal/cli.
package cmd

import (
	"os"

	"github.com/rancher/orbs-automation/cmd/ci"
	"github.com/rancher/orbs-automation/cmd/groups"
	"github.com/rancher/orbs-automation/cmd/version"
	"github.com/rancher/orbs-automation/internal/cli/buildinfo"
	"github.com/spf13/cobra"
)

// rootCmd is the command tree of this process. The REPL builds another with
// newRootCmd for each line, since cobra keeps flag values on a command between
// runs.
var rootCmd = newRootCmd()

func init() {
	buildinfo.Apply(version.Version)
}

// newRootCmd builds the whole command tree, wiring only: no I/O, and nothing
// shared with a tree built before it.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "gero",
		Short: "Automating workflows, orchestrating tasks, and powering team ORBS",
		Long: `gero is the automation CLI for team ORBS, named after Dr. Gero, the
scientist behind the Red Ribbon Army's autonomous androids.

It takes the repetitive release work across the Rancher orbs repositories
(workflow sync, line resolution, and release reconciliation) and runs it
without manual intervention, with gh-style commands: gero <category> <action>.`,
		SilenceUsage: true,
	}
	// Setting Version makes cobra print it for --version. The flag is declared
	// here, without a shorthand: cobra would otherwise claim -v, which belongs
	// to --verbose (docs/specs/CLI_FRAMEWORK.md). `gero version` is the
	// primary form (versioncmd.go).
	root.Version = version.String()
	root.Flags().Bool("version", false, "version for gero")
	addGlobalFlags(root)
	home, _ := os.UserHomeDir()
	root.PersistentPreRunE = loadSettings(os.Getenv, home)
	root.AddGroup(&groups.CIGroup)
	ci.RegisterSubcommands(root, settingsFrom)
	for _, add := range []func(*cobra.Command){
		addAuth, addCd, addConfig, addDoctor, addInit, addLines, addRepl, addRepo, addShell,
		addShip, addSemver, addVersion, addWorkflows,
	} {
		add(root)
	}
	return root
}

// Execute runs the root command. It is called once by main.main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
