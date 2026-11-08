package clicfg

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// UserFilePath returns where the user file lives: GERO_CONFIG, else
// $XDG_CONFIG_HOME/gero/config.yaml, else ~/.config/gero/config.yaml. explicit
// is true when GERO_CONFIG named it. We avoid os.UserConfigDir because it points
// at ~/Library/Application Support on macOS.
func UserFilePath(getenv func(string) string, home string) (path string, explicit bool) {
	if p := getenv("GERO_CONFIG"); p != "" {
		return p, true
	}
	if x := getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "gero", "config.yaml"), false
	}
	return filepath.Join(home, ".config", "gero", "config.yaml"), false
}

// OpenUserFile opens the user file for Load. It returns a nil reader (and no
// error) when the file does not apply: absent at the default path, or in CI
// without GERO_CONFIG. A GERO_CONFIG that names a missing file is an error,
// since it is almost certainly a typo. The caller closes a non-nil reader.
func OpenUserFile(getenv func(string) string, home string) (io.ReadCloser, string, error) {
	path, explicit := UserFilePath(getenv, home)
	ci, err := InCI(getenv)
	if err != nil {
		return nil, path, err
	}
	if ci && !explicit {
		return nil, path, nil
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) && !explicit {
		return nil, path, nil
	}
	if err != nil {
		return nil, path, fmt.Errorf("user config: %w", err)
	}
	return f, path, nil
}

// StatePath returns where gero keeps state of its own, such as the REPL's
// history, and name is the file: $XDG_STATE_HOME/gero/name, else
// ~/.local/state/gero/name. It is empty when neither is known. Like UserFilePath
// it does not use the platform's directory, which on macOS is under
// ~/Library/Application Support.
func StatePath(getenv func(string) string, home, name string) string {
	if x := getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "gero", name)
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "state", "gero", name)
}
