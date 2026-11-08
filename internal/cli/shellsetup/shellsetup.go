// Package shellsetup holds the logic of gero shell install and uninstall: they
// add or remove the marked block in the user's shell profile, after asking. It
// is framework-free: cmd/ owns the cobra wiring.
package shellsetup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/shellenv"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

// ErrNeedsApproval is returned when approval is needed and cannot be asked for:
// the input is not a terminal and --yes was not given.
var ErrNeedsApproval = errors.New("this changes your shell profile and needs approval, but there is no terminal to ask on: pass --yes")

// ErrDeclined is returned when the user answers no. Nothing was written.
var ErrDeclined = errors.New("not changed")

// Options are the choices of one run.
type Options struct {
	// Shell names the shell, and Profile the file to edit. Either left empty is
	// worked out: the login shell from $SHELL, and that shell's usual profile.
	Shell, Profile string
	// Yes approves without asking. Interactive says the input is a terminal.
	Yes, Interactive bool
	// DryRun shows what would change and writes nothing.
	DryRun bool
	// In and Out are where the question is asked and answered.
	In  io.Reader
	Out io.Writer
	// Terminal is what the terminal can do, for the look of the report and the
	// question. The zero value has no color.
	Terminal clicfg.Terminal
	// Getenv and GOOS are the environment and runtime.GOOS.
	Getenv func(string) string
	GOOS   string
}

// Install adds the gero block to the profile, or brings an old one up to date.
func Install(o Options) (string, error) {
	return run(o, true)
}

// Uninstall removes the gero block from the profile.
func Uninstall(o Options) (string, error) {
	return run(o, false)
}

func run(o Options, install bool) (string, error) {
	view := style.New(o.Terminal)
	shell, profile, err := target(o)
	if err != nil {
		return "", err
	}
	content, err := readProfile(profile)
	if err != nil {
		return "", err
	}
	var next string
	var changed bool
	if install {
		next, changed, err = shellenv.Insert(content, shellenv.Block(shell))
	} else {
		next, changed, err = shellenv.Remove(content)
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w; fix or delete the lines between %q and %q", profile, err, shellenv.BlockStart, shellenv.BlockEnd)
	}
	if !changed {
		if install {
			return view.Style(style.OK, "Already set up in "+profile) + "\n", nil
		}
		return view.Style(style.Dim, "Nothing to remove from "+profile) + "\n", nil
	}

	var out strings.Builder
	if install {
		fmt.Fprintf(&out, "%s\n\n%s\n", view.Style(style.Heading, "Add to "+profile+":"), shellenv.Block(shell))
	} else {
		out.WriteString(view.Style(style.Heading, "Remove the gero block from "+profile+".") + "\n")
	}
	if o.DryRun {
		out.WriteString(view.Style(style.Dim, "Dry run: nothing was written.") + "\n")
		return out.String(), nil
	}
	if !o.Yes {
		ok, err := approve(o, out.String())
		if err != nil {
			return "", err
		}
		if !ok {
			return "", ErrDeclined
		}
		out.Reset()
	}
	if err := writeProfile(profile, next); err != nil {
		return "", err
	}
	if install {
		fmt.Fprintf(&out, "%s %s. Open a new shell, or run: source %s\n", view.Style(style.OK, "Updated"), profile, profile)
	} else {
		fmt.Fprintf(&out, "%s %s. Open a new shell to drop the gero function.\n", view.Style(style.OK, "Updated"), profile)
	}
	return out.String(), nil
}

// target works out the shell and its profile from the options.
func target(o Options) (shellenv.Shell, string, error) {
	var shell shellenv.Shell
	var err error
	if o.Shell != "" {
		shell, err = shellenv.Parse(o.Shell)
	} else {
		shell, err = shellenv.Detect(o.Getenv)
	}
	if err != nil {
		return "", "", err
	}
	profile := o.Profile
	if profile == "" {
		profile, err = shellenv.Profile(shell, o.GOOS, o.Getenv, fileExists)
		if err != nil {
			return "", "", err
		}
	}
	return shell, profile, nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// approve shows what will change and asks. Only y or yes counts.
func approve(o Options, shown string) (bool, error) {
	if !o.Interactive {
		return false, ErrNeedsApproval
	}
	view := style.New(o.Terminal)
	if _, err := fmt.Fprintf(o.Out, "%s\n%s %s ", shown, view.Style(style.Heading, "Continue?"), view.Style(style.Dim, "[y/N]")); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(o.In).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}

// readProfile returns the file's content, or "" when it does not exist.
func readProfile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), nil
}

// writeProfile replaces the file's content through a temporary file beside it,
// so a failure leaves the old content. A symlink, such as a profile kept in a
// dotfiles repository, is followed so the link stays a link, and the mode is
// kept.
func writeProfile(path, content string) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".gero-profile-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// Init returns the script `gero shell init` prints for the named shell: the
// completion script, which complete generates for the shell's name (the command
// tree is cmd/'s), then the cd wrapper.
func Init(shell string, complete func(shell string) (string, error)) (string, error) {
	s, err := shellenv.Parse(shell)
	if err != nil {
		return "", err
	}
	completion, err := complete(string(s))
	if err != nil {
		return "", fmt.Errorf("generate %s completion: %w", s, err)
	}
	return shellenv.Init(s, completion)
}
