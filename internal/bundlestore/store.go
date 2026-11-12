package bundlestore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rancher/orbs-automation/internal/bundle"
)

// Origin says where a stored bundle came from.
type Origin string

const (
	// OriginRelease is a bundle downloaded from a release.
	OriginRelease Origin = "release"
	// OriginEmbedded is the bundle compiled from the config embedded in the binary.
	OriginEmbedded Origin = "embedded"
)

// Entry is a bundle in the store, and when it was last confirmed to be the one to
// use.
type Entry struct {
	// ID is what identifies the bundle: the hash of the revision of the compiler and
	// the digest of the files it was made from (bundle.Header.ID), or, for a bundle
	// made before the digest was written, a name from the hash of its bytes. Put works
	// it out.
	ID        string
	Tag       string    // the release it came from; the running gero's for an embedded one
	Digest    string    // the digest of the files it was made from, empty for an old bundle
	Origin    Origin    // empty is a release
	Data      []byte    // the bundle, as it was verified
	FetchedAt time.Time // when it was downloaded, compiled, or last confirmed current
}

// Store is the user's cache directory of bundles: several at once, each under its ID.
// It is a cache, not config, so it can be deleted at any time, and a damaged entry is
// the same as none. A Store with no Dir keeps nothing.
type Store struct {
	Dir string
}

// bundlesDir is the directory of the entries under the store's Dir, one directory for
// each, named by dirName.
const bundlesDir = "bundles"

// The files of an entry's directory.
const (
	bundleFile = "bundle.yaml"
	metaFile   = "meta.json"
)

// dirName is the name of an entry's directory: the start of its ID.
func dirName(id string) string { return id[:min(len(id), 16)] }

// record is what the store says of an entry beside its bundle. The hash is how a
// bundle that was edited, or that belongs to another record, is told from one that
// was written here.
type record struct {
	ID        string    `json:"id"`
	Tag       string    `json:"tag"`
	Digest    string    `json:"digest,omitempty"`
	Origin    Origin    `json:"origin"`
	FetchedAt time.Time `json:"fetchedAt"`
	SHA256    string    `json:"sha256"`
}

// entryID is the ID of a bundle: the one its header gives, or, for one that has none
// or that this gero cannot read, a name from the hash of its bytes.
func entryID(data []byte) string {
	if h, _, err := bundle.Parse(data); err == nil && h.ID() != "" {
		return h.ID()
	}
	return "legacy-" + hash(data)[:16]
}

// Get is the entry with the ID, and false when there is none that can be trusted.
func (s Store) Get(id string) (Entry, bool) {
	if s.Dir == "" || id == "" {
		return Entry{}, false
	}
	e, ok := readEntry(filepath.Join(s.Dir, bundlesDir, dirName(id)))
	return e, ok && e.ID == id
}

// List is every entry that can be trusted, newest first by FetchedAt, ties by ID.
func (s Store) List() ([]Entry, error) {
	if s.Dir == "" {
		return nil, nil
	}
	dirs, err := os.ReadDir(filepath.Join(s.Dir, bundlesDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the bundle store: %w", err)
	}
	var out []Entry
	for _, d := range dirs {
		if e, ok := readEntry(filepath.Join(s.Dir, bundlesDir, d.Name())); ok {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b Entry) int {
		if c := b.FetchedAt.Compare(a.FetchedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out, nil
}

// Put keeps the entry under its ID, replacing one with the same ID, and fills in the
// ID. A reader sees the old entry or the new one, never half of either. Data that is
// not a bundle is refused: a bundle in a format this gero cannot read is not.
func (s Store) Put(e Entry) (Entry, error) {
	h, _, err := bundle.Parse(e.Data)
	if err != nil && !errors.Is(err, bundle.ErrUnsupported) {
		return Entry{}, fmt.Errorf("keep the bundle of %s: %w", e.Tag, err)
	}
	e.ID, e.Digest = entryID(e.Data), h.InputsDigest
	if e.Origin == "" {
		e.Origin = OriginRelease
	}
	if s.Dir == "" {
		return e, nil
	}
	dir := filepath.Join(s.Dir, bundlesDir, dirName(e.ID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Entry{}, fmt.Errorf("make the bundle store: %w", err)
	}
	if err := writeFile(filepath.Join(dir, bundleFile), e.Data); err != nil {
		return Entry{}, err
	}
	return e, writeRecord(dir, record{ID: e.ID, Tag: e.Tag, Digest: e.Digest, Origin: e.Origin, FetchedAt: e.FetchedAt, SHA256: hash(e.Data)})
}

// Touch records that the entry was checked at the time and is still the one to use,
// without rewriting the bundle.
func (s Store) Touch(id string, at time.Time) error {
	e, ok := s.Get(id)
	if !ok {
		return nil
	}
	return writeRecord(filepath.Join(s.Dir, bundlesDir, dirName(id)),
		record{ID: e.ID, Tag: e.Tag, Digest: e.Digest, Origin: e.Origin, FetchedAt: at, SHA256: hash(e.Data)})
}

// Prune keeps the newest keep entries and removes the rest, an entry that cannot be
// trusted, and what is left of the single entry an older gero kept beside them.
func (s Store) Prune(keep int) error {
	if s.Dir == "" {
		return nil
	}
	list, err := s.List()
	if err != nil {
		return err
	}
	kept := map[string]bool{}
	for _, e := range list[:min(max(keep, 0), len(list))] {
		kept[dirName(e.ID)] = true
	}
	var errs []error
	dirs, err := os.ReadDir(filepath.Join(s.Dir, bundlesDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, err)
	}
	for _, d := range dirs {
		if !kept[d.Name()] {
			errs = append(errs, os.RemoveAll(filepath.Join(s.Dir, bundlesDir, d.Name())))
		}
	}
	for _, old := range []string{bundleFile, metaFile} {
		if err := os.Remove(filepath.Join(s.Dir, old)); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// readEntry is the entry in a directory, and false when it cannot be trusted: no
// record or bundle, a bundle that does not match its record's hash, a bundle that is
// not one (a newer format is one this gero cannot read, which is not damage), or one
// that is not the bundle its ID and directory name say.
func readEntry(dir string) (Entry, bool) {
	raw, err := os.ReadFile(filepath.Join(dir, metaFile))
	if err != nil {
		return Entry{}, false
	}
	var rec record
	if json.Unmarshal(raw, &rec) != nil || rec.ID == "" || dirName(rec.ID) != filepath.Base(dir) {
		return Entry{}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, bundleFile))
	if err != nil || hash(data) != rec.SHA256 {
		return Entry{}, false
	}
	if _, _, err := bundle.Parse(data); err != nil && !errors.Is(err, bundle.ErrUnsupported) {
		return Entry{}, false
	}
	if entryID(data) != rec.ID {
		return Entry{}, false
	}
	return Entry{ID: rec.ID, Tag: rec.Tag, Digest: rec.Digest, Origin: rec.Origin, Data: data, FetchedAt: rec.FetchedAt}, true
}

func writeRecord(dir string, rec record) error {
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, metaFile), raw)
}

func hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// writeFile writes through a temporary file in the same directory and renames it,
// so the file is never half written.
func writeFile(path string, data []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("write the bundle cache: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write the bundle cache: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("write the bundle cache: %w", err)
	}
	if err = os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("write the bundle cache: %w", err)
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write the bundle cache: %w", err)
	}
	return nil
}
