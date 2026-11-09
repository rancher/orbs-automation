package regexedit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chartValues is BRO's chart values.yaml in shape: a placeholder that is not YAML, and a
// nested tag to read.
const chartValues = `image:
  repository: rancher/backup-restore-operator
  tag: %TAG%

global:
  kubectl:
    repository: rancher/kuberlr-kubectl
    tag: v8.2.0  # the one gero reads
`

// kubectlTag finds the tag under kubectl, whatever else is in the file.
const kubectlTag = `kubectl:\n(?:[ \t]+.*\n)*?[ \t]+tag: (\S+)`

func TestGetReadsTheGroupOfTheMatch(t *testing.T) {
	tests := []struct {
		name, src, pattern, want string
	}{
		{"a key in a file that is not YAML", chartValues, kubectlTag, "v8.2.0"},
		{"a line anchored at both ends", "name: x\nversion: 1.2.3\nother: y\n", `^version: (.*)$`, "1.2.3"},
		{"a line anchor is a line anchor, not the file's", "a: 1\nb: 2\nc: 3\n", `^b: (\d)$`, "2"},
		{"a value in quotes", "tag: \"v1.0.0\"\n", `tag: "([^"]*)"`, "v1.0.0"},
		{"a dockerfile line", "FROM x\nARG TOOL_VERSION=v0.4.2\nRUN y\n", `ARG TOOL_VERSION=(\S+)`, "v0.4.2"},
		{"a group that matches nothing is an empty value", "tag:\n", `tag:(\S*)`, ""},
		{"a file with windows line endings", "a: 1\r\nversion: 2.0\r\n", `version: ([\d.]+)`, "2.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Get([]byte(tt.src), tt.pattern)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetWhenThePatternMatchesNowhere(t *testing.T) {
	_, err := Get([]byte("a: 1\n"), `version: (\S+)`)

	require.ErrorIs(t, err, ErrNoMatch)
	assert.Contains(t, err.Error(), `version: (\S+)`, "the message has the pattern")
}

func TestGetWhenThePatternMatchesMoreThanOnce(t *testing.T) {
	_, err := Get([]byte("tag: v1\ntag: v2\n"), `tag: (\S+)`)

	require.ErrorIs(t, err, ErrAmbiguous)
	assert.Contains(t, err.Error(), "anchor", "it says how to name one place")
}

func TestAPatternMustHaveExactlyOneGroup(t *testing.T) {
	tests := map[string]string{
		"none":             `tag: \S+`,
		"two":              `(tag): (\S+)`,
		"does not compile": `tag: (\S+`,
	}
	for name, pattern := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Get([]byte("tag: v1\n"), pattern)
			require.ErrorIs(t, err, ErrInvalidPattern)
			assert.Contains(t, err.Error(), pattern)

			_, _, err = Set([]byte("tag: v1\n"), pattern, "v2")
			require.ErrorIs(t, err, ErrInvalidPattern)
		})
	}
}

func TestAGroupThatDoesNotTakePartIsNoMatch(t *testing.T) {
	_, err := Get([]byte("a: 1\n"), `a: 1|(b)`)

	require.ErrorIs(t, err, ErrNoMatch)
}

func TestSetReplacesOnlyTheGroup(t *testing.T) {
	out, old, err := Set([]byte(chartValues), kubectlTag, "v9.0.0")

	require.NoError(t, err)
	assert.Equal(t, "v8.2.0", old)
	assert.Equal(t, strings.Replace(chartValues, "v8.2.0", "v9.0.0", 1), string(out), "the placeholder and the comment are as they were")
}

func TestSetWithTheSameValueChangesNothing(t *testing.T) {
	out, old, err := Set([]byte(chartValues), kubectlTag, "v8.2.0")

	require.NoError(t, err)
	assert.Equal(t, "v8.2.0", old)
	assert.Equal(t, chartValues, string(out))
}

func TestSetDoesNotModifyItsInput(t *testing.T) {
	src := []byte("version: 1.0.0\n")

	_, _, err := Set(src, `version: (\S+)`, "2.0.0")

	require.NoError(t, err)
	assert.Equal(t, "version: 1.0.0\n", string(src))
}

func TestSetOfAValueOfADifferentLength(t *testing.T) {
	out, _, err := Set([]byte("a: 1\nversion: 1.0.0\nz: 9\n"), `^version: (.*)$`, "10.20.30-rc.1")

	require.NoError(t, err)
	assert.Equal(t, "a: 1\nversion: 10.20.30-rc.1\nz: 9\n", string(out))
}

func TestSetOfAnEmptyGroupInsertsTheValue(t *testing.T) {
	out, old, err := Set([]byte("tag:\nz: 1\n"), `tag:(\S*)`, " v1")

	require.NoError(t, err)
	assert.Empty(t, old)
	assert.Equal(t, "tag: v1\nz: 1\n", string(out))
}

func TestSetRefusesAValueOfSeveralLines(t *testing.T) {
	for _, value := range []string{"a\nb", "a\r\nb", "a\n"} {
		_, _, err := Set([]byte("tag: v1\n"), `tag: (\S+)`, value)

		require.ErrorIs(t, err, ErrInvalidValue, "%q", value)
	}
}

func TestSetWhenThePatternMatchesNowhereOrMoreThanOnce(t *testing.T) {
	_, _, err := Set([]byte("a: 1\n"), `tag: (\S+)`, "v2")
	require.ErrorIs(t, err, ErrNoMatch)

	_, _, err = Set([]byte("tag: v1\ntag: v2\n"), `tag: (\S+)`, "v3")
	require.ErrorIs(t, err, ErrAmbiguous)
}

func TestSetThenGetAgree(t *testing.T) {
	out, _, err := Set([]byte(chartValues), kubectlTag, "v9.1.2")
	require.NoError(t, err)

	got, err := Get(out, kubectlTag)

	require.NoError(t, err)
	assert.Equal(t, "v9.1.2", got)
}
