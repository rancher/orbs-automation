package cmd

import (
	"fmt"

	"github.com/rancher/orbs-automation/internal/cli/authstatus"
	"github.com/spf13/cobra"
)

// addAuth adds the command to root.
func addAuth(root *cobra.Command) {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Check how gero authenticates to GitHub",
	}
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show the GitHub host and token gero would use, and whether GitHub accepts it",
		Long: `Show which GitHub host and token source gero would use, and ask GitHub who
the token belongs to. The token itself is never printed.

The token comes from, in order: GH_TOKEN, GITHUB_TOKEN, the gh config file,
then "gh auth token" (the gh keyring). For a GitHub Enterprise Server host the
environment variables are GH_ENTERPRISE_TOKEN and GITHUB_ENTERPRISE_TOKEN
instead. The host is --hostname, else GH_HOST, else the single host in gh's
config, else github.com.

A token that GitHub rejects is reported and the command exits non-zero.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			hostname, err := cmd.Flags().GetString("hostname")
			if err != nil {
				return err
			}
			// The result is written even when the token is rejected and the
			// command fails.
			out, err := authstatus.Run(cmd.Context(), settingsFrom(cmd), hostname)
			if out != "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), out)
			}
			return err
		},
	}
	statusCmd.Flags().String("hostname", "", "GitHub host to check (default: GH_HOST, then gh's config, then github.com)")
	authCmd.AddCommand(statusCmd)
	root.AddCommand(authCmd)
}
