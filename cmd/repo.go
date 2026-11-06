package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/branchtags"
	"github.com/rancher/orbs-automation/internal/cli/imagesexist"
	"github.com/rancher/orbs-automation/internal/cli/repobranch"
	"github.com/rancher/orbs-automation/internal/cli/repoinfo"
	"github.com/rancher/orbs-automation/internal/cli/repostatus"
	"github.com/rancher/orbs-automation/internal/cli/repotag"
	"github.com/spf13/cobra"
)

func init() {
	repoCmd := &cobra.Command{
		Use:   "repo",
		Short: "Inspect version and tag facts for a repository",
	}
	infoCmd := &cobra.Command{
		Use:   "info [name|owner/name]",
		Short: "Show the config entry and local clone of a repository, without the network",
		Long: `Print what is known about a repository without reading the network: its
central-config entry (owned, default branch, release policy and lines), and
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
	statusCmd := &cobra.Command{
		Use:   "status [name|owner/name]",
		Short: "Show where the releases of a repository stand, as the host has them",
		Long: `Print where the releases of a repository stand: for each release line (or
the one release branch of a Project without lines), the commit at the head of
the branch on the host, its latest stable and pre-release tags, the commits on
the branch since the latest tag, and whether the local clone's branch is in
sync, behind or ahead.

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
			out, err := repostatus.Run(ctx, settingsFrom(cmd), cwd, args)
			if out != "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), out)
			}
			return err
		},
	}
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
	branchCmd.Flags().BoolVar(&switchTo, "switch", false, "leave your checkout on the new branch")
	var yes, nonInteractive, stable, minor, major bool
	tagCmd := &cobra.Command{
		Use:   "tag [project] <line> [tag]",
		Short: "Tag the head of the upstream branch of a release line",
		Long: `Make the tag <tag> on the head of the upstream branch of a release line,
as the host has it now, and not on anything in your clone. The line is one of
the Project's release lines (v2.15, or 2.15), or a branch of it (release/v2.15,
main). The tag is made on GitHub, as a lightweight tag, so it needs a token that
may write to the repository and, for the release workflow to start, that is a
person's or an App's and not the workflow's default token.

The tag is checked first: it must be a version that matches the Project's tag
pattern, is in the line's series (0.8.x for a line whose versions are 0.8.x), and
is newer than the line's newest tag. A tag that is already on that commit is
reported as done, and a tag that is on another commit is an error: a tag is never
moved. It asks once before it makes the tag; --yes and GERO_NON_INTERACTIVE skip
the question, and --dry-run says what would be tagged without a token.

With two arguments, a version second is the line and the tag, and anything else
is the Project and the line.

Without a tag, one is chosen from the newest tag of the line's series. After a
stable tag it is the first prerelease of the next patch version (v0.8.4-rc.1),
or of the next minor or major version with --minor or --major; after a
prerelease it is the next one of the same version (rc.2). --stable asks for a
stable tag instead: the promotion of the newest prerelease, which is only made
while the branch is still on the commit that prerelease is on, and after a
stable tag the next stable one, unless the Project requires a prerelease before
every stable tag. A promotion always asks, and says which prerelease it is.`,
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
				Project: project, Line: line, Tag: tag, Stable: stable, Bump: bump, Yes: s.NonInteractive,
				In: cmd.InOrStdin(), Interactive: isTerminal(os.Stdin),
			})
		},
	}
	tagCmd.Flags().BoolVarP(&yes, "yes", "y", false, "go on without asking before the tag is made")
	tagCmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "the same as --yes: never ask")
	tagCmd.Flags().BoolVar(&stable, "stable", false, "choose a stable tag: the promotion of the newest prerelease, or the next stable one")
	tagCmd.Flags().BoolVar(&minor, "minor", false, "choose the first prerelease of the next minor version, after a stable tag")
	tagCmd.Flags().BoolVar(&major, "major", false, "choose the first prerelease of the next major version, after a stable tag")
	tagCmd.MarkFlagsMutuallyExclusive("minor", "major")
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

The images are the Project's images config, or the one named for its
repository. Name a Project (its config name or its owner/name slug, or . for the
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
	repoCmd.AddCommand(
		infoCmd,
		statusCmd,
		branchCmd,
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
	rootCmd.AddCommand(repoCmd)
}
