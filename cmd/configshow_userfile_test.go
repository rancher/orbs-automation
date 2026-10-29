package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigShowListsTheUserFileItLoaded(t *testing.T) {
	got := configShow(t, "output: text\n", nil)

	assert.Equal(t, "env", got["userFile"].Source, "configShow names the file through GERO_CONFIG")
	assert.True(t, strings.HasSuffix(got["userFile"].Value, "config.yaml"), got["userFile"].Value)
}

func TestConfigShowWithNoUserFile(t *testing.T) {
	got := configShow(t, "", nil)

	assert.Empty(t, got["userFile"].Value)
	assert.Equal(t, "default", got["userFile"].Source)
}
