package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/rancher/orbs-automation/internal/cli/cdpath"
	"github.com/rancher/orbs-automation/internal/cli/repl"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
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
				getenv: os.Getenv,
				setenv: os.Setenv,
			})
		},
	})
}

// replEnv is what runRepl takes from the process, so a test can give it others.
type replEnv struct {
	getenv func(string) string
	setenv func(key, value string) error
}

// runRepl checks that the REPL can start here, then runs it on the terminal of
// cmd until it ends.
func runRepl(cmd *cobra.Command, env replEnv) error {
	s := settingsFrom(cmd)
	if err := repl.Refuse(env.getenv, s.Terminal, s.CI); err != nil {
		return err
	}
	// A template that does not parse is an error now, once, naming its key: a
	// prompt must not fail a running session.
	tmpl, err := repl.PromptTemplate(s)
	if err != nil {
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
	home, _ := os.UserHomeDir()
	sess := &repl.Session{
		Template: tmpl, ProjectOf: repl.ProjectOf(s), Home: home,
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
	sess.UseTerminal(s.Terminal)
	history := clicfg.StatePath(env.getenv, home, "repl_history")
	reader, err := repl.NewReader(cmd.InOrStdin(), cmd.OutOrStdout(), history, replCompletion(s, sess).Complete)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	return repl.Loop(cmd.Context(), reader, cmd.OutOrStdout(), cmd.ErrOrStderr(), sess)
}

// globalFlags are the names addGlobalFlags declares, the only flags that can be
// given to gero repl and carried to its lines.
var globalFlags = []string{"central-config-dir", "config-source", "bundle-channel", "bundle-file", "upstream", "output", "json", "color", "no-color", "dry-run", "verbose"}

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

// treeComplete is what the command tree completes after words, for the word
// being typed: cobra's own answer, the one gero shell install completes from in
// a shell, taken from a tree built for the question.
func treeComplete(words []string, toComplete string) []string {
	var out bytes.Buffer
	root := newRootCmd()
	root.SilenceErrors = true
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs(append(append([]string{cobra.ShellCompRequestCmd}, words...), toComplete))
	if err := root.Execute(); err != nil {
		return nil
	}
	var candidates []string
	for _, line := range strings.Split(out.String(), "\n") {
		if line == "" || strings.HasPrefix(line, ":") { // the last line is ":" and the directive
			continue
		}
		word, _, _ := strings.Cut(line, "\t") // cobra adds a description after a tab
		candidates = append(candidates, word)
	}
	return candidates
}

// replCompletion is the completion of a line typed at the REPL of sess: the
// tree, the REPL's own commands, the projects of the config and the directories
// of wherever the session is.
func replCompletion(s clicfg.Settings, sess *repl.Session) repl.Completion {
	return repl.Completion{
		Tree:     treeComplete,
		Projects: func() []string { return repl.ProjectNames(s, sess.Dir) },
		Dir:      func() string { return sess.Dir },
	}
}
