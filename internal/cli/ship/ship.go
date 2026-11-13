// Package ship holds the logic of gero ship: shipping a release of a producer
// to one consumer repository. For now it runs the state check and writes
// nothing; the write steps follow (docs/specs/PUSH_TO_REPO.md). It is
// framework-free: cmd/ owns the cobra wiring.
package ship

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"slices"

	"github.com/rancher/orbs-automation/internal/platform/goproxy"
	"github.com/rancher/orbs-automation/internal/platform/goreleases"
	"github.com/rancher/orbs-automation/internal/platform/ociregistry"

	"github.com/rancher/orbs-automation/internal/cli/identity"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/github"
	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/steprun"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// ErrBlocked is returned, with the report still written, when the state check
// finds something in the way, such as an image that is not published.
var ErrBlocked = errors.New("the release cannot be shipped yet")

// Options is what the user asked to ship; the command fills it from its
// arguments and flags.
type Options struct {
	Project    string   // the producer, by config name or slug
	Arg        string   // a tag, a release line of the producer, or empty for the latest
	To         string   // the consumer, when the producer ships to several
	Lines      []string // the consumer's Rancher lines to update
	Prerelease bool     // let prereleases count
	Preview    bool     // print the commit message and PR body too, not only the branch and title
	// AllowDowngrade ships a release older than what the consumer holds.
	AllowDowngrade bool
	// NoPush stops after the commit, leaving the branch in the clone for review.
	NoPush bool
	// PushTo is --push-to: fork or upstream, to override the settings.
	PushTo string
	// Refresh is --refresh: rebuild an open PR of yours and replace its branch if
	// it differs.
	Refresh bool
	// Yes is --yes: go on without asking. Interactive says In is a terminal that
	// the question can be asked on; without it and without Yes a run that writes
	// is an error.
	Yes         bool
	Interactive bool
	In          io.Reader
}

// ParseArgs reads the positional arguments of gero ship: [project] [tag|line].
// A single argument is a project unless it looks like a release, a version
// such as v0.5.3 or a line such as v2.15 (no project is named like that), in
// which case the project is the one the working directory is in and the
// argument is the release. No arguments is that project's latest release.
func ParseArgs(args []string) (project, arg string) {
	switch len(args) {
	case 0:
		return "", ""
	case 1:
		if looksLikeARelease(args[0]) {
			return "", args[0]
		}
		return args[0], ""
	}
	return args[0], args[1]
}

// release matches a version or a line: v0.5.3, 0.6.0-rc.1, v2.15.
var release = regexp.MustCompile(`^v?[0-9]+(\.[0-9]+){1,2}([-+].*)?$`)

func looksLikeARelease(s string) bool { return release.MatchString(s) }

// Run ships what opts names: it checks the state, makes the commits and, unless
// --no-push or --dry-run, pushes them and opens the PRs after asking once. The
// report is written to w as it goes in text and as one document at the end in
// json, and it is written even when the error returned is ErrBlocked, ErrFailed
// or ErrDeclined.
func Run(ctx context.Context, w io.Writer, log *slog.Logger, s clicfg.Settings, cwd string, opts Options) error {
	if err := checkFormat(s.Output); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "gero-ship-")
	if err != nil {
		return fmt.Errorf("make a directory for worktrees: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	ex := &domain.Executor{Git: domain.GitCLI{}, Runner: steprun.Runner{}, TempDir: tmp, Sign: s.SignCommits, Identify: identify(s)}
	if !opts.NoPush {
		creds, err := github.Resolve(github.GoGh{}, "")
		switch {
		case err == nil:
			ctx = pushContext(ctx, s, creds)
			remote, err := newRemote(creds, s)
			if err != nil {
				return err
			}
			ex.Publish = &domain.Publisher{Git: domain.GitCLI{}, Remote: remote}
			ex.Confirm = confirmer(opts.In, w, opts.Interactive, opts.Yes, style.New(s.Terminal))
		case s.DryRun && errors.Is(err, github.ErrNoToken):
			// A dry run can still say what it would commit; it just cannot look at
			// the open PRs without a token.
			if log != nil {
				log.Warn("no GitHub token, so the open PRs were not looked at", "error", err)
			}
		default:
			return fmt.Errorf("opening PRs needs a GitHub token (--no-push does not): %w", err)
		}
	}
	return runToStyled(ctx, w, log, shipWorkspace{workspace.Open(s, cwd)}, probers(), ex, s.Output, inputFrom(s, opts), opts.Preview, style.New(s.Terminal))
}

// probers is what a run looks things up with: the registries for images, the proxy
// for Go modules, and the list of Go releases for a go.mod that pins no more than
// go 1.26, kept in the cache directory so it is read once a day.
func probers() registry.Probers {
	home, _ := os.UserHomeDir()
	return registry.Probers{
		Images: ociregistry.Checker{}, Modules: goproxy.Checker{},
		Go: goreleases.Client{CacheDir: clicfg.CacheDir(os.Getenv, home)},
	}
}

// inputFrom is what the user asked for, from the flags and the settings. The
// actor is the bot when the process runs in CI.
func inputFrom(s clicfg.Settings, opts Options) domain.Input {
	return domain.Input{
		Project: opts.Project, Arg: opts.Arg, To: opts.To, Lines: opts.Lines, Prerelease: opts.Prerelease,
		BranchPrefix: s.BranchPrefix, AllowDowngrade: opts.AllowDowngrade, DryRun: s.DryRun, NoPush: opts.NoPush,
		PushTo: opts.PushTo, CI: s.CI, Refresh: opts.Refresh, Team: s.Team,
	}
}

func checkFormat(format string) error {
	if format != clicfg.OutputText && format != clicfg.OutputJSON {
		return fmt.Errorf("output format %q is not supported by ship: use text or json", format)
	}
	return nil
}

// runWith is runToStyled with no color, with the report returned as a string.
func runWith(ctx context.Context, ws domain.Workspace, prober registry.Prober, ex *domain.Executor, format string, in domain.Input) (string, error) {
	var buf bytes.Buffer
	err := runToStyled(ctx, &buf, nil, ws, prober, ex, format, in, false, style.Plain())
	return buf.String(), err
}

// runToStyled checks the state and, with an executor, makes the commits; nil is
// a check only. The events of the run are also logged to log, when it is not
// nil, whatever the format, so a JSON handler on stderr gives machine-readable
// progress beside either report. The report is written to w as the run goes in
// text, styled by st, and as one document at the end in json, which is never
// styled. Errors that come after the check (no clone, a failed target, a
// blocked release) are returned after the report is written.
func runToStyled(ctx context.Context, w io.Writer, log *slog.Logger, ws domain.Workspace, prober registry.Prober, ex *domain.Executor, format string, in domain.Input, full bool, st *style.Renderer) error {
	if err := checkFormat(format); err != nil {
		return err
	}
	var (
		stream *ui.Stream
		fan    sinks
	)
	if format == clicfg.OutputText {
		stream = ui.NewStream(w, ui.WithWidth(st.Width))
		fan = append(fan, newStyledSink(stream, full, st))
	}
	if log != nil {
		fan = append(fan, logSink{log})
	}
	plan, err := domain.Ship(ctx, ws, prober, in, ex, fan)
	if stream != nil {
		stream.Flush()
	}
	if errors.Is(err, workspace.ErrNotProject) {
		return fmt.Errorf("%w: name the project to ship as the first argument", err)
	}
	if err != nil && plan.Producer == "" {
		return err // the check never made a plan: there is nothing to show
	}
	if format == clicfg.OutputJSON {
		out, err := renderJSON(plan)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, out); err != nil {
			return err
		}
	} else if err := stream.Err(); err != nil {
		return err
	}
	switch {
	case err != nil:
		return err // after the check: no clone, declined, no one to act as
	case plan.Status == domain.StatusBlocked:
		return ErrBlocked
	}
	return failedErr(plan)
}

// ErrFailed is returned, with the report still written, when a commit for a
// target failed.
var ErrFailed = errors.New("some targets failed")

// failedErr is ErrFailed when any target's result is a failure, else ErrBlocked
// when one is blocked by a newer PR, so the exit code says the run did not do
// everything it was asked.
func failedErr(p domain.Plan) error {
	blocked := false
	for _, t := range p.Targets {
		if t.Result == nil {
			continue
		}
		switch t.Result.Status {
		case domain.ResultFailed:
			return ErrFailed
		case domain.ResultBlocked:
			blocked = true
		}
	}
	if blocked {
		return ErrBlocked
	}
	return nil
}

func renderText(p domain.Plan, full bool) string {
	return renderTextStyled(p, full, style.Plain())
}

// renderTextStyled is renderText with the report styled by st.
func renderTextStyled(p domain.Plan, full bool, st *style.Renderer) string {
	var buf bytes.Buffer
	stream := ui.NewStream(&buf, ui.WithWidth(st.Width))
	sink := newStyledSink(stream, full, st)
	for _, e := range replay(p) {
		sink.Event(e)
	}
	stream.Flush()
	return buf.String()
}

// hasResults reports whether a run made, or tried to make, any commit.
func hasResults(p domain.Plan) bool {
	return slices.ContainsFunc(p.Targets, func(t domain.TargetPlan) bool { return t.Result != nil })
}

// group is one text and the branches it applies to.
type group struct {
	text     string
	branches []string
}

// groupTargets collects the texts of the targets (their blockers or warnings)
// and the branches each applies to, in order of first appearance, so the same
// reason for five branches is one line and not five.
func groupTargets(targets []domain.TargetPlan, texts func(domain.TargetPlan) []string) []group {
	var groups []group
	for _, t := range targets {
		for _, text := range texts(t) {
			i := slices.IndexFunc(groups, func(g group) bool { return g.text == text })
			if i < 0 {
				groups = append(groups, group{text: text})
				i = len(groups) - 1
			}
			groups[i].branches = append(groups[i].branches, t.Key())
		}
	}
	return groups
}

// previews returns the targets that have a PR to show.
func previews(p domain.Plan) []domain.TargetPlan {
	var out []domain.TargetPlan
	for _, t := range p.Targets {
		if t.Preview != nil {
			out = append(out, t)
		}
	}
	return out
}

func renderJSON(p domain.Plan) (string, error) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

// identify says who the commits are made as in the clone: the engineer's git
// identity there on a workstation, the bot's in CI.
func identify(s clicfg.Settings) func(context.Context, string) (domain.Identity, error) {
	return func(ctx context.Context, clone string) (domain.Identity, error) {
		id, err := identity.ResolveIn(ctx, s, clone)
		return domain.Identity{Name: id.Name, Email: id.Email}, err
	}
}
