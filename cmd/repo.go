package cmd

import (
	"context"
	"fmt"
	"os"

	clibackport "github.com/rancher/orbs-automation/internal/cli/backport"
	"github.com/rancher/orbs-automation/internal/cli/branchtags"
	"github.com/rancher/orbs-automation/internal/cli/imagesexist"
	"github.com/rancher/orbs-automation/internal/cli/logging"
	"github.com/rancher/orbs-automation/internal/cli/repobranch"
	"github.com/rancher/orbs-automation/internal/cli/repoclone"
	"github.com/rancher/orbs-automation/internal/cli/repoinfo"
	"github.com/rancher/orbs-automation/internal/cli/repolist"
	"github.com/rancher/orbs-automation/internal/cli/repostatus"
	"github.com/rancher/orbs-automation/internal/cli/reposync"
	"github.com/rancher/orbs-automation/internal/cli/repotag"
	"github.com/rancher/orbs-automation/internal/cli/termfacts"
	"github.com/spf13/cobra"
)

// addRepo adds the command to root.
func addRepo(root *cobra.Command) {
	repoCmd := &cobra.Command{
		Use:   "repo",
		Short: "Inspect version and tag facts for a repository",
	}
	infoCmd := &cobra.Command{
		Use:   "info [name|owner/name]",
		Short: "Show the config entry and local clone of a repository, without the network",
		Long: `Print what is known about a repository without reading the network: its
central-config entry (who manages it and whether your team owns it, default branch, release policy and lines), and
where it is cloned on this machine with the state of that clone.

Name a Project (its config name, such as scc-operator, or its owner/name
slug) to look at it, else the repository you are in is used (. says so
explicitly). A missing clone leaves the rest of the report.

For what the host has (the latest tags, the branch heads, and how the clone
compares), use repo status.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			out, err := repoinfo.Run(settingsFrom(cmd), cwd, args)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			return err
		},
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List which projects have a clone on this machine, and which do not",
		Long: `List the projects of the central config by whether they have a clone on this
machine, in three lists: the ones that are cloned, with where, the ones that are
not cloned yet, and the ones with several clones, which gero cannot choose
between. A project outside the organization is listed only when it is cloned,
since gero repo clone --all leaves it out.

It reads the disk and never the network. A repos.paths entry whose directory is
missing is not a clone. Clones are found as every command finds them: the
repository you are in, repos.paths, the repos.search directories, and the
neighbors of the repository you are in.

To put the ones that are missing in place, use gero repo clone.

To see how the user file resolves each repository, and from which source,
use gero config resolve repos.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			out, err := repolist.List(settingsFrom(cmd), cwd)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			return err
		},
	}
	var noConsumers, noSources bool
	statusCmd := &cobra.Command{
		Use:   "status [name|owner/name]",
		Short: "Show where the releases of a repository stand, as the host has them",
		Long: `Print where the releases of a repository stand, in sections that each
answer one question.

Releases: for each release line (or the one release branch of a Project without
lines), the commit at the head of the branch on the host, its latest stable and
pre-release tags, STATE (no release, released, work since release, rc open with
the head on the rc or past it) and NEXT, the tags gero repo tag could make:
a prerelease, and when the newest tag is a prerelease the promotion too, each
with the reason in parentheses when it may not be made now.

Local clone: the commits on the branch since the latest tag, and whether the
local clone's branch is in sync, behind or ahead.

Sources and Consumers, for a Project that takes from others or ships to others
(and left out for one that does neither): for each, the branches the releases go
to, what each holds, the latest stable release, and whether it is current,
behind, or on a prerelease. A source is a project that ships to this one, and
a consumer a project this one ships to. Reading them costs a request for each
branch, and --no-sources and --no-consumers skip them.

Name a Project (its config name, such as scc-operator, or its owner/name
slug) to look at it, else the repository you are in is used (. says so
explicitly). The tags and heads are read from the host, so they are current
even when your clone is not. A public repository needs no token. The commit
counts come from the local clone and are left out when it is missing or lacks
the commits (fetch to get them).

The report is printed before the command exits 1 when the host could not be
read. For config and the clone alone, with no network, use repo info.`,
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
			out, err := repostatus.Run(ctx, settingsFrom(cmd), cwd, args, repostatus.Options{NoConsumers: noConsumers, NoSources: noSources})
			if out != "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), out)
			}
			return err
		},
	}
	statusCmd.Flags().BoolVar(&noConsumers, "no-consumers", false, "leave out who has picked up what this project ships")
	statusCmd.Flags().BoolVar(&noSources, "no-sources", false, "leave out what this project takes from the projects that ship to it")
	var switchTo bool
	branchCmd := &cobra.Command{
		Use:   "branch [project] <line> <name>",
		Short: "Make a branch off the upstream branch of a release line",
		Long: `Make the branch <name> at the head of the upstream branch of a release
line, after fetching it, in your clone of the project. The line is one of the
Project's release lines (v2.15), or a branch of it (release/v2.15, main).

Your checkout stays where it is, as git branch does, and the command says how
to switch to the new branch. --switch moves you onto it, with git's own rules
for uncommitted changes. A branch of the name that already holds commits of
ours on the current head is kept as it is; one with commits that are not ours
is never touched. With --dry-run nothing is left behind.

Name a Project (its config name or its owner/name slug) before the line, else
the repository you are in is used.`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			project, line, name := repobranch.ParseArgs(args)
			return repobranch.Run(ctx, cmd.OutOrStdout(), settingsFrom(cmd), cwd, repobranch.Options{
				Project: project, Line: line, Name: name, Switch: switchTo,
			})
		},
	}
	branchCmd.Flags().BoolVarP(&switchTo, "switch", "s", false, "leave your checkout on the new branch")
	var cloneAll bool
	var cloneInto string
	cloneCmd := &cobra.Command{
		Use:   "clone [project...]",
		Short: "Clone the team's projects onto this machine",
		Long: `Clone each named Project, or with --all every Project your active team (team, GERO_TEAM) owns and that is not
external (an external one needs its name), into <root>/<name>, where <name> is
the repository's name. A project that is already cloned, wherever the user file
says it is or a repos.search directory finds it, is skipped.

The root is --into, else GERO_REPOS_CLONE_ROOT, else repos.cloneRoot in the user
file. A repos.paths entry for a project says where that one goes instead, and
needs no root. gero init asks for the root.

Clones are made over SSH, with your own git, so your SSH agent and keys are
used. When the user file has a fork remote and githubUser and the fork exists,
it is cloned as the fork remote and the upstream is added; when the fork does not
exist the upstream alone is cloned, under the upstream remote's name, and the
report says so. One project failing does not stop the others, and the exit is 1.

The projects come from the central config in centralConfigDir (or ./config).
A workstation with none needs it cloned first: the central bundle reader is not
built yet. --dry-run says what would be cloned and reads the host for forks.`,
		Example: `  # Everything the active team owns, under the configured root
  gero repo clone --all

  # Two projects, somewhere else
  gero repo clone --into ~/src rancher scc-operator`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			return repoclone.Run(ctx, cmd.OutOrStdout(), settingsFrom(cmd), cwd, repoclone.Options{
				Projects: args, All: cloneAll, Into: cloneInto,
			})
		},
	}
	cloneCmd.Flags().BoolVarP(&cloneAll, "all", "a", false, "clone every project the active team owns, and that is not external")
	cloneCmd.Flags().StringVarP(&cloneInto, "into", "i", "", "the directory to clone into, instead of repos.cloneRoot (env GERO_REPOS_CLONE_ROOT)")
	var push, yes, nonInteractive, stable, minor, major, retag bool
	tagCmd := &cobra.Command{
		Use:   "tag [project] <line> [tag]",
		Short: "Tag the head of the upstream branch of a release line",
		Long: `Make the tag <tag> in your clone, on the head of the upstream branch of a
release line as the host has it now (fetched, so not on whatever the clone last
saw). The line is one of the Project's release lines (v2.15, or 2.15), or a
branch of it (release/v2.15, main). The tag is a lightweight one, a local tag
that you push yourself, which the command says how to do, so the host sees you
pushing it and a workflow that runs on tags starts as you. --push sends it to
the upstream for you, with your own git credentials, after asking once.

The tag is checked first: it must be a version that matches the Project's tag
pattern, is in the line's series (0.8.x for a line whose versions are 0.8.x), and
is newer than the line's newest tag. A tag that the host has on that commit is
reported as done, one that is only in your clone is not made again, and a tag
that is on another commit is an error: a tag is never moved. Making a tag in
your clone asks nothing, as it is undone with git tag -d; --push asks, and
--yes and GERO_NON_INTERACTIVE skip the question. --dry-run says what would be
done and changes nothing.

With two arguments, a version second is the line and the tag, and anything else
is the Project and the line.

Without a tag, one is chosen from the newest tag of the line's series. After a
stable tag it is the first prerelease of the next patch version (v0.8.4-rc.1),
or of the next minor or major version with --minor or --major; after a
prerelease it is the next one of the same version (rc.2). --stable asks for a
stable tag instead: the promotion of the newest prerelease, which is only made
while the branch is still on the commit that prerelease is on, and after a
stable tag the next stable one, unless the Project requires a prerelease before
every stable tag. A promotion says which prerelease it is.

--retag makes a tag on a commit that already has the line's newest tag, for a
Project whose stream sets allowRetag (an image that takes in new packages when it
is built, so a fix is a rebuild of the same code under a new version). The version
is chosen as above, or named, and must still be newer than the line's newest tag:
a tag is never moved. Without --retag a commit that has the newest tag gets no
new one.`,
		Args: cobra.RangeArgs(1, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			s := settingsFrom(cmd)
			project, line, tag := repotag.ParseArgs(args)
			bump := repotag.BumpPatch
			switch {
			case minor:
				bump = repotag.BumpMinor
			case major:
				bump = repotag.BumpMajor
			}
			return repotag.Run(ctx, cmd.OutOrStdout(), s, cwd, repotag.Options{
				Project: project, Line: line, Tag: tag, Stable: stable, Bump: bump, Retag: retag, Push: push, Yes: s.NonInteractive,
				In: cmd.InOrStdin(), Interactive: termfacts.IsTerminal(os.Stdin),
			})
		},
	}
	tagCmd.Flags().BoolVar(&push, "push", false, "send the tag to the upstream as well, after asking once")
	tagCmd.Flags().BoolVarP(&yes, "yes", "y", false, "go on without asking before the tag is pushed")
	tagCmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "the same as --yes: never ask")
	tagCmd.Flags().BoolVar(&stable, "stable", false, "choose a stable tag: the promotion of the newest prerelease, or the next stable one")
	tagCmd.Flags().BoolVar(&minor, "minor", false, "choose the first prerelease of the next minor version, after a stable tag")
	tagCmd.Flags().BoolVar(&major, "major", false, "choose the first prerelease of the next major version, after a stable tag")
	tagCmd.MarkFlagsMutuallyExclusive("minor", "major")
	tagCmd.Flags().BoolVar(&retag, "retag", false, "allow a new tag on a commit that already has the line's newest tag, for a stream that sets allowRetag")
	imagesCmd := &cobra.Command{
		Use:   "images",
		Short: "Check the images a repository publishes",
	}
	imagesCmd.AddCommand(&cobra.Command{
		Use:   "exist [name|owner/name] <tag>",
		Short: "Check that the images of a release are published where they must be",
		Long: `Check that each image a Project publishes exists in the registries a release
of the tag must be in: Docker Hub and Prime staging always, and Prime
production only for a stable tag. A port of validate_image_exists.

The images are the image artifacts of the Project's streams in the central
config. Name a Project (its config name or its owner/name slug, or . for the
repository you are in) before the tag; with only a tag, the repository you
are in is used. The registries are read
anonymously. The report is printed before the command exits 1 when an image
is not found or a registry could not be reached.`,
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
			project, tag := "", args[0]
			if len(args) == 2 {
				project, tag = args[0], args[1]
			}
			out, err := imagesexist.Run(ctx, settingsFrom(cmd), cwd, project, tag)
			if out != "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), out)
			}
			return err
		},
	})
	repoCmd.AddCommand(imagesCmd)
	repoCmd.AddCommand(syncCommand())
	repoCmd.AddCommand(backportCommand())
	repoCmd.AddCommand(
		infoCmd,
		listCmd,
		statusCmd,
		branchCmd,
		cloneCmd,
		tagCmd,
		&cobra.Command{
			Use:     "build-tags",
			Aliases: []string{"current-tags"}, // its first name, which a workflow may still use
			Short:   "Name the tags of a build of this branch's head, and the previous one",
			Long: `Print the tags for a build of the head of this branch, for refs/heads/main and
refs/heads/release/<version>: the rolling tag of the branch (branch_tag), the
fixed tag for this commit (branch_static_tag), and the previous fixed tag
(prev_static_tag), which a release tool such as goreleaser can use as the start
of its changelog when it cannot work out the right one itself.

The ref and commit come from GITHUB_REF and GITHUB_SHA when both are set,
else from the git repository in the current directory.`,
			// Faithful to the script: errors are printed to stdout, once.
			SilenceErrors: true,
			RunE: func(cmd *cobra.Command, _ []string) error {
				res, err := branchtags.Resolve(settingsFrom(cmd), os.Getenv, ".")
				if err != nil {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), err)
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), branchtags.Render(res))
				return err
			},
		},
	)
	root.AddCommand(repoCmd)
}

// syncCommand is gero repo sync: bring what a project holds in line with config.
func syncCommand() *cobra.Command {
	var in reposync.Options
	cmd := &cobra.Command{
		Use:   "sync [project]",
		Short: "Bring what a project holds in line with config, one pull request for each branch",
		Long: `Check what a project holds against what config says it should, and open a pull
request for each of its branches that has something to correct, with every
correction of the branch in it.

Where gero ship pushes a release down to the projects that hold it, sync pulls: it
reads the project's own files and brings them in line. Today that is its series
holdings, files that name a series and no release (prometheus-federator's
kuberlrVersion: v9), which another tool reads to pick the newest release of it. A
holding is written only when an Edit follows it: nothing is written until an Edit says
what a write of it takes, so one with none is reported and left alone. gero repo status
says where a holding differs.

<project> is the project whose files are checked, by its config name or owner/name
slug; leave it out for the project whose repository you are in. A pull request has
one base branch, so a project with a branch for each Rancher line gets one for each
branch that differs, and a branch with several corrections gets them all in one:
the write of every holding, then each step of the Edits that follow them, run once.
--holding limits the run to the holdings named and --line to the Rancher lines.

It works as gero ship does: in your own clone, on a new branch (or a worktree when the
clone is dirty); it asks once before it pushes the branches and opens the PRs, and --yes
goes on without asking. A later sync that finds other corrections closes your own
earlier pull request for the branch. --no-push stops after the commits and --dry-run
does everything but write.`,
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
			return reposync.Run(ctx, cmd.OutOrStdout(), log, s, cwd, in)
		},
	}
	flags := cmd.Flags()
	flags.StringSliceVarP(&in.Holdings, "holding", "H", nil, "sync only this holding, by name; repeat it or separate names with commas for several")
	flags.StringSliceVarP(&in.Lines, "line", "l", nil, "sync only this Rancher line (v2.15); repeat it or separate lines with commas for several")
	flags.BoolVarP(&in.Preview, "preview", "p", false, "print the commit message and the PR body too")
	flags.StringVarP(&in.PushTo, "push-to", "T", "", "push the branches to the fork or the upstream, instead of what your settings say")
	flags.BoolVarP(&in.Yes, "yes", "y", false, "go on without asking before pushing branches and opening PRs")
	flags.BoolVar(&in.Yes, "non-interactive", false, "the same as --yes: never ask")
	flags.BoolVar(&in.NoPush, "no-push", false, "commit on a branch in your clone for review, but do not push or open a PR")
	return cmd
}

// backportCommand is gero repo backport: carry a merged PR down to release branches.
func backportCommand() *cobra.Command {
	var in clibackport.Options
	cmd := &cobra.Command{
		Use:   "backport [project] <line>[,<line>...] <pr>[,<pr>...]",
		Short: "Carry merged pull requests down to release lines, one pull request for each line",
		Long: `Cherry-pick what merged pull requests put on their base branch onto one or more
release branches of the same repo, and open one pull request for each line.

<pr> is the last argument: the number, #number, or URL of a PR, which also names
the repo, so the project may be left out. Several, separated by commas, are a
bundle: they go to each line together, in one pull request titled "Backport of:
#12, #15" whose body links each by its title. They are picked in the order they
were applied to the base, whatever order you typed, and all must be merged to the
same base. A PR that a line already has, whether someone cherry-picked it by hand or
gero did (found by its "cherry picked from" line, or by its patch), is left out of
that line's pull request and named in its body; one left is a regular backport, and
none is reported as already backported. An open backport of yours that the new one
covers, every PR of it among the new one's, is closed once the new one is open;
another that overlaps is only named, and anyone else's is never touched. Every PR
must be merged, which is checked on GitHub. <line> is a Rancher line (v2.15 or 2.15) or a release branch of the
project, as gero repo tag takes it; several are separated by commas. [project] is
the config name or owner/name slug, and by default the repo you are in. The lines
are worked newest first, and a line that is the branch the PR was merged to is
refused.

Which commits are picked follows how the PR landed, read from the merge commit on
GitHub: one commit, or a squash merge, picks the merged commit; a rebase merge
picks each of its commits, oldest first; a merge commit is picked against its first
parent, as one commit. What fits none of these is an error and not a guess. Every
pick is a git cherry-pick -x, so its message names the commit it came from, and the
author of each commit is kept. A target whose history already holds the commits is
reported as already backported and gets no PR.

When a pick conflicts, gero stops everything: nothing else runs until you have
resolved it or abandoned it. At a terminal it prints where the files are, opens
your editor on that directory (it asks which, the first time, and remembers), and
waits for you to answer: continue, once the conflicts are resolved (gero checks
that no conflict marker is left, finishes the pick, and notes the files you
resolved in the PR body), or abort, which abandons the pick, leaves your clone as
it was, ends the run, and leaves the lines not yet tried as not attempted. With no
terminal a conflict is abandoned and the run fails, naming the files. --dry-run
never waits: it reports a conflict and goes on to the next line.

It works as gero repo sync does: in your own clone, on a new branch (or a worktree
when the clone is dirty); it asks once before it pushes the branches and opens the
PRs, and --yes goes on without asking. Running it again is safe: your open PR for
the same commits is found, and one for other commits is closed once the new one is
open. --no-push stops after the commits and --dry-run does everything but write.`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed, err := clibackport.ParseArgs(args)
			if err != nil {
				return err
			}
			in.Args = parsed
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			s := settingsFrom(cmd)
			in.Yes = s.NonInteractive
			log := logging.New(cmd.ErrOrStderr(), s.LogLevel, s.CI)
			in.In, in.Interactive = cmd.InOrStdin(), termfacts.IsTerminal(os.Stdin)
			return clibackport.Run(ctx, cmd.OutOrStdout(), log, s, cwd, in)
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&in.Preview, "preview", "p", false, "print the PR bodies too")
	flags.StringVarP(&in.PushTo, "push-to", "T", "", "push the branches to the fork or the upstream, instead of what your settings say")
	flags.BoolVarP(&in.Yes, "yes", "y", false, "go on without asking before pushing branches and opening PRs")
	flags.BoolVar(&in.Yes, "non-interactive", false, "the same as --yes: never ask")
	flags.BoolVar(&in.NoPush, "no-push", false, "commit on a branch in your clone for review, but do not push or open a PR")
	return cmd
}

// branchOptions is what gero repo branch runs with: the project, line and name
// from its arguments, and whether to leave the checkout on the new branch.
func branchOptions(args []string, noSwitch bool) repobranch.Options {
	project, line, name := repobranch.ParseArgs(args)
	return repobranch.Options{Project: project, Line: line, Name: name}
}
