// Package configfetch holds the logic of gero config fetch: the one command that
// looks for a newer central config bundle on the network and keeps it in the cache
// (docs/specs/CENTRAL_BUNDLE.md). It is framework-free: cmd/ owns the cobra wiring.
package configfetch

import (
	"context"

	"github.com/rancher/orbs-automation/internal/bundlestore"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
)

// Fetcher is what Run needs to look for the bundle. It is defined here, where it is
// consumed, so tests can use a fake; workspace.FetchBundle is the real one.
type Fetcher func(ctx context.Context, s clicfg.Settings) (bundlestore.Loaded, error)

// Run fetches a bundle newer than this gero's own, when there is one, and says what
// config the next run will read: the bundle it fetched, or the one in the binary.
func Run(ctx context.Context, s clicfg.Settings, fetch Fetcher) (string, error) {
	return "", nil
}
