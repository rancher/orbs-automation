package embedded

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// header is what tools/compilebundle writes with no flags: the same bytes for the
// same config, whoever builds and whenever.
var header = bundle.Header{GeroVersion: "dev", SourceCommit: "none", CompiledAt: time.Unix(0, 0).UTC()}

func TestTheEmbeddedBundleParses(t *testing.T) {
	h, fsys, err := bundle.Parse(Bytes())

	require.NoError(t, err)
	assert.Equal(t, "dev", h.GeroVersion)
	assert.NotNil(t, fsys)
}

// TestTheEmbeddedBundleIsTheConfigDirectoryCompiled fails when config/ changed and
// the embedded file was not made again: run go generate ./internal/bundle/embedded.
func TestTheEmbeddedBundleIsTheConfigDirectoryCompiled(t *testing.T) {
	want, err := bundle.Compile(os.DirFS("../../../config"), header)
	require.NoError(t, err)

	// Not assert.Equal: a diff of two bundles is the whole of config/ twice.
	assert.True(t, bytes.Equal(want, Bytes()),
		"the embedded bundle (%d bytes) is not config/ compiled (%d bytes): run go generate ./internal/bundle/embedded",
		len(Bytes()), len(want))
}
