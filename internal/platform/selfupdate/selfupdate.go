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
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/mallardduck/binup"
	binupgithub "github.com/mallardduck/binup/github"
	"github.com/rancher/orbs-automation/internal/platform/releasesign"
	"golang.org/x/mod/semver"
)

const (
	// Repo is where gero's releases are published.
	Repo = "rancher/orbs-automation"
	// AppName is the binary, and the {name} of an asset.
	AppName = "gero"
	// AssetTemplate names the archive of a platform, as .goreleaser.yaml does.
	AssetTemplate = "{name}_{os}_{arch}.{ext}"
	// ChecksumAsset is the manifest of the archives' hashes.
	ChecksumAsset = "checksums.txt"
	// SignerPattern matches the one identity that may sign a release.
	SignerPattern = releasesign.SignerPattern
	// SignerIssuer is the OIDC issuer of GitHub Actions.
	SignerIssuer = releasesign.SignerIssuer
)

// Config is what New needs.
type Config struct {
	// Current is the version of the running binary.
	Current string
	// Prereleases says whether a pre-release may be the newest release. Nil
	// leaves it to the running binary: a pre-release sees them, a stable
	// release does not.
	Prereleases *bool
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

	release *binup.Release
}

// Updater checks for and installs a release.
type Updater struct {
	u *binup.Updater
}

// New builds an Updater that verifies every release against SignerPattern.
func New(cfg Config) (*Updater, error) {
	return newUpdater(cfg, nil, "")
}

// signatureVerifier is what authenticates a manifest or an archive against its
// detached signature.
type signatureVerifier = binupgithub.SignatureVerifier

// newUpdater is New with the verifier and the file to replace given, so a test
// can sign with its own and update a scratch file. A nil verifier is the real
// one, and an empty target is the running binary.
func newUpdater(cfg Config, verifier signatureVerifier, target string) (*Updater, error) {
	if verifier == nil {
		v, err := releasesign.New(cfg.TrustedRoot)
		if err != nil {
			return nil, fmt.Errorf("self-update: %w", err)
		}
		verifier = v
	}
	opts := []binupgithub.Option{
		binupgithub.WithAppName(AppName),
		binupgithub.WithAssetTemplate(AssetTemplate),
		binupgithub.WithChecksumAsset(ChecksumAsset),
		binupgithub.WithTemplateMode(binupgithub.TemplateStrict),
		// Both, so the archive is authenticated itself and not only through the
		// hash the signed manifest holds for it.
		binupgithub.WithManifestVerifier(verifier),
		binupgithub.WithArtifactVerifier(verifier),
	}
	// Left to the binary, a pre-release is only offered to one: GitHub's
	// "latest" is always stable, and so has none for it to move to.
	prereleases := isPrerelease(cfg.Current)
	if cfg.Prereleases != nil {
		prereleases = *cfg.Prereleases
	}
	if prereleases {
		opts = append(opts, binupgithub.WithPrereleases())
	}
	if cfg.Token != "" {
		opts = append(opts, binupgithub.WithToken(cfg.Token))
	}
	if cfg.HTTPClient != nil {
		opts = append(opts, binupgithub.WithHTTPClient(cfg.HTTPClient))
	}
	updaterOpts := []binup.Option{binup.WithVersionProvider(binup.StaticVersion(cfg.Current))}
	if target != "" {
		updaterOpts = append(updaterOpts, binup.WithTargetPath(target))
	}
	u, err := binup.New(binupgithub.New(Repo, opts...), updaterOpts...)
	if err != nil {
		return nil, fmt.Errorf("self-update: %w", err)
	}
	return &Updater{u: u}, nil
}

// isPrerelease reports whether version is a semver pre-release, with or without
// the leading "v".
func isPrerelease(version string) bool {
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return semver.Prerelease(version) != ""
}

// Check says whether a newer release exists. It changes nothing.
func (u *Updater) Check(ctx context.Context) (Status, error) {
	res, err := u.u.Check(ctx)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Current:   res.Current.Version,
		Latest:    Release{Version: res.Latest, Notes: res.Release.Notes, Prerelease: res.Release.Prerelease},
		Available: res.UpdateAvailable,
		release:   res.Release,
	}, nil
}

// Blocked returns why this binary may not be replaced, or "" when it may.
func (u *Updater) Blocked() string {
	if skip := u.u.CanApply(); skip != nil {
		return skip.Hint
	}
	return ""
}

// Apply downloads, verifies and installs the release of s, and returns the
// version now installed.
func (u *Updater) Apply(ctx context.Context, s Status) (string, error) {
	if s.release == nil {
		return "", errors.New("self-update: no release to install: call Check first")
	}
	res, err := u.u.Apply(ctx, s.release)
	if err != nil {
		return "", err
	}
	if res.Skipped != nil {
		return "", fmt.Errorf("self-update: %s", res.Skipped.Hint)
	}
	return res.Version, nil
}
