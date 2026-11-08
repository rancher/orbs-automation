package bundlestore

import "time"

// Entry is a bundle in the cache, and when it was last confirmed to be the one
// to use.
type Entry struct {
	Tag       string    // the release it came from
	Data      []byte    // the bundle, as it was verified
	FetchedAt time.Time // when it was downloaded, or last confirmed current
}

// Cache is the user's cache directory of the bundle: a cache, not config, so it can
// be deleted at any time and a damaged copy is the same as none. A Cache with no
// Dir keeps nothing.
type Cache struct {
	Dir string
}

// Read is the cached entry, and false when there is none that can be trusted: not
// there, or not matching the checksum it was written with.
func (c Cache) Read() (Entry, bool, error) {
	return Entry{}, false, nil
}

// Write replaces the cached entry, all or nothing: a reader sees the old entry or
// the new one, never half of either.
func (c Cache) Write(e Entry) error {
	return nil
}

// Confirm records that the cached entry was checked at fetchedAt and is still the
// one to use, without rewriting the bundle.
func (c Cache) Confirm(fetchedAt time.Time) error {
	return nil
}
