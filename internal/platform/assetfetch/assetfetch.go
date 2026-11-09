// Package assetfetch finds a release of a GitHub repository and downloads one of
// its assets, verified against the release's published checksums. It is the one
// implementation of that in gero (docs/specs/CENTRAL_BUNDLE.md), built on
// ghreleases, which no other package imports. An asset that has no published
// checksum, or does not match it, is an error and never returned.
package assetfetch

import (
	"context"
	"errors"
	"fmt"
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

// CompareVersions orders two release tags by semantic versioning: negative when a
// is older than b, zero when equal, positive when newer. A leading v is optional
// and a prerelease is older than its release. ok is false when either is not a
// semantic version, such as "dev".
func CompareVersions(a, b string) (cmp int, ok bool) {
	return ghrelease.CompareVersions(a, b)
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

// New returns a Fetcher for owner/repo. Requests, and redirects, go only to
// GitHub's own hosts and to the base URL.
func New(owner, repo string, o Options) *Fetcher {
	opts := []ghrelease.Option{ghrelease.WithAllowedHosts("github.com", "githubusercontent.com")}
	if o.BaseURL != "" {
		opts = append(opts, ghrelease.WithBaseURL(o.BaseURL))
	}
	var c *ghrelease.Client
	if o.HTTP != nil {
		c = ghrelease.NewClientWithHTTP(o.HTTP, o.Token, opts...)
	} else {
		c = ghrelease.NewClient(o.Token, opts...)
	}
	return &Fetcher{client: c, owner: owner, repo: repo}
}

func release(r *ghrelease.Release) Release {
	return Release{Tag: r.TagName, Prerelease: r.Prerelease, PublishedAt: r.PublishedAt}
}

// Latest is the newest stable release: a prerelease or a draft is never the
// latest.
func (f *Fetcher) Latest(ctx context.Context) (Release, error) {
	r, err := f.client.LatestReleaseInfo(ctx, f.owner, f.repo)
	if err != nil {
		return Release{}, err
	}
	return release(r), nil
}

// Release is the release with the tag.
func (f *Fetcher) Release(ctx context.Context, tag string) (Release, error) {
	r, err := f.client.GetRelease(ctx, f.owner, f.repo, tag)
	if err != nil {
		return Release{}, err
	}
	return release(r), nil
}

// Asset downloads the asset called name of the release with the tag, and returns
// its bytes only when they match the hash in the release's checksums.txt.
func (f *Fetcher) Asset(ctx context.Context, tag, name string) ([]byte, error) {
	r, err := f.client.GetRelease(ctx, f.owner, f.repo, tag)
	if err != nil {
		return nil, err
	}
	var asset, sums *ghrelease.Asset
	for i := range r.Assets {
		switch r.Assets[i].Name {
		case name:
			asset = &r.Assets[i]
		case ChecksumsName:
			sums = &r.Assets[i]
		}
	}
	if asset == nil {
		return nil, fmt.Errorf("%w: %s is not on release %s", ErrAssetNotFound, name, tag)
	}
	if sums == nil {
		return nil, fmt.Errorf("%w: release %s has no %s", ErrNoChecksum, tag, ChecksumsName)
	}
	hashes, err := f.client.FetchChecksums(ctx, sums.URL)
	if err != nil {
		return nil, fmt.Errorf("read %s of release %s: %w", ChecksumsName, tag, err)
	}
	want, ok := hashes[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s of release %s does not list %s", ErrNoChecksum, ChecksumsName, tag, name)
	}
	data, _, err := f.client.DownloadToBytes(asset.URL, ghrelease.DownloadOptions{ExpectedHash: want, Context: ctx})
	if err != nil {
		return nil, fmt.Errorf("download %s of release %s: %w", name, tag, err)
	}
	return data, nil
}
