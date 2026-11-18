package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/initcmd"
	"github.com/rancher/orbs-automation/internal/cli/repl"
	"github.com/rancher/orbs-automation/internal/cli/termfacts"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/spf13/cobra"
)

// addInit adds gero init to root.
func addInit(root *cobra.Command) {
	home, _ := os.UserHomeDir()
	var o initcmd.Options
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create your user config file",
		Long: `Create the user file, where gero keeps your own settings. On a terminal it asks
for the least that makes gero work on your workstation: your GitHub login, the
names of your fork and upstream remotes, the directories where your clones are,
and the directory new clones go in.

Each is skipped when its flag gives it (--github-user, --fork, --upstream,
--search, --clone-root). The login and the search directories are asked again
when left empty. Enter on a remote takes origin for the fork and upstream for
the upstream. The clone root is strongly suggested: without it gero repo clone
needs --into on every run. Your branches are prefixed with your login and a
slash unless you set --branch-prefix, and the central config comes with the
release unless you set --central-config-dir; neither is asked. Without a
terminal nothing is asked or defaulted, and a value with no flag is skipped.

Two of the keys are alike and not the same. repos.search (--search) lists the
directories gero scans for clones you already have: it only finds what is there.
repos.cloneRoot (--clone-root) is the one directory gero repo clone will put new
clones in, as <root>/<name>. A directory you search need not be where new clones
go. The clone root is scanned too, so it need not be listed in repos.search.

An existing file is not touched unless --force replaces it. --dry-run shows the
file and writes nothing. The file is at $GERO_CONFIG, else
$XDG_CONFIG_HOME/gero/config.yaml, else ~/.config/gero/config.yaml. Its first
line points an editor at the JSON Schema of the file.

In CI it is the same file: the user file of whoever the job runs as, which is how
a workflow tells gero where its checkouts are, with the flags given and nothing
asked. CI reads a user file only when GERO_CONFIG names it, so set GERO_CONFIG to
the file for this command and for the commands that follow.`,
		Args: cobra.NoArgs,
		// The file to write may be the one that cannot be loaded, so this loads
		// without it when it has to, as gero doctor does.
		PersistentPreRunE: loadSettingsForDoctor(os.Getenv, home),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if os.Getenv(repl.EnvRepl) != "" {
				return errors.New("gero init asks questions and writes a file, which the REPL is not for: exit the REPL, or run it in your shell")
			}
			// --upstream and --central-config-dir are the global flags, which name the same
			// remote and directory. The options outlive a run in a test, so a flag not
			// given is cleared and not left from the last.
			o.Upstream, o.CentralConfigDir = "", ""
			if f := cmd.Flags().Lookup("upstream"); f != nil && f.Changed {
				o.Upstream = f.Value.String()
			}
			if f := cmd.Flags().Lookup("central-config-dir"); f != nil && f.Changed {
				o.CentralConfigDir = f.Value.String()
			}
			o.Interactive = asksQuestions(settingsFrom(cmd), termfacts.IsTerminal(os.Stdin))
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

// asksQuestions says whether gero init may ask for a value it was not given:
// only at a terminal, and never in CI or when non-interactive is set, so a job
// that gives its flags gets the same file whether or not it has a terminal.
func asksQuestions(s clicfg.Settings, terminal bool) bool {
	return terminal && !s.CI && !s.NonInteractive
}
