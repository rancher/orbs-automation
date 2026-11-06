package createpr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rancher/orbs-automation/internal/github/pulls"
)

var bg = context.Background()

// fakeRemote records the calls in order, and answers with what it is given.
type fakeRemote struct {
	calls     []string
	labels    []string
	labelsErr error
	addErr    error
	createErr error
}

func (f *fakeRemote) Create(_ context.Context, slug string, in pulls.NewPR) (pulls.PR, error) {
	f.calls = append(f.calls, fmt.Sprintf("create %s head=%s base=%s title=%q body=%q", slug, in.Head, in.Base, in.Title, in.Body))
	if f.createErr != nil {
		return pulls.PR{}, f.createErr
	}
	return pulls.PR{Number: 104, URL: "https://github.com/" + slug + "/pull/104"}, nil
}

func (f *fakeRemote) RepoLabels(context.Context, string) ([]string, error) {
	f.calls = append(f.calls, "labels")
	return f.labels, f.labelsErr
}

func (f *fakeRemote) AddLabels(_ context.Context, _ string, n int, labels []string) error {
	f.calls = append(f.calls, fmt.Sprintf("add labels #%d %s", n, strings.Join(labels, ",")))
	return f.addErr
}

func (f *fakeRemote) has(prefix string) bool {
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func newRemote() *fakeRemote {
	return &fakeRemote{labels: []string{"status/auto-created", "team/orbs"}}
}

func input() Input {
	return Input{
		Repo: "rancher/rancher", Title: "[v2.15] Update scc-operator", Body: "the body",
		Head: "danpock:bot/x-v1", Base: "release/v2.15",
		Labels: []string{"status/auto-created", "team/orbs"},
	}
}

func TestRunOpensThePRThenReadsTheLabelsThenAppliesThem(t *testing.T) {
	r := newRemote()

	out, err := Run(bg, r, input())

	require.NoError(t, err)
	assert.Equal(t, []string{
		`create rancher/rancher head=danpock:bot/x-v1 base=release/v2.15 title="[v2.15] Update scc-operator" body="the body"`,
		"labels",
		"add labels #104 status/auto-created,team/orbs",
	}, r.calls)
	assert.Equal(t, Output{
		Number: 104, URL: "https://github.com/rancher/rancher/pull/104",
		Labels: []string{"status/auto-created", "team/orbs"},
	}, out)
}

func TestRunAFailedCreateIsAnErrorAndNothingIsLabelled(t *testing.T) {
	r := newRemote()
	r.createErr = errors.New("a pull request already exists")

	_, err := Run(bg, r, input())

	require.ErrorIs(t, err, r.createErr)
	assert.False(t, r.has("labels"))
	assert.False(t, r.has("add labels"))
}

func TestRunOnlyLabelsThatExistAreAppliedAndTheOthersAreWarnings(t *testing.T) {
	r := newRemote()
	r.labels = []string{"status/auto-created"}

	out, err := Run(bg, r, input())

	require.NoError(t, err, "a missing label never fails the PR")
	assert.Contains(t, r.calls, "add labels #104 status/auto-created")
	assert.Equal(t, []string{"status/auto-created"}, out.Labels)
	require.Len(t, out.Warnings, 1)
	assert.Contains(t, out.Warnings[0], "team/orbs")
	assert.Contains(t, out.Warnings[0], "not in rancher/rancher")
}

func TestRunLabelsMatchWithoutRegardToCaseAndTheRepositorysSpellingIsSent(t *testing.T) {
	r := newRemote()
	r.labels = []string{"Status/Auto-Created", "TEAM/orbs"}

	out, err := Run(bg, r, input())

	require.NoError(t, err)
	assert.Contains(t, r.calls, "add labels #104 Status/Auto-Created,TEAM/orbs")
	assert.Equal(t, []string{"Status/Auto-Created", "TEAM/orbs"}, out.Labels)
	assert.Empty(t, out.Warnings)
}

func TestRunALabelNamedTwiceIsAppliedOnce(t *testing.T) {
	r := newRemote()
	in := input()
	in.Labels = []string{"team/orbs", "TEAM/ORBS"}

	_, err := Run(bg, r, in)

	require.NoError(t, err)
	assert.Contains(t, r.calls, "add labels #104 team/orbs")
}

func TestRunNoLabelsReadsNoLabelsAndAppliesNone(t *testing.T) {
	r := newRemote()
	in := input()
	in.Labels = nil

	out, err := Run(bg, r, in)

	require.NoError(t, err)
	assert.False(t, r.has("labels"))
	assert.False(t, r.has("add labels"))
	assert.Empty(t, out.Labels)
	assert.Equal(t, 104, out.Number)
}

func TestRunNotBeingAbleToReadTheLabelsIsAWarningAndNoneAreSent(t *testing.T) {
	r := newRemote()
	r.labelsErr = errors.New("rate limited")

	out, err := Run(bg, r, input())

	require.NoError(t, err)
	assert.False(t, r.has("add labels"), "no labels are sent that may not exist")
	require.Len(t, out.Warnings, 1)
	assert.Contains(t, out.Warnings[0], "rate limited")
	assert.Equal(t, 104, out.Number, "the PR is still reported")
}

func TestRunAFailureToApplyTheLabelsIsAWarning(t *testing.T) {
	r := newRemote()
	r.addErr = errors.New("label deleted meanwhile")

	out, err := Run(bg, r, input())

	require.NoError(t, err)
	require.Len(t, out.Warnings, 1)
	assert.Contains(t, out.Warnings[0], "label deleted meanwhile")
	assert.Empty(t, out.Labels, "none were applied")
	assert.Equal(t, 104, out.Number)
}
