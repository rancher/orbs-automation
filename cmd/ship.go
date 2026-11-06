package cmd

import (
	"context"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/logging"
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
lines, or the branch for the release's line for one that is. --line limits it
to one Rancher line of the consumer (v2.15 or 2.15), on whichever branch the
consumer keeps that line, so the same flag works for consumers that name their
branches differently.

Before anything is written the state is checked, from the host and not from
any clone: the release and the target branch must exist, the consumer's
current value is read and compared with what the release needs, and the
release's images must be published, and the release must not be older than what
the consumer holds (--allow-downgrade ships it anyway). A consumer that already
has the release is reported as already up to date.

It shows the branch, title and labels the PR would have; --preview adds the
commit message and body.

For each branch that needs a PR it works in your own clone of the consumer
(found as gero cd finds it): on a new branch when the clone is clean and on a
branch, else in a temporary worktree that leaves your checkout alone. It
commits the edit and then each step that changed files. Before it writes
anything it looks at the open PRs of each branch and asks once: it pushes the
branch to the fork or the upstream (--push-to, else your settings, else the fork
when you have one) and opens the PR, and closes your own older PRs for the same
bump, with a comment. PRs by anyone else are never touched. --yes goes on
without asking, and is needed where there is no terminal.

When your own PR for this release is already open, nothing is rebuilt, and the
report says whether it is behind its base. --refresh rebuilds it on a scratch
branch from the current head of the target and compares the change with the
branch that is there: if they are the same nothing is pushed, and if they differ
the branch is replaced (a force push that is only allowed while the remote branch
is still where it was seen), the PR's title and body are refreshed, and the
refresh is recorded in one comment that gero keeps on the PR. It only touches a
branch that is yours to change: in the repository you push to, under the name this
run would use, with every commit made by gero as you.

--no-push stops after the commits and keeps the branches in your clone for
review, without contacting GitHub. --dry-run does everything but write: it uses a
worktree, removes it and the branch, and says what it would push, open and close;
with both flags it is a dry run.`,
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
			s := settingsFrom(cmd)
			log := logging.New(cmd.ErrOrStderr(), s.LogLevel, s.CI)
			in.In, in.Interactive = cmd.InOrStdin(), isTerminal(os.Stdin)
			return ship.Run(ctx, cmd.OutOrStdout(), log, s, cwd, in)
		},
	}
	shipCmd.Flags().StringVar(&in.To, "to", "", "the consumer to ship to, when the project ships to several")
	shipCmd.Flags().StringVar(&in.Method, "method", "", "the consumer's update method, when the project uses several of one consumer's")
	shipCmd.Flags().StringSliceVar(&in.Lines, "line", nil, "ship only to this Rancher line of the consumer (v2.15), on whichever branch it keeps that line, instead of every line the release goes to; repeat it or separate lines with commas for several")
	shipCmd.Flags().BoolVar(&in.Prerelease, "prerelease", false, "let prereleases count")
	shipCmd.Flags().BoolVar(&in.AllowDowngrade, "allow-downgrade", false, "ship a release older than what the consumer holds")
	shipCmd.Flags().BoolVar(&in.Preview, "preview", false, "print the commit message and PR body too")
	shipCmd.Flags().StringVar(&in.PushTo, "push-to", "", "push the branches to the fork or the upstream, instead of what your settings say")
	shipCmd.Flags().BoolVar(&in.Refresh, "refresh", false, "rebuild your open PR for this release and replace its branch if it differs")
	shipCmd.Flags().BoolVarP(&in.Yes, "yes", "y", false, "go on without asking before pushing branches and opening PRs")
	shipCmd.Flags().BoolVar(&in.Yes, "non-interactive", false, "the same as --yes: never ask")
	shipCmd.Flags().BoolVar(&in.NoPush, "no-push", false, "commit on a branch in your clone for review, but do not push or open a PR")
	rootCmd.AddCommand(shipCmd)
}
