package cmd

import (
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/initcmd"
	"github.com/rancher/orbs-automation/internal/cli/termfacts"
	"github.com/spf13/cobra"
)

// addInit adds gero init to root.
func addInit(root *cobra.Command) {
	home, _ := os.UserHomeDir()
	var o initcmd.Options
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create your user config file",
		Long: `Create the user file, where gero keeps your own settings: your GitHub login,
the names of your fork and upstream remotes, and the prefix of your branches.

Each value is asked for on a terminal unless its flag gives it (--github-user,
--fork, --upstream, --branch-prefix). Without a
terminal every flag is needed, as nothing can be asked. An existing file is not
touched unless --force replaces it. --dry-run shows the file and writes nothing.

The file is at $GERO_CONFIG, else $XDG_CONFIG_HOME/gero/config.yaml, else
~/.config/gero/config.yaml. Its first line points an editor at the JSON Schema
of the file.`,
		Args: cobra.NoArgs,
		// The file to write may be the one that cannot be loaded, so this loads
		// without it when it has to, as gero doctor does.
		PersistentPreRunE: loadSettingsForDoctor(os.Getenv, home),
		RunE: func(cmd *cobra.Command, _ []string) error {
			// --upstream is the global flag, which names the same remote.
			if f := cmd.Flags().Lookup("upstream"); f != nil && f.Changed {
				o.Upstream = f.Value.String()
			}
			o.Interactive = termfacts.IsTerminal(os.Stdin)
			o.DryRun = settingsFrom(cmd).DryRun
			o.In, o.Out = cmd.InOrStdin(), cmd.OutOrStdout()
			o.Terminal = settingsFrom(cmd).Terminal
			o.Getenv, o.Home = os.Getenv, home
			out, err := initcmd.Run(o)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			return err
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.GitHubUser, "github-user", "", "your GitHub login")
	f.StringVar(&o.Fork, "fork", "", "the remote that holds your fork, such as origin")
	f.StringVar(&o.BranchPrefix, "branch-prefix", "", "the prefix of your branch names, such as alice/")
	f.BoolVar(&o.Force, "force", false, "replace the user file if it exists")
	root.AddCommand(cmd)
}
