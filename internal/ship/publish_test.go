package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/github/pulls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const me = "danpock"

// bump is the marker of shipping scc-operator at a version to release/v2.15.
func bump(version string) Marker {
	return Marker{Source: "rancher/scc-operator", Consumer: "rancher/rancher", Target: "release/v2.15", Version: version}
}

// pr is an open PR by author with the marker of a bump at a version.
func pr(number int, author string, m Marker) pulls.PR {
	return pulls.PR{Number: number, Author: author, Body: "## Summary\n\n" + m.String()}
}

func numbers(prs []pulls.PR) []int {
	out := make([]int, len(prs))
	for i, p := range prs {
		out[i] = p.Number
	}
	return out
}

func TestDecideNothingOpenIsCreate(t *testing.T) {
	got := decidePublish(nil, me, bump("v0.5.4"))

	assert.Equal(t, PublishCreate, got.Action)
	assert.Empty(t, got.Supersede)
	assert.Nil(t, got.Open)
}

func TestDecideAnOlderPROfMineIsSuperseded(t *testing.T) {
	open := []pulls.PR{pr(101, me, bump("v0.5.3"))}

	got := decidePublish(open, me, bump("v0.5.4"))

	assert.Equal(t, PublishCreate, got.Action)
	assert.Equal(t, []int{101}, numbers(got.Supersede))
}

func TestDecideEveryOlderPROfMineIsSuperseded(t *testing.T) {
	open := []pulls.PR{pr(101, me, bump("v0.5.3")), pr(90, me, bump("v0.5.2")), pr(80, me, bump("v0.5.0-rc.1"))}

	got := decidePublish(open, me, bump("v0.5.4"))

	assert.Equal(t, PublishCreate, got.Action)
	assert.Equal(t, []int{101, 90, 80}, numbers(got.Supersede), "all of them, in the order given")
}

func TestDecideThePRAtTheSameVersionIsAlreadyOpen(t *testing.T) {
	open := []pulls.PR{pr(104, me, bump("v0.5.4"))}

	got := decidePublish(open, me, bump("v0.5.4"))

	assert.Equal(t, PublishAlreadyOpen, got.Action)
	require.NotNil(t, got.Open)
	assert.Equal(t, 104, got.Open.Number)
	assert.Empty(t, got.Supersede)
}

func TestDecideAlreadyOpenStillClosesTheOlderOnes(t *testing.T) {
	open := []pulls.PR{pr(104, me, bump("v0.5.4")), pr(101, me, bump("v0.5.3"))}

	got := decidePublish(open, me, bump("v0.5.4"))

	assert.Equal(t, PublishAlreadyOpen, got.Action)
	assert.Equal(t, []int{101}, numbers(got.Supersede))
}

func TestDecideTwoOfMineAtTheSameVersionKeepsTheNewest(t *testing.T) {
	open := []pulls.PR{pr(106, me, bump("v0.5.4")), pr(104, me, bump("v0.5.4"))}

	got := decidePublish(open, me, bump("v0.5.4"))

	assert.Equal(t, PublishAlreadyOpen, got.Action)
	require.NotNil(t, got.Open)
	assert.Equal(t, 106, got.Open.Number, "the first, which is the newest")
	assert.Equal(t, []int{104}, numbers(got.Supersede), "the duplicate is closed")
}

func TestDecideANewerPROfMineBlocksTheTargetAndTouchesNothing(t *testing.T) {
	open := []pulls.PR{pr(110, me, bump("v0.5.5")), pr(101, me, bump("v0.5.3"))}

	got := decidePublish(open, me, bump("v0.5.4"))

	assert.Equal(t, PublishBlocked, got.Action)
	require.NotNil(t, got.Newer)
	assert.Equal(t, 110, got.Newer.Number)
	assert.Empty(t, got.Supersede, "nothing is closed for a target that is blocked")
}

func TestDecideAPrereleaseIsOlderThanItsRelease(t *testing.T) {
	open := []pulls.PR{pr(101, me, bump("v0.5.4-rc.1"))}

	got := decidePublish(open, me, bump("v0.5.4"))

	assert.Equal(t, PublishCreate, got.Action)
	assert.Equal(t, []int{101}, numbers(got.Supersede), "the rc is replaced by the release")

	got = decidePublish([]pulls.PR{pr(102, me, bump("v0.5.4"))}, me, bump("v0.5.4-rc.2"))
	assert.Equal(t, PublishBlocked, got.Action, "and the release is newer than a later rc")
}

func TestDecideThePRsOfOthersAreNeverTouched(t *testing.T) {
	open := []pulls.PR{
		pr(120, "rancher-bot[bot]", bump("v0.5.3")), // older, someone else's
		pr(119, "alice", bump("v0.5.2")),
	}

	got := decidePublish(open, me, bump("v0.5.4"))

	assert.Equal(t, PublishCreate, got.Action, "they do not block")
	assert.Empty(t, got.Supersede, "and are not closed")
	assert.Empty(t, got.Also, "an older one of someone else's is not even worth a mention")
}

func TestDecideASameOrNewerPROfAnotherAuthorIsReportedAndLeftAlone(t *testing.T) {
	open := []pulls.PR{
		pr(120, "rancher-bot[bot]", bump("v0.5.4")),
		pr(121, "alice", bump("v0.5.5")),
	}

	got := decidePublish(open, me, bump("v0.5.4"))

	assert.Equal(t, PublishCreate, got.Action)
	assert.Equal(t, []int{120, 121}, numbers(got.Also))
	assert.Empty(t, got.Supersede)
}

func TestDecideMixesMineAndOthers(t *testing.T) {
	open := []pulls.PR{
		pr(120, "rancher-bot[bot]", bump("v0.5.4")),
		pr(101, me, bump("v0.5.3")),
	}

	got := decidePublish(open, me, bump("v0.5.4"))

	assert.Equal(t, PublishCreate, got.Action)
	assert.Equal(t, []int{101}, numbers(got.Supersede), "only mine")
	assert.Equal(t, []int{120}, numbers(got.Also))
}

func TestDecideOnlyThisBumpsKeyCounts(t *testing.T) {
	other := Marker{Source: "rancher/kuberlr-kubectl", Consumer: "rancher/rancher", Target: "release/v2.15", Version: "v0.5.3"}
	otherTarget := bump("v0.5.3")
	otherTarget.Target = "main"
	otherConsumer := bump("v0.5.3")
	otherConsumer.Consumer = "rancher/charts"
	open := []pulls.PR{
		pr(1, me, other), pr(2, me, otherTarget), pr(3, me, otherConsumer),
		{Number: 4, Author: me, Body: "no marker here"},
		{Number: 5, Author: me, Body: "<!-- gero:ship v9\nsource: rancher/scc-operator\n-->"},
	}

	got := decidePublish(open, me, bump("v0.5.4"))

	assert.Equal(t, PublishCreate, got.Action)
	assert.Empty(t, got.Supersede, "different source, target or consumer, or no readable marker: not this bump")
}

func TestDecideAVersionThatIsNotSemverIsLeftAlone(t *testing.T) {
	open := []pulls.PR{pr(101, me, bump("nightly-1")), pr(102, me, bump("v0.5.3"))}

	got := decidePublish(open, me, bump("v0.5.4"))

	assert.Equal(t, PublishCreate, got.Action)
	assert.Equal(t, []int{102}, numbers(got.Supersede), "what cannot be ordered is not closed on a guess")
}
