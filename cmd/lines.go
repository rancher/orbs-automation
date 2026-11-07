package cmd

import (
	"context"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/linesresolve"
	"github.com/spf13/cobra"
)

// addLines adds the command to root.
func addLines(root *cobra.Command) {
	linesCmd := &cobra.Command{
		Use:   "lines",
		Short: "Work with release lines",
	}
	var hub string
	var phases []string
	resolveCmd := &cobra.Command{
		Use:   "resolve [line...]",
		Short: "Show how the release lines of the projects connect",
		Long: `Print the map of the release lines, newest first. Each Rancher line shows its
phase, its dates and its Kubernetes range from the lines document, then the
project the lines are connected through (the hub) and its branch for the line,
what it takes from, with each project's series and branch for the line and the
update method used. Projects that have the line but no direct link to the hub on
it are listed under it with their own connections, which is where a chain
through another project shows.

The hub is the project marked finalDestination: true in the config, the one that
releases end in (rancher/rancher), and nothing is shown as shipping on from it.
--hub names another project to connect through, which also shows what it ships
to.

What does not connect is marked with ! and counted at the end: a project that
ships to the hub but has no such line, say.

Name lines (v2.15 or 2.15) to show only those, and --phase to show only the
lines in some phases (development, ga, eom, eol). It reads the central config and
the clock and never the network; for what each branch holds on the host, use
repo status.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			return linesresolve.Run(ctx, cmd.OutOrStdout(), settingsFrom(cmd), cwd, linesresolve.Options{Hub: hub, Lines: args, Phases: phases})
		},
	}
	resolveCmd.Flags().StringVar(&hub, "hub", "", "the project to connect the lines through, instead of the one marked finalDestination")
	resolveCmd.Flags().StringSliceVar(&phases, "phase", nil, "only the lines in these phases: development, ga, eom, eol")
	linesCmd.AddCommand(resolveCmd)
	root.AddCommand(linesCmd)
}
