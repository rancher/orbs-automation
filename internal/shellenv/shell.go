// Package shellenv is what gero needs to live in a user's shell: which shell
// that is, which profile file it reads, the script that adds completion and the
// cd wrapper, and the marked block that holds a line of it in the profile. It
// writes nothing: the command that uses it asks first and does the writing.
package shellenv

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Shell is a shell gero can set up.
type Shell string

// The supported shells.
const (
	Bash Shell = "bash"
	Zsh  Shell = "zsh"
)

// Shells lists the supported shells, for help and errors.
var Shells = []Shell{Bash, Zsh}

// ErrUnsupportedShell is returned for a shell that is not Bash or Zsh.
var ErrUnsupportedShell = errors.New("unsupported shell")

// Parse returns the shell named by name, which is its program name or path.
func Parse(name string) (Shell, error) {
	base := strings.TrimPrefix(filepath.Base(strings.TrimSpace(name)), "-") // a login shell is "-zsh"
	for _, s := range Shells {
		if string(s) == base {
			return s, nil
		}
	}
	if base == "" || base == "." {
		return "", fmt.Errorf("%w: none given", ErrUnsupportedShell)
	}
	return "", fmt.Errorf("%w %q: supported shells are bash and zsh", ErrUnsupportedShell, base)
}

// Detect returns the user's login shell from $SHELL.
func Detect(getenv func(string) string) (Shell, error) {
	shell := getenv("SHELL")
	if shell == "" {
		return "", fmt.Errorf("%w: $SHELL is not set, so name one with --shell", ErrUnsupportedShell)
	}
	return Parse(shell)
}

// Profile returns the file the shell reads for interactive use, for the given
// operating system (a runtime.GOOS value). exists says whether a file does.
//
// zsh reads .zshrc in $ZDOTDIR, else in the home directory. bash reads .bashrc
// in a non-login shell, and a login shell, which is what macOS terminals start,
// reads .bash_profile first: there it is used when it exists, and .bashrc
// otherwise when that does, else the new file is the one the OS's shell reads.
func Profile(shell Shell, goos string, getenv func(string) string, exists func(string) bool) (string, error) {
	home := getenv("HOME")
	switch shell {
	case Zsh:
		dir := getenv("ZDOTDIR")
		if dir == "" {
			dir = home
		}
		if dir == "" {
			return "", errors.New("neither $ZDOTDIR nor $HOME is set, so name a profile with --profile")
		}
		return filepath.Join(dir, ".zshrc"), nil
	case Bash:
		if home == "" {
			return "", errors.New("$HOME is not set, so name a profile with --profile")
		}
		rc, login := filepath.Join(home, ".bashrc"), filepath.Join(home, ".bash_profile")
		if goos != "darwin" {
			return rc, nil
		}
		if !exists(login) && exists(rc) {
			return rc, nil
		}
		return login, nil
	}
	return "", fmt.Errorf("%w %q", ErrUnsupportedShell, shell)
}
