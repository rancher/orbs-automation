package ship

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	shipBlock    = "<!-- gero:ship v1\nsource: rancher/scc-operator\nconsumer: rancher/rancher\ntarget: release/v2.15\nversion: v0.5.3\n-->"
	trackerBlock = "<!-- gero:tracker v1\nissue: 12\nstate: a > b\n-->"
)

func generatedBody() string {
	return "## Summary\nUpdate scc-operator to [`v0.5.3`](https://example.test)\n\n## Changes\n- Updated `image`\n\n" + shipBlock + "\n"
}

func TestMergeBodyWithNothingHiddenOnThePRIsTheNewBody(t *testing.T) {
	existing := "## Summary\nan older text\n\n" + shipBlock + "\n"

	got := mergeBody(existing, generatedBody())

	assert.Equal(t, generatedBody(), got, "the visible text is the new one")
}

func TestMergeBodyKeepsAnotherGeroBlockExactlyAsItIs(t *testing.T) {
	existing := "## Summary\nan older text\n\n" + shipBlock + "\n\n" + trackerBlock + "\n"

	got := mergeBody(existing, generatedBody())

	assert.Contains(t, got, "Updated `image`", "the new visible text")
	assert.NotContains(t, got, "an older text")
	assert.Contains(t, got, trackerBlock, "the tracker's block, byte for byte, even with a > in it")
	assert.Equal(t, 1, strings.Count(got, "<!-- gero:ship"), "and our own block once")
	assert.Equal(t, 1, strings.Count(got, "gero:tracker"))
	assert.Less(t, strings.Index(got, "gero:ship"), strings.Index(got, "gero:tracker"), "after what gero ship writes")
}

func TestMergeBodyOurOwnBlockComesFromTheNewBodyNotTheOldOne(t *testing.T) {
	old := strings.Replace(shipBlock, "v0.5.3", "v0.5.2", 1)
	existing := "text\n\n" + old + "\n"

	got := mergeBody(existing, generatedBody())

	assert.Contains(t, got, "version: v0.5.3")
	assert.NotContains(t, got, "version: v0.5.2")
}

func TestMergeBodyKeepsSeveralOtherBlocksInOrder(t *testing.T) {
	second := "<!-- gero:audit v2\nby: someone\n-->"
	existing := "text\n\n" + trackerBlock + "\n\n" + shipBlock + "\n\n" + second + "\n"

	got := mergeBody(existing, generatedBody())

	assert.Less(t, strings.Index(got, "gero:tracker"), strings.Index(got, "gero:audit"), "the order they were in")
	assert.Contains(t, got, second)
	assert.Contains(t, got, trackerBlock)
}

func TestMergeBodyIsIdempotent(t *testing.T) {
	existing := "text\n\n" + shipBlock + "\n\n" + trackerBlock + "\n"

	once := mergeBody(existing, generatedBody())
	twice := mergeBody(once, generatedBody())

	assert.Equal(t, once, twice)
}

func TestMergeBodyWithNoBlocksAtAllReplacesTheText(t *testing.T) {
	assert.Equal(t, generatedBody(), mergeBody("written by hand\n", generatedBody()))
	assert.Equal(t, generatedBody(), mergeBody("", generatedBody()))
}

func TestMergeBodyDoesNotKeepCommentsThatAreNotGeros(t *testing.T) {
	existing := "<!-- a template's own comment -->\ntext\n\n" + shipBlock + "\n"

	got := mergeBody(existing, generatedBody())

	assert.NotContains(t, got, "a template's own comment", "only gero's blocks are carried over")
}

func TestSameBodyIgnoresLineEndingsAndTheWhitespaceAtTheEnds(t *testing.T) {
	assert.True(t, sameBody("a\nb\n", "a\r\nb"))
	assert.True(t, sameBody("  a\nb \n\n", "a\nb"))
	assert.False(t, sameBody("a\nb", "a\nc"))
	assert.False(t, sameBody("a b", "a  b"), "whitespace inside is part of the text")
}
