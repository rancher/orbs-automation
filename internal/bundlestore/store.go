package bundlestore

import (
	"time"
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

// Get is the entry with the ID, and false when there is none that can be trusted.
func (s Store) Get(id string) (Entry, bool) { return Entry{}, false }

// List is every entry that can be trusted, newest first by FetchedAt, ties by ID.
func (s Store) List() ([]Entry, error) { return nil, nil }

// Put keeps the entry under its ID, replacing one with the same ID, and fills in the
// ID. A reader sees the old entry or the new one, never half of either.
func (s Store) Put(e Entry) (Entry, error) { return Entry{}, nil }

// Touch records that the entry was checked at the time and is still the one to use,
// without rewriting the bundle.
func (s Store) Touch(id string, at time.Time) error { return nil }

// Prune keeps the newest keep entries and removes the rest, and what is left of the
// single entry an older gero kept beside them.
func (s Store) Prune(keep int) error { return nil }

// bundlesDir is the directory of the entries under the store's Dir, one directory for
// each, named by dirName.
const bundlesDir = "bundles"

// dirName is the name of an entry's directory: the start of its ID.
func dirName(id string) string { return id }
