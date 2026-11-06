package cmd

import (
	"bytes"
	"fmt"
	"os"
	"runtime"

	"github.com/rancher/orbs-automation/internal/cli/shellsetup"
	"github.com/spf13/cobra"
)

func init() {
	shellCmd := &cobra.Command{
		Use:   "shell",
		Short: "Set up completion and the cd function in your shell",
	}

	initCmd := &cobra.Command{
		Use:   "init <bash|zsh|fish|powershell>",
		Short: "Print the shell script for completion and the gero function",
		Long: `Print what a shell needs to complete gero commands and to run gero cd: the
completion script, then a gero function that handles gero cd itself, since a
program cannot change its parent shell's directory, and passes everything else
to the gero binary unchanged.

gero shell install adds a line to your profile that runs this, so a newer gero
changes the script without the profile being edited again. To do it by hand:

  eval "$(gero shell init zsh)"`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			out, err := shellsetup.Init(args[0], func(shell string) (string, error) {
				var buf bytes.Buffer
				var err error
				switch shell {
				case "zsh":
					err = root.GenZshCompletion(&buf)
				case "fish":
					err = root.GenFishCompletion(&buf, true)
				case "powershell":
					err = root.GenPowerShellCompletionWithDesc(&buf)
				default:
					err = root.GenBashCompletionV2(&buf, true)
				}
				return buf.String(), err
			})
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			return err
		},
	}

	var shell, profile string
	var yes bool
	setup := func(run func(shellsetup.Options) (string, error)) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, _ []string) error {
			out, err := run(shellsetup.Options{
				Shell: shell, Profile: profile, Yes: yes,
				Interactive: isTerminal(os.Stdin), DryRun: settingsFrom(cmd).DryRun,
				In: cmd.InOrStdin(), Out: cmd.OutOrStdout(),
				Getenv: os.Getenv, GOOS: runtime.GOOS,
			})
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			return err
		}
	}
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Add completion and the gero function to your shell profile",
		Long: `Add a marked block to your shell's profile (.zshrc, or for bash .bashrc, or
.bash_profile on macOS) that sets up completion and a gero function, so that
gero cd <project> moves your shell into the project's clone. The shell is
detected from $SHELL; bash, zsh, fish and powershell (named with --shell, as it is not a login shell) are supported.

The block is shown and you are asked before anything is written. It sits
between "# >>> gero shell >>>" and "# <<< gero shell <<<" lines, and running
this again updates it in place; nothing outside those lines is touched.
Without a terminal, pass --yes to approve. --dry-run shows the change only.`,
		Args: cobra.NoArgs,
		RunE: setup(shellsetup.Install),
	}
	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove what gero shell install added to your shell profile",
		Args:  cobra.NoArgs,
		RunE:  setup(shellsetup.Uninstall),
	}
	for _, c := range []*cobra.Command{installCmd, uninstallCmd} {
		c.Flags().StringVar(&shell, "shell", "", "the shell to set up (bash or zsh); default is the one in $SHELL")
		c.Flags().StringVar(&profile, "profile", "", "the profile file to edit; default is the shell's usual one")
		c.Flags().BoolVarP(&yes, "yes", "y", false, "approve the change without asking")
	}

	shellCmd.AddCommand(initCmd, installCmd, uninstallCmd)
	rootCmd.AddCommand(shellCmd)
}

// isTerminal reports whether f is a terminal, so a question can be asked on it.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
