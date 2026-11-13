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
	Bash       Shell = "bash"
	Zsh        Shell = "zsh"
	Fish       Shell = "fish"
	PowerShell Shell = "powershell"
)

// Shells lists the supported shells, for help and errors.
var Shells = []Shell{Bash, Zsh, Fish, PowerShell}

// ErrUnsupportedShell is returned for a shell that is not supported.
var ErrUnsupportedShell = errors.New("unsupported shell")

// Parse returns the shell named by name, which is its program name or path.
// PowerShell is also known by its program names pwsh and powershell.exe.
func Parse(name string) (Shell, error) {
	base := strings.TrimPrefix(filepath.Base(strings.TrimSpace(name)), "-") // a login shell is "-zsh"
	base = strings.TrimSuffix(strings.ToLower(base), ".exe")
	if base == "pwsh" {
		base = string(PowerShell)
	}
	for _, s := range Shells {
		if string(s) == base {
			return s, nil
		}
	}
	if base == "" || base == "." {
		return "", fmt.Errorf("%w: none given", ErrUnsupportedShell)
	}
	return "", fmt.Errorf("%w %q: supported shells are bash, zsh, fish and powershell", ErrUnsupportedShell, base)
}

// Detect returns the user's login shell from $SHELL. PowerShell is never found
// this way, since it is usually started from another login shell: name it with
// --shell.
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
//   - zsh reads .zshrc in $ZDOTDIR, else in the home directory.
//   - bash reads .bashrc in a non-login shell, and a login shell, which is what
//     macOS terminals start, reads .bash_profile first: there it is used when it
//     exists, and .bashrc otherwise when that does, else the new file is the one
//     the OS's shell reads.
//   - fish reads config/fish/config.fish under $XDG_CONFIG_HOME, else ~/.config.
//   - PowerShell (7, pwsh) reads Microsoft.PowerShell_profile.ps1 in
//     config/powershell likewise, and on Windows in Documents\PowerShell of the
//     user profile directory.
func Profile(shell Shell, goos string, getenv func(string) string, exists func(string) bool) (string, error) {
	home := getenv("HOME")
	if goos == "windows" && home == "" {
		home = getenv("USERPROFILE")
	}
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
			return "", errNoHome
		}
		rc, login := filepath.Join(home, ".bashrc"), filepath.Join(home, ".bash_profile")
		if goos != "darwin" {
			return rc, nil
		}
		if !exists(login) && exists(rc) {
			return rc, nil
		}
		return login, nil
	case Fish:
		dir, err := configDir(getenv, home)
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "fish", "config.fish"), nil
	case PowerShell:
		const file = "Microsoft.PowerShell_profile.ps1"
		if goos == "windows" {
			if home == "" {
				return "", errNoHome
			}
			return filepath.Join(home, "Documents", "PowerShell", file), nil
		}
		dir, err := configDir(getenv, home)
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "powershell", file), nil
	}
	return "", fmt.Errorf("%w %q", ErrUnsupportedShell, shell)
}

var errNoHome = errors.New("$HOME is not set, so name a profile with --profile")

// configDir is $XDG_CONFIG_HOME, else .config in the home directory.
func configDir(getenv func(string) string, home string) (string, error) {
	if dir := getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir, nil
	}
	if home == "" {
		return "", errNoHome
	}
	return filepath.Join(home, ".config"), nil
}
