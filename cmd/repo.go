package cmd

import (
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/checksemver"
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
			Use:   "check-semver [tag]",
			Short: "Derive version facts from a semver string",
			Long: `Report whether a tag has a prerelease or build metadata.

The tag is the first argument, else GITHUB_REF_NAME. One leading "v" is
stripped. Any text is accepted, SemVer or not.`,
			Args: cobra.ArbitraryArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				tag, err := checksemver.ResolveTag(args, os.Getenv)
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), checksemver.Render(checksemver.Classify(tag)))
				return err
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
