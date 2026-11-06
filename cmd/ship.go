package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/ship"
	"github.com/spf13/cobra"
)

func init() {
	var in ship.Options
	shipCmd := &cobra.Command{
		Use:   "ship [project] [tag|line]",
		Short: "Ship a release of a project to a project that takes updates from it",
		Long: `Check what shipping a release of <project> to one of its consumers would do.

<project> is the project being shipped, by its config name or owner/name slug;
leave it out to ship the project whose repository you are in. A lone argument
that looks like a version or a release line (v0.5.3, v2.15) is the release,
and any other lone argument is the project.

The release is a tag, used as given, or a release line of the project, which
resolves to the line's latest stable tag; with neither, the project's latest
stable tag. --prerelease lets prereleases count. The project's
downstream edges name the consumers and the update method of each; name one
with --to, and a method with --method, when there are several. Every branch of the consumer that the release goes to is
checked: all its release branches for a project that is not tied to release
lines, or the branch for the release's line for one that is. --branch limits it
to one branch of the consumer.

Before anything is written the state is checked, from the host and not from
any clone: the release and the target branch must exist, the consumer's
current value is read and compared with what the release needs, and the
release's images must be published, and the release must not be older than what
the consumer holds (--allow-downgrade ships it anyway). A consumer that already
has the release is reported as already up to date.

It shows the branch, title and labels the PR would have; --preview adds the
commit message and body.

A plain run only checks for now: pushing and opening the PR are not built.
--no-push and --dry-run go further and make the commit for each branch that
needs one, in your own clone of the consumer (found as gero cd finds it): on a
new branch when the clone is clean and on a branch, else in a temporary
worktree that leaves your checkout alone. --no-push keeps the branch for you to
review. --dry-run always uses a worktree and removes it and the branch
afterwards; with both flags it is a dry run.`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			in.Project, in.Arg = ship.ParseArgs(args)
			out, err := ship.Run(ctx, settingsFrom(cmd), cwd, in)
			if out != "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), out)
			}
			return err
		},
	}
	shipCmd.Flags().StringVar(&in.To, "to", "", "the consumer to ship to, when the project ships to several")
	shipCmd.Flags().StringVar(&in.Method, "method", "", "the consumer's update method, when the project uses several of one consumer's")
	shipCmd.Flags().StringVar(&in.Branch, "branch", "", "check only this branch of the consumer, instead of every branch the release goes to")
	shipCmd.Flags().BoolVar(&in.Prerelease, "prerelease", false, "let prereleases count")
	shipCmd.Flags().BoolVar(&in.AllowDowngrade, "allow-downgrade", false, "ship a release older than what the consumer holds")
	shipCmd.Flags().BoolVar(&in.Preview, "preview", false, "print the commit message and PR body too")
	shipCmd.Flags().BoolVar(&in.NoPush, "no-push", false, "commit on a branch in your clone for review, but do not push or open a PR")
	rootCmd.AddCommand(shipCmd)
}
