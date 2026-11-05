package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/branchtags"
	"github.com/rancher/orbs-automation/internal/cli/repoinfo"
	"github.com/spf13/cobra"
)

func init() {
	repoCmd := &cobra.Command{
		Use:   "repo",
		Short: "Inspect version and tag facts for a repository",
	}
	var offline bool
	infoCmd := &cobra.Command{
		Use:   "info [name|owner/name]",
		Short: "Show the config entry, latest releases and local state of a repository",
		Long: `Print what is known about a repository: its central-config entry (owned,
default branch, release policy and lines), its latest stable and pre-release
tags as the host reports them, and where it is cloned on this machine with the
state of that clone.

Name a Project (its config name, such as scc-operator, or its owner/name
slug) to look at it, else the repository you are in is used. For a project
with a release branch per line, the latest tags are shown for each line.

The tags are read from the host, so they are current even when your clone is
not. A public repository needs no token. Pass --offline to skip the network;
the rest of the report is still shown, as it is when no clone is found.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			out, err := repoinfo.Run(ctx, settingsFrom(cmd), cwd, args, repoinfo.Options{Offline: offline})
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			return err
		},
	}
	infoCmd.Flags().BoolVar(&offline, "offline", false, "do not read tags from the host")
	repoCmd.AddCommand(
		infoCmd,
		&cobra.Command{
			Use:   "branch-tags",
			Short: "Resolve branch and previous tags from the repository state",
			Long: `Print the branch tag, the static tag for this commit and the previous
static tag, for refs/heads/main and refs/heads/release/<version>.

The ref and commit come from GITHUB_REF and GITHUB_SHA when both are set,
else from the git repository in the current directory.`,
			// Faithful to the script: errors are printed to stdout, once.
			SilenceErrors: true,
			RunE: func(cmd *cobra.Command, _ []string) error {
				res, err := branchtags.Resolve(settingsFrom(cmd), os.Getenv, ".")
				if err != nil {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), err)
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), branchtags.Render(res))
				return err
			},
		},
	)
	rootCmd.AddCommand(repoCmd)
}
