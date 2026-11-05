package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/ship"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/spf13/cobra"
)

func init() {
	var in domain.Input
	shipCmd := &cobra.Command{
		Use:   "ship <project> [tag|line]",
		Short: "Ship a release of a project to a project that takes updates from it",
		Long: `Check what shipping a release of <project> to one of its consumers would do.

<project> is the project being shipped, by its config name or owner/name slug.
The argument is a tag, used as given, or a release line of the project, which
resolves to the line's latest stable tag; with neither, the project's latest
stable tag. --prerelease lets prereleases count. The consumers are the
projects whose config takes updates from <project>; name one with --to when
there are several, and the branch of the consumer to update with --branch.

Before anything is written the state is checked, from the host and not from
any clone: the release and the target branch must exist, the consumer's
current value is read and compared with what the release needs, and the
release's images must be published. A consumer that already has the release
is reported as already up to date.

This only checks for now: it writes nothing and creates no PR yet.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			in.Project = args[0]
			in.Arg = ""
			if len(args) == 2 {
				in.Arg = args[1]
			}
			out, err := ship.Run(ctx, settingsFrom(cmd), cwd, in)
			if out != "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), out)
			}
			return err
		},
	}
	shipCmd.Flags().StringVar(&in.To, "to", "", "the consumer to ship to, when several take updates from the project")
	shipCmd.Flags().StringVar(&in.Branch, "branch", "", "the consumer's branch to update")
	shipCmd.Flags().BoolVar(&in.Prerelease, "prerelease", false, "let prereleases count")
	rootCmd.AddCommand(shipCmd)
}
