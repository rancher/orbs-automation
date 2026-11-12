package embedded

import (
	"os"
	"testing"

	"github.com/rancher/orbs-automation/internal/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTheEmbeddedFilesAreTheConfigDirectory fails when a YAML file of config/ is not
// embedded, such as one in a subdirectory, which the pattern in config/embed.go does
// not take.
func TestTheEmbeddedFilesAreTheConfigDirectory(t *testing.T) {
	embedded, err := bundle.Digest(FS())
	require.NoError(t, err)
	onDisk, err := bundle.Digest(os.DirFS("../../../config"))
	require.NoError(t, err)

	assert.Equal(t, onDisk, embedded, "config/ holds a YAML file that is not embedded")
}

func TestTheEmbeddedConfigCompilesToABundleThatParses(t *testing.T) {
	data, err := Compile()
	require.NoError(t, err, "the config in config/ must validate")

	h, fsys, err := bundle.Parse(data)

	require.NoError(t, err)
	assert.Equal(t, "dev", h.GeroVersion)
	assert.NotEmpty(t, h.InputsDigest)
	assert.NotNil(t, fsys)
}

func TestTheEmbeddedBundleIsCompiledOnceAndTheSameEveryTime(t *testing.T) {
	first, err := Bytes()
	require.NoError(t, err)
	second, err := Bytes()
	require.NoError(t, err)
	again, err := Compile()
	require.NoError(t, err)

	assert.Equal(t, string(first), string(second))
	assert.Equal(t, string(first), string(again), "compiling is repeatable")
}
