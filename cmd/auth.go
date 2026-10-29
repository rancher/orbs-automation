package cmd

import (
	"github.com/rancher/orbs-automation/internal/errs"
	"github.com/spf13/cobra"
)

func init() {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Check how gero authenticates to GitHub",
	}
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show the GitHub host and token gero would use, and whether GitHub accepts it",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return errs.NotImplemented("gero auth status",
				"Read-only. Shows which GitHub host and token source gero would use (environment, gh config or the gh keyring) "+
					"and asks GitHub who the token belongs to. The token is never printed.")
		},
	}
	statusCmd.Flags().String("hostname", "", "GitHub host to check (default: GH_HOST, then gh's config, then github.com)")
	authCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(authCmd)
}
