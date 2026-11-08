// Package initcmd holds the logic of gero init: it writes a new user file from
// a few answers, given as flags or asked on a terminal. It is framework-free:
// cmd/ owns the cobra wiring. The package is not named init, which Go reserves.
package initcmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/rancher/orbs-automation/internal/platform/atomicfile"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

// ErrExists is returned when the user file exists and Force was not given.
var ErrExists = errors.New("the user file exists")

// ErrNeedsValues is returned when a value is missing and cannot be asked for:
// the input is not a terminal.
var ErrNeedsValues = errors.New("values are missing and there is no terminal to ask on")

// Options are the choices of one run.
type Options struct {
	// The four values of the file. One left empty is asked for on a terminal, and
	// is an error without one.
	GitHubUser, Fork, Upstream, BranchPrefix string
	// Force replaces a user file that exists, even one that cannot be loaded.
	Force bool
	// Interactive says the input is a terminal, where a missing value is asked.
	Interactive bool
	// DryRun shows the file and writes nothing.
	DryRun bool
	// In and Out are where a question is asked and answered.
	In  io.Reader
	Out io.Writer
	// Terminal is what the terminal can do, for the look of the report. The zero
	// value has no color.
	Terminal clicfg.Terminal
	// Getenv and Home find the user file, as clicfg.UserFilePath takes them.
	Getenv func(string) string
	Home   string
}

// Run writes the user file and returns the report to print.
func Run(o Options) (string, error) {
	view := style.New(o.Terminal)
	path, explicit := clicfg.UserFilePath(o.Getenv, o.Home)
	ci, err := clicfg.InCI(o.Getenv)
	if err != nil {
		return "", err
	}
	if ci && !explicit {
		return "", errors.New("a user file is not read in CI unless GERO_CONFIG names it, so writing one is no use: set GERO_CONFIG to write it elsewhere")
	}
	_, statErr := os.Stat(path)
	exists := statErr == nil
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("user file: %w", statErr)
	}
	if exists && !o.Force {
		return "", fmt.Errorf("%w: %s; pass --force to replace it", ErrExists, path)
	}

	v := clicfg.UserFileValues{GitHubUser: o.GitHubUser, Fork: o.Fork, Upstream: o.Upstream, BranchPrefix: o.BranchPrefix}
	if err := fill(o, &v); err != nil {
		return "", err
	}
	for _, f := range []struct{ name, value string }{
		{"GitHub login", v.GitHubUser}, {"fork remote", v.Fork},
		{"upstream remote", v.Upstream}, {"branch prefix", v.BranchPrefix},
	} {
		if strings.IndexFunc(f.value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
			return "", fmt.Errorf("the %s %q must not contain spaces or control characters", f.name, f.value)
		}
	}

	content := clicfg.RenderUserFile(v)
	var out strings.Builder
	if o.DryRun {
		fmt.Fprintf(&out, "%s\n\n%s\n", view.Style(style.Heading, "Write to "+path+":"), content)
		out.WriteString(view.Style(style.Dim, "Dry run: nothing was written.") + "\n")
		return out.String(), nil
	}
	if err := atomicfile.Write(path, content); err != nil {
		return "", err
	}
	verb := "Created"
	if exists {
		verb = "Replaced"
	}
	fmt.Fprintf(&out, "%s %s\n", view.Style(style.OK, verb), path)
	return out.String(), nil
}

// fill gives v the values still empty: asked for on a terminal, and without one
// an error that names the flags.
func fill(o Options, v *clicfg.UserFileValues) error {
	if !o.Interactive {
		var missing []string
		for _, f := range []struct{ flag, value string }{
			{"--github-user", v.GitHubUser}, {"--fork", v.Fork},
			{"--upstream", v.Upstream}, {"--branch-prefix", v.BranchPrefix},
		} {
			if f.value == "" {
				missing = append(missing, f.flag)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("%w: pass %s", ErrNeedsValues, strings.Join(missing, ", "))
		}
		return nil
	}
	view := style.New(o.Terminal)
	in := bufio.NewReader(o.In)
	ask := func(question, def string) (string, error) {
		prompt := view.Style(style.Heading, question)
		if def != "" {
			prompt += " " + view.Style(style.Dim, "["+def+"]")
		}
		if _, err := fmt.Fprint(o.Out, prompt+" "); err != nil {
			return "", err
		}
		line, err := in.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		if answer := strings.TrimSpace(line); answer != "" {
			return answer, nil
		}
		return def, nil
	}
	var err error
	if v.GitHubUser == "" {
		if v.GitHubUser, err = ask("Your GitHub login:", ""); err != nil {
			return err
		}
		if v.GitHubUser == "" {
			return errors.New("a GitHub login is needed")
		}
	}
	if v.Fork == "" {
		if v.Fork, err = ask("Remote that holds your fork:", "origin"); err != nil {
			return err
		}
	}
	if v.Upstream == "" {
		if v.Upstream, err = ask("Remote that is the real source:", "upstream"); err != nil {
			return err
		}
	}
	if v.BranchPrefix == "" {
		if v.BranchPrefix, err = ask("Prefix of your branch names:", v.GitHubUser+"/"); err != nil {
			return err
		}
	}
	return nil
}
