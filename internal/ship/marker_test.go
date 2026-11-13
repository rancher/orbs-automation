package ship

import (
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
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
key: rancher/scc-operator|rancher/rancher|release/v2.15
-->`, marker.String())
}

func TestMarkerKeyIsTheOverlapKeyInOneString(t *testing.T) {
	assert.Equal(t, "rancher/scc-operator|rancher/rancher|release/v2.15", marker.Key())
	newer := marker
	newer.Version = "v0.5.4"
	assert.Equal(t, marker.Key(), newer.Key(), "the version is data, not part of the key")
	other := marker
	other.Target = "main"
	assert.NotEqual(t, marker.Key(), other.Key())
}

func TestRecordedKeyIsWhatTheMarkerSays(t *testing.T) {
	got, ok := RecordedKey("## Summary\n\n" + marker.String() + "\n")
	assert.True(t, ok)
	assert.Equal(t, "rancher/scc-operator|rancher/rancher|release/v2.15", got)
}

func TestRecordedKeyOfAMarkerWithoutOneIsNotThere(t *testing.T) {
	old := "<!-- gero:ship v1\nsource: a/b\nconsumer: c/d\ntarget: main\nversion: v1\n-->"
	_, ok := RecordedKey(old)
	assert.False(t, ok, "a marker from before the key is still a marker")
	_, parsed := ParseMarker(old)
	assert.True(t, parsed)
	_, ok = RecordedKey("just a body")
	assert.False(t, ok)
}

func TestAMarkerNeverHasTheEndOfACommentInItsKey(t *testing.T) {
	assert.NotContains(t, strings.TrimSuffix(marker.String(), "-->"), "-->")
	assert.NotContains(t, marker.Key(), "--")
}

func headed(head string, m Marker, recorded string) pulls.PR {
	body := m.String()
	if recorded != "" {
		body = strings.ReplaceAll(body, "key: "+m.Key(), "key: "+recorded)
	}
	return pulls.PR{Number: 7, HeadRef: head, Body: body}
}

func TestMarkerWarningsHasNoneWhenTheBranchAndTheKeyAgree(t *testing.T) {
	assert.Empty(t, markerWarnings(headed("bot/scc-operator-v0.5.3-v2.15", marker, ""), marker))
	assert.Empty(t, markerWarnings(headed("dan/whatever-the-prefix-v0.5.3-v2.15", marker, ""), marker), "the prefix is anyone's")
}

func TestMarkerWarningsSaysWhenTheBranchWasRenamed(t *testing.T) {
	got := markerWarnings(headed("dan/my-own-name", marker, ""), marker)
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "#7")
	assert.Contains(t, got[0], "dan/my-own-name")
}

func TestMarkerWarningsSaysWhenTheRecordedKeyIsNotTheFieldsKey(t *testing.T) {
	got := markerWarnings(headed("bot/scc-operator-v0.5.3-v2.15", marker, "rancher/other|rancher/rancher|release/v2.15"), marker)
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "#7")
	assert.Contains(t, got[0], "key")
}

func TestMarkerWarningsAreSilentWithoutAHeadOrAKey(t *testing.T) {
	assert.Empty(t, markerWarnings(headed("", marker, ""), marker), "no head to compare")
	old := pulls.PR{Number: 7, HeadRef: "bot/scc-operator-v0.5.3-v2.15",
		Body: "<!-- gero:ship v1\nsource: rancher/scc-operator\nconsumer: rancher/rancher\ntarget: release/v2.15\nversion: v0.5.3\n-->"}
	assert.Empty(t, markerWarnings(old, marker), "a marker from before the key says nothing to disagree with")
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
