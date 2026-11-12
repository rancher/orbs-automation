package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShort(t *testing.T) {
	for in, want := range map[string]string{
		"abcdef1234567": "abcdef1",
		"abcdef1":       "abcdef1",
		"abc":           "abc",
		"":              "",
	} {
		assert.Equal(t, want, Short(in), "Short(%q)", in)
	}
}
