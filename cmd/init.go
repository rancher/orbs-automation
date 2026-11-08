package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/initcmd"
	"github.com/rancher/orbs-automation/internal/cli/repl"
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
the names of your fork and upstream remotes, the prefix of your branches, where
the central config is, and where your clones are and go.

Each value is asked for on a terminal unless its flag gives it (--github-user,
--fork, --upstream, --branch-prefix, --config-dir, --search, --clone-root). An
empty answer skips the question and leaves its key out, so nothing is ever
filled in for you. Without a terminal nothing is asked, and a value with no flag
is skipped.

Two of the keys are alike and not the same. repos.search (--search) lists the
directories gero scans for clones you already have: it only finds what is there.
repos.cloneRoot (--clone-root) is the one directory gero repo clone will put new
clones in, as <root>/<name>. A directory you search need not be where new clones
go, and the clone root is not added to the search list for you.

An existing file is not touched unless --force replaces it. --dry-run shows the
file and writes nothing. The file is at $GERO_CONFIG, else
$XDG_CONFIG_HOME/gero/config.yaml, else ~/.config/gero/config.yaml. Its first
line points an editor at the JSON Schema of the file.`,
		Args: cobra.NoArgs,
		// The file to write may be the one that cannot be loaded, so this loads
		// without it when it has to, as gero doctor does.
		PersistentPreRunE: loadSettingsForDoctor(os.Getenv, home),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if os.Getenv(repl.EnvRepl) != "" {
				return errors.New("gero init asks questions and writes a file, which the REPL is not for: exit the REPL, or run it in your shell")
			}
			// --upstream is the global flag, which names the same remote.
			if f := cmd.Flags().Lookup("upstream"); f != nil && f.Changed {
				o.Upstream = f.Value.String()
			}
			if f := cmd.Flags().Lookup("config-dir"); f != nil && f.Changed {
				o.ConfigDir = f.Value.String()
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
	f.StringSliceVar(&o.Search, "search", nil, "directories to scan for clones you already have, repos.search (comma-separated or repeated)")
	f.StringVar(&o.CloneRoot, "clone-root", "", "the one directory gero repo clone puts new clones in, repos.cloneRoot")
	f.BoolVar(&o.Force, "force", false, "replace the user file if it exists")
	root.AddCommand(cmd)
}
