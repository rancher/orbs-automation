package yamledit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lockYAML = `chart-versions:
  "2.16":
    upstream-refs:
      dev:
        charts:
          commit: aaa111
      prod:
        charts:
          commit: bbb222
  "2.15":
    upstream-refs:
      prod:
        charts:
          commit: ccc333
additionalCharts:
  - name: crd
    upstreamOptions:
      url: https://example.com/crd.tgz   # the crd chart
  - name: main
    upstreamOptions:
      url: https://example.com/main.tgz
dotted.key:
  inner: one
`

func TestGetWithThePathSyntaxOfYq(t *testing.T) {
	tests := []struct {
		name, path, want string
	}{
		{"quoted keys for a version with a dot", `.chart-versions."2.16".upstream-refs.dev.charts.commit`, "aaa111"},
		{"the same without the leading dot", `chart-versions."2.15".upstream-refs.prod.charts.commit`, "ccc333"},
		{"a list item", ".additionalCharts[0].upstreamOptions.url", "https://example.com/crd.tgz"},
		{"another list item", ".additionalCharts[1].name", "main"},
		{"a quoted key with a dot at the top", `."dotted.key".inner`, "one"},
		{"a quoted key that needed no quotes", `.chart-versions."2.16".upstream-refs."prod".charts.commit`, "bbb222"},
		{"a leading dot on a plain path", ".additionalCharts[0].name", "crd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Get([]byte(lockYAML), tt.path)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetOfAMappingIsNotAScalar(t *testing.T) {
	_, err := Get([]byte(lockYAML), ".chart-versions")

	assert.ErrorIs(t, err, ErrNotScalar)
}

func TestGetAListAtTheRoot(t *testing.T) {
	got, err := Get([]byte("- one\n- two\n"), "[1]")

	require.NoError(t, err)
	assert.Equal(t, "two", got)
}

func TestGetPathErrors(t *testing.T) {
	tests := []struct {
		name, path string
		want       error
	}{
		{"an empty step", "chart-versions..x", ErrInvalidPath},
		{"a quote left open", `chart-versions."2.16`, ErrInvalidPath},
		{"an index that is not a number", "additionalCharts[x].name", ErrInvalidPath},
		{"a bracket left open", "additionalCharts[0.name", ErrInvalidPath},
		{"text after a quote", `chart-versions."2.16"x`, ErrInvalidPath},
		{"an index past the end", "additionalCharts[5].name", ErrKeyNotFound},
		{"an index into a mapping", "chart-versions[0]", ErrKeyNotFound},
		{"a key of a list", "additionalCharts.name", ErrKeyNotFound},
		{"a missing quoted key", `chart-versions."2.99".x`, ErrKeyNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Get([]byte(lockYAML), tt.path)

			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestSetWithThePathSyntaxOfYqChangesOnlyThatValue(t *testing.T) {
	out, old, err := Set([]byte(lockYAML), `.chart-versions."2.15".upstream-refs.prod.charts.commit`, "ddd444")

	require.NoError(t, err)
	assert.Equal(t, "ccc333", old)
	assert.Equal(t, replaceOnce(lockYAML, "ccc333", "ddd444"), string(out))

	out, old, err = Set([]byte(lockYAML), ".additionalCharts[0].upstreamOptions.url", "https://example.com/new.tgz")

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/crd.tgz", old)
	assert.Equal(t, replaceOnce(lockYAML, "https://example.com/crd.tgz", "https://example.com/new.tgz"), string(out), "the comment after it stays")
}

func replaceOnce(s, old, repl string) string { return strings.Replace(s, old, repl, 1) }
