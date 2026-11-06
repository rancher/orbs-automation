package ship

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func ourCommit(hash string) BranchCommit {
	return BranchCommit{Hash: hash, AuthorEmail: "dev@example.com", Message: "Update scc-operator to v0.5.3\n\nAutomated update.\n\nAutomation: gero ship"}
}

func TestCommitsAreOursWhenEveryOneHasTheTrailerAndTheActorsEmail(t *testing.T) {
	ours, why := commitsAreOurs([]BranchCommit{ourCommit("aaaaaaaa11"), ourCommit("bbbbbbbb22")}, "dev@example.com")

	assert.True(t, ours)
	assert.Empty(t, why)
}

func TestCommitsAreOursWithNoCommits(t *testing.T) {
	ours, _ := commitsAreOurs(nil, "dev@example.com")

	assert.True(t, ours)
}

func TestCommitsAreOursComparesTheEmailWithoutRegardToCase(t *testing.T) {
	c := ourCommit("aaaaaaaa11")
	c.AuthorEmail = "Dev@Example.COM"

	ours, _ := commitsAreOurs([]BranchCommit{c}, "dev@example.com")

	assert.True(t, ours)
}

func TestCommitsAreNotOursWithoutTheTrailer(t *testing.T) {
	c := ourCommit("cccccccc33")
	c.Message = "a fixup by hand"

	ours, why := commitsAreOurs([]BranchCommit{ourCommit("aaaaaaaa11"), c}, "dev@example.com")

	assert.False(t, ours)
	assert.Contains(t, why, "cccccccc", "the commit is named")
	assert.Contains(t, why, "trailer")
}

func TestCommitsAreNotOursWhenAnotherMadeThem(t *testing.T) {
	c := ourCommit("dddddddd44")
	c.AuthorEmail = "someone@example.com"

	ours, why := commitsAreOurs([]BranchCommit{c}, "dev@example.com")

	assert.False(t, ours)
	assert.Contains(t, why, "dddddddd")
	assert.Contains(t, why, "someone@example.com", "and who made it")
}

func TestCommitsAreNotOursWhenTheTrailerIsOnlyMentionedInTheText(t *testing.T) {
	c := ourCommit("eeeeeeee55")
	c.Message = "I copied the line Automation: gero ship into this message but it is not a trailer"

	ours, _ := commitsAreOurs([]BranchCommit{c}, "dev@example.com")

	assert.False(t, ours, "it has to be a line of its own")
}

func TestCommitsAreNotOursWithNoActorEmail(t *testing.T) {
	ours, why := commitsAreOurs([]BranchCommit{ourCommit("aaaaaaaa11")}, "")

	assert.False(t, ours, "without an identity there is nothing to compare, so nothing is claimed")
	assert.Contains(t, why, "email")
}

func TestCommitsAreNotOursStopsAtTheFirstOffender(t *testing.T) {
	a, b := ourCommit("11111111aa"), ourCommit("22222222bb")
	a.Message, b.Message = "hand made", "also hand made"

	_, why := commitsAreOurs([]BranchCommit{a, b}, "dev@example.com")

	assert.Contains(t, why, "11111111")
	assert.NotContains(t, why, "22222222")
}
