package yamledit

import (
	"strings"
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

// set runs Set and checks the property every edit must keep: reading the path
// back gives the new value, and the first return is what was there.
func set(t *testing.T, src, path, value string) (string, string) {
	t.Helper()
	out, old, err := Set([]byte(src), path, value)
	require.NoError(t, err)
	got, err := Get(out, path)
	require.NoError(t, err)
	assert.Equal(t, value, got, "the value reads back")
	return string(out), old
}

func TestSetKeepsEverythingButTheValue(t *testing.T) {
	out, old := set(t, buildYAML, "defaultSccOperatorImage", "rancher/scc-operator:v0.5.3")

	assert.Equal(t, "rancher/scc-operator:v0.5.2", old)
	want := strings.Replace(buildYAML, `"rancher/scc-operator:v0.5.2"`, `"rancher/scc-operator:v0.5.3"`, 1)
	assert.Equal(t, want, out, "a double-quoted value stays double-quoted, and the rest is byte for byte the same")
}

func TestSetStyles(t *testing.T) {
	tests := []struct {
		name, src, path, value, want string
	}{
		{"plain stays plain", "a: v1\nb: 2\n", "a", "v2", "a: v2\nb: 2\n"},
		{"single-quoted stays single-quoted", "a: 'v1'\n", "a", "v2", "a: 'v2'\n"},
		{"double-quoted stays double-quoted", "a: \"v1\"\n", "a", "v2", "a: \"v2\"\n"},
		{"a trailing comment survives", "a: v1   # keep me\n", "a", "v2", "a: v2   # keep me\n"},
		{"a leading comment survives", "# top\na: v1\n", "a", "v2", "# top\na: v2\n"},
		{"a nested path keeps its indentation", "g:\n  k:\n    t: v1\n  other: x\n", "g.k.t", "v2", "g:\n  k:\n    t: v2\n  other: x\n"},
		{"a value in a flow mapping", "g: {t: v1, u: x}\n", "g.t", "v2", "g: {t: v2, u: x}\n"},
		{"no final newline", "a: v1", "a", "v2", "a: v2"},
		{"windows line endings", "a: v1\r\nb: x\r\n", "a", "v2", "a: v2\r\nb: x\r\n"},
		{"text before the value on the line is not ascii", "é: v1\n", "é", "v2", "é: v2\n"},
		{"only the first document", "a: v1\n---\na: v1\n", "a", "v2", "a: v2\n---\na: v1\n"},
		{"a plain value that needs quotes is quoted", "a: v1\n", "a", "x: y", "a: 'x: y'\n"},
		{"a string that looks like a number stays a string", "a: v1\n", "a", "12", "a: \"12\"\n"},
		{"a string that looks like a boolean stays a string", "a: v1\n", "a", "true", "a: \"true\"\n"},
		{"a single quote in a single-quoted value", "a: 'v1'\n", "a", "it's", "a: 'it''s'\n"},
		{"a double quote in a double-quoted value", "a: \"v1\"\n", "a", "say \"hi\"", "a: \"say \\\"hi\\\"\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _ := set(t, tt.src, tt.path, tt.value)
			assert.Equal(t, tt.want, out)
		})
	}
}

func TestSetWithTheSameValueChangesNothing(t *testing.T) {
	out, old, err := Set([]byte(buildYAML), "webhookVersion", "v0.9.0")
	require.NoError(t, err)
	assert.Equal(t, "v0.9.0", old)
	assert.Equal(t, buildYAML, string(out))
}

func TestSetDoesNotModifyItsInput(t *testing.T) {
	src := []byte(buildYAML)
	_, _, err := Set(src, "webhookVersion", "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, buildYAML, string(src))
}

func TestSetRefusesWhatItCannotReplaceInPlace(t *testing.T) {
	tests := []struct{ name, src string }{
		{"a literal block scalar", "a: |\n  v1\n"},
		{"a folded block scalar", "a: >\n  v1\n"},
		{"an anchored value", "a: &x v1\nb: *x\n"},
		{"a tagged value", "a: !!str v1\n"},
		{"a plain value over several lines", "a: first\n  second\n"},
		{"a double-quoted value with an escape", "a: \"v\\n1\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Set([]byte(tt.src), "a", "v2")
			assert.ErrorIs(t, err, ErrUnsupported)
		})
	}
}

func TestSetRefusesAMultiLineValue(t *testing.T) {
	_, _, err := Set([]byte("a: v1\n"), "a", "two\nlines")
	assert.ErrorIs(t, err, ErrUnsupported)
}

func TestSetErrors(t *testing.T) {
	_, _, err := Set([]byte(buildYAML), "nope", "x")
	assert.ErrorIs(t, err, ErrKeyNotFound)
	_, _, err = Set([]byte(buildYAML), "global", "x")
	assert.ErrorIs(t, err, ErrNotScalar)
	_, _, err = Set([]byte("a: [unclosed\n"), "a", "x")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrKeyNotFound)
}

func TestADuplicateKeyIsAmbiguous(t *testing.T) {
	src := "a: v1\nb: x\na: v2\n"
	_, _, err := Set([]byte(src), "a", "v3")
	assert.ErrorIs(t, err, ErrAmbiguous)
	_, err = Get([]byte(src), "a")
	assert.ErrorIs(t, err, ErrAmbiguous, "reading is as strict as writing, so they agree")
}
