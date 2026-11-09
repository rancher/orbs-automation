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

// Options are the choices of one run.
type Options struct {
	// The values of the file. One left empty is asked for on a terminal, and an
	// empty answer skips it: the key is left out. Without a terminal an empty one
	// is skipped.
	GitHubUser, Fork, Upstream, BranchPrefix string
	// CentralConfigDir is centralConfigDir. Search and CloneRoot are repos.search, the
	// directories scanned for clones that exist, and repos.cloneRoot, the one
	// directory gero repo clone puts new clones in.
	CentralConfigDir, CloneRoot string
	Search                      []string
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

	v := clicfg.UserFileValues{
		GitHubUser: o.GitHubUser, Fork: o.Fork, Upstream: o.Upstream, BranchPrefix: o.BranchPrefix,
		CentralConfigDir: o.CentralConfigDir, Search: cleanList(o.Search), CloneRoot: o.CloneRoot,
	}
	if o.Interactive {
		if err := ask(o, &v); err != nil {
			return "", err
		}
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
	// What is written has to be what the next command accepts: a path that is not
	// absolute is refused here, naming its key, and not by every command after.
	if _, err := clicfg.Load(clicfg.Inputs{
		Getenv:       withoutGeroVars(o.Getenv),
		UserFile:     strings.NewReader(content),
		UserFileName: "the new user file",
		Home:         o.Home,
	}); err != nil {
		return "", err
	}

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
	if content == clicfg.UserFileSchemaComment+"\n" {
		out.WriteString(view.Style(style.Dim, "Nothing was entered, so the file holds only the schema comment.") + "\n")
	}
	return out.String(), nil
}

// searchVersusCloneRoot says how the two path keys differ. They look alike, and
// a person who takes one for the other puts new clones where they are not found,
// or scans the wrong place.
const searchVersusCloneRoot = `repos.search and repos.cloneRoot are not the same:
  repos.search     directories gero scans for clones you already have. It only
                   finds what is there; it never puts anything there.
  repos.cloneRoot  the one directory gero repo clone puts new clones in, as
                   <root>/<name>. It is for clones that do not exist yet.
The clone root is scanned as well, so it need not be listed in repos.search.`

// ask puts a question to each value still empty, on o.Out, and reads the answer
// from o.In. An empty answer, or the end of the input, skips the question: its
// value stays empty and its key is left out of the file.
func ask(o Options, v *clicfg.UserFileValues) error {
	view := style.New(o.Terminal)
	in := bufio.NewReader(o.In)
	var started bool
	line := func(question string) (string, error) {
		if !started {
			started = true
			if _, err := fmt.Fprintln(o.Out, view.Style(style.Dim, "Enter to skip any question: its key is then left out of the file.")); err != nil {
				return "", err
			}
		}
		if _, err := fmt.Fprint(o.Out, view.Style(style.Heading, question)+" "); err != nil {
			return "", err
		}
		text, err := in.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimSpace(text), nil
	}
	for _, q := range []struct {
		dest     *string
		question string
	}{
		{&v.GitHubUser, "Your GitHub login:"},
		{&v.Fork, "Remote that holds your fork (for example origin):"},
		{&v.Upstream, "Remote that is the real source (for example upstream):"},
		{&v.BranchPrefix, "Prefix of your branch names (for example me/):"},
		{&v.CentralConfigDir, "Directory of the central config, centralConfigDir:"},
	} {
		if *q.dest != "" {
			continue
		}
		answer, err := line(q.question)
		if err != nil {
			return err
		}
		*q.dest = answer
	}
	if len(v.Search) == 0 || v.CloneRoot == "" {
		if _, err := fmt.Fprintln(o.Out, "\n"+view.Style(style.Dim, searchVersusCloneRoot)+"\n"); err != nil {
			return err
		}
	}
	if len(v.Search) == 0 {
		answer, err := line("Directories to scan for your existing clones, repos.search (comma-separated):")
		if err != nil {
			return err
		}
		v.Search = cleanList(strings.Split(answer, ","))
	}
	if v.CloneRoot == "" {
		answer, err := line("Directory for new clones, repos.cloneRoot:")
		if err != nil {
			return err
		}
		v.CloneRoot = answer
	}
	return nil
}

// cleanList trims each entry and drops the empty ones.
func cleanList(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// withoutGeroVars hides the GERO_* variables from the loader, so a bad value in
// the environment is not blamed on the file about to be written.
func withoutGeroVars(getenv func(string) string) func(string) string {
	return func(name string) string {
		if strings.HasPrefix(name, "GERO_") {
			return ""
		}
		return getenv(name)
	}
}
