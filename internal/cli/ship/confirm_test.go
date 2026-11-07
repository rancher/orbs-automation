package ship

import (
	"bytes"
	"context"
	"strings"
	"testing"

	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func opening(branches ...string) domain.Confirmation {
	return domain.Confirmation{
		Consumer: "rancher/rancher", Release: "v0.5.3",
		Push:  domain.PushTarget{Role: domain.RoleFork, Remote: "origin"},
		Opens: branches,
	}
}

func TestQuestionSaysWhatIsPushedWhereAndWhatIsOpened(t *testing.T) {
	q := question(opening("release/v2.15"))

	for _, want := range []string{"release/v2.15", "origin", "fork", "1 PR", "rancher/rancher", "[y/N]"} {
		assert.Contains(t, q, want)
	}
	assert.NotContains(t, q, "close")

	q = question(opening("release/v2.15", "main"))
	assert.Contains(t, q, "release/v2.15, main")
	assert.Contains(t, q, "2 PRs")
}

func TestQuestionSaysWhichOlderPRsWillBeClosed(t *testing.T) {
	c := opening("release/v2.15")
	c.Closes = []domain.PRRef{{Number: 101}, {Number: 90}}

	q := question(c)

	assert.Contains(t, q, "close #101, #90")
}

func TestQuestionWithNothingToOpenIsAboutTheClosing(t *testing.T) {
	c := opening()
	c.Closes = []domain.PRRef{{Number: 101}}

	q := question(c)

	assert.Contains(t, q, "Close #101 in rancher/rancher")
	assert.NotContains(t, q, "Push", "nothing is pushed")
}

func TestConfirmerYesSaysYesWithoutAsking(t *testing.T) {
	var out bytes.Buffer

	ok, err := confirmer(strings.NewReader(""), &out, false, true, style.Plain())(context.Background(), opening("main"))

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, out.String(), "nothing was asked")
}

func TestConfirmerWithoutATerminalIsAnErrorNotASilentYes(t *testing.T) {
	var out bytes.Buffer

	ok, err := confirmer(strings.NewReader("y\n"), &out, false, false, style.Plain())(context.Background(), opening("main"))

	require.ErrorIs(t, err, ErrNeedsApproval)
	assert.False(t, ok)
	assert.Empty(t, out.String())
}

func TestConfirmerAsksOnTheTerminalAndOnlyYesIsYes(t *testing.T) {
	for answer, want := range map[string]bool{
		"y\n": true, "yes\n": true, "Y\n": true, " yes \n": true,
		"n\n": false, "\n": false, "": false, "maybe\n": false, "yep\n": false,
	} {
		var out bytes.Buffer

		ok, err := confirmer(strings.NewReader(answer), &out, true, false, style.Plain())(context.Background(), opening("main"))

		require.NoError(t, err, "%q", answer)
		assert.Equal(t, want, ok, "%q", answer)
		assert.Contains(t, out.String(), "[y/N]", "the question was shown")
	}
}

func TestQuestionSaysWhichPRsWillBeRebuilt(t *testing.T) {
	c := opening()
	c.Refreshes = []string{"release/v2.15", "main"}

	q := question(c)

	assert.Contains(t, q, "Rebuild the branches for release/v2.15, main")
	assert.Contains(t, q, "origin", "where the replacing branch is pushed")
	assert.Contains(t, q, "if they differ")
	assert.Contains(t, q, "[y/N]")
}

func TestQuestionWithAnOpenAndARefreshAsksAboutBoth(t *testing.T) {
	c := opening("main")
	c.Refreshes = []string{"release/v2.15"}

	q := question(c)

	assert.Contains(t, q, "open 1 PR")
	assert.Contains(t, q, "rebuild the branch for release/v2.15")
}

func TestQuestionSaysWhenABranchOnTheRemoteMayBeReplaced(t *testing.T) {
	c := opening("main")
	c.MayReplace = true

	q := question(c)

	assert.Contains(t, q, "replacing the branch on origin if an earlier attempt left one of ours")
	assert.NotContains(t, question(opening("main")), "replacing")
}
