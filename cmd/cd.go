package cmd

import (
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/cdpath"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "cd <name|owner/name>",
		Short: "Print the directory of a project's local clone, for a shell function to cd into",
		Long: `Print the local clone of a project, found only by your own config: a
repos.paths entry, else a clone under a repos.search directory (matched by
remote URL). The repository you are in plays no part, so the project is always
named, by its config name or owner/name slug.

A program cannot change its parent shell's directory, so this only prints the
path. Wrap it in a shell function to move there:

  gcd() { local d; d=$(gero cd "$@") && cd "$d"; }

The command fails when the project is not in the central config, or has no
clone, or several equally good ones.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			out, err := cdpath.Path(settingsFrom(cmd), cwd, args)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			return err
		},
	})
}
