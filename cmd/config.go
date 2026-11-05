package cmd

import (
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/configresolve"
	"github.com/rancher/orbs-automation/internal/cli/configshow"
	"github.com/rancher/orbs-automation/internal/cli/configvalidate"
	"github.com/spf13/cobra"
)

func init() {
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
	rootCmd.AddCommand(configCmd)
}
