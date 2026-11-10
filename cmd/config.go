package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/configfetch"
	"github.com/rancher/orbs-automation/internal/cli/configresolve"
	"github.com/rancher/orbs-automation/internal/cli/configshow"
	"github.com/rancher/orbs-automation/internal/cli/configvalidate"
	"github.com/rancher/orbs-automation/internal/cli/graphresolve"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/spf13/cobra"
)

// addConfig adds the command to root.
func addConfig(root *cobra.Command) {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Validate and inspect the application config",
	}
	resolveCmd := &cobra.Command{
		Use:   "resolve",
		Short: "Show what the CLI config resolves to on this machine",
	}
	resolveCmd.AddCommand(&cobra.Command{
		Use:   "repos [name|owner/name|.]",
		Short: "Show where each project's repository is cloned and how that was found",
		Long: `For each Project in the central config, print the local clone of its
repository and how it was found: the repository you are in (cwd), a
repos.paths entry, a scan of the repos.search directories matched by remote
URL (repos.search scan), or a sibling directory of the repository you are in.

A project with no clone, or with several equally good ones, is listed and the
command exits non-zero. Name one project (its config name or owner/name
slug, or . for the project whose repository you are in) to look at only that.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			// The table is the result, so it is written even when a repo is
			// unresolved and the command fails.
			out, err := configresolve.Repos(settingsFrom(cmd), cwd, args)
			if out != "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), out)
			}
			return err
		},
	})
	resolveCmd.AddCommand(&cobra.Command{
		Use:   "graph",
		Short: "Draw the whole application config as a D2 diagram",
		Long: `Draw every Project of the application config as a node, and every holding as
a node between the Project that produces a release and the Project that holds
it, so releases read as flowing one way from producers to consumers. A Project
is marked managed, shared or external, the final destination is marked as the
hub, and each holding's edge into its holder is labelled with the release lines
it serves.

The whole config is drawn, not one team's. The output is D2 source: render it
with d2 graph.d2 graph.svg.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			out, err := graphresolve.Graph(settingsFrom(cmd), cwd)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			return err
		},
	})
	configCmd.AddCommand(
		&cobra.Command{
			Use:   "validate",
			Short: "Validate the application config",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				out, err := configvalidate.Run(settingsFrom(cmd), cwd)
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), out)
				return err
			},
		},
		&cobra.Command{
			Use:   "fetch",
			Short: "Fetch a newer central config bundle than the one in this binary",
			Long: `Look at the releases of the central repository and, when the newest on this
binary's channel is newer than the binary itself, download its config bundle,
verify its checksum and signature, and keep it in the user cache. Every later
run reads that bundle in place of the one built into the binary, until a newer
binary is installed. A local config directory still wins over both.

This is the only command that fetches the bundle. It needs the network, so it
fails under --offline. The channel is --bundle-channel: a stable binary looks at
stable releases and a pre-release binary at pre-releases too.`,
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				ctx := cmd.Context()
				if ctx == nil {
					ctx = context.Background()
				}
				out, err := configfetch.Run(ctx, settingsFrom(cmd), workspace.FetchBundle)
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), out)
				return err
			},
		},
		&cobra.Command{
			Use:   "show",
			Short: "Show the resolved CLI config and where each value came from",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				out, err := configshow.Run(settingsFrom(cmd), cwd)
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), out)
				return err
			},
		},
		resolveCmd,
	)
	root.AddCommand(configCmd)
}
