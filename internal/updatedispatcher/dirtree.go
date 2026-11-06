package updatedispatcher

import (
	"context"
	"errors"
)

// ErrOutsideTree is returned for a path that leaves the directory of a DirTree.
var ErrOutsideTree = errors.New("the path is outside the directory")

// DirTree is a WritableTree of the files below a directory: a clone, or a
// worktree of one. A path that leaves the directory is an error.
func DirTree(dir string) WritableTree { return dirTree{dir: dir} }

type dirTree struct{ dir string }

// ReadFile reads path below the directory.
//
// STUB: reads nothing until a later commit.
func (t dirTree) ReadFile(ctx context.Context, path string) ([]byte, error) { return nil, nil }

// WriteFile writes path below the directory, keeping the file's mode.
//
// STUB: writes nothing until a later commit.
func (t dirTree) WriteFile(ctx context.Context, path string, data []byte) error { return nil }
