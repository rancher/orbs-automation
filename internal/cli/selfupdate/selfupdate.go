// Package selfupdate holds the logic of gero doctor self-update: it looks for a
// newer release, says what it found, asks, and installs it. What proves the
// release is ours is internal/platform/selfupdate. It is framework-free: cmd/
// owns the cobra wiring.
package selfupdate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/github"
	platform "github.com/rancher/orbs-automation/internal/platform/selfupdate"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

// ErrNeedsApproval is returned when approval is needed and cannot be asked for:
// the input is not a terminal and --yes was not given.
var ErrNeedsApproval = errors.New("self-update replaces the gero binary and needs approval, but there is no terminal to ask on: pass --yes")

// ErrDeclined is returned when the user answers no. Nothing was changed.
var ErrDeclined = errors.New("not updated")

// Updater is what Run needs of internal/platform/selfupdate.
type Updater interface {
	Check(ctx context.Context) (platform.Status, error)
	Blocked() string
	Apply(ctx context.Context, s platform.Status) (string, error)
}

// Options are the choices of one run.
type Options struct {
	Updater Updater
	// CheckOnly says whether a newer release exists and stops. DryRun does the
	// same, naming what it would have installed.
	CheckOnly, DryRun bool
	// Yes approves without asking. Interactive says the input is a terminal.
	Yes, Interactive bool
	// In and Out are where the question is asked and answered.
	In  io.Reader
	Out io.Writer
	// Terminal is what the terminal can do, for the look of the report.
	Terminal clicfg.Terminal
}

// Run checks for a newer release and, when there is one and the user approves,
// installs it. It returns what to print.
func Run(ctx context.Context, o Options) (string, error) {
	view := style.New(o.Terminal)
	status, err := o.Updater.Check(ctx)
	if err != nil {
		return "", fmt.Errorf("checking for a newer gero: %w", err)
	}
	latest := status.Latest.Version
	if !status.Available {
		if status.Current == "" || status.Current == "dev" {
			return fmt.Sprintf("This gero is a development build, which self-update leaves alone. The latest release is %s.\n", latest), nil
		}
		return view.Style(style.OK, fmt.Sprintf("gero %s is up to date", status.Current)) +
			fmt.Sprintf(" (the latest release is %s)\n", latest), nil
	}
	found := fmt.Sprintf("%s is available (this is gero %s)", latest, status.Current)
	if status.Latest.Prerelease {
		found += ", and it is a pre-release"
	}
	found += "\n"
	switch {
	case o.DryRun:
		return found + fmt.Sprintf("Would update gero %s to %s, after checking its signatures. Nothing was changed.\n", status.Current, latest), nil
	case o.CheckOnly:
		return found, nil
	}
	if hint := o.Updater.Blocked(); hint != "" {
		return found + hint + "\n", nil
	}
	if !o.Yes {
		if !o.Interactive {
			return "", ErrNeedsApproval
		}
		question := fmt.Sprintf("Update gero %s to %s? [y/N] ", status.Current, latest)
		if status.Latest.Prerelease {
			question = fmt.Sprintf("Update gero %s to the pre-release %s? [y/N] ", status.Current, latest)
		}
		yes, err := ask(o.In, o.Out, question)
		if err != nil {
			return "", err
		}
		if !yes {
			return "", ErrDeclined
		}
	}
	installed, err := o.Updater.Apply(ctx, status)
	if err != nil {
		return "", fmt.Errorf("updating gero to %s: %w", latest, err)
	}
	return view.Style(style.OK, "Updated gero to "+installed) + "\n", nil
}

// ask writes question to out and reads one line from in. Only y or yes, in
// either case, is a yes.
func ask(in io.Reader, out io.Writer, question string) (bool, error) {
	if _, err := fmt.Fprint(out, question); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}

// Auth is where Build looks for a token. Tests replace it.
var Auth github.Auth = github.GoGh{}

// Build returns the Updater for the running gero, whose version is current. A
// non-nil prereleases says whether a pre-release may be offered; nil follows
// current. It uses the engineer's GitHub token when there is one, and goes without when
// there is none: the releases are fetched the same way either way, and a token
// only lifts the rate limit and reaches a private repository.
func Build(current string, prereleases *bool) (Updater, error) {
	var token string
	if creds, err := github.Resolve(Auth, ""); err == nil {
		token = creds.Token
	}
	return platform.New(platform.Config{Current: current, Prereleases: prereleases, Token: token})
}
