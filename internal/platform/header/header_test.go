package header

import (
	"encoding/base64"
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

func TestBytesThatAreNotTheArtDoNotDecode(t *testing.T) {
	notGzip := base64.StdEncoding.EncodeToString([]byte("plain text"))
	for name, encoded := range map[string]string{"not base64": "not base64!", "not gzip": notGzip, "empty": ""} {
		_, err := headerFromBytes([]byte(encoded))

		assert.Error(t, err, name)
		assert.Empty(t, render([]byte(encoded)), name)
	}
}

func TestRenderOfTheArtIsPrint(t *testing.T) {
	assert.Equal(t, Print(), render(headerBytes))
	assert.NotEmpty(t, Print())
}
