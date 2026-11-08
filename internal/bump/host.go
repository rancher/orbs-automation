package bump

import (
	"context"
	"fmt"
)

// FileSource reads one file of a repository at a commit, as the host has it.
type FileSource interface {
	UpstreamFile(ctx context.Context, slug, ref, path string) ([]byte, error)
}

// HostTree is a Tree of a repository's files at one commit, read from the host,
// so a strategy can check a branch without a clone.
func HostTree(src FileSource, slug, commit string) Tree {
	return hostTree{src: src, slug: slug, commit: commit}
}

type hostTree struct {
	src          FileSource
	slug, commit string
}

// ReadFile reads path at the commit.
func (t hostTree) ReadFile(ctx context.Context, path string) ([]byte, error) {
	data, err := t.src.UpstreamFile(ctx, t.slug, t.commit, path)
	if err != nil {
		return nil, fmt.Errorf("read %s of %s at %s: %w", path, t.slug, t.commit, err)
	}
	return data, nil
}
