package cmd

import (
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/branchtags"
	"github.com/spf13/cobra"
)

func init() {
	repoCmd := &cobra.Command{
		Use:   "repo",
		Short: "Inspect version and tag facts for a repository",
	}
	repoCmd.AddCommand(
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
				res, err := branchtags.Resolve(os.Getenv, ".")
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
