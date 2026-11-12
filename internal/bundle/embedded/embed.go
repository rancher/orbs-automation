// Package embedded holds the central config that is compiled into the binary, so a
// gero with no checkout and no network still has config
// (docs/specs/CENTRAL_BUNDLE.md). It is the config/ directory itself, embedded as
// files; the bundle is made from it by bundle.Compile when it is wanted, so there is
// no generated copy to keep in step with config/.
package embedded

import (
	"io/fs"
	"sync"
	"time"

	"github.com/rancher/orbs-automation/config"
	"github.com/rancher/orbs-automation/internal/bundle"
)

// Header is the header of the embedded bundle: the compile tool's default, not a
// release's, so the same files make the same bytes. The release of the binary is
// what it was built as, not what this header says.
var Header = bundle.Header{GeroVersion: "dev", SourceCommit: "none", CompiledAt: time.Unix(0, 0).UTC()}

// FS is the embedded config directory, as the files it holds.
func FS() fs.FS { return config.Files }

// Compile is the bundle of the embedded config, in the format bundle.Parse reads.
func Compile() ([]byte, error) { return bundle.Compile(config.Files, Header) }

var (
	once    sync.Once
	bundled []byte
	err     error
)

// Bytes is the embedded bundle, compiled the first time it is asked for.
func Bytes() ([]byte, error) {
	once.Do(func() { bundled, err = Compile() })
	return bundled, err
}
