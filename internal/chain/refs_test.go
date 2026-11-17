package chain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The refs of a document are followed through v1alpha2.Ref and RefList, which
// check the key against the table the rules across documents use, so the join
// never has a way of reading a ref of its own (docs/design/CHAIN_MODEL.md).
func TestTheGraphReadsNoRefsButThroughTheResolver(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		assert.NotContains(t, string(src), "Refs[", "%s reads a ref by hand", f)
	}
}
