package templates

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultsOnlyUseTheClosedVariables(t *testing.T) {
	for name, text := range map[string]string{
		"title": DefaultTitle(), "branch prefix": DefaultBranchPrefix(), "commit message": CommitMessage(), "body": Body(),
	} {
		require.NotEmpty(t, text, name)
		assert.NoError(t, Validate(text, TextVars), name)
	}
}

func TestDefaultTitleAndPrefix(t *testing.T) {
	assert.Equal(t, "[{branch}] Update {source} to {tag}", DefaultTitle())
	assert.Equal(t, "bot/{source}-", DefaultBranchPrefix())
	assert.NotContains(t, DefaultTitle(), "\n", "a title is one line")
}

func TestCommitMessageHasASubjectAndTrailers(t *testing.T) {
	lines := strings.Split(strings.TrimRight(CommitMessage(), "\n"), "\n")
	assert.Equal(t, "Update {source} to {tag}", lines[0])
	assert.Empty(t, lines[1], "a blank line after the subject")
	assert.Contains(t, CommitMessage(), "Automated update from {sourceRepo} release {tag}")
	last := lines[len(lines)-1]
	assert.Equal(t, "Automation: gero ship", last, "the trailer block is last, so git reads it as trailers")
}

func TestBodyHasTheSectionsAndTheMarker(t *testing.T) {
	assert.Contains(t, Body(), "## Summary")
	assert.Contains(t, Body(), "[`{tag}`]({sourceReleaseURL})")
	assert.Contains(t, Body(), "## Changes\n{changes}")
	assert.Contains(t, Body(), "{marker}")
}

func TestDefaultsAreTrimmedOfTheFileEnding(t *testing.T) {
	assert.False(t, strings.HasSuffix(DefaultTitle(), "\n"))
	assert.False(t, strings.HasSuffix(DefaultBranchPrefix(), "\n"))
}
