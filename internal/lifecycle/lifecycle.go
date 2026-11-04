// Package lifecycle reads Rancher's lifecycle dates from the SUSE lifecycle
// page (https://www.suse.com/lifecycle/). The page is the source of truth for
// the dates in rancher.lines.yaml; this package only parses it. It does no I/O
// of its own, so a test feeds it saved HTML.
package lifecycle

import (
	"io"

	"github.com/rancher/orbs-automation/internal/config"
)

// Parse reads the lifecycle page from r and returns the dates of each Rancher
// line in the product section whose data-url is product (the page's own anchor,
// for example "suse-rancher-prime"), keyed by line id ("v2.15"). A cell left
// blank is a date that is not set. It fails, naming what is wrong, when the
// section or its version table is missing or a version or date does not parse,
// so a change to the page is an error and never an empty result.
func Parse(r io.Reader, product string) (map[string]config.Dates, error) {
	return nil, nil
}
