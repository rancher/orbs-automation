// Package releasesign is the proof that a release is ours: a keyless Sigstore
// verifier pinned to the one identity that may sign one, this repository's release
// workflow run for a version tag. Self-update checks the binary with it and the
// central config bundle is fetched with it (docs/specs/SELF_UPDATE.md,
// docs/specs/CENTRAL_BUNDLE.md). It is the one place that imports Sigstore.
package releasesign

import (
	"fmt"

	"github.com/mallardduck/ghreleases/sigstore"
)

const (
	// SignerPattern matches the one identity that may sign a release: this
	// repository's release workflow, run for a version tag.
	SignerPattern = `^https://github\.com/rancher/orbs-automation/\.github/workflows/release\.yaml@refs/tags/v[0-9][0-9A-Za-z.+-]*$`
	// SignerIssuer is the OIDC issuer of GitHub Actions.
	SignerIssuer = "https://token.actions.githubusercontent.com"
)

// New returns the verifier of releases signed by SignerPattern. trustedRoot is a
// Sigstore trusted_root.json for offline use and tests; nil fetches the
// public-good root over TUF, when the first signature is checked.
func New(trustedRoot []byte) (*sigstore.Verifier, error) {
	var opts []sigstore.Option
	if trustedRoot != nil {
		opts = append(opts, sigstore.WithTrustedRoot(trustedRoot))
	}
	v, err := sigstore.NewVerifier(sigstore.Identity{SubjectRegexp: SignerPattern, Issuer: SignerIssuer}, opts...)
	if err != nil {
		return nil, fmt.Errorf("release signature: %w", err)
	}
	return v, nil
}
