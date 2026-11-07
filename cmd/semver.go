package cmd

import (
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/checksemver"
	"github.com/spf13/cobra"
)

// addSemver adds the command to root.
func addSemver(root *cobra.Command) {
	semverCmd := &cobra.Command{
		Use:   "semver",
		Short: "Work with version strings",
	}
	semverCmd.AddCommand(&cobra.Command{
		Use:   "check [tag]",
		Short: "Derive version facts from a semver string",
		Long: `Report whether a tag has a prerelease or build metadata.

The tag is the first argument, else GITHUB_REF_NAME. One leading "v" is
stripped. Any text is accepted, SemVer or not: this is the lenient check the
release workflows rely on, not a strict SemVer validator.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			tag, err := checksemver.ResolveTag(args, os.Getenv)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), checksemver.Render(checksemver.Classify(tag)))
			return err
		},
	})
	root.AddCommand(semverCmd)
}
