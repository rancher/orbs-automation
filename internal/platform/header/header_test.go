package header

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeHeader(t *testing.T) {
	art, err := headerFromBytes(headerBytes)

	require.NoError(t, err)
	assert.NotEmpty(t, art)
}

func TestPrintIsTheArtAsText(t *testing.T) {
	art := Print()

	assert.True(t, utf8.ValidString(art), "the art is text, not the compressed bytes")
	assert.Contains(t, art, "█")
	assert.Greater(t, strings.Count(art, "\n"), 10, "it is many lines")
}

func TestPrintOfBytesThatAreNotTheArtIsEmpty(t *testing.T) {
	_, err := headerFromBytes([]byte("not base64!"))

	assert.Error(t, err)
}
