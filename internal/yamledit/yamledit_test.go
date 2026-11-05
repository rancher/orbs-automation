package yamledit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const buildYAML = `# build.yaml
defaultShellVersion: rancher/shell:v0.6.2   # comment after the value
defaultSccOperatorImage: "rancher/scc-operator:v0.5.2"
webhookVersion: 'v0.9.0'
replicas: 3
enabled: true
global:
  kubectl:
    tag: v1.2.3
  list:
    - a
    - b
`

func TestGet(t *testing.T) {
	tests := []struct {
		name, path, want string
	}{
		{"a plain top-level value", "defaultShellVersion", "rancher/shell:v0.6.2"},
		{"a double-quoted value comes back unquoted", "defaultSccOperatorImage", "rancher/scc-operator:v0.5.2"},
		{"a single-quoted value comes back unquoted", "webhookVersion", "v0.9.0"},
		{"a number is its text", "replicas", "3"},
		{"a boolean is its text", "enabled", "true"},
		{"a nested path", "global.kubectl.tag", "v1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Get([]byte(buildYAML), tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetErrors(t *testing.T) {
	tests := []struct {
		name, src, path string
		want            error
	}{
		{"a missing key", buildYAML, "nope", ErrKeyNotFound},
		{"a missing nested key", buildYAML, "global.kubectl.nope", ErrKeyNotFound},
		{"a path through a scalar", buildYAML, "replicas.x", ErrKeyNotFound},
		{"a mapping is not a scalar", buildYAML, "global", ErrNotScalar},
		{"a list is not a scalar", buildYAML, "global.list", ErrNotScalar},
		{"an empty document", "", "a", ErrKeyNotFound},
		{"a document that is not a mapping", "- a\n- b\n", "a", ErrKeyNotFound},
		{"an empty path", buildYAML, "", ErrKeyNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Get([]byte(tt.src), tt.path)
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestGetInvalidYAML(t *testing.T) {
	_, err := Get([]byte("a: [unclosed\n"), "a")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrKeyNotFound, "a broken file is not a missing key")
}

func TestGetReadsTheFirstDocument(t *testing.T) {
	got, err := Get([]byte("a: one\n---\na: two\n"), "a")
	require.NoError(t, err)
	assert.Equal(t, "one", got)
}
