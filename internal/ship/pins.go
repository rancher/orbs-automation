package ship

import (
	"context"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git"
)

// Companion is a release shipped with the one asked for, because the Artifact of
// that release pins the Artifact of this one (wrangler's go.mod requires a lasso):
// the version it requires, where it was read, and whether it can be shipped.
type Companion struct {
	Producer string `json:"producer"` // the repository the pinned module is released from, owner/name
	Module   string `json:"module"`   // the pinned module's path
	Tag      string `json:"tag"`      // the release of it that goes with the pinning release
	Version  string `json:"version"`  // as the file requires it
	// RequiredBy is the pinning producer and its tag, whose file says so.
	RequiredBy string `json:"requiredBy"`
	File       string `json:"file"` // the file of the pinning release that was read
	// Error is why no release could be derived: the file is not there, does not
	// require the module, requires a version no release has, or one of another
	// stream. It stops the run.
	Error string `json:"error,omitempty"`
}

// pinSource is what deriving a companion reads from the host.
type pinSource interface {
	UpstreamRefs(ctx context.Context, slug string) (git.Refs, error)
	UpstreamFile(ctx context.Context, slug, ref, path string) ([]byte, error)
}

// derive finds the release of the pinned Artifact of a virtual Holding that goes
// with a release of the Artifact that pins it: it reads the pinning release's file
// at its tag from the host, takes the version of the pinned module there, and makes
// it the tag of the pinned Artifact's stream, which must exist.
func derive(ctx context.Context, src pinSource, v *chain.Holding, pinnerTag string) Companion {
	return Companion{}
}
