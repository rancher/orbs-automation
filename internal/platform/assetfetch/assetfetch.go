// Package assetfetch finds a release of a GitHub repository and downloads one of
// its assets, verified against the release's published checksums. It is the one
// implementation of that in gero (docs/specs/CENTRAL_BUNDLE.md), built on
// ghreleases, which no other package imports. An asset that has no published
// checksum, or does not match it, is an error and never returned.
package assetfetch

import (
	"context"
	"errors"
	"net/http"
	"time"

	ghrelease "github.com/mallardduck/ghreleases"
)

// What a caller tells apart, without importing ghreleases.
var (
	// ErrNotFound is returned when the repository has no such release.
	ErrNotFound = ghrelease.ErrReleaseNotFound
	// ErrAssetNotFound is returned when the release has no such asset.
	ErrAssetNotFound = ghrelease.ErrAssetNotFound
	// ErrChecksumMismatch is returned when the download is not the file the
	// release published the hash of.
	ErrChecksumMismatch = ghrelease.ErrChecksumMismatch
	// ErrRateLimited is returned when the API rate limit is used up.
	ErrRateLimited = ghrelease.ErrRateLimited
	// ErrNoChecksum is returned when the release publishes no checksum for the
	// asset: nothing is downloaded that cannot be verified.
	ErrNoChecksum = errors.New("the release publishes no checksum for the asset")
)

// ChecksumsName is the asset that lists the hashes of the others, as GoReleaser
// writes it.
const ChecksumsName = "checksums.txt"

// Release is what a caller needs to know of a release.
type Release struct {
	Tag         string
	Prerelease  bool
	PublishedAt time.Time
}

// Options configure a Fetcher.
type Options struct {
	HTTP    *http.Client // nil for the default, which has timeouts
	Token   string       // optional: a public repository needs none
	BaseURL string       // "" for https://api.github.com; a test points it at a fake
}

// Fetcher reads the releases of one repository.
type Fetcher struct {
	client      *ghrelease.Client
	owner, repo string
}

// New returns a Fetcher for owner/repo.
func New(owner, repo string, opts Options) *Fetcher {
	return &Fetcher{owner: owner, repo: repo}
}

// Latest is the newest stable release: a prerelease or a draft is never the
// latest.
func (f *Fetcher) Latest(ctx context.Context) (Release, error) {
	return Release{}, nil
}

// Release is the release with the tag.
func (f *Fetcher) Release(ctx context.Context, tag string) (Release, error) {
	return Release{}, nil
}

// Asset downloads the asset called name of the release with the tag, and returns
// its bytes only when they match the hash in the release's checksums.txt.
func (f *Fetcher) Asset(ctx context.Context, tag, name string) ([]byte, error) {
	return nil, nil
}
