package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"

	binupgithub "github.com/mallardduck/binup/github"
	"github.com/mallardduck/ghreleases/sigstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const workflowSubject = "https://github.com/rancher/orbs-automation/.github/workflows/release.yaml@refs/tags/"

func TestSignerIsThisRepositorysReleaseWorkflowOnAVersionTag(t *testing.T) {
	// NewVerifier refuses a pattern that is unanchored or matches strangers.
	_, err := sigstore.NewVerifier(sigstore.Identity{SubjectRegexp: SignerPattern, Issuer: SignerIssuer})
	require.NoError(t, err)
	assert.Equal(t, "https://token.actions.githubusercontent.com", SignerIssuer)

	re := regexp.MustCompile(SignerPattern)
	assert.True(t, re.MatchString(workflowSubject+"v1.2.3"))
	assert.True(t, re.MatchString(workflowSubject+"v1.2.3-rc.1"))
	for name, subject := range map[string]string{
		"a branch":                 "https://github.com/rancher/orbs-automation/.github/workflows/release.yaml@refs/heads/main",
		"a tag that is no version": workflowSubject + "nightly",
		"another workflow":         "https://github.com/rancher/orbs-automation/.github/workflows/ci.yml@refs/tags/v1.2.3",
		"a fork":                   "https://github.com/mallory/orbs-automation/.github/workflows/release.yaml@refs/tags/v1.2.3",
		"another repository":       "https://github.com/rancher/other/.github/workflows/release.yaml@refs/tags/v1.2.3",
		"a suffix on the owner":    "https://github.com/rancher-evil/orbs-automation/.github/workflows/release.yaml@refs/tags/v1.2.3",
		"text after the tag":       workflowSubject + "v1.2.3\n.evil",
	} {
		assert.False(t, re.MatchString(subject), name)
	}
}

// fakeVerifier accepts a signature that is exactly "good" and records what it
// was asked to authenticate.
type fakeVerifier struct {
	mu     sync.Mutex
	signed []string
}

func (*fakeVerifier) SignatureName(name string) string { return name + ".sigstore.json" }

func (v *fakeVerifier) Verify(_ context.Context, content io.Reader, sig []byte) error {
	if _, err := io.Copy(io.Discard, content); err != nil {
		return err
	}
	v.mu.Lock()
	v.signed = append(v.signed, string(sig))
	v.mu.Unlock()
	if string(sig) != "good" {
		return assert.AnError
	}
	return nil
}

// fakeBase is where the fake release says its assets are.
const fakeBase = "https://assets.invalid"

type fakeRelease struct {
	handler http.Handler
	assets  map[string][]byte // by name
}

func tarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// newRelease serves v1.2.3 of rancher/orbs-automation as goreleaser lays it
// out: one archive per platform and checksums.txt, plus signature bundles
// named after what they sign when signed is true.
func newRelease(t *testing.T, binary []byte, signed bool, sig string) *fakeRelease {
	t.Helper()
	archive := tarGz(t, AppName, binary)
	archiveName := AppName + "_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	sum := sha256.Sum256(archive)
	manifest := []byte(hex.EncodeToString(sum[:]) + "  " + archiveName + "\n")
	fr := &fakeRelease{assets: map[string][]byte{archiveName: archive, "checksums.txt": manifest}}
	if signed {
		fr.assets[archiveName+".sigstore.json"] = []byte(sig)
		fr.assets["checksums.txt.sigstore.json"] = []byte(sig)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/rancher/orbs-automation/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v1.2.3", "body": "notes"})
	})
	mux.HandleFunc("/repos/rancher/orbs-automation/releases/tags/v1.2.3", func(w http.ResponseWriter, _ *http.Request) {
		var assets []map[string]any
		for name, body := range fr.assets {
			assets = append(assets, map[string]any{"name": name, "url": fakeBase + "/assets/" + name, "size": len(body)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "tag_name": "v1.2.3", "name": "v1.2.3", "assets": assets})
	})
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		body, ok := fr.assets[strings.TrimPrefix(r.URL.Path, "/assets/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	})
	fr.handler = mux
	return fr
}

type serve func(*http.Request) (*http.Response, error)

func (f serve) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// client answers every request, to api.github.com or to the assets, from the
// fake in this process: the tests open no socket.
func (fr *fakeRelease) client() *http.Client {
	return &http.Client{Transport: serve(func(r *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		fr.handler.ServeHTTP(rec, r)
		return rec.Result(), nil
	})}
}

// installed is a stand-in for the running binary.
func installed(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gero")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o755))
	return path
}

func TestNewRefusesAReleaseWithoutSignatures(t *testing.T) {
	fr := newRelease(t, []byte("new"), false, "")
	target := installed(t)
	u, err := newUpdater(Config{Current: "v1.0.0", Token: "t0ken", HTTPClient: fr.client()}, nil, target)
	require.NoError(t, err)
	status, err := u.Check(context.Background())
	require.NoError(t, err)

	_, err = u.Apply(context.Background(), status)

	require.Error(t, err)
	assert.ErrorIs(t, err, binupgithub.ErrSignatureMissing)
	got, _ := os.ReadFile(target)
	assert.Equal(t, "old", string(got), "the binary is left as it was")
}

func TestApplyAuthenticatesTheManifestAndTheArchive(t *testing.T) {
	fr := newRelease(t, []byte("new"), true, "good")
	target := installed(t)
	v := &fakeVerifier{}
	u, err := newUpdater(Config{Current: "v1.0.0", HTTPClient: fr.client()}, v, target)
	require.NoError(t, err)
	status, err := u.Check(context.Background())
	require.NoError(t, err)

	version, err := u.Apply(context.Background(), status)

	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", version)
	assert.Equal(t, []string{"good", "good"}, v.signed, "both the manifest and the archive")
	got, _ := os.ReadFile(target)
	assert.Equal(t, "new", string(got))
}

func TestApplyRefusesABadSignature(t *testing.T) {
	fr := newRelease(t, []byte("new"), true, "forged")
	target := installed(t)
	u, err := newUpdater(Config{Current: "v1.0.0", HTTPClient: fr.client()}, &fakeVerifier{}, target)
	require.NoError(t, err)
	status, err := u.Check(context.Background())
	require.NoError(t, err)

	_, err = u.Apply(context.Background(), status)

	require.Error(t, err)
	got, _ := os.ReadFile(target)
	assert.Equal(t, "old", string(got))
}

func TestCheck(t *testing.T) {
	tests := []struct {
		current string
		want    bool
	}{
		{"v1.0.0", true},
		{"1.2.2", true},
		{"v1.2.3", false},
		{"v2.0.0", false},
		{"dev", false},
	}
	for _, tt := range tests {
		t.Run(tt.current, func(t *testing.T) {
			fr := newRelease(t, []byte("new"), true, "good")
			u, err := newUpdater(Config{Current: tt.current, HTTPClient: fr.client()}, &fakeVerifier{}, installed(t))
			require.NoError(t, err)

			status, err := u.Check(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.want, status.Available)
			assert.Equal(t, "v1.2.3", status.Latest.Version)
			assert.Equal(t, "notes", status.Latest.Notes)
			assert.Equal(t, tt.current, status.Current)
		})
	}
}

func TestBlockedSaysWhenTheBinaryCannotBeReplaced(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	fr := newRelease(t, []byte("new"), true, "good")
	target := installed(t)
	u, err := newUpdater(Config{Current: "v1.0.0", HTTPClient: fr.client()}, &fakeVerifier{}, target)
	require.NoError(t, err)
	assert.Empty(t, u.Blocked())

	require.NoError(t, os.Chmod(filepath.Dir(target), 0o500))
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(target), 0o700) })

	assert.NotEmpty(t, u.Blocked())
}
