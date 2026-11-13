package dockerfileedit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rancherDockerfile is a Dockerfile in shape: ARGs with defaults before the first
// stage, a redeclared ARG with no value, and ENVs in each of their forms.
const rancherDockerfile = `FROM scratch AS base
ARG GODEP_STEVE=scratch
ARG CHART_DEFAULT_BRANCH=dev-v2.15
ARG PARTNER_CHART_DEFAULT_BRANCH=main

FROM base
ARG CHART_DEFAULT_BRANCH
ENV CATTLE_CHART_DEFAULT_BRANCH=$CHART_DEFAULT_BRANCH
ENV CATTLE_SERVER_URL "https://example.test/a b"
ENV QUOTED="v1.0.0" SINGLE='x y' PLAIN=1.2.3
ENV FIRST=1 \
    SECOND=two
RUN echo done
`

func TestGetReadsTheValueOfAnInstruction(t *testing.T) {
	tests := []struct{ name, key, want string }{
		{"an ARG with a default", "CHART_DEFAULT_BRANCH", "dev-v2.15"},
		{"an ARG whose name is the end of another", "PARTNER_CHART_DEFAULT_BRANCH", "main"},
		{"an ENV with an expansion", "CATTLE_CHART_DEFAULT_BRANCH", "$CHART_DEFAULT_BRANCH"},
		{"an ENV in the old form, the rest of the line", "CATTLE_SERVER_URL", "https://example.test/a b"},
		{"a value in double quotes", "QUOTED", "v1.0.0"},
		{"a value in single quotes", "SINGLE", "x y"},
		{"one of several pairs on an ENV", "PLAIN", "1.2.3"},
		{"a pair on a continued line", "SECOND", "two"},
		{"the first pair of a continued ENV", "FIRST", "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Get([]byte(rancherDockerfile), tt.key)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetIgnoresTheInstructionKeywordsCaseAndTheRestOfTheFile(t *testing.T) {
	src := "# ARG TOOL=commented\nRUN echo ARG TOOL=in-a-command\narg TOOL=v0.4.2\n"

	got, err := Get([]byte(src), "TOOL")

	require.NoError(t, err)
	assert.Equal(t, "v0.4.2", got)
}

func TestGetReadsAFileWithWindowsLineEndings(t *testing.T) {
	got, err := Get([]byte("ARG TOOL=v1\r\nRUN x\r\n"), "TOOL")

	require.NoError(t, err)
	assert.Equal(t, "v1", got)
}

func TestGetWhenTheNameHasNoValue(t *testing.T) {
	tests := map[string]string{
		"it is not in the file":          "ARG OTHER=1\n",
		"it is declared with no default": "ARG TOOL\n",
		"it is only in a comment":        "# ARG TOOL=1\n",
		"its name is only a prefix":      "ARG TOOL_VERSION=1\n",
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Get([]byte(src), "TOOL")

			require.ErrorIs(t, err, ErrNotFound)
			assert.Contains(t, err.Error(), "TOOL", "the message has the name")
		})
	}
}

func TestGetWhenTheNameIsGivenAValueMoreThanOnce(t *testing.T) {
	_, err := Get([]byte("ARG TOOL=1\nFROM x\nENV TOOL=2\n"), "TOOL")

	require.ErrorIs(t, err, ErrAmbiguous)
	assert.Contains(t, err.Error(), "TOOL")
}

func TestSetChangesOnlyTheValue(t *testing.T) {
	tests := []struct{ name, key, value, want string }{
		{"an ARG default", "CHART_DEFAULT_BRANCH", "release-v2.15", "ARG CHART_DEFAULT_BRANCH=release-v2.15\n"},
		{"a value in double quotes keeps its quotes", "QUOTED", "v2.0.0", "ENV QUOTED=\"v2.0.0\" SINGLE='x y' PLAIN=1.2.3\n"},
		{"a value in single quotes keeps its quotes", "SINGLE", "z", "ENV QUOTED=\"v1.0.0\" SINGLE='z' PLAIN=1.2.3\n"},
		{"one of several pairs", "PLAIN", "9.9.9", "ENV QUOTED=\"v1.0.0\" SINGLE='x y' PLAIN=9.9.9\n"},
		{"the old form", "CATTLE_SERVER_URL", "https://other.test", "ENV CATTLE_SERVER_URL \"https://other.test\"\n"},
		{"a pair on a continued line", "SECOND", "three", "    SECOND=three\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, old, err := Set([]byte(rancherDockerfile), tt.key, tt.value)

			require.NoError(t, err)
			assert.Contains(t, string(out), tt.want)
			assert.NotEmpty(t, old)
			back, err := Get(out, tt.key)
			require.NoError(t, err)
			assert.Equal(t, tt.value, back, "what was set is what is read")
		})
	}
}

func TestSetLeavesEveryOtherByteAsItWas(t *testing.T) {
	out, old, err := Set([]byte(rancherDockerfile), "CHART_DEFAULT_BRANCH", "release-v2.15")

	require.NoError(t, err)
	assert.Equal(t, "dev-v2.15", old)
	want := "FROM scratch AS base\nARG GODEP_STEVE=scratch\nARG CHART_DEFAULT_BRANCH=release-v2.15\n"
	assert.Equal(t, want, string(out[:len(want)]))
	assert.Equal(t, len(rancherDockerfile)-len("dev-v2.15")+len("release-v2.15"), len(out))
	assert.Contains(t, string(out), "ARG CHART_DEFAULT_BRANCH\nENV CATTLE_CHART", "the redeclared ARG with no value is not touched")
}

func TestSetKeepsWindowsLineEndings(t *testing.T) {
	out, _, err := Set([]byte("ARG TOOL=v1\r\nRUN x\r\n"), "TOOL", "v2")

	require.NoError(t, err)
	assert.Equal(t, "ARG TOOL=v2\r\nRUN x\r\n", string(out))
}

func TestSetWhenThereIsNothingToSet(t *testing.T) {
	_, _, err := Set([]byte("ARG TOOL\n"), "TOOL", "v2")
	require.ErrorIs(t, err, ErrNotFound)

	_, _, err = Set([]byte("ARG TOOL=1\nENV TOOL=2\n"), "TOOL", "v2")
	require.ErrorIs(t, err, ErrAmbiguous)
}

func TestSetRefusesAValueThatCannotBeWrittenAsItIs(t *testing.T) {
	tests := map[string]struct{ src, value string }{
		"a new line":                      {"ARG TOOL=1\n", "a\nb"},
		"a space in an unquoted value":    {"ARG TOOL=1\n", "a b"},
		"a quote in an unquoted value":    {"ARG TOOL=1\n", `a"b`},
		"the quote of the value it is in": {"ARG TOOL=\"1\"\n", `a"b`},
		"a backslash":                     {"ARG TOOL=1\n", `a\b`},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, _, err := Set([]byte(tt.src), "TOOL", tt.value)

			require.ErrorIs(t, err, ErrInvalidValue)
		})
	}
}

func TestSetAllowsASpaceInAQuotedValue(t *testing.T) {
	out, _, err := Set([]byte("ARG TOOL=\"1\"\n"), "TOOL", "a b")

	require.NoError(t, err)
	assert.Equal(t, "ARG TOOL=\"a b\"\n", string(out))
}
