package repotag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/task/createtag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bg = context.Background()

const (
	head215  = "8c613980aabbccddeeff00112233445566778899"
	headMain = "7cc6dab0aabbccddeeff00112233445566778899"
)

type fakeWorkspace struct {
	project config.Project
	refs    git.Refs
	err     error
}

func (f fakeWorkspace) Project(string) (config.Project, error) { return f.project, nil }
func (f fakeWorkspace) UpstreamRefs(context.Context, string) (git.Refs, error) {
	return f.refs, f.err
}

// shell is a producer with a version series for each Rancher line.
func shell() fakeWorkspace {
	return fakeWorkspace{
		project: config.Project{
			Name: "shell", Repo: config.Repo{Name: "rancher/shell", DefaultBranch: "main"},
			Release: config.Release{Branching: config.BranchingPerLine, TagPattern: "v*", Lines: []config.ProjectLine{
				{Line: "v2.16", Version: "0.9.x"},
				{Line: "v2.15", Version: "0.8.x", Branch: "release/v2.15"},
			}},
		},
		refs: git.Refs{
			Branches: []git.Branch{{Name: "main", Commit: headMain}, {Name: "release/v2.15", Commit: head215}},
			Tags: []git.Tag{
				{Name: "v0.8.2", Commit: "1111"}, {Name: "v0.8.3", Commit: "2222"}, {Name: "v0.9.0", Commit: "3333"},
			},
		},
	}
}

type fakeRemote struct {
	made []string
	err  error
}

func (f *fakeRemote) Create(_ context.Context, slug, name, commit string) error {
	f.made = append(f.made, slug+" "+name+" "+commit)
	return f.err
}

// tagging runs the command and gives back what it wrote, whether it asked for a
// remote, and the remote.
type tagging struct {
	out        bytes.Buffer
	remote     *fakeRemote
	remoteUsed bool
}

func (g *tagging) run(ws Workspace, format string, dryRun bool, opts Options) error {
	g.remote = &fakeRemote{}
	return run(bg, &g.out, ws, func() (createtag.Remote, error) { g.remoteUsed = true; return g.remote, nil }, format, dryRun, opts)
}

func answering(answer string) Options {
	return Options{Line: "v2.15", Tag: "v0.8.4-rc.1", Interactive: true, In: strings.NewReader(answer)}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		args              []string
		project, line, tg string
	}{
		{[]string{"v2.15"}, "", "v2.15", ""},
		{[]string{"release/v2.15"}, "", "release/v2.15", ""},
		{[]string{"v2.15", "v0.8.4-rc.1"}, "", "v2.15", "v0.8.4-rc.1"},
		{[]string{"shell", "v2.15"}, "shell", "v2.15", ""},
		{[]string{"rancher/shell", "release/v2.15"}, "rancher/shell", "release/v2.15", ""},
		{[]string{"shell", "v2.15", "v0.8.4-rc.1"}, "shell", "v2.15", "v0.8.4-rc.1"},
		{[]string{"shell", "main", "v0.8.4"}, "shell", "main", "v0.8.4"},
		{[]string{"main", "0.8.4"}, "", "main", "0.8.4"},
	}
	for _, tt := range tests {
		project, line, tag := ParseArgs(tt.args)

		assert.Equal(t, [3]string{tt.project, tt.line, tt.tg}, [3]string{project, line, tag}, "%v", tt.args)
	}
}

func TestRunAsksOnceAndMakesTheTagOnTheHeadOfTheLinesBranch(t *testing.T) {
	var g tagging

	err := g.run(shell(), "text", false, answering("y\n"))

	require.NoError(t, err)
	assert.Equal(t, []string{"rancher/shell v0.8.4-rc.1 " + head215}, g.remote.made, "on release/v2.15's head, as the host has it")
	out := g.out.String()
	assert.Contains(t, out, "Tag v0.8.4-rc.1 on release/v2.15 at 8c61398 in rancher/shell?")
	assert.Contains(t, out, "Tagged v0.8.4-rc.1")
	assert.Contains(t, out, "https://github.com/rancher/shell/releases/tag/v0.8.4-rc.1")
}

func TestRunDeclinedMakesNothing(t *testing.T) {
	var g tagging

	err := g.run(shell(), "text", false, answering("n\n"))

	require.ErrorIs(t, err, ErrDeclined)
	assert.Empty(t, g.remote.made)
}

func TestRunWithYesDoesNotAsk(t *testing.T) {
	var g tagging
	opts := answering("")
	opts.Yes, opts.Interactive, opts.In = true, false, nil

	err := g.run(shell(), "text", false, opts)

	require.NoError(t, err)
	assert.Len(t, g.remote.made, 1)
	assert.NotContains(t, g.out.String(), "[y/N]")
}

func TestRunWithoutATerminalOrYesIsRefusedBeforeAnyWrite(t *testing.T) {
	var g tagging
	opts := answering("")
	opts.Interactive, opts.In = false, nil

	err := g.run(shell(), "text", false, opts)

	require.ErrorIs(t, err, ErrNeedsApproval)
	assert.Empty(t, g.remote.made)
}

func TestRunADryRunSaysWhatItWouldTagAndNeverAsksOrReachesGitHub(t *testing.T) {
	var g tagging
	opts := answering("")
	opts.In = nil

	err := g.run(shell(), "text", true, opts)

	require.NoError(t, err)
	assert.False(t, g.remoteUsed, "no token is needed to say what would be done")
	assert.Contains(t, g.out.String(), "Would tag v0.8.4-rc.1 on release/v2.15 at 8c61398 in rancher/shell")
	assert.NotContains(t, g.out.String(), "[y/N]")
}

func TestRunATagAlreadyOnTheCommitIsDoneAndAsksNothing(t *testing.T) {
	var g tagging
	ws := shell()
	ws.refs.Tags = append(ws.refs.Tags, git.Tag{Name: "v0.8.4-rc.1", Commit: head215})
	opts := answering("")
	opts.In, opts.Interactive = nil, false

	err := g.run(ws, "text", false, opts)

	require.NoError(t, err)
	assert.Empty(t, g.remote.made)
	assert.Contains(t, g.out.String(), "already tagged")
	assert.False(t, g.remoteUsed)
}

func TestRunATagTheLineCannotHaveIsAnErrorBeforeAnyQuestion(t *testing.T) {
	var g tagging
	opts := answering("y\n")
	opts.Tag = "v0.9.1" // the series of v2.16

	err := g.run(shell(), "text", false, opts)

	require.ErrorIs(t, err, createtag.ErrNotInSeries)
	assert.Empty(t, g.remote.made)
	assert.NotContains(t, g.out.String(), "[y/N]")
}

func TestRunALineTheProjectDoesNotHaveListsTheOnesItDoes(t *testing.T) {
	var g tagging
	opts := answering("y\n")
	opts.Line = "v2.99"

	err := g.run(shell(), "text", false, opts)

	require.Error(t, err)
	assert.ErrorContains(t, err, "v2.99")
	assert.ErrorContains(t, err, "v2.16, v2.15")
	assert.Empty(t, g.remote.made)
}

func TestRunTheLineMayBeWrittenWithoutTheVOrAsItsBranch(t *testing.T) {
	for _, line := range []string{"2.15", "V2.15", "release/v2.15"} {
		var g tagging
		opts := answering("y\n")
		opts.Line = line

		err := g.run(shell(), "text", false, opts)

		require.NoError(t, err, line)
		assert.Equal(t, []string{"rancher/shell v0.8.4-rc.1 " + head215}, g.remote.made, line)
	}
}

func TestRunTheDevelopmentLineIsTheDefaultBranchWithItsOwnSeries(t *testing.T) {
	var g tagging
	opts := answering("y\n")
	opts.Line, opts.Tag = "v2.16", "v0.9.1-rc.1"

	err := g.run(shell(), "text", false, opts)

	require.NoError(t, err)
	assert.Equal(t, []string{"rancher/shell v0.9.1-rc.1 " + headMain}, g.remote.made)
}

func TestRunAProjectWithNoLinesTagsABranchOfItsOwnWithNoSeries(t *testing.T) {
	var g tagging
	ws := shell()
	ws.project.Release = config.Release{Branching: config.BranchingSingle, TagPattern: "v*"}
	ws.project.Repo.Name = "rancher/scc-operator"
	ws.refs.Tags = []git.Tag{{Name: "v0.5.3", Commit: "1111"}}
	opts := answering("y\n")
	opts.Line, opts.Tag = "main", "v0.5.4-rc.1"

	err := g.run(ws, "text", false, opts)

	require.NoError(t, err)
	assert.Equal(t, []string{"rancher/scc-operator v0.5.4-rc.1 " + headMain}, g.remote.made)
}

func TestRunWithNoTagChoosesThePrereleaseThatFollowsTheNewestOfTheLine(t *testing.T) {
	var g tagging
	opts := answering("y\n")
	opts.Tag = ""

	err := g.run(shell(), "text", false, opts)

	require.NoError(t, err)
	assert.Equal(t, []string{"rancher/shell v0.8.4-rc.1 " + head215}, g.remote.made)
	assert.Contains(t, g.out.String(), "Tag v0.8.4-rc.1 on release/v2.15 at 8c61398 in rancher/shell?")
}

func TestRunStablePromotesThePrereleaseOnTheHeadAndSaysSo(t *testing.T) {
	var g tagging
	ws := shell()
	ws.refs.Tags = append(ws.refs.Tags, git.Tag{Name: "v0.8.4-rc.2", Commit: head215})
	opts := answering("y\n")
	opts.Tag, opts.Stable = "", true

	err := g.run(ws, "text", false, opts)

	require.NoError(t, err)
	assert.Equal(t, []string{"rancher/shell v0.8.4 " + head215}, g.remote.made)
	assert.Contains(t, g.out.String(), "Tag v0.8.4 on release/v2.15 at 8c61398, which is v0.8.4-rc.2 with no commits since, in rancher/shell?")
}

func TestRunStableThatPromotesSaysWhichPrereleaseInJSONAndInADryRun(t *testing.T) {
	ws := shell()
	ws.refs.Tags = append(ws.refs.Tags, git.Tag{Name: "v0.8.4-rc.2", Commit: head215})
	opts := Options{Line: "v2.15", Stable: true, Yes: true}

	var j tagging
	require.NoError(t, j.run(ws, "json", false, opts))
	var res Result
	require.NoError(t, json.Unmarshal(j.out.Bytes(), &res), j.out.String())
	assert.Equal(t, "v0.8.4", res.Tag)
	assert.Equal(t, "v0.8.4-rc.2", res.Promotes)

	var d tagging
	require.NoError(t, d.run(ws, "text", true, opts))
	assert.Contains(t, d.out.String(), "Would tag v0.8.4 on release/v2.15 at 8c61398, which is v0.8.4-rc.2 with no commits since, in rancher/shell")
	assert.Empty(t, d.remote.made)
}

func TestRunStableRefusesToPromoteWhenTheBranchHasMovedOn(t *testing.T) {
	var g tagging
	ws := shell()
	ws.refs.Tags = append(ws.refs.Tags, git.Tag{Name: "v0.8.4-rc.2", Commit: "ccccccccdddd"})
	opts := answering("y\n")
	opts.Tag, opts.Stable = "", true

	err := g.run(ws, "text", false, opts)

	require.ErrorIs(t, err, createtag.ErrMovedSincePrerelease)
	assert.Empty(t, g.remote.made)
	assert.NotContains(t, g.out.String(), "[y/N]")
}

func TestRunAProjectThatRequiresAPrereleaseHasNoStableAfterAStable(t *testing.T) {
	var g tagging
	ws := shell()
	ws.project.Release.RequiresPrerelease = true
	opts := answering("y\n")
	opts.Tag, opts.Stable = "", true

	err := g.run(ws, "text", false, opts)

	require.ErrorIs(t, err, createtag.ErrNeedsPrerelease)
	assert.Empty(t, g.remote.made)
}

func TestRunANamedStableTagOfAProjectThatRequiresAPrereleaseNeedsOneOnTheHead(t *testing.T) {
	var g tagging
	ws := shell()
	ws.project.Release.RequiresPrerelease = true
	opts := answering("y\n")
	opts.Tag = "v0.8.4"

	err := g.run(ws, "text", false, opts)

	require.ErrorIs(t, err, createtag.ErrNeedsPrerelease)
	assert.Empty(t, g.remote.made)
}

func TestRunMinorAndMajorRaiseTheVersionAfterAStableTag(t *testing.T) {
	for bump, want := range map[Bump]string{BumpMinor: "v0.6.0-rc.1", BumpMajor: "v1.0.0-rc.1"} {
		var g tagging
		ws := shell()
		ws.project.Release = config.Release{Branching: config.BranchingSingle, TagPattern: "v*"}
		ws.refs.Tags = []git.Tag{{Name: "v0.5.3", Commit: "1111"}}
		opts := Options{Line: "main", Bump: bump, Yes: true}

		err := g.run(ws, "text", false, opts)

		require.NoError(t, err)
		assert.Equal(t, []string{"rancher/shell " + want + " " + headMain}, g.remote.made)
	}
}

func TestRunWithNothingNewOnTheHeadChoosesNothing(t *testing.T) {
	var g tagging
	ws := shell()
	ws.refs.Tags = append(ws.refs.Tags, git.Tag{Name: "v0.8.4", Commit: head215})
	opts := answering("y\n")
	opts.Tag = ""

	err := g.run(ws, "text", false, opts)

	require.ErrorIs(t, err, createtag.ErrNothingNew)
	assert.Empty(t, g.remote.made)
}

func TestRunATagAndAWayToChooseOneAreNotBothAllowed(t *testing.T) {
	for _, opts := range []Options{
		{Line: "v2.15", Tag: "v0.8.4", Stable: true, Yes: true},
		{Line: "v2.15", Tag: "v0.8.4", Bump: BumpMinor, Yes: true},
	} {
		var g tagging

		err := g.run(shell(), "text", false, opts)

		require.ErrorIs(t, err, ErrTagAndChoice)
		assert.Empty(t, g.remote.made)
	}
}

func TestRunABranchTheHostDoesNotHaveIsAnError(t *testing.T) {
	var g tagging
	ws := shell()
	ws.refs.Branches = ws.refs.Branches[:1] // no release/v2.15

	err := g.run(ws, "text", false, answering("y\n"))

	require.Error(t, err)
	assert.ErrorContains(t, err, "release/v2.15")
	assert.Empty(t, g.remote.made)
}

func TestRunAFailureToReadTheHostIsAnError(t *testing.T) {
	var g tagging
	ws := shell()
	ws.err = errors.New("network down")

	err := g.run(ws, "text", false, answering("y\n"))

	require.Error(t, err)
	assert.ErrorContains(t, err, "network down")
}

func TestRunAFailureToMakeTheTagIsAnError(t *testing.T) {
	var g tagging
	failing := func() (createtag.Remote, error) {
		return &fakeRemote{err: errors.New("403 Resource not accessible")}, nil
	}

	err := run(bg, &g.out, shell(), failing, "text", false, answering("y\n"))

	require.Error(t, err)
	assert.ErrorContains(t, err, "403")
	assert.NotContains(t, g.out.String(), "Tagged")
}

func TestRunJSONReportsWhatWasDone(t *testing.T) {
	var g tagging
	opts := answering("")
	opts.Yes, opts.In = true, nil

	err := g.run(shell(), "json", false, opts)

	require.NoError(t, err)
	var res Result
	require.NoError(t, json.Unmarshal(g.out.Bytes(), &res), g.out.String())
	assert.Equal(t, Result{
		Project: "rancher/shell", Branch: "release/v2.15", Commit: head215, Tag: "v0.8.4-rc.1", Newest: "v0.8.3", Action: "tagged",
	}, res)
}

func TestRunAnOutputFormatItDoesNotSupportIsAnErrorBeforeAnything(t *testing.T) {
	var g tagging

	err := g.run(shell(), "env", false, answering("y\n"))

	require.Error(t, err)
	assert.Empty(t, g.remote.made)
}
