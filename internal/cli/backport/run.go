package backport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	domain "github.com/rancher/orbs-automation/internal/backport"
	"github.com/rancher/orbs-automation/internal/cli/proposer"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/execx"
	"github.com/rancher/orbs-automation/internal/platform/github"
	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
	"github.com/rancher/orbs-automation/internal/proposal"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Options is what the user asked to backport; the command fills it from its arguments and flags.
type Options struct {
	Args    Args
	Preview bool // print the PR bodies too
	NoPush  bool
	PushTo  string
	Yes     bool
	// Interactive says In is a terminal that questions can be asked on, a conflict included.
	Interactive bool
	In          io.Reader
}

// Run carries a merged PR to the release lines named: it checks the PR on the host, makes the
// picks for each target and, unless --no-push or --dry-run, pushes each branch and opens its PR
// after asking once. A conflict waits for the person at the terminal. The report is written to w,
// as text or as one document in json, and it is written even when the error returned is ErrFailed
// or ErrDeclined.
func Run(ctx context.Context, w io.Writer, log *slog.Logger, s clicfg.Settings, cwd string, opts Options) error {
	if err := checkFormat(s.Output); err != nil {
		return err
	}
	// The PR is read from the host whatever is pushed, so the token is needed even for --no-push.
	creds, err := github.Resolve(github.GoGh{}, "")
	if err != nil {
		return fmt.Errorf("backporting reads the PR from GitHub and needs a token: %w", err)
	}
	reader, err := pulls.New(creds)
	if err != nil {
		return err
	}
	ctx, ex, cleanup, err := proposer.Build(ctx, w, log, s, proposer.Options{NoPush: opts.NoPush, Yes: opts.Yes, Interactive: opts.Interactive, In: opts.In})
	if err != nil {
		return err
	}
	defer cleanup()
	project := opts.Args.Project
	if project == "" {
		project = opts.Args.Repo // a PR URL says which repo, so no clone to stand in is needed
	}
	in := domain.Input{
		Project: project, Lines: opts.Args.Lines, PR: opts.Args.PR, Repo: opts.Args.Repo,
		BranchPrefix: s.BranchPrefix, DefaultBranchPrefix: s.DefaultBranchPrefix(),
		DryRun: s.DryRun, NoPush: opts.NoPush, PushTo: opts.PushTo, CI: s.CI,
	}
	if opts.Interactive {
		in.Resolver = newTerminal(opts.In, w)
	}
	ws := site{Workspace: proposer.Workspace{Workspace: workspace.Open(s, cwd)}, host: reader}
	return runTo(ctx, w, ws, ex, s.Output, in, opts.Preview, style.New(s.Terminal))
}

// site is the invocation's workspace and the host the PR is read from.
type site struct {
	proposer.Workspace
	host domain.Host
}

func (s site) PullRequests() domain.Host { return s.host }

// runTo backports, with an executor, and writes the report. Errors that come after the check (no
// clone, a failed target, declined) are returned after it.
func runTo(ctx context.Context, w io.Writer, ws domain.Workspace, ex *proposal.Executor, format string, in domain.Input, full bool, st *style.Renderer) error {
	if err := checkFormat(format); err != nil {
		return err
	}
	plan, err := domain.Run(ctx, ws, in, ex)
	if errors.Is(err, workspace.ErrNotProject) {
		return fmt.Errorf("%w: name the project as the first argument, or use the URL of the PR", err)
	}
	if err != nil && len(plan.Targets) == 0 {
		return err // nothing was found, so there is no report to give
	}
	out := renderText(plan, full, st)
	if format == clicfg.OutputJSON {
		b, jerr := json.MarshalIndent(plan, "", "  ")
		if jerr != nil {
			return jerr
		}
		out = string(b) + "\n"
	}
	if _, werr := io.WriteString(w, out); werr != nil {
		return werr
	}
	if err != nil {
		return err
	}
	return failed(plan)
}

// checkFormat refuses an output format that the command does not write.
func checkFormat(format string) error {
	if format != clicfg.OutputText && format != clicfg.OutputJSON {
		return fmt.Errorf("output format %q is not supported by repo backport: use text or json", format)
	}
	return nil
}

// newTerminal asks at a conflict on the terminal, with the editor chosen the last time. What is
// chosen is kept in gero's state directory, since it is an answer and not a setting.
func newTerminal(in io.Reader, out io.Writer) *Terminal {
	home, _ := os.UserHomeDir()
	path := clicfg.StatePath(os.Getenv, home, "backport-editor")
	t := &Terminal{
		In: in, Out: out,
		Installed: execx.OnPath,
		Start:     func(_ context.Context, command, dir string) error { return execx.Launch(command, dir) },
	}
	if data, err := os.ReadFile(path); path != "" && err == nil {
		t.Editor = strings.TrimSpace(string(data))
	}
	if path != "" {
		t.Remember = func(command string) error {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			return os.WriteFile(path, []byte(command+"\n"), 0o644)
		}
	}
	return t
}
