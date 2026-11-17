package cmd

import (
	"context"
	"os"

	"github.com/rancher/orbs-automation/cmd/shipflags"
	"github.com/rancher/orbs-automation/internal/cli/logging"
	"github.com/rancher/orbs-automation/internal/cli/ship"
	"github.com/rancher/orbs-automation/internal/cli/termfacts"
	"github.com/spf13/cobra"
)

// addShip adds the command to root.
func addShip(root *cobra.Command) {
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
stable tag. A project with a version series for each Rancher line (shell's 0.9.x
for v2.16, 0.8.x for v2.15) ships each line the latest tag of its own series
when none is given, and a tag only to the line it is in. --prerelease lets
prereleases count. The projects
that hold a release of this one, in their config, are its consumers; name one
with --to, when there are several. A run writes every holding the project has in
the consumer, in one pull request for each branch. Every branch of the consumer that the release goes to is
checked: all its release branches for a project that is not tied to release
lines, or the branch for the release's line for one that is. --line limits it
to one Rancher line of the consumer (v2.15 or 2.15), on whichever branch the
consumer keeps that line, so the same flag works for consumers that name their
branches differently.

A consumer that holds a different stream of the project for each line (BRO holds
a different wrangler minor on v2.15 and v2.16) needs the line, or a tag, to know
which: "gero ship wrangler v2.15 --to=bro" ships the latest stable release of the
stream BRO holds on v2.15, and the plan compares it with what rancher/rancher
holds on that line. A tag picks the stream it belongs to.

Before anything is written the state is checked, from the host and not from
any clone: the release and the target branch must exist, the consumer's
current value is read and compared with what the release needs, and the
release's images must be published, and the release must not be older than what
the consumer holds (--allow-downgrade ships it anyway). A consumer that already
has the release is reported as already up to date.

It shows the branch, title and labels the PR would have; --preview adds the
commit message and body, and the commit each step would make if it changes
files. Each commit made is listed with the files it changed.

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
			in.Yes = s.NonInteractive // --yes and --non-interactive, or GERO_NON_INTERACTIVE
			log := logging.New(cmd.ErrOrStderr(), s.LogLevel, s.CI)
			in.In, in.Interactive = cmd.InOrStdin(), termfacts.IsTerminal(os.Stdin)
			return ship.Run(ctx, cmd.OutOrStdout(), log, s, cwd, in)
		},
	}
	shipflags.Bind(shipCmd, &in)
	shipCmd.AddCommand(seriesCommand())
	shipCmd.Flags().StringVar(&in.PushTo, "push-to", "", "push the branches to the fork or the upstream, instead of what your settings say")
	shipCmd.Flags().BoolVarP(&in.Yes, "yes", "y", false, "go on without asking before pushing branches and opening PRs")
	shipCmd.Flags().BoolVar(&in.Yes, "non-interactive", false, "the same as --yes: never ask")
	shipCmd.Flags().BoolVar(&in.NoPush, "no-push", false, "commit on a branch in your clone for review, but do not push or open a PR")
	root.AddCommand(shipCmd)
}

// seriesCommand is gero ship series: fix the series holdings of a project.
func seriesCommand() *cobra.Command {
	var in ship.SeriesOptions
	cmd := &cobra.Command{
		Use:   "series [project]",
		Short: "Fix the series holdings of a project that hold another series than their stream has",
		Long: `Check what a project holds of the series of a stream, line by line, and open a pull
request for each line whose branch holds another series than the one its stream
has for that line.

A series holding is a file that names a series and no release (prometheus-federator's
kuberlrVersion: v9), which another tool reads to pick the newest release of it.
gero repo status says where one differs. This writes it, and only when an Edit
follows the holding: nothing is written until an Edit says what a write of it
takes, so a holding with none is reported and left alone.

<project> is the project whose files hold the series, by its config name or
owner/name slug; leave it out for the project whose repository you are in.
--holding limits the run to one holding and --line to one Rancher line of the
project. It works as gero ship does for a release: in your own clone, on a new
branch (or a worktree when the clone is dirty), committing the write and then each
step of the Edit that changed files; it asks once before it pushes the branches and
opens the PRs, and --yes goes on without asking. --no-push stops after the commits and
--dry-run does everything but write.`,
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
			if len(args) > 0 {
				in.Project = args[0]
			}
			s := settingsFrom(cmd)
			in.Yes = s.NonInteractive
			log := logging.New(cmd.ErrOrStderr(), s.LogLevel, s.CI)
			in.In, in.Interactive = cmd.InOrStdin(), termfacts.IsTerminal(os.Stdin)
			return ship.RunSeries(ctx, cmd.OutOrStdout(), log, s, cwd, in)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&in.Holding, "holding", "", "fix only this series holding, by name")
	flags.StringSliceVar(&in.Lines, "line", nil, "fix only this Rancher line (v2.15); repeat it or separate lines with commas for several")
	flags.BoolVar(&in.Preview, "preview", false, "print the commit message and the PR body too")
	flags.StringVar(&in.PushTo, "push-to", "", "push the branches to the fork or the upstream, instead of what your settings say")
	flags.BoolVarP(&in.Yes, "yes", "y", false, "go on without asking before pushing branches and opening PRs")
	flags.BoolVar(&in.Yes, "non-interactive", false, "the same as --yes: never ask")
	flags.BoolVar(&in.NoPush, "no-push", false, "commit on a branch in your clone for review, but do not push or open a PR")
	return cmd
}
