package cmd

import (
	"os"

	"github.com/rancher/orbs-automation/cmd/ci"
	"github.com/rancher/orbs-automation/cmd/groups"
	"github.com/rancher/orbs-automation/cmd/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gero",
	Short: "Automating workflows, orchestrating tasks, and powering team ORBS",
	Long: `gero is the automation CLI for team ORBS, named after Dr. Gero, the
scientist behind the Red Ribbon Army's autonomous androids.

It takes the repetitive release work across the Rancher orbs repositories
(workflow sync, line resolution, and release reconciliation) and runs it
without manual intervention, with gh-style commands: gero <category> <action>.`,
	SilenceUsage: true,
}

func init() {
	// Setting Version makes cobra add the --version flag.
	rootCmd.Version = version.String()
	rootCmd.AddGroup(&groups.CIGroup)
	ci.RegisterSubcommands(rootCmd)
}

// Execute runs the root command. It is called once by main.main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
