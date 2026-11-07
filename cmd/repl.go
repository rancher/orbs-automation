package cmd

import (
	"context"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/rancher/orbs-automation/internal/cli/cdpath"
	"github.com/rancher/orbs-automation/internal/cli/repl"
	"github.com/spf13/cobra"
)

// addRepl adds gero repl to root.
func addRepl(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:   "repl",
		Short: "Run gero commands one line at a time at a prompt",
		Long: `Run gero commands one line at a time at a prompt, with pwd, ls, cd, shell and
exit of its own. A line is a command typed without the leading gero. The global flags given
here, such as --dry-run, apply to every line.

It needs a terminal and is not offered in CI. It is started from your shell and
exiting returns to it.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRepl(cmd, replEnv{
				tty:    isTerminal(os.Stdin) && isTerminal(os.Stdout),
				getenv: os.Getenv,
				setenv: os.Setenv,
			})
		},
	})
}

// replEnv is what runRepl takes from the process, so a test can give it others.
type replEnv struct {
	tty    bool
	getenv func(string) string
	setenv func(key, value string) error
}

// runRepl checks that the REPL can start here, then runs it on the terminal of
// cmd until it ends.
func runRepl(cmd *cobra.Command, env replEnv) error {
	s := settingsFrom(cmd)
	if err := repl.Refuse(env.getenv, env.tty, s.CI); err != nil {
		return err
	}
	// A gero repl typed at this prompt is refused like one in a child shell.
	if err := env.setenv(repl.EnvRepl, "1"); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	sess := &repl.Session{
		Dir: cwd, Env: os.Environ(),
		StartShell: repl.ShellOn(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()),
	}
	sess.ProjectDir = func(key string) (string, error) {
		out, err := cdpath.Path(s, sess.Dir, []string{key})
		return strings.TrimSuffix(out, "\n"), err
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	defer func() {
		signal.Stop(sigs)
		close(sigs)
	}()
	sess.Run = repl.Interruptible(sigs, lineRunner(baseFlags(cmd), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()))
	return repl.Loop(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), sess)
}

// globalFlags are the names addGlobalFlags declares, the only flags that can be
// given to gero repl and carried to its lines.
var globalFlags = []string{"config-dir", "upstream", "output", "json", "dry-run", "verbose"}

// baseFlags returns the global flags given explicitly to cmd, as arguments to
// put in front of every line.
func baseFlags(cmd *cobra.Command) []string {
	var args []string
	for _, name := range globalFlags {
		if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
			args = append(args, "--"+name+"="+f.Value.String())
		}
	}
	return args
}

// lineRunner returns the Runner of the REPL: each line is run on a command tree
// built for it, with base, the global flags given to gero repl, in front of the
// words of the line. The tree reads in and writes to out and errOut. It does
// not print the error of a line: the loop does, once. The commands read the
// working directory from the process, which only the REPL's cd moves.
func lineRunner(base []string, in io.Reader, out, errOut io.Writer) repl.Runner {
	return func(ctx context.Context, _ string, args []string) error {
		root := newRootCmd()
		root.SilenceErrors = true
		root.SetIn(in)
		root.SetOut(out)
		root.SetErr(errOut)
		root.SetArgs(append(append([]string{}, base...), args...))
		return root.ExecuteContext(ctx)
	}
}
