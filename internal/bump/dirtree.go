package bump

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrOutsideTree is returned for a path that leaves the directory of a DirTree.
var ErrOutsideTree = errors.New("the path is outside the directory")

// Within returns the absolute path of rel below dir. It is an error, wrapping
// ErrOutsideTree, for an absolute path, one that climbs out with "..", or one
// that a symlink inside the directory leads out of, whether or not the file
// exists yet.
func Within(dir, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: %s is an absolute path", ErrOutsideTree, rel)
	}
	abs := filepath.Join(dir, rel)
	if escapes(dir, abs) {
		return "", fmt.Errorf("%w: %s", ErrOutsideTree, rel)
	}
	// A symlink must not lead out either. The file may not exist yet, so resolve
	// the nearest part of the path that does.
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", dir, err)
	}
	for existing := abs; ; existing = filepath.Dir(existing) {
		resolved, err := filepath.EvalSymlinks(existing)
		if err == nil {
			if escapes(root, resolved) {
				return "", fmt.Errorf("%w: %s resolves outside it", ErrOutsideTree, rel)
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) || existing == filepath.Dir(existing) {
			return "", fmt.Errorf("resolve %s: %w", rel, err)
		}
	}
	return abs, nil
}

// escapes reports whether path is not dir or something below it.
func escapes(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// DirTree is a WritableTree of the files below a directory: a clone, or a
// worktree of one. A path that leaves the directory is an error.
func DirTree(dir string) WritableTree { return dirTree{dir: dir} }

type dirTree struct{ dir string }

// ReadFile reads path below the directory.
func (t dirTree) ReadFile(_ context.Context, path string) ([]byte, error) {
	abs, err := Within(t.dir, path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// WriteFile writes path below the directory, creating the directories above it.
// An existing file keeps its mode.
func (t dirTree) WriteFile(_ context.Context, path string, data []byte) error {
	abs, err := Within(t.dir, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
