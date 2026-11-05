package ship

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var marker = Marker{Source: "rancher/scc-operator", Consumer: "rancher/rancher", Target: "release/v2.15", Version: "v0.5.3"}

func TestMarkerString(t *testing.T) {
	assert.Equal(t, `<!-- gero:ship v1
source: rancher/scc-operator
consumer: rancher/rancher
target: release/v2.15
version: v0.5.3
-->`, marker.String())
}

func TestMarkerRoundTrips(t *testing.T) {
	got, ok := ParseMarker(marker.String())
	require.True(t, ok)
	assert.Equal(t, marker, got)
}

func TestParseMarkerInABody(t *testing.T) {
	body := "## Summary\nUpdate to `v0.5.3`\n\n" + marker.String() + "\n\nRun: https://example/run/1\n"
	got, ok := ParseMarker(body)
	require.True(t, ok)
	assert.Equal(t, marker, got)
}

func TestParseMarkerToleratesWindowsLineEndings(t *testing.T) {
	got, ok := ParseMarker(strings.ReplaceAll(marker.String(), "\n", "\r\n"))
	require.True(t, ok)
	assert.Equal(t, marker, got)
}

func TestParseMarkerIgnoresFieldsItDoesNotKnow(t *testing.T) {
	body := "<!-- gero:ship v1\nsource: a/b\nconsumer: c/d\nextra: later\ntarget: main\nversion: v1\n-->"
	got, ok := ParseMarker(body)
	require.True(t, ok)
	assert.Equal(t, Marker{Source: "a/b", Consumer: "c/d", Target: "main", Version: "v1"}, got)
}

func TestParseMarkerNotAMarker(t *testing.T) {
	for name, body := range map[string]string{
		"no marker":           "just a body",
		"empty":               "",
		"another comment":     "<!-- something else -->",
		"a later format":      "<!-- gero:ship v2\nsource: a/b\nconsumer: c/d\ntarget: main\nversion: v1\n-->",
		"a missing field":     "<!-- gero:ship v1\nsource: a/b\nconsumer: c/d\ntarget: main\n-->",
		"an unclosed comment": "<!-- gero:ship v1\nsource: a/b\nconsumer: c/d\ntarget: main\nversion: v1\n",
		"an empty value":      "<!-- gero:ship v1\nsource: \nconsumer: c/d\ntarget: main\nversion: v1\n-->",
	} {
		_, ok := ParseMarker(body)
		assert.False(t, ok, name)
	}
}

func TestSameBump(t *testing.T) {
	newer := marker
	newer.Version = "v0.5.4"
	assert.True(t, marker.SameBump(newer), "the version is data, not part of the key")

	other := marker
	other.Target = "main"
	assert.False(t, marker.SameBump(other))
	other = marker
	other.Consumer = "rancher/charts"
	assert.False(t, marker.SameBump(other))
	other = marker
	other.Source = "rancher/other"
	assert.False(t, marker.SameBump(other))
}
