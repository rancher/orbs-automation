package bundlestore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Cache is the user's cache directory of the bundle: a cache, not config, so it can
// be deleted at any time and a damaged copy is the same as none. A Cache with no
// Dir keeps nothing.
type Cache struct {
	Dir string
}

const (
	bundleFile = "bundle.yaml"
	metaFile   = "meta.json"
)

// meta is what the cache says about the bundle beside it. The hash is how a
// bundle that was edited, or that belongs to another record, is told from one that
// was written here.
type meta struct {
	Tag       string    `json:"tag"`
	FetchedAt time.Time `json:"fetchedAt"`
	SHA256    string    `json:"sha256"`
}

// Read is the cached entry, and false when there is none that can be trusted: not
// there, or not matching the checksum it was written with.
func (c Cache) Read() (Entry, bool, error) {
	if c.Dir == "" {
		return Entry{}, false, nil
	}
	raw, err := os.ReadFile(filepath.Join(c.Dir, metaFile))
	if errors.Is(err, os.ErrNotExist) {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, fmt.Errorf("read the bundle cache: %w", err)
	}
	var m meta
	if json.Unmarshal(raw, &m) != nil || m.Tag == "" {
		return Entry{}, false, nil
	}
	data, err := os.ReadFile(filepath.Join(c.Dir, bundleFile))
	if errors.Is(err, os.ErrNotExist) {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, fmt.Errorf("read the bundle cache: %w", err)
	}
	if hash(data) != m.SHA256 {
		return Entry{}, false, nil
	}
	return Entry{Tag: m.Tag, Data: data, FetchedAt: m.FetchedAt}, true, nil
}

// Write replaces the cached entry, all or nothing: a reader sees the old entry or
// the new one, never half of either. The record is written last, so a write that
// stops in between leaves a bundle that does not match its record, which Read
// treats as no entry.
func (c Cache) Write(e Entry) error {
	if c.Dir == "" {
		return nil
	}
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return fmt.Errorf("make the bundle cache: %w", err)
	}
	if err := writeFile(filepath.Join(c.Dir, bundleFile), e.Data); err != nil {
		return err
	}
	return c.writeMeta(meta{Tag: e.Tag, FetchedAt: e.FetchedAt, SHA256: hash(e.Data)})
}

// Confirm records that the cached entry was checked at fetchedAt and is still the
// one to use, without rewriting the bundle.
func (c Cache) Confirm(fetchedAt time.Time) error {
	e, ok, err := c.Read()
	if err != nil || !ok {
		return err
	}
	return c.writeMeta(meta{Tag: e.Tag, FetchedAt: fetchedAt, SHA256: hash(e.Data)})
}

func (c Cache) writeMeta(m meta) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(c.Dir, metaFile), raw)
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
