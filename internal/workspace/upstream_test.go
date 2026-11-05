package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const upstreamUserFile = `upstream: upstream
repos:
  overrides:
    rancher/scc-operator:
      upstream: rancher
`

func TestUpstreamFor(t *testing.T) {
	w := Open(settings(t, upstreamUserFile, nil, nil), t.TempDir())

	assert.Equal(t, "rancher", w.UpstreamFor("rancher/scc-operator"), "a per-repo override wins")
	assert.Equal(t, "rancher", w.UpstreamFor("Rancher/SCC-Operator"), "slugs compare case-insensitively")
	assert.Equal(t, "upstream", w.UpstreamFor("rancher/rancher"), "otherwise the global setting")
}

func TestUpstreamForUnset(t *testing.T) {
	w := Open(settings(t, "", nil, nil), t.TempDir())
	assert.Empty(t, w.UpstreamFor("rancher/rancher"))
}
