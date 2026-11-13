package texts

import (
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/gero/authorship"
	"github.com/rancher/orbs-automation/internal/platform/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultsOnlyUseTheClosedVariables(t *testing.T) {
	for name, text := range map[string]string{
		"title": DefaultTitle(), "branch prefix": DefaultBranchPrefix(), "commit message": CommitMessage(), "body": Body(),
	} {
		require.NotEmpty(t, text, name)
		assert.NoError(t, templates.Validate(text, templates.TextVars), name)
	}
}

func TestDefaultTitleAndPrefix(t *testing.T) {
	assert.Equal(t, "[{branch}] Update {source} to {tag}", DefaultTitle())
	assert.Equal(t, "bot/", DefaultBranchPrefix(), "the key that gero appends has the source in it")
	assert.NotContains(t, DefaultTitle(), "\n", "a title is one line")
}

func TestCommitMessageHasASubjectAndTrailers(t *testing.T) {
	lines := strings.Split(strings.TrimRight(CommitMessage(), "\n"), "\n")
	assert.Equal(t, "Update {source} to {tag}", lines[0])
	assert.Empty(t, lines[1], "a blank line after the subject")
	assert.Contains(t, CommitMessage(), "Automated update from {sourceRepo} release {tag}")
	last := lines[len(lines)-1]
	assert.Equal(t, authorship.Trailer, last, "the trailer block is last, so git reads it as trailers")
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

func TestStrategyCommitMessageOnlyUsesItsVariables(t *testing.T) {
	require.NotEmpty(t, StrategyCommitMessage())
	assert.NoError(t, templates.Validate(StrategyCommitMessage(), StrategyCommitVars))
}

func TestStrategyCommitMessageNamesTheStrategyWhatItChangedAndKeepsTheTrailers(t *testing.T) {
	msg := StrategyCommitMessage()
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")

	assert.Equal(t, "Run {strategy}", lines[0], "the subject says which strategy ran")
	assert.Contains(t, msg, "{changes}")
	assert.Contains(t, msg, "{source} to {tag}")
	assert.Contains(t, msg, "{edit}")
	assert.Equal(t, authorship.Trailer, lines[len(lines)-1])
	assert.True(t, strings.HasSuffix(msg, "\n"))
}

func TestStepCommitMessageOnlyUsesItsVariables(t *testing.T) {
	require.NotEmpty(t, StepCommitMessage())
	assert.NoError(t, templates.Validate(StepCommitMessage(), StepCommitVars))
}

func TestStepCommitMessageNamesTheCommandAndKeepsTheTrailers(t *testing.T) {
	lines := strings.Split(strings.TrimRight(StepCommitMessage(), "\n"), "\n")
	require.GreaterOrEqual(t, len(lines), 3)
	assert.Equal(t, "Run `{command}`", lines[0])
	assert.Empty(t, lines[1], "a blank line after the subject")
	assert.Contains(t, StepCommitMessage(), "{phase}")
	assert.Contains(t, StepCommitMessage(), "{source} to {tag}")
	assert.Equal(t, authorship.Trailer, lines[len(lines)-1], "the same trailers as the edit's commit")
}

func TestTheTextsForABumpWithCompanionsUseOnlyTheVariablesOfTheTexts(t *testing.T) {
	assert.Equal(t, "[{branch}] Update {source} to {tag} and {companions}", DefaultTitleWith())
	assert.NotContains(t, DefaultTitleWith(), "\n", "a title is one line")
	assert.True(t, strings.HasPrefix(CommitMessageWith(), "Update {source} to {tag} and {companions}\n"))
	assert.Contains(t, CommitMessageWith(), authorship.Trailer)
	for name, text := range map[string]string{"title": DefaultTitleWith(), "commit message": CommitMessageWith()} {
		assert.NoError(t, templates.Validate(text, templates.TextVars), name)
	}
}

// A branch is known as gero's own by the trailer its commits end with
// (authorship.AreOurs), so every message that makes a commit must end with it:
// one that does not would make a branch the next run takes for somebody else's.
func TestEveryCommitMessageEndsWithTheTrailerAuthorshipLooksFor(t *testing.T) {
	for name, msg := range map[string]string{
		"bump": CommitMessage(), "bump with companions": CommitMessageWith(),
		"step": StepCommitMessage(), "strategy": StrategyCommitMessage(),
	} {
		lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
		assert.Equal(t, authorship.Trailer, lines[len(lines)-1], name)
		ours, why := authorship.AreOurs([]authorship.Commit{{Hash: "abc", AuthorEmail: "a@b", Message: msg}}, "a@b")
		assert.True(t, ours, "%s: %s", name, why)
	}
}
