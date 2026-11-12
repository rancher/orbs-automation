// Package fakehub is a fake GitHub releases API for tests: an http.RoundTripper,
// so it opens no port, that serves the few endpoints gero reads (the latest release, a release by tag, the
// list of releases, and an asset's bytes) for releases a test publishes. It lets a
// test imagine what a published release looks like, including the ways one goes
// wrong: a tampered asset, a missing checksum, a rate limit. It is test support,
// and no production code imports it.
package fakehub

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ChecksumsName is the asset that lists the others' SHA-256 hashes, as GoReleaser
// writes it.
const ChecksumsName = "checksums.txt"

// baseURL is where the fake host claims to be. No request leaves the process.
const baseURL = "http://fakehub.test"

// Hub is the fake host. Publish releases on it and make requests with Client.
type Hub struct {
	mu       sync.Mutex
	releases []*release
	assets   map[int64]*asset
	nextID   int64
	clock    time.Time
	requests []string
	failures []failure
}

type release struct {
	id         int64
	tag        string
	prerelease bool
	draft      bool
	published  time.Time
	assets     []*asset
}

type asset struct {
	id   int64
	name string
	data []byte
}

type failure struct {
	contains string
	status   int
	header   map[string]string
}

// New returns an empty Hub.
func New(t testing.TB) *Hub {
	t.Helper()
	return &Hub{assets: map[int64]*asset{}, nextID: 1000, clock: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

// URL is the base URL of the API, for the client to use in place of
// https://api.github.com.
func (h *Hub) URL() string { return baseURL }

// Client is an HTTP client whose requests the Hub answers, and whose redirects and
// timeouts are the defaults.
func (h *Hub) Client() *http.Client {
	return &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		h.serve(rec, r)
		return rec.Result(), nil
	})}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Option changes a release as it is published.
type Option func(*spec)

type spec struct {
	prerelease, draft bool
	files             []file
	noChecksums       bool
	checksumOverride  map[string]string
	signatures        map[string]string // asset name -> signature content; "" leaves it off
	signAll           *string
}

type file struct {
	name string
	data []byte
}

// Prerelease marks the release as a prerelease, so it is never the latest.
func Prerelease() Option { return func(s *spec) { s.prerelease = true } }

// Draft marks the release as a draft, which the API does not show as the latest.
func Draft() Option { return func(s *spec) { s.draft = true } }

// Asset adds a file to the release. Checksums are made for it unless
// WithoutChecksums is given.
func Asset(name string, data []byte) Option {
	return func(s *spec) { s.files = append(s.files, file{name, data}) }
}

// WithoutChecksums leaves checksums.txt off the release.
func WithoutChecksums() Option { return func(s *spec) { s.noChecksums = true } }

// ChecksumOf lists hash for the asset in checksums.txt, in place of the real one,
// or with an empty hash leaves the asset out of the list.
func ChecksumOf(name, hash string) Option {
	return func(s *spec) {
		if s.checksumOverride == nil {
			s.checksumOverride = map[string]string{}
		}
		s.checksumOverride[name] = hash
	}
}

// Signed publishes a detached signature with content sig, named as Verifier names
// it, for every asset of the release and for checksums.txt.
func Signed(sig string) Option { return func(s *spec) { s.signAll = &sig } }

// Signature publishes, or with an empty sig leaves off, the detached signature of
// one asset, in place of what Signed gives it.
func Signature(name, sig string) Option {
	return func(s *spec) {
		if s.signatures == nil {
			s.signatures = map[string]string{}
		}
		s.signatures[name] = sig
	}
}

// SignatureSuffix is what Verifier adds to an asset's name to find its signature.
const SignatureSuffix = ".sigstore.json"

// Verifier is a stand-in for a Sigstore verifier: a signature authenticates content
// when its bytes are "good". It names signatures as the real one does.
type Verifier struct{}

// SignatureName implements the verifier of ghreleases.
func (Verifier) SignatureName(asset string) string { return asset + SignatureSuffix }

// Verify implements the verifier of ghreleases.
func (Verifier) Verify(_ context.Context, content io.Reader, sig []byte) error {
	if _, err := io.Copy(io.Discard, content); err != nil {
		return err
	}
	if string(sig) != "good" {
		return errors.New("fakehub: the signature is not good")
	}
	return nil
}

// Publish adds a release, newest last, as GitHub orders by publish time, and
// returns its tag.
func (h *Hub) Publish(tag string, opts ...Option) string {
	var sp spec
	for _, o := range opts {
		o(&sp)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clock = h.clock.Add(time.Hour)
	r := &release{id: h.next(), tag: tag, prerelease: sp.prerelease, draft: sp.draft, published: h.clock}
	var sums strings.Builder
	for _, f := range sp.files {
		r.assets = append(r.assets, h.add(f.name, f.data))
		hash := sha256Hex(f.data)
		if o, ok := sp.checksumOverride[f.name]; ok {
			hash = o
		}
		if hash != "" {
			fmt.Fprintf(&sums, "%s  %s\n", hash, f.name)
		}
	}
	if !sp.noChecksums {
		r.assets = append(r.assets, h.add(ChecksumsName, []byte(sums.String())))
	}
	for _, a := range append([]*asset(nil), r.assets...) {
		sig, ok := sp.signatures[a.name]
		if !ok && sp.signAll != nil {
			sig, ok = *sp.signAll, true
		}
		if ok && sig != "" {
			r.assets = append(r.assets, h.add(a.name+SignatureSuffix, []byte(sig)))
		}
	}
	h.releases = append(h.releases, r)
	return tag
}

// Tamper replaces the bytes the host serves for an asset of a release, and leaves
// checksums.txt as it was: the download no longer matches its published hash.
func (h *Hub) Tamper(tag, name string, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.releases {
		for _, a := range r.assets {
			if r.tag == tag && a.name == name {
				a.data = data
			}
		}
	}
}

// Fail makes every request whose path contains substr answer with status, and with
// the headers given as name, value pairs, until the test ends. A 403 with
// X-RateLimit-Remaining 0 is how GitHub says a rate limit is used up.
func (h *Hub) Fail(substr string, status int, headers ...string) {
	f := failure{contains: substr, status: status, header: map[string]string{}}
	for i := 0; i+1 < len(headers); i += 2 {
		f.header[headers[i]] = headers[i+1]
	}
	h.mu.Lock()
	h.failures = append(h.failures, f)
	h.mu.Unlock()
}

// Heal removes the failures set by Fail.
func (h *Hub) Heal() {
	h.mu.Lock()
	h.failures = nil
	h.mu.Unlock()
}

// Requests is how many requests were made to paths that contain substr, all of
// them for "". A test that expects no network use asserts zero.
func (h *Hub) Requests(substr string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, p := range h.requests {
		if strings.Contains(p, substr) {
			n++
		}
	}
	return n
}

func (h *Hub) next() int64 { h.nextID++; return h.nextID }

func (h *Hub) add(name string, data []byte) *asset {
	a := &asset{id: h.next(), name: name, data: data}
	h.assets[a.id] = a
	return a
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (h *Hub) serve(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	path := r.URL.Path
	h.requests = append(h.requests, path)
	for _, f := range h.failures {
		if strings.Contains(path, f.contains) {
			for k, v := range f.header {
				w.Header().Set(k, v)
			}
			http.Error(w, `{"message":"fake failure"}`, f.status)
			return
		}
	}
	switch {
	case strings.Contains(path, "/releases/assets/"):
		id, _ := strconv.ParseInt(path[strings.LastIndex(path, "/")+1:], 10, 64)
		a, ok := h.assets[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(a.data)
	case strings.HasSuffix(path, "/releases/latest"):
		for i := len(h.releases) - 1; i >= 0; i-- {
			if rel := h.releases[i]; !rel.draft && !rel.prerelease {
				h.writeRelease(w, rel)
				return
			}
		}
		http.NotFound(w, r)
	case strings.Contains(path, "/releases/tags/"):
		tag := path[strings.Index(path, "/releases/tags/")+len("/releases/tags/"):]
		for _, rel := range h.releases {
			if rel.tag == tag && !rel.draft {
				h.writeRelease(w, rel)
				return
			}
		}
		http.NotFound(w, r)
	case strings.HasSuffix(path, "/releases"):
		var out []any
		if r.URL.Query().Get("page") == "" || r.URL.Query().Get("page") == "1" {
			for i := len(h.releases) - 1; i >= 0; i-- {
				out = append(out, h.apiRelease(h.releases[i]))
			}
		}
		writeJSON(w, out)
	default:
		http.NotFound(w, r)
	}
}

func (h *Hub) writeRelease(w http.ResponseWriter, rel *release) { writeJSON(w, h.apiRelease(rel)) }

func (h *Hub) apiRelease(rel *release) map[string]any {
	var assets []map[string]any
	for _, a := range rel.assets {
		assets = append(assets, map[string]any{
			"name": a.name, "size": len(a.data),
			"url": fmt.Sprintf("%s/repos/rancher/orbs-automation/releases/assets/%d", baseURL, a.id),
		})
	}
	return map[string]any{
		"id": rel.id, "tag_name": rel.tag, "name": rel.tag, "draft": rel.draft, "prerelease": rel.prerelease,
		"published_at": rel.published.Format(time.RFC3339), "assets": assets,
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
