// Package selfupdate replaces the running gero with the newest release, after
// proving the release is ours. It is the one place that imports binup, the
// ghreleases verifier and Sigstore (docs/specs/SELF_UPDATE.md).
//
// The proof is the highest binup offers: the checksum manifest and every
// archive must each carry a keyless Sigstore bundle, signed by this repository's
// release workflow on a version tag, and the archive must match the manifest. A
// release without the bundles, or with one signed by anyone else, is refused;
// nothing falls back to a checksum alone.
package selfupdate

import (
	"context"
	"io"
	"net/http"
)

const (
	// Repo is where gero's releases are published.
	Repo = "rancher/orbs-automation"
	// AppName is the binary, and the {name} of an asset.
	AppName = "gero"
	// AssetTemplate names the archive of a platform, as .goreleaser.yaml does.
	AssetTemplate = ""
	// ChecksumAsset is the manifest of the archives' hashes.
	ChecksumAsset = ""
	// SignerPattern matches the one identity that may sign a release: this
	// repository's release workflow, run for a version tag.
	SignerPattern = ""
	// SignerIssuer is the OIDC issuer of GitHub Actions.
	SignerIssuer = ""
)

// Config is what New needs.
type Config struct {
	// Current is the version of the running binary.
	Current string
	// Token authenticates the GitHub API, and may be empty.
	Token string
	// HTTPClient carries every request; nil means the default client.
	HTTPClient *http.Client
	// TrustedRoot is a Sigstore trusted_root.json for offline use and tests;
	// nil fetches the public-good root over TUF.
	TrustedRoot []byte
}

// Release is the newest release.
type Release struct {
	Version    string
	Notes      string
	Prerelease bool
}

// Status is what Check found.
type Status struct {
	Current   string
	Latest    Release
	Available bool
}

// Updater checks for and installs a release.
type Updater struct{}

// New builds an Updater that verifies every release.
func New(Config) (*Updater, error) { return &Updater{}, nil }

// Check says whether a newer release exists. It changes nothing.
func (u *Updater) Check(context.Context) (Status, error) { return Status{}, nil }

// Blocked returns why this binary may not be replaced, or "" when it may.
func (u *Updater) Blocked() string { return "" }

// Apply downloads, verifies and installs the release of s, and returns the
// version now installed.
func (u *Updater) Apply(context.Context, Status) (string, error) { return "", nil }

// newUpdater is New with the verifier and the file to replace given, so a test
// can sign with its own and update a scratch file. A nil verifier is the real
// one, which pins SignerPattern.
func newUpdater(Config, signatureVerifier, string) (*Updater, error) { return &Updater{}, nil }

// signatureVerifier is what authenticates a manifest or an archive against its
// detached signature.
type signatureVerifier interface {
	SignatureName(assetName string) string
	Verify(ctx context.Context, content io.Reader, signature []byte) error
}
