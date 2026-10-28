package cmd

import (
	"github.com/rancher/orbs-automation/internal/errs"
	"github.com/spf13/cobra"
)

func init() {
	repoCmd := &cobra.Command{
		Use:   "repo",
		Short: "Inspect version and tag facts for a repository",
	}
	repoCmd.AddCommand(
		&cobra.Command{
			Use:   "check-semver <version>",
			Short: "Derive version facts from a semver string",
			RunE: func(*cobra.Command, []string) error {
				return errs.NotImplemented("gero repo check-semver")
			},
		},
		&cobra.Command{
			Use:   "branch-tags",
			Short: "Resolve branch and previous tags from the repository state",
			RunE: func(*cobra.Command, []string) error {
				return errs.NotImplemented("gero repo branch-tags")
			},
		},
	)
	rootCmd.AddCommand(repoCmd)
}
